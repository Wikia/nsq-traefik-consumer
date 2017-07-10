package metrics

import (
	"regexp"

	"math/rand"

	"fmt"

	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/Wikia/nsq-traefik-consumer/common"
	"github.com/Wikia/nsq-traefik-consumer/model"
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

func (mp TraefikMetricProcessor) getMetrics(logEntry model.TraefikLog, values map[string]string) map[string]model.TimePoint {
	group := map[string]model.TimePoint{}

	// status codes
	integer, err := strconv.ParseInt(values["status"], 10, 64)

	if err != nil {
		log.WithError(err).WithField("value", values["status"]).Error("Error parsing status code value as integer")
	} else {
		group["response_code"] = model.NewTimePointUInt64(logEntry.Time, uint64(integer))
	}

	// response time
	integer, err = strconv.ParseInt(values["request_time"], 10, 64)

	if err != nil {
		log.WithError(err).WithField("value", values["request_time"]).Error("Error parsing request time value as integer")
	} else {
		group["request_time"] = model.NewTimePointUInt64(logEntry.Time, uint64(integer))
	}

	// request count
	integer, err = strconv.ParseInt(values["request_count"], 10, 64)

	if err != nil {
		log.WithError(err).WithField("value", values["request_count"]).Error("Error parsing request count value as integer")
	} else {
		group["request_count"] = model.NewTimePointUInt64(logEntry.Time, uint64(integer))
	}

	return group
}

func (mp TraefikMetricProcessor) Process(entry model.TraefikLog) (model.MetricsBuffer, error) {
	matches := ApacheCombinedLogRegex.FindStringSubmatch(entry.Log)
	metrics := model.MetricsBuffer{}
	mappedMatches := map[string]string{}

	if len(matches) != 15 {
		log.WithFields(log.Fields{
			"matches_cnt": len(matches),
			"entry":       entry.Log,
		}).Error("Error matching Traefik log")

		return metrics, fmt.Errorf("Error matching Traefik log")
	}

	for idx, name := range ApacheCombinedLogRegex.SubexpNames() {
		if idx == 0 {
			continue
		}
		mappedMatches[name] = matches[idx]
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

		groupedPoints := mp.getMetrics(entry, mappedMatches)
		tags := map[string]string{
			"frontend_name": mappedMatches["frontend_name"],
			"host_name":     entry.Kubernetes.Host,
			"cluster_name":  entry.KubernetesClusterName,
			"data_center":   entry.Datacenter,
		}
		commonValues := map[string]string{
			"request_method": mappedMatches["method"],
		}

		for name, point := range groupedPoints {
			group, has := metrics.Metrics[name]

			if !has {
				group = model.PointGroup{
					Points:      []model.TimePoint{},
					Tags:        tags,
					ExtraValues: commonValues,
				}
			}

			group.Points = append(group.Points, point)
		}
	}

	return metrics, nil
}
