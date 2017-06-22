package queue

import (
	"fmt"

	"os"
	"os/signal"
	"syscall"

	"encoding/json"

	log "github.com/Sirupsen/logrus"
	"github.com/Wikia/nsq-traefik-consumer/common"
	"github.com/Wikia/nsq-traefik-consumer/model"
	"github.com/nsqio/go-nsq"
)

type Config struct {
	Address      string
	Topic        string
	Channel      string
	ClientConfig *nsq.Config
}

func NewConfig() Config {
	config := Config{}
	config.ClientConfig = nsq.NewConfig()
	return config
}

func NewConsumer(config Config) (*nsq.Consumer, error) {
	consumer, err := nsq.NewConsumer(config.Topic, config.Channel, config.ClientConfig)
	if err != nil {
		return nil, err
	}

	logger, level := common.NewNSQLogrusLoggerAtLevel(log.GetLevel())
	consumer.SetLogger(logger, level)

	return consumer, nil
}

func Consume(config Config) error {
	if len(config.Topic) == 0 {
		return fmt.Errorf("NSQ Topic is empty")
	}

	if len(config.Channel) == 0 {
		return fmt.Errorf("NSQ Channel is empty")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	consumer, err := NewConsumer(config)

	if err != nil {
		return err
	}

	consumer.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		log.WithField("message", string(message.ID[:nsq.MsgIDLength])).Info("Got a message")
		entry := model.TraefikLog{}
		err := json.Unmarshal(message.Body, &entry)
		if err != nil {
			log.WithError(err).WithField("body", string(message.Body)).Errorf("Error unmarshaling message")
		} else {
			log.WithFields(log.Fields{
				"log":        entry.Log,
				"time":       entry.Time,
				"stream":     entry.Stream,
				"docker":     entry.Docker,
				"kubernetes": entry.Kubernetes,
				"datacenter": entry.Datacenter,
				"cluster":    entry.KubernetesClusterName,
				"ts":         entry.Ts,
			}).Info("Message decoded")
		}

		return nil
	}))

	err = consumer.ConnectToNSQLookupd(config.Address)
	if err != nil {
		log.WithField("address", config.Address).Panic("Could not connect")
	}
	defer consumer.DisconnectFromNSQLookupd(config.Address)

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
