package queue

import (
	"time"

	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/Wikia/nsq-traefik-consumer/common"
	"github.com/influxdata/influxdb/client/v2"
)

var influxClient client.Client

type MetricsBuffer struct {
	Metrics []client.BatchPoints
	sync.RWMutex
}

func GetInfluxClient(config common.InfluxDbConfig) (client.Client, error) {
	if influxClient != nil {
		return influxClient, nil
	}

	clientConfig := client.HTTPConfig{
		Addr:     config.Address,
		Username: config.Username,
		Password: config.Password,
	}

	influxClient, err := client.NewHTTPClient(clientConfig)

	if err != nil {
		return nil, err
	}

	return influxClient, nil
}

func RunSender(config common.InfluxDbConfig, metrics *MetricsBuffer) {
	go func() {
		for {
			<-time.After(config.SendInterval)
			err := sendMetrics(config, metrics)
			if err != nil {
				log.WithError(err).Error("Error sending metrics")
			}
		}
	}()
}

func NewMetricsBuffer() MetricsBuffer {
	return MetricsBuffer{Metrics: []client.BatchPoints{}}
}

func sendMetrics(config common.InfluxDbConfig, metrics *MetricsBuffer) error {
	if len(metrics.Metrics) == 0 {
		return nil
	}

	influxClient, err := GetInfluxClient(config)

	if err != nil {
		return err
	}

	metrics.Lock()
	defer metrics.Unlock()
	cnt := 0
	for _, bucket := range metrics.Metrics {
		bucket.SetDatabase(config.Database)
		bucket.SetPrecision("s")
		bucket.SetRetentionPolicy(config.RetentionPolicy)

		err = influxClient.Write(bucket)
		if err != nil {
			log.WithError(err).Error("Error sending metrics to Influx DB")
		}
		cnt += len(bucket.Points())
	}

	log.WithField("count", cnt).Info("Finished writing metrics to InfluxDB")

	metrics.Metrics = []client.BatchPoints{}

	return nil
}
