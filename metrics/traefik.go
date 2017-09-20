package metrics

import (
	"regexp"

	"math/rand"

	"fmt"

	"strconv"

	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/Wikia/nsq-traefik-consumer/common"
	"github.com/Wikia/nsq-traefik-consumer/model"
	"github.com/influxdata/influxdb/client/v2"
)

type RuleFilter func(model.TraefikLog) bool

type ProcessRule struct {
	Id           string
	PathRegexp   *regexp.Regexp
	MethodRegexp *regexp.Regexp
	Filter       RuleFilter
}

type TraefikMetricProcessor struct {
	Rules           map[*regexp.Regexp]ProcessRule
	randomGenerator *rand.Rand
}

func NewTraefikMetricProcessor(config []common.RulesConfig) (*TraefikMetricProcessor, error) {
	mp := TraefikMetricProcessor{Rules: map[*regexp.Regexp]ProcessRule{}}
	s1 := rand.NewSource(time.Now().UnixNano())
	mp.randomGenerator = rand.New(s1)

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
		rule.Filter = func(traefikLog model.TraefikLog) bool { return mp.randomGenerator.Float64() < cfg.Sampling }

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

func (mp TraefikMetricProcessor) getMetrics(logEntry model.TraefikLog, values map[string]string) (map[string]interface{}, error) {
	m := map[string]interface{}{}

	// status codes
	integer, err := strconv.ParseInt(values["status"], 10, 64)

	if err != nil {
		return nil, err
	}

	m["response_code"] = integer

	// response time
	integer, err = strconv.ParseInt(values["request_time"], 10, 64)

	if err != nil {
		common.Log.WithError(err).WithField("value", values["request_time"]).Error("Error parsing request time value as integer")
	}

	m["request_time"] = integer

	// request count
	integer, err = strconv.ParseInt(values["request_count"], 10, 64)

	if err != nil {
		return nil, err
	}

	m["request_count"] = integer

	// content size
	integer, err = strconv.ParseInt(values["content_size"], 10, 64)

	if err != nil {
		return nil, err
	}

	m["response_size"] = integer

	logTimestamp, err := time.Parse("02/Jan/2006:15:04:05 -0700", values["timestamp"])

	if err != nil {
		common.Log.WithError(err).WithField("value", values["timestamp"]).Error("Error parsing timestamp for log entry")

		return nil, err
	}

	m["log_timestamp"] = logTimestamp
	m["backend_url"] = values["backend_url"]
	m["request_method"] = values["method"]
	m["client_username"] = values["username"]
	m["http_referer"] = values["referer"]
	m["http_user_agent"] = values["useragent"]
	m["request_url"] = values["path"]

	return m, nil
}

func (mp TraefikMetricProcessor) Process(entry model.TraefikLog, timestamp int64, measurement string) (client.BatchPoints, error) {
	result, err := client.NewBatchPoints(client.BatchPointsConfig{})
	if err != nil {
		return nil, err
	}

	matches := ApacheCombinedLogRegex.FindStringSubmatch(entry.Log)
	mappedMatches := map[string]string{}

	if len(matches) != 15 {
		common.Log.WithFields(log.Fields{
			"matches_cnt": len(matches),
			"entry":       entry.Log,
		}).Error("Error matching Traefik log")

		return nil, fmt.Errorf("Error matching Traefik log")
	}

	for idx, name := range ApacheCombinedLogRegex.SubexpNames() {
		if idx == 0 {
			continue
		}
		mappedMatches[name] = matches[idx]
	}

	// filtering and rule processing
	for rxp, rule := range mp.Rules {
		if !rxp.MatchString(mappedMatches["frontend_name"]) {
			common.Log.WithFields(log.Fields{
				"entry":   mappedMatches,
				"rule_id": rule.Id,
			}).Debug("Frontend name doesn't match regex - skipping")
			continue
		}

		if rule.PathRegexp != nil && !rule.PathRegexp.MatchString(mappedMatches["path"]) {
			common.Log.WithFields(log.Fields{
				"entry":   mappedMatches,
				"rule_id": rule.Id,
			}).Debug("Path doesn't match regex - skipping")
			continue
		}

		if rule.MethodRegexp != nil && !rule.MethodRegexp.MatchString(mappedMatches["method"]) {
			common.Log.WithFields(log.Fields{
				"entry":   mappedMatches,
				"rule_id": rule.Id,
			}).Debug("Method doesn't match regex - skipping")
			continue
		}

		if !rule.Filter(entry) {
			common.Log.WithFields(log.Fields{
				"entry":   mappedMatches,
				"rule_id": rule.Id,
			}).Debug("Entry below threshold (sampling) - skipping")
			continue
		}

		values, err := mp.getMetrics(entry, mappedMatches)
		if err != nil {
			common.Log.WithError(err).WithFields(log.Fields{
				"entry":   mappedMatches,
				"rule_id": rule.Id,
			}).Error("Error processing log")
			continue
		}

		common.Log.WithFields(log.Fields{
			"metrics": values,
			"rule_id": rule.Id,
		}).Debug("Successfully derived metrics")

		tags := map[string]string{
			"frontend_name": mappedMatches["frontend_name"],
			"host_name":     entry.Kubernetes.Host,
			"cluster_name":  entry.KubernetesClusterName,
			"data_center":   entry.Datacenter,
			"rule_id":       rule.Id,
		}

		pt, err := client.NewPoint(measurement, tags, values, time.Now())
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
