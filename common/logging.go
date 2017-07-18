package common

import (
	log "github.com/Sirupsen/logrus"
)

var Log = log.WithField("appname", "nsq-traefik-consumer")
