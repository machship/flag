package flag

import (
	"encoding/json"
	"net"
	neturl "net/url"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	decimal "github.com/shopspring/decimal"
)

// TestParseStruct_GuardErrors covers nil pointer, non-pointer, pointer to non-struct, and called-after-parse errors.
func TestParseStruct_GuardErrors(t *testing.T) {
	ResetForTesting(nil)
	// nil pointer
	var p *struct {
		A int `flag:"a"`
	}
	if err := ParseStruct(p); err == nil || !containsSub(err.Error(), "non-nil pointer") {
		t.Fatalf("expected nil pointer error, got %v", err)
	}
	// non-pointer
	if err := ParseStruct(struct{}{}); err == nil || !containsSub(err.Error(), "pointer to a struct") {
		t.Fatalf("expected non-pointer error, got %v", err)
	}
	// pointer to non-struct
	x := 5
	if err := ParseStruct(&x); err == nil || !containsSub(err.Error(), "pointer to a struct") {
		t.Fatalf("expected pointer to non-struct error, got %v", err)
	}
	// called after parse
	ResetForTesting(nil)
	Bool("already", false, "")
	Parse()
	type C struct {
		A int `flag:"a"`
	}
	var c C
	if err := ParseStruct(&c); err == nil || !containsSub(err.Error(), "must be called before") {
		t.Fatalf("expected called-after-parse error, got %v", err)
	}
}

// TestParseStruct_AllRequiredMissing defines one field per explicit concrete type with required=true to hit required branches.
func TestParseStruct_AllRequiredMissing(t *testing.T) {
	ResetForTesting(nil)
	type All struct {
		T   time.Time         `flag:"t" required:"true"`
		D   decimal.Decimal   `flag:"d" required:"true"`
		IP  net.IP            `flag:"ip" required:"true"`
		IPN net.IPNet         `flag:"ipn" required:"true"`
		U   neturl.URL        `flag:"u" required:"true"`
		ID  uuid.UUID         `flag:"id" required:"true"`
		BS  ByteSize          `flag:"bs" required:"true"`
		DS  []time.Duration   `flag:"ds" required:"true"`
		MP  map[string]string `flag:"mp" required:"true"`
		JM  json.RawMessage   `flag:"jm" required:"true"`
		RX  *regexp.Regexp    `flag:"rx" required:"true"`
		B   bool              `flag:"b" required:"true"`
		I   int               `flag:"i" required:"true"`
		I64 int64             `flag:"i64" required:"true"`
		U32 uint              `flag:"u32" required:"true"`
		U64 uint64            `flag:"u64" required:"true"`
		S   string            `flag:"s" required:"true"`
		F   float64           `flag:"f" required:"true"`
	}
	var a All
	if err := ParseStruct(&a); err == nil || !containsSub(err.Error(), "missing required flags") {
		t.Fatalf("expected missing required flags error, got %v", err)
	}
}

// TestParseStruct_DefaultsValid exercises defTag parsing for each explicit concrete type and kind branch.
func TestParseStruct_DefaultsValid(t *testing.T) {
	ResetForTesting(nil)
	now := time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC)
	type C struct {
		T   time.Time         `flag:"t" default:"2023-01-02T03:04:05Z"`
		TL  time.Time         `flag:"tl" layout:"2006-01-02" default:"2023-01-02"`
		D   decimal.Decimal   `flag:"d" default:"12.34"`
		IP  net.IP            `flag:"ip" default:"127.0.0.1"`
		IPN net.IPNet         `flag:"ipn" default:"10.0.0.0/8"`
		U   neturl.URL        `flag:"u" default:"https://example.com/x"`
		ID  uuid.UUID         `flag:"id" default:"123e4567-e89b-12d3-a456-426614174000"`
		BS  ByteSize          `flag:"bs" default:"10KiB"`
		DS  []time.Duration   `flag:"ds" sep:"," default:"1s,2s"`
		MP  map[string]string `flag:"mp" default:"k1=v1,,k2=v2"`
		JM  json.RawMessage   `flag:"jm" default:"{\"a\":1}"`
		RX  *regexp.Regexp    `flag:"rx" default:"^abc$"`
		B   bool              `flag:"b" default:"true"`
		I   int               `flag:"i" default:"42"`
		I64 int64             `flag:"i64" default:"43"`
		U32 uint              `flag:"u32" default:"44"`
		U64 uint64            `flag:"u64" default:"45"`
		S   string            `flag:"s" default:"hello"`
		SE  string            `flag:"se" enum:"red,green" default:"red"`
		F   float64           `flag:"f" default:"3.14"`
		// non-zero initial value with empty default to cover defTag == "" path for time without layout
		TZ time.Time `flag:"tz"`
	}
	var c C
	c.TZ = now
	if err := ParseStruct(&c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.D.String() != "12.34" || c.BS == 0 || c.I != 42 || c.U64 != 45 || c.SE != "red" || len(c.DS) != 2 {
		t.Fatalf("unexpected parsed defaults: %+v", c)
	}
	if c.TZ != now {
		t.Fatalf("expected TZ retained, got %v", c.TZ)
	}
}

// TestParseStruct_InvalidDefaults exercises each invalid default error branch individually.
func TestParseStruct_InvalidDefaults(t *testing.T) {
	cases := []struct {
		name    string
		make    func() error
		wantSub string
	}{
		{"time", func() error {
			ResetForTesting(nil)
			type S struct {
				T time.Time `flag:"t" default:"bad"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default time"},
		{"timeLayout", func() error {
			ResetForTesting(nil)
			type S struct {
				T time.Time `flag:"t" layout:"2006-01-02" default:"bad-date"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default time"},
		{"decimal", func() error {
			ResetForTesting(nil)
			type S struct {
				D decimal.Decimal `flag:"d" default:"bad"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default decimal"},
		{"ip", func() error {
			ResetForTesting(nil)
			type S struct {
				IP net.IP `flag:"ip" default:"bad"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default ip"},
		{"ipnet", func() error {
			ResetForTesting(nil)
			type S struct {
				N net.IPNet `flag:"n" default:"bad"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default cidr"},
		{"url", func() error {
			ResetForTesting(nil)
			type S struct {
				U neturl.URL `flag:"u" default:"http://[bad"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default url"},
		{"uuid", func() error {
			ResetForTesting(nil)
			type S struct {
				ID uuid.UUID `flag:"id" default:"bad"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default uuid"},
		{"bytesize", func() error {
			ResetForTesting(nil)
			type S struct {
				B ByteSize `flag:"b" default:"10XB"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default bytesize"},
		{"durslice", func() error {
			ResetForTesting(nil)
			type S struct {
				D []time.Duration `flag:"d" default:"1s,notdur"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default duration slice element"},
		{"map", func() error {
			ResetForTesting(nil)
			type S struct {
				M map[string]string `flag:"m" default:"badentry"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default map entry"},
		{"json", func() error {
			ResetForTesting(nil)
			type S struct {
				J json.RawMessage `flag:"j" default:"{bad"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default json"},
		{"regexp", func() error {
			ResetForTesting(nil)
			type S struct {
				R *regexp.Regexp `flag:"r" default:"["`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default regexp"},
		{"bool", func() error {
			ResetForTesting(nil)
			type S struct {
				B bool `flag:"b" default:"notbool"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default bool"},
		{"int", func() error {
			ResetForTesting(nil)
			type S struct {
				I int `flag:"i" default:"NaN"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default int"},
		{"int64", func() error {
			ResetForTesting(nil)
			type S struct {
				I int64 `flag:"i" default:"NaN"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default int64"},
		{"duration", func() error {
			ResetForTesting(nil)
			type S struct {
				D time.Duration `flag:"d" default:"nonsense"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default duration"},
		{"uint", func() error {
			ResetForTesting(nil)
			type S struct {
				U uint `flag:"u" default:"bad"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default uint"},
		{"uint64", func() error {
			ResetForTesting(nil)
			type S struct {
				U uint64 `flag:"u" default:"bad"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default uint64"},
		{"float", func() error {
			ResetForTesting(nil)
			type S struct {
				F float64 `flag:"f" default:"bad"`
			}
			var s S
			return ParseStruct(&s)
		}, "invalid default float64"},
	}
	for _, c := range cases {
		if err := c.make(); err == nil || !containsSub(err.Error(), c.wantSub) {
			t.Fatalf("%s: expected error containing %q, got %v", c.name, c.wantSub, err)
		}
	}
}

// contains helper (simple subset to avoid importing strings here again inside package).
func containsSub(hay, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
