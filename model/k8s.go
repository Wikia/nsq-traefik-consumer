package model

import "time"

type DockerMeta struct {
	ContainerId string `json:"container_id"`
}

type GenericInfluxAnnotation struct {
	ContainerName string `mapstructure:"container_name"`
	MetricsType   string `mapstructure:"type"`
}

type KubernetesMeta struct {
	NamespaceName string            `json:"namespace_name"`
	PodName       string            `json:"pod_name"`
	PodId         string            `json:"pod_id"`
	Labels        map[string]string `json:"labels"`
	Host          string            `json:"host"`
	Annotations   map[string]string `json:"annotations"`
	ContainerName string            `json:"container_name"`
}

type LogEntry struct {
	Log                   string         `json:"log"`
	Stream                string         `json:"stream"`
	Time                  time.Time      `json:"time"`
	Docker                DockerMeta     `json:"docker"`
	Kubernetes            KubernetesMeta `json:"kubernetes"`
	Datacenter            string         `json:"datacenter,omitempty"`
	KubernetesClusterName string         `json:"kubernetes_cluster_name,omitempty"`
	Ts                    uint64         `json:"_ts"`
}
