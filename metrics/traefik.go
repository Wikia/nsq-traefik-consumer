package metrics

import (
	"fmt"

	"github.com/Wikia/nsq-traefik-consumer/model"
)

type TraefikMetricProcessor struct {
}

func (mp TraefikMetricProcessor) Process(entry model.TraefikLog) ([]model.TimePoint, error) {
	matches := ApacheCombinedLogRegex.FindStringSubmatch(entry.Log)

	if len(matches) == 0 {
		return nil, fmt.Errorf("Could not match log to regex")
	}

	return []model.TimePoint{}, nil
}
