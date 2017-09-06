package queue

import (
	"time"

	"sync"

	"container/list"

	"github.com/Wikia/nsq-traefik-consumer/common"
	"github.com/influxdata/influxdb/client/v2"
	stats "github.com/rcrowley/go-metrics"
)

var influxClient client.Client

type MetricsBuffer struct {
	Metrics *list.List
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
	return &MetricsBuffer{Metrics: list.New()}
}

func sendMetrics(config common.InfluxDbConfig, metrics *MetricsBuffer) error {
	if metrics.Metrics.Len() == 0 {
		return nil
	}

	influxClient, err := GetInfluxClient(config)

	if err != nil {
		return err
	}

	gauge := stats.GetOrRegisterGauge("buffer_size", stats.DefaultRegistry)

	cnt := 0
	for {
		if metrics.Metrics.Len() == 0 {
			break
		}

		var bucket client.BatchPoints

		metrics.Lock()
		element := metrics.Metrics.Front()
		metrics.Metrics.Remove(element)
		metrics.Unlock()
		gauge.Update(int64(metrics.Metrics.Len()))

		bucket, _ = element.Value.(client.BatchPoints)
		bucket.SetDatabase(config.Database)
		bucket.SetPrecision("ns")
		bucket.SetRetentionPolicy(config.RetentionPolicy)

		err = influxClient.Write(bucket)
		if err != nil {
			common.Log.WithError(err).Error("Error sending metrics to Influx DB")
		}

		counter := stats.GetOrRegisterCounter("points_sent", stats.DefaultRegistry)
		cnt += len(bucket.Points())
		counter.Inc(int64(len(bucket.Points())))
	}

	common.Log.WithField("count", cnt).Info("Finished writing metrics to InfluxDB")

	return nil
}
