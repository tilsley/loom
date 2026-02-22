package steps

// Config holds worker-wide configuration passed to step handlers.
type Config struct {
	GitopsOwner string
	GitopsRepo  string
	Envs        []string
}
