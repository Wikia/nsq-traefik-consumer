package queue

import (
	"time"

	"sync"

	"container/list"

	"github.com/Wikia/nsq-traefik-consumer/common"
	"github.com/containous/traefik/log"
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
	counter := stats.GetOrRegisterCounter("points_sent", stats.DefaultRegistry)

	buffer, err := client.NewBatchPoints(client.BatchPointsConfig{
		Precision:       "ns",
		Database:        config.Database,
		RetentionPolicy: config.RetentionPolicy,
	})

	if err != nil {
		common.Log.WithError(err).Error("Error creating batch points for Influx")
	}

	for {
		if metrics.Metrics.Len() == 0 {
			break
		}

		metrics.Lock()
		element := metrics.Metrics.Front()
		metrics.Metrics.Remove(element)
		metrics.Unlock()
		gauge.Update(int64(metrics.Metrics.Len()))

		bucket, _ := element.Value.(client.BatchPoints)
		buffer.AddPoints(bucket.Points())

		if len(buffer.Points()) >= config.BatchSize {
			err = influxClient.Write(buffer)
			if err != nil {
				common.Log.WithError(err).Error("Error sending metrics to Influx DB")
			}

			counter.Inc(int64(len(buffer.Points())))
			buffer, err = client.NewBatchPoints(client.BatchPointsConfig{
				Precision:       "ns",
				Database:        config.Database,
				RetentionPolicy: config.RetentionPolicy,
			})
			if err != nil {
				log.WithError(err).Error("Error recreating batch points for Influx")
			}
		}
	}

	if len(buffer.Points()) > 0 {
		err = influxClient.Write(buffer)
		if err != nil {
			common.Log.WithError(err).Error("Error sending metrics to Influx DB")
		}

		counter.Inc(int64(len(buffer.Points())))
	}

	common.Log.WithField("count", counter.Count()).Info("Finished writing metrics to InfluxDB")

	return nil
}
