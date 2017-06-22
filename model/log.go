package model

import "time"

type DockerMeta struct {
	ContainerId string `json:"container_id"`
}

type KubernetesMeta struct {
	NamespaceName string            `json:"namespace_name"`
	PodName       string            `json:"pod_name"`
	PodId         string            `json:"pod_id"`
	Labels        map[string]string `json:"labesl"`
	Host          string            `json:"host"`
	Annotations   map[string]string `json:"annotations"`
	ContainerName string            `json:"container_name"`
}

type TraefikLog struct {
	Log                   string         `json:"log"`
	Stream                string         `json:"stream"`
	Time                  time.Time      `json:"time"`
	Docker                DockerMeta     `json:"docker"`
	Kubernetes            KubernetesMeta `json:"kubernetes"`
	Datacenter            string         `json:"datacenter"`
	KubernetesClusterName string         `json:"kubernetes_cluster_name"`
	Ts                    uint64         `json:"_ts"`
}
