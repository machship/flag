package flag

import (
	"testing"
)

func TestKeysHelperAndEnumError(t *testing.T) {
	ResetForTesting(nil)
	// create enum with limited values then attempt invalid set
	var s string
	EnumVar(&s, "color", "red", []string{"red", "green"}, "")
	// manually fetch flag and invoke Set with invalid value to hit error branch and keys()
	fl := Lookup("color")
	if fl == nil {
		t.Fatalf("enum flag not found")
	}
	if err := fl.Value.Set("blue"); err == nil || !contains(err.Error(), "allowed") {
		t.Fatalf("expected enum error, got %v", err)
	}
}

// minimal contains to avoid importing strings inside package under test twice
func contains(hay, needle string) bool {
	return len(needle) == 0 || (len(hay) >= len(needle) && index(hay, needle) >= 0)
}

// naive substring search (avoid pulling strings). For small test usage only.
func index(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
