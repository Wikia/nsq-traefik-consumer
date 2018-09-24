package queue

import (
	"fmt"

	"os"
	"os/signal"
	"syscall"

	"encoding/json"

	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/Wikia/nsq-traefik-consumer/common"
	metrics "github.com/Wikia/nsq-traefik-consumer/metrics"
	"github.com/Wikia/nsq-traefik-consumer/model"
	"github.com/mitchellh/mapstructure"
	"github.com/nsqio/go-nsq"
	stats "github.com/rcrowley/go-metrics"
)

func NewConsumer(config common.NsqConfig) (*nsq.Consumer, error) {
	if config.MaxInFlight > 0 {
		config.ClientConfig.MaxInFlight = config.MaxInFlight
	}
	consumer, err := nsq.NewConsumer(config.Topic, config.Channel, config.ClientConfig)

	if err != nil {
		return nil, err
	}

	logger, level := common.NewNSQLogrusLoggerAtLevel(log.GetLevel())
	consumer.SetLogger(logger, level)

	return consumer, nil
}

func metricsProcessor(k8sConfig common.KubernetesConfig, measurement string, metricsConfig []common.RulesConfig, fields []string, metricsBuffer *MetricsBuffer) nsq.HandlerFunc {
	processor, err := metrics.NewTraefikMetricProcessor(metricsConfig, fields)

	if err != nil {
		common.Log.WithError(err).Panic("Could not create metric processor")
	}

	gauge := stats.GetOrRegisterGauge("buffer_size", stats.DefaultRegistry)

	return func(message *nsq.Message) error {
		common.Log.WithField("message_id", string(message.ID[:nsq.MsgIDLength])).Info("Got a message")
		entry := model.LogEntry{}
		err := json.Unmarshal(message.Body, &entry)
		if err != nil {
			common.Log.WithError(err).WithField("body", string(message.Body)).Errorf("Error unmarshaling message")
		} else {
			entry.Log = strings.TrimSpace(entry.Log)
			value, has := entry.Kubernetes.Annotations[k8sConfig.AnnotationKey]
			if err != nil {
				common.Log.WithError(err).WithField("container_name", entry.Kubernetes.ContainerName).Errorf("Error getting annotation")
				return nil
			}

			if !has || len(value) == 0 {
				common.Log.WithField("container_name", entry.Kubernetes.ContainerName).Debug("Skipping message - no proper annotation found")
				return nil
			}

			var wikiaConfig map[string]interface{}

			err = json.Unmarshal([]byte(value), &wikiaConfig)

			if err != nil {
				common.Log.WithError(err).WithFields(log.Fields{
					"value":          string(value),
					"container_name": entry.Kubernetes.ContainerName,
				}).Error("Error unmarshaling pod config")
				return nil
			}

			influxConfig, has := wikiaConfig["influx_metrics"]
			if !has {
				common.Log.WithFields(log.Fields{
					"annotation":     value,
					"container_name": entry.Kubernetes.ContainerName,
				}).Info("Skipping message - no metrics config found")
				return nil
			}

			var annotationConfig model.GenericInfluxAnnotation
			err = mapstructure.Decode(influxConfig, &annotationConfig)

			if err != nil {
				common.Log.WithError(err).WithField("container_name", entry.Kubernetes.ContainerName).Error("Could not unmarshal metrics config")
				return nil
			}

			if entry.Kubernetes.ContainerName != annotationConfig.ContainerName {
				common.Log.WithField("container_name", entry.Kubernetes.ContainerName).Info("Skipping message - container not configured for metrics")
				return nil
			}

			processedMetrics, err := processor.Process(entry, annotationConfig.MetricsType, message.Timestamp, measurement)

			if err != nil {
				common.Log.WithError(err).Error("Error processing metrics")
				return nil
			} else if len(processedMetrics.Points()) == 0 {
				return nil
			}

			counter := stats.GetOrRegisterCounter("logs_consumed", stats.DefaultRegistry)
			metricsBuffer.Lock()
			metricsBuffer.Metrics.PushBack(processedMetrics)
			metricsBuffer.Unlock()
			counter.Inc(int64(len(processedMetrics.Points())))
			gauge.Update(int64(metricsBuffer.Metrics.Len()))

		}

		return nil
	}
}

func Consume(config common.Config, metricsBuffer *MetricsBuffer) error {
	if len(config.Nsq.Topic) == 0 {
		return fmt.Errorf("NSQ Topic is empty")
	}

	if len(config.Nsq.Channel) == 0 {
		return fmt.Errorf("NSQ Channel is empty")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	consumer, err := NewConsumer(config.Nsq)

	if err != nil {
		return err
	}

	consumer.AddHandler(metricsProcessor(config.Kubernetes, config.InfluxDB.Measurement, config.Rules, config.Fields, metricsBuffer))

	err = consumer.ConnectToNSQLookupds(config.Nsq.Addresses)
	if err != nil {
		common.Log.WithField("address", config.Nsq.Addresses).Panic("Could not connect")
	}

	for {
		select {
		case <-consumer.StopChan:
			return nil
		case <-sigChan:
			consumer.Stop()
		}
	}
}
