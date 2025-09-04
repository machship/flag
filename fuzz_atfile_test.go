package flag

import (
	"os"
	"testing"
)

func FuzzAtFile(f *testing.F) {
	f.Add("@does-not-exist")
	f.Add("@@literal")
	f.Add("plain")
	f.Fuzz(func(t *testing.T, input string) {
		// create temp file occasionally when pattern starts with '@'
		if len(input) > 1 && input[0] == '@' && input != "@@literal" {
			tmp, err := os.CreateTemp(t.TempDir(), "af")
			if err != nil {
				t.Skip()
			}
			_, _ = tmp.WriteString("data")
			_ = tmp.Close()
			input = "@" + tmp.Name()
		}
		_, _ = expandAtFile(input) // ignore errors; just ensure no panic
	})
}
