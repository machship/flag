package flag_test

import (
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/machship/flag"
)

// Helper to set os.Args for ParseStruct which internally calls flag.Parse.
func withArgs(args []string, fn func()) {
	old := os.Args
	os.Args = append([]string{"cmd"}, args...)
	defer func() { os.Args = old }()
	fn()
}

func TestParseStruct_MissingRequired(t *testing.T) {
	ResetForTesting(nil)
	type Config struct {
		Host  string `flag:"host" default:"localhost" help:"host name"`
		Port  int    `flag:"port" default:"8080" help:"port number"`
		Debug bool   `flag:"debug" required:"true" help:"enable debug"`
	}
	var cfg Config
	withArgs([]string{}, func() {
		err := ParseStruct(&cfg)
		if err == nil || !strings.Contains(err.Error(), "missing required flags: debug") {
			if err == nil {
				t.Fatal("expected error for missing required flag 'debug', got nil")
			}
			t.Fatalf("expected missing required debug flag, got: %v", err)
		}
	})
}

func TestParseStruct_SuccessAndDefaults(t *testing.T) {
	ResetForTesting(nil)
	type Config struct {
		Host  string `flag:"host" default:"localhost"`
		Port  int    `flag:"port" default:"8080"`
		Debug bool   `flag:"debug" required:"true"`
	}
	var cfg Config
	withArgs([]string{"-debug", "-port", "9090"}, func() {
		if err := ParseStruct(&cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if cfg.Host != "localhost" {
		t.Errorf("expected default host 'localhost', got %q", cfg.Host)
	}
	if cfg.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Port)
	}
	if !cfg.Debug {
		t.Errorf("expected debug true")
	}
}

func TestParseStruct_RequiredIgnoresDefault(t *testing.T) {
	ResetForTesting(nil)
	type C struct {
		Foo string `flag:"foo" default:"bar" required:"true"`
	}
	var c C
	withArgs([]string{}, func() {
		err := ParseStruct(&c)
		if err == nil {
			t.Fatalf("expected error for missing required foo")
		}
	})
	ResetForTesting(nil)
	withArgs([]string{"-foo", "baz"}, func() {
		var d C
		if err := ParseStruct(&d); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if d.Foo != "baz" {
			t.Errorf("expected foo=baz, got %s", d.Foo)
		}
	})
}

func TestParseStruct_InvalidDefault(t *testing.T) {
	ResetForTesting(nil)
	type C struct {
		Port int `flag:"port" default:"NaN"`
	}
	var c C
	withArgs([]string{}, func() {
		err := ParseStruct(&c)
		if err == nil || !strings.Contains(err.Error(), "invalid default int") {
			t.Fatalf("expected invalid default int error, got: %v", err)
		}
	})
}

// Verify []string slice support via ParseStruct (regression for previous unsupported type test)
func TestParseStruct_StringSliceSupported(t *testing.T) {
	ResetForTesting(nil)
	type C struct {
		List []string `flag:"list" default:"a,b,c" sep:","`
	}
	var c C
	withArgs([]string{"-list", "x,y"}, func() {
		if err := ParseStruct(&c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if len(c.List) != 2 || c.List[0] != "x" || c.List[1] != "y" {
		t.Fatalf("expected list [x y], got %#v", c.List)
	}
}

// An actually unsupported type should error (e.g., complex64 not implemented)
func TestParseStruct_UnsupportedType(t *testing.T) {
	ResetForTesting(nil)
	type C struct {
		Num complex64 `flag:"num"`
	}
	var c C
	withArgs([]string{}, func() {
		err := ParseStruct(&c)
		if err == nil || !strings.Contains(err.Error(), "unsupported field type") {
			t.Fatalf("expected unsupported field type error, got: %v", err)
		}
	})
}

func TestParseStruct_EnvSatisfiesRequired(t *testing.T) {
	ResetForTesting(nil)
	if err := os.Setenv("API_KEY", "secret"); err != nil {
		t.Fatalf("set env: %v", err)
	}
	defer os.Unsetenv("API_KEY")
	type C struct {
		APIKey string `flag:"api-key" required:"true"`
	}
	var c C
	withArgs([]string{}, func() {
		if err := ParseStruct(&c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if c.APIKey != "secret" {
		t.Errorf("expected APIKey from env 'secret', got %q", c.APIKey)
	}
}

func TestParseStruct_DurationAndOthers(t *testing.T) {
	ResetForTesting(nil)
	type C struct {
		Timeout time.Duration `flag:"timeout" default:"5s"`
		Rate    float64       `flag:"rate" default:"1.5"`
		Max     int64         `flag:"max" default:"100"`
		Size    uint64        `flag:"size" default:"42"`
	}
	var c C
	withArgs([]string{}, func() {
		if err := ParseStruct(&c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if c.Timeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", c.Timeout)
	}
	if c.Rate != 1.5 {
		t.Errorf("expected rate 1.5, got %v", c.Rate)
	}
	if c.Max != 100 {
		t.Errorf("expected max 100, got %d", c.Max)
	}
	if c.Size != 42 {
		t.Errorf("expected size 42, got %d", c.Size)
	}
}

func TestParseStruct_CalledAfterParse(t *testing.T) {
	ResetForTesting(nil)
	withArgs([]string{"-test_dummy"}, func() {
		Bool("test_dummy", false, "")
		Parse()
		type C struct {
			Foo string `flag:"foo"`
		}
		var c C
		err := ParseStruct(&c)
		if err == nil || !strings.Contains(err.Error(), "must be called before") {
			t.Fatalf("expected pre-parse error, got: %v", err)
		}
	})
}
