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
				common.Log.WithError(err).Error("Error sending metrics")
			}
		}
	}()
}

func NewMetricsBuffer() *MetricsBuffer {
	return &MetricsBuffer{Metrics: []client.BatchPoints{}}
}

func sendMetrics(config common.InfluxDbConfig, metrics *MetricsBuffer) error {
	if len(metrics.Metrics) == 0 {
		return nil
	}

	influxClient, err := GetInfluxClient(config)

	if err != nil {
		return err
	}

	cnt := 0
	curCnt := 0
	for {
		if len(metrics.Metrics) == 0 {
			break
		}

		var bucket client.BatchPoints

		metrics.Lock()
		bucket, metrics.Metrics = metrics.Metrics[0], metrics.Metrics[1:]
		metrics.Unlock()

		bucket.SetDatabase(config.Database)
		bucket.SetPrecision("ns")
		bucket.SetRetentionPolicy(config.RetentionPolicy)

		err = influxClient.Write(bucket)
		if err != nil {
			common.Log.WithError(err).Error("Error sending metrics to Influx DB")
		}
		curCnt = len(bucket.Points())
		cnt += curCnt

		common.Log.WithFields(log.Fields{
			"count":        cnt,
			"buckets_left": len(metrics.Metrics),
		}).Info("Wrote metrics to InfluxDB")
	}

	common.Log.WithField("count", cnt).Info("Finished writing metrics to InfluxDB")

	metrics.Metrics = []client.BatchPoints{}

	return nil
}
