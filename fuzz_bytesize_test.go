package flag

import "testing"

func FuzzByteSize(f *testing.F) {
	for _, s := range []string{"0", "1", "10K", "5MiB", "2g", "3Ti", "-1", "@@literal", "1.5MB"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		_, _ = parseByteSize(in) // must not panic
	})
}
