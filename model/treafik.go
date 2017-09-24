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
