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
	"github.com/Wikia/nsq-traefik-consumer/metrics"
	"github.com/Wikia/nsq-traefik-consumer/model"
	"github.com/mitchellh/mapstructure"
	"github.com/nsqio/go-nsq"
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

func metricsProcessor(k8sConfig common.KubernetesConfig, measurement string, metricsConfig []common.RulesConfig, metricsBuffer *MetricsBuffer) nsq.HandlerFunc {
	processor, err := metrics.NewTraefikMetricProcessor(metricsConfig)

	if err != nil {
		common.Log.WithError(err).Panic("Could not create metric processor")
	}

	return func(message *nsq.Message) error {
		common.Log.WithField("message", string(message.ID[:nsq.MsgIDLength])).Info("Got a message")
		entry := model.TraefikLog{}
		err := json.Unmarshal(message.Body, &entry)
		if err != nil {
			common.Log.WithError(err).WithField("body", string(message.Body)).Errorf("Error unmarshaling message")
		} else {
			entry.Log = strings.TrimSpace(entry.Log)
			value, has := entry.Kubernetes.Annotations[k8sConfig.AnnotationKey]
			if err != nil {
				common.Log.WithError(err).Errorf("Error getting annotation")
				return nil
			}

			if !has || len(value) == 0 {
				common.Log.Debug("Skipping message - no proper annotation found")
				return nil
			}

			var wikiaConfig map[string]interface{}

			err = json.Unmarshal([]byte(value), &wikiaConfig)

			if err != nil {
				common.Log.WithError(err).WithField("value", string(value)).Error("Error unmarshaling pod config")
				return nil
			}

			influxConfig, has := wikiaConfig["influx_metrics"]
			if !has {
				common.Log.WithField("annotation", value).Info("Skipping message - no metrics config found")
				return nil
			}

			var annotationConfig model.GenericInfluxAnnotation
			err = mapstructure.Decode(influxConfig, &annotationConfig)

			if err != nil {
				common.Log.WithError(err).Error("Could not unmarshal metrics config")
				return nil
			}

			if entry.Kubernetes.ContainerName != annotationConfig.ContainerName {
				common.Log.WithField("container_name", entry.Kubernetes.ContainerName).Info("Skipping message - container not configured for metrics")
				return nil
			}

			processedMetrics, err := processor.Process(entry, message.Timestamp, measurement)

			if err != nil {
				common.Log.WithError(err).Error("Error processing metrics")
				return nil
			}

			common.Log.WithField("metrics_cnt", len(processedMetrics.Points())).Debug("Gathered metrics")

			metricsBuffer.Lock()
			metricsBuffer.Metrics = append(metricsBuffer.Metrics, processedMetrics)
			metricsBuffer.Unlock()
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

	consumer.AddHandler(metricsProcessor(config.Kubernetes, config.InfluxDB.Measurement, config.Rules, metricsBuffer))

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
