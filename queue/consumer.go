package queue

import (
	"fmt"

	"os"
	"os/signal"
	"syscall"

	"encoding/json"

	"time"

	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/Wikia/nsq-traefik-consumer/common"
	"github.com/Wikia/nsq-traefik-consumer/metrics"
	"github.com/Wikia/nsq-traefik-consumer/model"
	"github.com/mitchellh/mapstructure"
	"github.com/nsqio/go-nsq"
)

func NewConsumer(config common.NsqConfig) (*nsq.Consumer, error) {
	consumer, err := nsq.NewConsumer(config.Topic, config.Channel, config.ClientConfig)
	if err != nil {
		return nil, err
	}

	logger, level := common.NewNSQLogrusLoggerAtLevel(log.GetLevel())
	consumer.SetLogger(logger, level)

	return consumer, nil
}

func metricsProcessor(config common.KuberenetesConfig) nsq.HandlerFunc {
	processor := metrics.TraefikMetricProcessor{}
	series := model.NewTimeSeries(10 * time.Second)

	return func(message *nsq.Message) error {
		log.WithField("message", string(message.ID[:nsq.MsgIDLength])).Info("Got a message")
		entry := model.TraefikLog{}
		err := json.Unmarshal(message.Body, &entry)
		if err != nil {
			log.WithError(err).WithField("body", string(message.Body)).Errorf("Error unmarshaling message")
		} else {
			entry.Log = strings.TrimSpace(entry.Log)
			value, has := entry.Kubernetes.Annotations[config.AnnotationKey]
			if err != nil {
				log.WithError(err).Errorf("Error getting annotation")
				return nil
			}

			if !has || len(value) == 0 {
				log.Debug("Skipping message - no proper annotation found")
				return nil
			}

			var wikiaConfig map[string]interface{}

			err = json.Unmarshal([]byte(value), &wikiaConfig)

			if err != nil {
				log.WithError(err).WithField("value", string(value)).Error("Error unmarshaling pod config")
				return nil
			}

			k8sConfig, has := wikiaConfig["influx_metrics"]
			if !has {
				log.WithField("annotation", value).Info("Skipping message - no metrics config found")
				return nil
			}

			var metricConfig model.GenericInfluxAnnotation
			err = mapstructure.Decode(k8sConfig, &metricConfig)

			if err != nil {
				log.WithError(err).Error("Could not unmarshal metrics config")
				return nil
			}

			if entry.Kubernetes.ContainerName != metricConfig.ContainerName {
				log.WithField("container_name", entry.Kubernetes.ContainerName).Info("Skipping message - container not configured for metrics")
				return nil
			}

			points, err := processor.Process(entry)

			if err != nil {
				log.WithError(err).Error("Error processing metrics")
			}

			for _, point := range points {
				err := series.Append(point)

				if err != nil {
					log.WithError(err).Error("Could not add point to Time Serie")
				}
			}
		}

		return nil
	}
}

func Consume(config common.Config) error {
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

	consumer.AddHandler(metricsProcessor(config.Kubernetes))

	err = consumer.ConnectToNSQLookupd(config.Nsq.Address)
	if err != nil {
		log.WithField("address", config.Nsq.Address).Panic("Could not connect")
	}

	defer consumer.DisconnectFromNSQLookupd(config.Nsq.Address)

	for {
		select {
		case <-consumer.StopChan:
			return nil
		case <-sigChan:
			consumer.Stop()
		}
	}

	return nil
}
