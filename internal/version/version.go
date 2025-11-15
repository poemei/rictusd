package version

var (
	// set via -ldflags
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func String() string {
	return Version + " (" + Commit + " " + Date + ")"
}
