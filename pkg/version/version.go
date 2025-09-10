package version

// These variables are populated via -ldflags at build-time by GoReleaser.
// Defaults are useful for local builds.
var (
    version   = "dev"
    gitSHA    = ""
    buildTime = ""
)

// String returns a human-readable version string.
func String() string {
    v := version
    if v == "" {
        v = "dev"
    }
    return v
}

