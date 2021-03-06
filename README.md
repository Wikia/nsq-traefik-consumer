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
      "type": "access_log_as_json"
    }
  }
}
```

* `container_name` needs to be specified to properly indicate POD running Traefik instance (there can be more containers per POD).
* `type` specifies the log format application expects when parsing events from NSQ. Possible values are:
    - access_log_combined (legacy access log compatible with Apache/Nginx)
    - access_log_as_json (introduced in Traefik 1.4)
 
Since Traefik sends access logs with only precision of 1 second this tools uses time of processing as
a timestamp sent to InfluxDB. This may cause offsets and delays or even data being compressed when
queue is not being processed fast enough. This mitigates problem with data points being overwritten in
InfluxDB (at least it lowers risk greatly).

Data being sent to InfluxDB are in the form of:

##### Values
* `duration`
* `backend_url`
* `backend_address`
* `client_address`
* `client_host`
* `client_port`
* `client_username`
* `request_address`
* `request_host`
* `request_port`
* `request_method`
* `request_path`
* `request_protocol`
* `request_line`
* `request_content_size`
* `origin_duration`
* `origin_content_size`
* `origin_status`
* `origin_status_line`
* `downstream_status`
* `downstream_status_line`
* `downstream_content_size`
* `request_count`
* `gzip_ratio`
* `overhead`
* `retry_attempts`
* `original_timestamp`
* request headers prefixed with `request__` (i.e. `request__user-agent`)
* origin headers prefixed with `origin__` (i.e. `origin__content-size`)

##### Tags
* `frontend_name` - name of the Traefik frontend that handled the request
* `backend_name` - name of the Traefik backend
* `host_name` - host name which Traefik is running on
* `cluster_name` - k8s cluster name
* `data_center` - k8s data centre
* `rule_id` - Id of the rule that matched to the given request

### Sample configuration
```yaml
LogLevel: debug
LogAsJson: true
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
Fields:
  - duration
  - backend_url
  - backend_address
  - client_address
  - client_host
  - client_port
  - client_username
  - request_address
  - request_host
  - request_port
  - request_method
  - request_path
  - request_protocol
  - request_line
  - request_content_size
  - origin_duration
  - origin_content_size
  - origin_status
  - origin_status_line
  - downstream_status
  - downstream_status_line
  - downstream_content_size
  - request_count
  - gzip_ratio
  - overhead
  - retry_attempts
  - original_timestamp
  - referrer
  - client_user_agent
Rules:
  - Id: k8s_helios
    UrlRegexp: ^/info
    MethodRegexp: ^POST$
    FrontendRegexp: \.k8s\.wikia\.net/helios
    Sampling: 1.0
```
