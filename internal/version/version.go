package version

// Edit Version here to bump the release.
var Version = "0.2.1"

// Commit is injected at build time via -ldflags (local=dev, CI=git hash).
var Commit string
