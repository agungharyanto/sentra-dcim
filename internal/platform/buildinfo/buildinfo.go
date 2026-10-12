// Package buildinfo exposes build metadata injected at link time via -ldflags.
package buildinfo

import "runtime"

// Set via -ldflags "-X .../buildinfo.Version=..." (see Makefile and Dockerfile).
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = ""
)

// Info is a snapshot of build metadata.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date,omitempty"`
	GoVersion string `json:"go_version"`
}

// Get returns the current build metadata.
func Get() Info {
	return Info{Version: Version, Commit: Commit, BuildDate: BuildDate, GoVersion: runtime.Version()}
}
