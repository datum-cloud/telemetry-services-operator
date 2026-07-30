package unbatchprocessor // import "go.datum.net/o11y/processor/unbatchprocessor"

// Config is intentionally empty: unbatch always explodes every incoming
// batch down to one record per outgoing batch, for every signal type it's
// wired into. There is nothing to configure.
type Config struct{}

func (*Config) Validate() error {
	return nil
}
