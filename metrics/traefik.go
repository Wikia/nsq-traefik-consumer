package metrics

import (
	"regexp"

	"math/rand"

	"fmt"

	"time"

	"encoding/json"

	"unicode"

	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/Wikia/nsq-traefik-consumer/common"
	"github.com/Wikia/nsq-traefik-consumer/model"
	"github.com/influxdata/influxdb/client/v2"
)

const (
	Combined = "access_log_combined"
	JSON     = "access_log_as_json"
)

type RuleFilter func(model.LogEntry) bool

type ProcessRule struct {
	Id           string
	PathRegexp   *regexp.Regexp
	MethodRegexp *regexp.Regexp
	Filter       RuleFilter
}

type TraefikMetricProcessor struct {
	Rules           map[*regexp.Regexp]ProcessRule
	randomGenerator *rand.Rand
	fields          []string
}

func NewTraefikMetricProcessor(config []common.RulesConfig, fields []string) (*TraefikMetricProcessor, error) {
	mp := TraefikMetricProcessor{Rules: map[*regexp.Regexp]ProcessRule{}}
	s1 := rand.NewSource(time.Now().UnixNano())
	mp.randomGenerator = rand.New(s1)
	mp.fields = fields

	for _, cfg := range config {
		rule := ProcessRule{}
		rule.Id = cfg.Id

		if len(cfg.UrlRegexp) > 0 {
			rxp, err := regexp.Compile(cfg.UrlRegexp)
			if err != nil {
				return nil, err
			}
			rule.PathRegexp = rxp
		}

		if len(cfg.MethodRegexp) > 0 {
			rxp, err := regexp.Compile(cfg.MethodRegexp)
			if err != nil {
				return nil, err
			}
			rule.MethodRegexp = rxp
		}

		rxp, err := regexp.Compile(cfg.FrontendRegexp)
		if err != nil {
			return nil, err
		}
		rule.Filter = func(traefikLog model.LogEntry) bool { return mp.randomGenerator.Float64() < cfg.Sampling }

		_, has := mp.Rules[rxp]
		if has {
			common.Log.WithFields(log.Fields{
				"rule_id":      cfg.Id,
				"duplicate_id": mp.Rules[rxp].Id,
			}).Error("Duplicated rules")
			return nil, fmt.Errorf("UrlRegexp for rules duplicates")
		}
		mp.Rules[rxp] = rule
	}

	return &mp, nil
}

func parseCommonLog(entry model.LogEntry) (map[string]interface{}, error) {
	matches := ApacheCombinedLogRegex.FindStringSubmatch(entry.Log)

	if len(matches) != 15 {
		return nil, fmt.Errorf("invalid log format - did not match necessary fields (matched fields number: %d)", len(matches))
	}

	logEntries := map[string]interface{}{}

	for idx, name := range ApacheCombinedLogRegex.SubexpNames() {
		if idx == 0 {
			continue
		}

		if name == "timestamp" {
			value, err := time.Parse("02/Jan/2006:15:04:05 -0700", matches[idx])
			if err != nil {
				log.WithError(err).WithField("timestamp", matches[idx]).Error("could not parse timestamp")
				return nil, err
			}
			logEntries["original_timestamp"] = value
		} else if name == "request__useragent" {
			logEntries["request__user-agent"] = matches[idx]
		} else {
			logEntries[name] = matches[idx]
		}
	}

	return logEntries, nil
}

// ToSnake convert the given string to snake case following the Golang format:
// acronyms are converted to lower-case and preceded by an underscore.
func toSnake(in string) string {
	runes := []rune(in)
	length := len(runes)

	var out []rune
	for i := 0; i < length; i++ {
		if i > 0 && unicode.IsUpper(runes[i]) && unicode.IsLetter(runes[i]) && ((i+1 < length && unicode.IsLower(runes[i+1])) || unicode.IsLower(runes[i-1])) {
			out = append(out, '_')
		}
		out = append(out, unicode.ToLower(runes[i]))
	}

	return string(out)
}

func parseJsonLog(entry model.LogEntry) (map[string]interface{}, error) {
	logEntries := map[string]interface{}{}

	err := json.Unmarshal([]byte(entry.Log), &logEntries)

	if err != nil {
		return nil, err
	}

	ret := map[string]interface{}{}

	for k, v := range logEntries {
		key := strings.Replace(toSnake(k), "-_", "-", -1)
		ret[key] = v
	}

	return ret, nil
}

func (mp TraefikMetricProcessor) Process(entry model.LogEntry, logFormat string, timestamp int64, measurement string) (client.BatchPoints, error) {
	result, err := client.NewBatchPoints(client.BatchPointsConfig{})
	if err != nil {
		return nil, err
	}

	var parsedLog map[string]interface{}

	switch logFormat {
	case Combined:
		parsedLog, err = parseCommonLog(entry)
	case JSON:
		parsedLog, err = parseJsonLog(entry)
	default:
		return nil, fmt.Errorf("unknown log format: %s", logFormat)
	}

	if err != nil {
		common.Log.WithFields(log.Fields{
			"entry": entry.Log,
		}).WithError(err).Error("could not parse Traefik log")
		return nil, fmt.Errorf("could not parse Traefik log")
	}

	// filtering and rule processing
	for rxp, rule := range mp.Rules {
		if !rxp.MatchString(parsedLog["frontend_name"].(string)) {
			common.Log.WithFields(log.Fields{
				"entry":   parsedLog,
				"rule_id": rule.Id,
			}).Debug("Frontend name doesn't match regex - skipping")
			continue
		}

		if rule.PathRegexp != nil && !rule.PathRegexp.MatchString(parsedLog["request_path"].(string)) {
			common.Log.WithFields(log.Fields{
				"entry":   parsedLog,
				"rule_id": rule.Id,
			}).Debug("Path doesn't match regex - skipping")
			continue
		}

		if rule.MethodRegexp != nil && !rule.MethodRegexp.MatchString(parsedLog["request_method"].(string)) {
			common.Log.WithFields(log.Fields{
				"entry":   parsedLog,
				"rule_id": rule.Id,
			}).Debug("Method doesn't match regex - skipping")
			continue
		}

		if !rule.Filter(entry) {
			common.Log.WithFields(log.Fields{
				"entry":   parsedLog,
				"rule_id": rule.Id,
			}).Debug("Entry below threshold (sampling) - skipping")
			continue
		}

		tags := map[string]string{
			"frontend_name": parsedLog["frontend_name"].(string),
			"backend_name":  parsedLog["backend_name"].(string),
			"host_name":     entry.Kubernetes.Host,
			"cluster_name":  entry.KubernetesClusterName,
			"data_center":   entry.Datacenter,
			"rule_id":       rule.Id,
		}

		values := map[string]interface{}{}

		var timestamp time.Time
		original_timestamp, has := parsedLog["start_utc"]
		if has {
			timestamp, err = time.Parse("", original_timestamp.(string))
			if err != nil {
				common.Log.WithError(err).WithField("original_timestamp", original_timestamp).Error("Error parsing timestamp")
				continue
			}
		} else {
			timestamp = time.Now()
		}

		for _, k := range mp.fields {
			_, has := parsedLog[k]
			if !has {
				continue
			}

			values[k] = parsedLog[k]
		}

		pt, err := client.NewPoint(measurement, tags, values, timestamp)
		if err != nil {
			common.Log.WithFields(log.Fields{
				"values":      values,
				"tags":        tags,
				"measurement": measurement,
			}).WithError(err).Error("Error creating time point from log entry")
			continue
		}

		result.AddPoint(pt)
	}

	return result, nil
}
