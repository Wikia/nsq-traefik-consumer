package metrics

import (
	"regexp"

	"math/rand"

	"fmt"

	"time"

	"encoding/json"

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

func parseCommonLog(entry model.LogEntry) (*model.TraefikAccessLog, error) {
	matches := ApacheCombinedLogRegex.FindStringSubmatch(entry.Log)

	if len(matches) != 15 {
		return nil, fmt.Errorf("invalid log format - did not match necessary fields (matched fields number: %d)", len(matches))
	}

	logEntry := model.TraefikAccessLog{}
	for idx, name := range ApacheCombinedLogRegex.SubexpNames() {
		if idx == 0 {
			continue
		}

		switch name {
		case "client_host":
			logEntry.ClientHost = matches[idx]
		case "username":
			logEntry.ClientUsername = matches[idx]
		case "timestamp":
			value, err := time.Parse("02/Jan/2006:15:04:05 -0700", matches[idx])
			if err != nil {
				log.WithError(err).WithField("timestamp", matches[idx]).Error("could not parse timestamp")
				return nil, err
			}
			logEntry.OriginalTimestamp = value
		case "method":
			logEntry.RequestMethod = matches[idx]
		case "path":
			logEntry.RequestPath = matches[idx]
		case "protocol":
			logEntry.RequestProtocol = matches[idx]
		case "status":
			value, err := strconv.ParseInt(matches[idx], 10, 64)
			if err != nil {
				log.WithError(err).WithField("status", matches[idx]).Error("could not parse http status code")
				return nil, err
			}
			logEntry.OriginStatus = value
		case "content_size":
			value, err := strconv.ParseInt(matches[idx], 10, 64)
			if err != nil {
				log.WithError(err).WithField("content_size", matches[idx]).Error("could not parse content size")
				return nil, err
			}
			logEntry.OriginContentSize = value
		case "referrer":
			logEntry.Referrer = matches[idx]
		case "useragent":
			logEntry.ClientUserAgent = matches[idx]
		case "request_count":
			value, err := strconv.ParseInt(matches[idx], 10, 64)
			if err != nil {
				log.WithError(err).WithField("request_count", matches[idx]).Error("error parsing request count")
				return nil, err
			}
			logEntry.RequestCount = value
		case "frontend_name":
			logEntry.FrontendName = matches[idx]
		case "backend_url":
			logEntry.BackendName = matches[idx]
		case "request_time":
			value, err := time.ParseDuration(matches[idx])
			if err != nil {
				log.WithError(err).WithField("request_time", matches[idx]).Error("could not parse request time")
				return nil, err
			}
			logEntry.Duration = value
		}
	}

	return &logEntry, nil
}

func parseJsonLog(entry model.LogEntry) (*model.TraefikAccessLog, error) {
	logEntry := model.TraefikAccessLog{}

	err := json.Unmarshal([]byte(entry.Log), &logEntry)

	if err != nil {
		return nil, err
	}

	return &logEntry, nil
}

func (mp TraefikMetricProcessor) Process(entry model.LogEntry, logFormat string, timestamp int64, measurement string) (client.BatchPoints, error) {
	result, err := client.NewBatchPoints(client.BatchPointsConfig{})
	if err != nil {
		return nil, err
	}

	var parsedLog *model.TraefikAccessLog

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
		if !rxp.MatchString(parsedLog.FrontendName) {
			common.Log.WithFields(log.Fields{
				"entry":   parsedLog,
				"rule_id": rule.Id,
			}).Debug("Frontend name doesn't match regex - skipping")
			continue
		}

		if rule.PathRegexp != nil && !rule.PathRegexp.MatchString(parsedLog.RequestPath) {
			common.Log.WithFields(log.Fields{
				"entry":   parsedLog,
				"rule_id": rule.Id,
			}).Debug("Path doesn't match regex - skipping")
			continue
		}

		if rule.MethodRegexp != nil && !rule.MethodRegexp.MatchString(parsedLog.RequestMethod) {
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

		tags := parsedLog.GetTags()
		tags["host_name"] = entry.Kubernetes.Host
		tags["cluster_name"] = entry.KubernetesClusterName
		tags["data_center"] = entry.Datacenter
		tags["rule_id"] = rule.Id
		values := parsedLog.GetValues()

		var timestamp time.Time
		if parsedLog.StartUTC.IsZero() {
			timestamp = time.Now().UTC()
		} else {
			timestamp = parsedLog.StartUTC
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
