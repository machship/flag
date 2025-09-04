package flag

import "testing"

func FuzzEnumParse(f *testing.F) {
	for _, s := range []string{"red", "blue", "green", "", "INVALID", "@@"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, val string) {
		var dst string
		ev := newEnumStringValue("red", []string{"red", "green", "blue"}, &dst)
		_ = ev.Set(val) // error allowed, just no panic
	})
}
