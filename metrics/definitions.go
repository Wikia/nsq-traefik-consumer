package metrics

import "regexp"

var ApacheCombinedLogRegex = regexp.MustCompile(`^(?P<client_host>\S+)\s-\s+(?P<username>\S+\s+)+\[(?P<timestamp>[^]]+)\]\s"(?P<method>\S*)\s?(?P<path>(?:[^"]*(?:\\")?)*)\s(?P<protocol>[^"]*)"\s(?P<status>\d+)\s(?P<content_size>\d+)\s"(?P<referrer>(?:[^"]*(?:\\")?)*)"\s"(?P<useragent>.*)"\s(?P<request_count>\d+)\s"(?P<frontend_name>[^"]+)"\s"(?P<backend_url>[^"]+)"\s(?P<request_time>\d+ms)$`)
