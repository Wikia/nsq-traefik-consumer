package model

import (
	"net/url"
	"time"
)

type TraefikAccessLog struct {
	StartUTC              time.Time
	StartLocal            time.Time
	Duration              time.Duration
	FrontendName          string
	BackendName           string
	BackendURL            url.URL
	BackendAddr           string
	ClientAddr            string
	ClientHost            string
	ClientPort            string
	ClientUsername        string
	RequestAddr           string
	RequestHost           string
	RequestPort           string
	RequestMethod         string
	RequestPath           string
	RequestProtocol       string
	RequestLine           string
	RequestContentSize    int64
	OriginDuration        time.Duration
	OriginContentSize     int64
	OriginStatus          int64
	OriginStatusLine      string
	DownstreamStatus      int64
	DownstreamStatusLine  string
	DownstreamContentSize int64
	RequestCount          int64
	GzipRatio             float64
	Overhead              time.Duration
	RetryAttempts         int64
	OriginalTimestamp     time.Time
	Referrer              string
	ClientUserAgent       string
}

func (t *TraefikAccessLog) GetValues() map[string]interface{} {
	v := map[string]interface{}{}

	v["duration"] = t.Duration
	v["backend_url"] = t.BackendURL.String()
	v["backend_address"] = t.BackendAddr
	v["client_address"] = t.ClientAddr
	v["client_host"] = t.ClientHost
	v["client_port"] = t.ClientPort
	v["client_username"] = t.ClientUsername
	v["request_address"] = t.RequestAddr
	v["request_host"] = t.RequestHost
	v["request_port"] = t.RequestPort
	v["request_method"] = t.RequestMethod
	v["request_path"] = t.RequestPath
	v["request_protocol"] = t.RequestProtocol
	v["request_line"] = t.RequestLine
	v["request_content_size"] = t.RequestContentSize
	v["origin_duration"] = t.OriginDuration
	v["origin_content_size"] = t.OriginContentSize
	v["origin_status"] = t.OriginStatus
	v["origin_status_line"] = t.OriginStatusLine
	v["downstream_status"] = t.DownstreamStatus
	v["downstream_status_line"] = t.DownstreamStatusLine
	v["downstream_content_size"] = t.DownstreamContentSize
	v["request_count"] = t.RequestCount
	v["gzip_ratio"] = t.GzipRatio
	v["overhead"] = t.Overhead
	v["retry_attempts"] = t.RetryAttempts
	if !t.OriginalTimestamp.IsZero() {
		v["original_timestamp"] = t.OriginalTimestamp
	}
	v["referrer"] = t.Referrer
	v["client_user_agent"] = t.ClientUserAgent

	return v
}

func (t *TraefikAccessLog) GetTags() map[string]string {
	v := map[string]string{}

	v["frontend_name"] = t.FrontendName
	v["backend_name"] = t.BackendName
	return v
}
