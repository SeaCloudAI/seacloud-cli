package buildinfo

const Name = "seacloud"

// Version is injected at build time.
// Default to "dev" for local development builds.
var Version = "dev"

func UserAgent() string {
	return Name + "-cli/" + Version
}
