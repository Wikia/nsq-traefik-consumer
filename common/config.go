package common

import "github.com/nsqio/go-nsq"

type NsqConfig struct {
	Address      string
	Topic        string
	Channel      string
	ClientConfig *nsq.Config
}

type KuberenetesConfig struct {
	AnnotationKey string
}

type Config struct {
	Nsq        NsqConfig
	LogLevel   string
	Kubernetes KuberenetesConfig
}

func NewConfig() Config {
	config := Config{}
	config.Nsq.ClientConfig = nsq.NewConfig()
	return config
}
