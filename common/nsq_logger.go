package common

import (
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/nsqio/go-nsq"
)

var (
	nsqDebugLevel = nsq.LogLevelDebug.String()
	nsqInfoLevel  = nsq.LogLevelInfo.String()
	nsqWarnLevel  = nsq.LogLevelWarning.String()
	nsqErrLevel   = nsq.LogLevelError.String()
)

// NSQLogrusLogger is an adaptor between the weird go-nsq Logger and our
// standard logrus logger.
type NSQLogrusLogger struct{}

// NewNSQLogrusLogger returns a new NSQLogrusLogger and the current log level.
// This is a format to easily plug into nsq.SetLogger.
func NewNSQLogrusLogger() (logger NSQLogrusLogger, level nsq.LogLevel) {
	return NewNSQLogrusLoggerAtLevel(log.GetLevel())
}

// NewNSQLogrusLoggerAtLevel returns a new NSQLogrusLogger with the provided log level mapped to nsq.LogLevel for easily plugging into nsq.SetLogger.
func NewNSQLogrusLoggerAtLevel(l log.Level) (logger NSQLogrusLogger, level nsq.LogLevel) {
	logger = NSQLogrusLogger{}
	level = nsq.LogLevelWarning
	switch l {
	case log.DebugLevel:
		level = nsq.LogLevelDebug
	case log.InfoLevel:
		level = nsq.LogLevelInfo
	case log.WarnLevel:
		level = nsq.LogLevelWarning
	case log.ErrorLevel:
		level = nsq.LogLevelError
	}
	return
}

// Output implements stdlib log.Logger.Output using logrus
// Decodes the go-nsq log messages to figure out the log level
func (n NSQLogrusLogger) Output(_ int, s string) error {
	if len(s) > 3 {
		msg := strings.TrimSpace(s[3:])
		switch s[:3] {
		case nsqDebugLevel:
			Log.Debugln(msg)
		case nsqInfoLevel:
			Log.Infoln(msg)
		case nsqWarnLevel:
			Log.Warnln(msg)
		case nsqErrLevel:
			Log.Errorln(msg)
		default:
			Log.Infoln(msg)
		}
	}
	return nil
}
