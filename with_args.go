package flag

import "os"

// internalReset recreates CommandLine similar to test helper when not in test build.
func internalReset() {
	CommandLine = NewFlagSet(os.Args[0], ContinueOnError)
}

// WithArgs temporarily sets os.Args, reparses flags, runs fn, then restores state.
// Flags must be re-registered prior to calling if required.
func WithArgs(args []string, fn func() error) error {
	origArgs := os.Args
	if len(args) == 0 {
		os.Args = []string{origArgs[0]}
	} else {
		os.Args = args
	}
	internalReset()
	if err := CommandLine.Parse(os.Args[1:]); err != nil {
		os.Args = origArgs
		return err
	}
	err := fn()
	os.Args = origArgs
	return err
}
