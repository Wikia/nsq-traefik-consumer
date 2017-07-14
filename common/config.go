package common

import (
	"time"

	"github.com/nsqio/go-nsq"
)

type NsqConfig struct {
	Addresses    []string
	Topic        string
	Channel      string
	MaxInFlight  int
	ClientConfig *nsq.Config
}

type KubernetesConfig struct {
	AnnotationKey string
}

type InfluxDbConfig struct {
	Address         string
	Username        string
	Password        string
	Database        string
	Measurement     string
	RetentionPolicy string
	SendInterval    time.Duration
}

type RulesConfig struct {
	Id             string
	UrlRegexp      string
	FrontendRegexp string
	MethodRegexp   string
	Sampling       float64
}

type Config struct {
	Nsq        NsqConfig
	LogLevel   string
	Kubernetes KubernetesConfig
	InfluxDB   InfluxDbConfig
	Rules      []RulesConfig
}

func NewConfig() Config {
	config := Config{}
	config.Nsq.ClientConfig = nsq.NewConfig()
	return config
}
