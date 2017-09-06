package common

import (
	"encoding/json"
	"net/http"

	"github.com/fukata/golang-stats-api-handler"
	"github.com/rcrowley/go-metrics"
)

func ServeStats() {
	http.HandleFunc("/stats/gc", stats_api.Handler)
	http.HandleFunc("/stats/internal", handleInternalMetrics)
	http.ListenAndServe(":8080", nil)
}

func handleInternalMetrics(resp http.ResponseWriter, req *http.Request) {
	data := map[string]interface{}{}
	metrics.Each(func(name string, metric interface{}) {
		switch metric.(type) {
		case metrics.Meter:
			meter := metric.(metrics.Meter)
			data[name] = meter.RateMean()
			data[name+"_cnt"] = meter.Count()
		case metrics.Counter:
			counter := metric.(metrics.Counter)
			data[name] = counter.Count()
		case metrics.Gauge:
			gauge := metric.(metrics.Gauge)
			data[name] = gauge.Value()
		}
	})

	bytes, err := json.Marshal(data)

	if err != nil {
		Log.WithError(err).Error("Error encoding stats into json")
		resp.WriteHeader(500)
		return
	}

	resp.Write(bytes)
	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(200)

	return
}
