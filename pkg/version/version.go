package version

// These variables are populated via -ldflags at build-time by GoReleaser.
// Defaults are useful for local builds.
var (
    version   = "dev"
    gitSHA    = ""
    buildTime = ""
)

// Version returns the version string (e.g. v0.4.0 or dev).
func Version() string {
    if version == "" {
        return "dev"
    }
    return version
}

// GitSHA returns the git commit SHA, if available.
func GitSHA() string { return gitSHA }

// BuildTime returns the build time, if available.
func BuildTime() string { return buildTime }

// String returns a human-readable version string.
func String() string { return Version() }

// Info returns a multi-field version line suitable for CLI output.
func Info() string {
    v := Version()
    if gitSHA == "" && buildTime == "" {
        return v
    }
    if gitSHA != "" && buildTime != "" {
        return v + " (" + gitSHA + ", " + buildTime + ")"
    }
    if gitSHA != "" {
        return v + " (" + gitSHA + ")"
    }
    return v + " (" + buildTime + ")"
}
