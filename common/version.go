package common

import "fmt"

// VersionInfo holds the information about current app version
type VersionInfo struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
}

func (v VersionInfo) String() string {
	if v.BuildTime != "" {
		return fmt.Sprintf("%s (time: %s)", v.Version, v.BuildTime)
	}

	return fmt.Sprintf("%s", v.Version)
}

var (
	// Version current version - semantic format
	Version string
	// BuildTime time of the build
	BuildTime string
)

// GetCurrentVersion returns information about current version of the app
func GetCurrentVersion() VersionInfo {
	return VersionInfo{Version, BuildTime}
}
