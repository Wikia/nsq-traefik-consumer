# nsq-traefik-consumer
[![Build Status](https://travis-ci.org/Wikia/nsq-traefik-consumer.svg?branch=master)](https://travis-ci.org/Wikia/nsq-traefik-consumer)

Consumer for NSQ Traefik access logs in Kubernetes

This tool attaches to NSQ queue with Treafik logs and looks for a entry with proper k8s annotation
(configured via `AnnotationKey` config variable). When it finds entry it grabs and parses Traefik's
access log and performs filtering. Following conditions (configured as individual rules) must be met for an entry to be forwarded to
InfluxDB:
* frontend name must match Regexp specified in a given rule
* (optional) path matches specified Regexp
* (optional) HTTP method matches specified Regexp
* random generated number is lower or equal to one specified as threshold (sampling)

Annotation should have proper fields with proper values defined. Here is the sample annotation:
```yaml
{
  "wikia_com/keys": {
    "influx_metrics": {
      "container_name": "foo-bar", 
      "type": "traefik"
    }
  }
}
```

* `container_name` needs to be specified to properly indicate POD running Traefik instance (there can be more containers per POD).
* `type` needs to specified but it's value is ignored for now
 
Since Traefik sends access logs with only precision of 1 second this tools uses time of processing as
a timestamp sent to InfluxDB. This may cause offsets and delays or even data being compressed when
queue is not being processed fast enough. This mitigates problem with data points being overwritten in
InfluxDB (at least it lowers risk greatly).

Data being sent to InfluxDB are in the form of:

##### Values
* `log_timestamp` - the actual timestamp from the Traefik access log
* `backend_url` - url of the Traefik backed request was handled by
* `request_method` - HTTP method of the request
* `client_username` - HTTP Auth username if valid
* `http_referer` - value of HTTP referer header
* `http_user_agent` - value of HTTP user agent header
* `request_url` - full request url path
* `response_code` - HTTP response code
* `request_time` - request time in ms
* `request_count` - auto incremented request counter
* `response_size` - response size in bytes

##### Tags
* `frontend_name` - name of the Traefik frontend that handled the request
* `host_name` - host name which Traefik is running on
* `cluster_name` - k8s cluster name
* `data_center` - k8s data centre
* `rule_id` - Id of the rule that matched to the given request

### Sample configuration
```yaml
LogLevel: debug
Nsq:
  Addresses:
    - http://nsqlookupd1.sjc.k8s.wikia.net
    - http://nsqlookupd2.sjc.k8s.wikia.net
  Topic: logstash-k8s
  Channel: logstash-k8s-influx-consumers
Kubernetes:
  AnnotationKey: wikia_com/keys
InfluxDB:
  Address: http://prod.app-metrics-db.service.sjc.consul:8086
  Database: apps_test
  SendInterval: 5s
  Measurement: k8s_traefik
  RetentionPolicy: short_term
Rules:
  - Id: k8s_helios
    UrlRegexp: ^/info
    MethodRegexp: ^POST$
    FrontendRegexp: \.k8s\.wikia\.net/helios
    Sampling: 1.0
```
