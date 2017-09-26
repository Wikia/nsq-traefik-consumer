package metrics

import "regexp"

var ApacheCombinedLogRegex = regexp.MustCompile(`^(?P<client_host>\S+)\s-\s+(?P<client_username>\S+\s+)+\[(?P<timestamp>[^]]+)\]\s"(?P<request_method>\S*)\s?(?P<request_path>(?:[^"]*(?:\\")?)*)\s(?P<request_protocol>[^"]*)"\s(?P<origin_status>\d+)\s(?P<origin_content_size>\d+)\s"(?P<request__referer>(?:[^"]*(?:\\")?)*)"\s"(?P<request__useragent>.*)"\s(?P<request_count>\d+)\s"(?P<frontend_name>[^"]+)"\s"(?P<backend_url>[^"]+)"\s(?P<duration>\d+)ms$`)
