package version

var (
	// Version is populated at build time via -ldflags.
	Version = "1.0.2"
	// GitCommit is populated at build time via -ldflags.
	GitCommit = "dev"
	// BuildDate is populated at build time via -ldflags.
	BuildDate = "unknown"
)

// Info contains build metadata.
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildDate string `json:"build_date"`
}

// GetInfo returns the application version metadata.
func GetInfo() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
	}
}
