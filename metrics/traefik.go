package metrics

import (
	"regexp"

	"math/rand"

	"fmt"

	"time"

	"encoding/json"

	"unicode"

	"strings"

	"strconv"

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
	Id             string
	PathRegexp     *regexp.Regexp
	MethodRegexp   *regexp.Regexp
	FrontEndRegexp *regexp.Regexp
	Filter         RuleFilter
}

type TraefikMetricProcessor struct {
	Rules           []ProcessRule
	randomGenerator *rand.Rand
	fields          []string
}

func NewTraefikMetricProcessor(config []common.RulesConfig, fields []string) (*TraefikMetricProcessor, error) {
	mp := TraefikMetricProcessor{Rules: []ProcessRule{}}
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
		sampling := cfg.Sampling
		rule.Filter = func(traefikLog model.LogEntry) bool { return mp.randomGenerator.Float64() < sampling }
		rule.FrontEndRegexp = rxp

		mp.Rules = append(mp.Rules, rule)
	}

	return &mp, nil
}

func parseCommonLog(entry model.LogEntry) (map[string]interface{}, error) {
	matches := ApacheCombinedLogRegex.FindStringSubmatch(entry.Log)

	const CombinedLogFieldCount = 15
	if len(matches) != CombinedLogFieldCount {
		return nil, fmt.Errorf("invalid log format - did not match necessary fields (matched fields number: %d)", len(matches))
	}

	logEntries := map[string]interface{}{}

	for idx, name := range ApacheCombinedLogRegex.SubexpNames() {
		if idx == 0 || len(matches[idx]) == 0 {
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
		} else if name == "duration" || name == "origin_status" || name == "origin_content_size" || name == "request_count" {
			value, err := strconv.ParseFloat(matches[idx], 64)
			if err != nil {
				log.WithError(err).WithField(name, matches[idx]).Error("could not parse field")
				return nil, err
			}
			logEntries[name] = value
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

	return common.Flatten(ret), nil
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
	for _, rule := range mp.Rules {
		if parsedLog["frontend_name"] == nil || !rule.FrontEndRegexp.MatchString(parsedLog["frontend_name"].(string)) {
			common.Log.WithFields(log.Fields{
				"entry":   parsedLog,
				"rule_id": rule.Id,
			}).Debug("Frontend name doesn't match regex - skipping")
			continue
		}

		if parsedLog["request_path"] == nil || rule.PathRegexp != nil && !rule.PathRegexp.MatchString(parsedLog["request_path"].(string)) {
			common.Log.WithFields(log.Fields{
				"entry":   parsedLog,
				"rule_id": rule.Id,
			}).Debug("Path doesn't match regex - skipping")
			continue
		}

		if rule.MethodRegexp != nil && (parsedLog["request_method"] == nil || !rule.MethodRegexp.MatchString(parsedLog["request_method"].(string))) {
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
			return result, nil
		}

		backendName, has := parsedLog["backend_name"]
		if !has {
			backendName = ""
		}

		tags := map[string]string{
			"frontend_name": parsedLog["frontend_name"].(string),
			"backend_name":  backendName.(string),
			"host_name":     entry.Kubernetes.Host,
			"cluster_name":  entry.KubernetesClusterName,
			"data_center":   entry.Datacenter,
			"rule_id":       rule.Id,
		}

		values := map[string]interface{}{}

		var timestamp time.Time
		original_timestamp, has := parsedLog["start_utc"]
		if has {
			timestamp, err = time.Parse(time.RFC3339Nano, original_timestamp.(string))
			if err != nil {
				common.Log.WithError(err).WithField("original_timestamp", original_timestamp).Error("error parsing timestamp")
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
			return nil, err
		}

		result.AddPoint(pt)
		return result, nil
	}

	return result, nil
}
