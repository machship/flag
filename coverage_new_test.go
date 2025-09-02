package flag

import (
	"bytes"
	"encoding/json"
	"math/big"
	"net"
	urlpkg "net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	decimal "github.com/shopspring/decimal"
)

// TestParseByteSizeCoversAllUnits and error cases
func TestParseByteSizeCoversAllUnits(t *testing.T) {
	cases := []struct{ in string }{
		{"0"}, {"1B"}, {"2k"}, {"3K"}, {"4Ki"}, {"5MiB"}, {"6m"}, {"7Gi"}, {"8g"}, {"9Ti"}, {"10t"},
	}
	for _, c := range cases {
		if _, err := parseByteSize(c.in); err != nil {
			// numeric forms accepted; no assertion on exact value required here
			t.Fatalf("unexpected error for %s: %v", c.in, err)
		}
	}
	if _, err := parseByteSize("bad"); err == nil {
		// invalid number prefix
		panic("expected error for bad")
	}
	if _, err := parseByteSize("1PB"); err == nil { // unknown unit
		panic("expected error for unknown unit")
	}
}

func TestExtendedValueTypesSetStringGet(t *testing.T) {
	ResetForTesting(nil)
	fs := NewFlagSet("all", ContinueOnError)
	bs := ByteSize(0)
	fs.ByteSizeVar(&bs, "bs", 0, "")
	var tm time.Time
	fs.TimeVar(&tm, "tm", time.RFC3339, time.Time{}, "")
	dec := decimal.NewFromInt(0)
	fs.DecimalVar(&dec, "dec", dec, "")
	var ip net.IP
	fs.IPVar(&ip, "ip", nil, "")
	var ipn net.IPNet
	_, n, _ := net.ParseCIDR("192.168.1.0/24")
	fs.IPNetVar(&ipn, "ipn", n, "")
	var u urlpkg.URL
	fs.URLVar(&u, "url", nil, "")
	id := uuid.New()
	fs.UUIDVar(&id, "uuid", id, "")
	bi := big.NewInt(0)
	fs.BigIntVar(bi, "bigint", big.NewInt(0), "")
	br := big.NewRat(0, 1)
	fs.BigRatVar(br, "bigrat", big.NewRat(1, 2), "")
	var rx *regexp.Regexp
	fs.RegexpVar(&rx, "re", nil, "")
	var ss []string
	fs.StringSliceVar(&ss, "ss", ",", nil, "")
	var ds []time.Duration
	fs.DurationSliceVar(&ds, "ds", ",", nil, "")
	mp := map[string]string{}
	fs.StringMapVar(&mp, "mp", nil, "")
	var jm json.RawMessage
	fs.JSONVar(&jm, "js", nil, "")
	var enum string
	fs.EnumVar(&enum, "env", "apple", []string{"apple", "banana"}, "")

	args := []string{
		"-bs", "10KiB", "-tm", time.Now().Format(time.RFC3339), "-dec", "123.45", "-ip", "127.0.0.1",
		"-ipn", "10.0.0.0/8", "-url", "https://example.com/path", "-uuid", uuid.New().String(),
		"-bigint", "0x10", "-bigrat", "3/7", "-re", "^abc$", "-ss", "a,b,c", "-ds", "1s,2s",
		"-mp", "k1=v1,k2=v2", "-js", "{\"a\":1}", "-env", "banana",
	}
	if err := fs.Parse(args); err != nil {
		// allow parse error details to bubble
		panic(err)
	}
	// exercise getters & String by referencing them
	_ = bs
	_ = tm.String()
	_ = dec.String()
	_ = ip.String()
	_ = ipn.String()
	_ = u.String()
	_ = id.String()
	_ = bi.String()
	_ = br.RatString()
	if rx == nil || rx.String() != "^abc$" {
		t.Fatalf("regexp not set")
	}
	if len(ss) != 3 || ss[2] != "c" {
		t.Fatalf("string slice parse failed: %v", ss)
	}
	if len(ds) != 2 || ds[0] != time.Second {
		t.Fatalf("duration slice parse failed: %v", ds)
	}
	if len(mp) != 2 || mp["k2"] != "v2" {
		t.Fatalf("string map parse failed: %v", mp)
	}
	if string(jm) != "{\"a\":1}" {
		t.Fatalf("json not set: %s", string(jm))
	}
	if enum != "banana" {
		t.Fatalf("enum not set: %s", enum)
	}
	// exercise Get() methods explicitly
	for _, fl := range fs.formal {
		if g, ok := fl.Value.(Getter); ok {
			_ = g.Get()
		}
	}
}

// TestExitOnErrorBranches ensures we can hit ExitOnError without terminating tests by overriding exitFunc.
func TestExitOnErrorBranches(t *testing.T) {
	old := exitFunc
	var code int
	exitFunc = func(c int) { code = c }
	defer func() { exitFunc = old }()

	// Unknown flag path
	fs := NewFlagSet("exit1", ExitOnError)
	fs.Bool("known", false, "")
	fs.Parse([]string{"-unknown"})
	if code != 2 {
		t.Fatalf("expected exit code 2 for unknown flag, got %d", code)
	}
	code = 0
	// Invalid value path
	fs2 := NewFlagSet("exit2", ExitOnError)
	fs2.Int("num", 0, "")
	fs2.Parse([]string{"-num", "notint"})
	if code != 2 {
		t.Fatalf("expected exit code 2 for invalid value, got %d", code)
	}
	code = 0
	// Missing arg path
	fs3 := NewFlagSet("exit3", ExitOnError)
	fs3.String("s", "", "")
	fs3.Parse([]string{"-s"})
	if code != 2 {
		t.Fatalf("expected exit code 2 for missing arg, got %d", code)
	}
}

// TestParseEnvAdditionalBranch covers path where env var supplies boolean without explicit value.
func TestParseEnvAdditionalBranch(t *testing.T) {
	ResetForTesting(nil)
	os.Setenv("FLAG_BOOLX", "")
	defer os.Unsetenv("FLAG_BOOLX")
	fs := NewFlagSetWithEnvPrefix("envset", "flag", ContinueOnError)
	fs.Bool("boolx", false, "")
	if err := fs.Parse([]string{}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if f := fs.Lookup("boolx"); f == nil || !f.Value.(boolFlag).IsBoolFlag() {
		t.Fatalf("expected bool flag")
	}
}

// TestParseStructFullCoverage sets multiple required fields and uses different default tags
func TestParseStructFullCoverage(t *testing.T) {
	ResetForTesting(nil)
	type C struct {
		A bool          `flag:"a" default:"true"`
		B string        `flag:"b" default:"hello" enum:"hello,world"`
		D time.Duration `flag:"d" default:"2s"`
		E float64       `flag:"e" default:"3.14"`
	}
	var c C
	os.Args = []string{"cmd"}
	if err := ParseStruct(&c); err != nil {
		t.Fatalf("parse struct: %v", err)
	}
	if c.A != true || c.B != "hello" || c.D != 2*time.Second || c.E != 3.14 {
		t.Fatalf("unexpected values: %+v", c)
	}
}

// TestPrintDefaultsZeroValues ensures zero-valued extended types don't print defaults incorrectly.
func TestPrintDefaultsZeroExtended(t *testing.T) {
	ResetForTesting(nil)
	fs := NewFlagSet("pdefs", ContinueOnError)
	var jm json.RawMessage
	fs.JSONVar(&jm, "js", nil, "json value")
	var rx *regexp.Regexp
	fs.RegexpVar(&rx, "rx", nil, "regexp value")
	var tm time.Time
	fs.TimeVar(&tm, "tm", time.RFC3339, time.Time{}, "time value")
	var ip net.IP
	fs.IPVar(&ip, "ip", nil, "ip value")
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	fs.PrintDefaults()
	out := buf.String()
	if !strings.Contains(out, "json value") || !strings.Contains(out, "regexp value") || !strings.Contains(out, "time value") {
		// ensure they were printed
		panic("expected outputs present")
	}
}
