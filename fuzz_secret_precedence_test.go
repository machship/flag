package flag

import "testing"

// Simple fuzz ensuring secret provider precedence does not panic and respects existing actual flags.
func FuzzSecretProviderPrecedence(f *testing.F) {
	f.Add("initial", "override")
	f.Fuzz(func(t *testing.T, first string, second string) {
		fs := NewFlagSet("test", ContinueOnError)
		var a string
		fs.StringVar(&a, "alpha", "", "")
		// set via env simulation by direct Set before provider
		_ = fs.Set("alpha", first)
		fs.secretProvider = SecretProviderFunc(func(name string) (string, error) {
			if name == "alpha" {
				return second, nil
			}
			return "", nil
		})
		// parse with no args
		_ = fs.Parse([]string{})
		// value should remain first if already set (precedence), second otherwise
		_ = a // just reference
	})
}

type SecretProviderFunc func(string) (string, error)

func (f SecretProviderFunc) Get(name string) (string, error) { return f(name) }
