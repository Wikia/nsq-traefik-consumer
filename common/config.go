package common

import "github.com/nsqio/go-nsq"

type NsqConfig struct {
	Address      string
	Topic        string
	Channel      string
	ClientConfig *nsq.Config
}

type KubernetesConfig struct {
	AnnotationKey string
}

type RulesConfig struct {
	Id             string
	UrlRegexp      string
	FrontendRegexp string
	Sampling       float64
}

type Config struct {
	Nsq        NsqConfig
	LogLevel   string
	Kubernetes KubernetesConfig
	Rules      []RulesConfig
}

func NewConfig() Config {
	config := Config{}
	config.Nsq.ClientConfig = nsq.NewConfig()
	return config
}
