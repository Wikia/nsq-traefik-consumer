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
	Id             string
	FrontendRegexp *regexp.Regexp
	Filter         RuleFilter
}

type TraefikMetricProcessor struct {
	Rules map[*regexp.Regexp]ProcessRule
}

func NewTraefikMetricProcessor(config []common.RulesConfig) (*TraefikMetricProcessor, error) {
	mp := TraefikMetricProcessor{Rules: map[*regexp.Regexp]ProcessRule{}}

	for _, cfg := range config {
		rule := ProcessRule{}
		rule.Id = cfg.Id

		rxp, err := regexp.Compile(cfg.FrontendRegexp)
		if err != nil {
			return nil, err
		}
		rule.FrontendRegexp = rxp

		rxp, err = regexp.Compile(cfg.UrlRegexp)
		if err != nil {
			return nil, err
		}
		rule.Filter = func(traefikLog model.TraefikLog) bool { return rand.NormFloat64() <= cfg.Sampling }

		_, has := mp.Rules[rxp]
		if has {
			log.WithFields(log.Fields{
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
		log.WithError(err).WithField("value", values["request_time"]).Error("Error parsing request time value as integer")
	}

	m["request_time"] = integer

	// request count
	integer, err = strconv.ParseInt(values["request_count"], 10, 64)

	if err != nil {
		return nil, err
	}

	m["request_count"] = integer
	m["request_method"] = values["method"]

	return m, nil
}

func (mp TraefikMetricProcessor) Process(entry model.TraefikLog, measurement string) (client.BatchPoints, error) {
	result, err := client.NewBatchPoints(client.BatchPointsConfig{})
	if err != nil {
		return nil, err
	}

	matches := ApacheCombinedLogRegex.FindStringSubmatch(entry.Log)
	mappedMatches := map[string]string{}

	if len(matches) != 15 {
		log.WithFields(log.Fields{
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

	timestamp, err := time.Parse("02/Jan/2006:15:04:05 -0700", mappedMatches["timestamp"])

	if err != nil {
		log.WithError(err).WithField("entry", entry).Error("Error parsing timestamp for log entry")

		return nil, fmt.Errorf("Error parsing timestamp for log entry")
	}

	// filtering and rule processing
	for rxp, rule := range mp.Rules {
		if !rule.FrontendRegexp.MatchString(mappedMatches["frontend_name"]) {
			continue
		}

		if !rxp.MatchString(mappedMatches["path"]) {
			continue
		}

		if !rule.Filter(entry) {
			continue
		}

		metrics, err := mp.getMetrics(entry, mappedMatches)
		if err != nil {
			log.WithError(err).WithField("entry", entry).Error("Error processing log")
			continue
		}

		tags := map[string]string{
			"frontend_name": mappedMatches["frontend_name"],
			"host_name":     entry.Kubernetes.Host,
			"cluster_name":  entry.KubernetesClusterName,
			"data_center":   entry.Datacenter,
		}

		pt, err := client.NewPoint(measurement, tags, metrics, timestamp)
		if err != nil {
			log.WithError(err).Error("Error creating time point from log entry")
			continue
		}

		result.AddPoint(pt)
	}

	return result, nil
}
