package flag_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	. "github.com/machship/flag"
)

// customValue implements flag.Value for testing top-level Var.
type customValue struct{ s *string }

func (c *customValue) String() string {
	if c.s == nil {
		return ""
	}
	return *c.s
}
func (c *customValue) Set(v string) error {
	if c.s == nil {
		c.s = new(string)
	}
	*c.s = v
	return nil
}

// simpleValue used in TestVarOnFlagSetDirect
type simpleValue struct{ ptr *string }

func (v *simpleValue) String() string {
	if v.ptr == nil {
		return ""
	}
	return *v.ptr
}
func (v *simpleValue) Set(val string) error {
	if v.ptr == nil {
		v.ptr = new(string)
	}
	*v.ptr = val
	return nil
}

func TestLookupAndTopLevelFunctions(t *testing.T) {
	ResetForTesting(nil)
	// Before defining
	if Lookup("missing") != nil {
		t.Fatalf("expected nil lookup for missing flag")
	}

	// Define a few flags with different zero defaults to exercise isZeroValue cases via PrintDefaults
	Bool("b", false, "bool flag")
	Int("i", 0, "int flag")
	String("s", "", "string flag")
	Uint("u", 5, "uint flag non-zero") // non-zero ensures (default X) path

	// top-level UintVar coverage (redefine new var)
	var uv uint
	UintVar(&uv, "uv", 0, "top level uint var")

	// custom value with top-level Var
	var custom string
	cv := &customValue{&custom}
	Var(cv, "custom", "custom value")
	if err := Set("custom", "hello"); err != nil {
		t.Fatalf("Set custom failed: %v", err)
	}
	if custom != "hello" {
		t.Fatalf("expected custom=hello got %s", custom)
	}

	// Lookup after defining
	if f := Lookup("b"); f == nil || f.Name != "b" {
		t.Fatalf("expected to find flag b")
	}

	// NFlag before any set
	if NFlag() != 1 { // custom set above via Set
		// Actually we've set one flag (custom) so expect 1
		t.Fatalf("expected NFlag()==1 got %d", NFlag())
	}

	// Use Set to set another flag
	if err := Set("b", "true"); err != nil {
		t.Fatalf("Set b failed: %v", err)
	}

	if NFlag() != 2 {
		t.Fatalf("expected NFlag()==2 got %d", NFlag())
	}

	// Capture PrintDefaults output (global)
	var buf bytes.Buffer
	CommandLine.SetOutput(&buf)
	PrintDefaults()
	out := buf.String()
	// Ensure zero-value defaults suppressed and non-zero included
	if strings.Contains(out, "(default false)") {
		t.Errorf("did not expect default false to appear: %s", out)
	}
	if strings.Contains(out, "(default 0)") {
		t.Errorf("did not expect default 0 to appear: %s", out)
	}
	if strings.Contains(out, "(default \"\")") {
		t.Errorf("did not expect default empty string to appear: %s", out)
	}
	if !strings.Contains(out, "(default 5)") {
		t.Errorf("expected non-zero uint default to appear: %s", out)
	}
}

func TestFlagSetArgNArgNFlag(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	b := fs.Bool("b", false, "")
	if fs.NFlag() != 0 {
		t.Fatalf("expected 0 flags set initially")
	}
	// Provide args with one positional
	if err := fs.Parse([]string{"-b", "--", "positional"}); err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !*b {
		t.Fatalf("expected b set true")
	}
	if fs.NFlag() != 1 {
		t.Fatalf("expected NFlag 1 got %d", fs.NFlag())
	}
	if fs.Arg(-1) != "" {
		t.Errorf("expected empty for negative Arg")
	}
	if fs.Arg(1) != "" {
		t.Errorf("expected empty for out of range Arg")
	}
	if fs.NArg() != 1 {
		t.Fatalf("expected 1 positional arg got %d", fs.NArg())
	}
	// Do not assert global Arg here; we're using a custom FlagSet
}

// panicVal used for duplicate registration test
type panicVal struct{ v string }

func (p *panicVal) String() string     { return p.v }
func (p *panicVal) Set(s string) error { p.v = s; return nil }

func TestVarRedefinitionPanic(t *testing.T) {
	var recovered bool
	fs := NewFlagSet("p", ContinueOnError)
	// Silence output to avoid clutter
	fs.SetOutput(&bytes.Buffer{})
	fs.Var(&panicVal{v: "a"}, "dup", "help")
	func() {
		defer func() {
			if r := recover(); r != nil {
				recovered = true
			}
		}()
		fs.Var(&panicVal{v: "b"}, "dup", "help") // should panic
	}()
	if !recovered {
		t.Fatalf("expected panic on duplicate Var definition")
	}
}

func TestNewFlagSetWithEnvPrefix(t *testing.T) {
	os.Setenv("APP_PORT", "7777")
	defer os.Unsetenv("APP_PORT")
	fs := NewFlagSetWithEnvPrefix("app", "APP", ContinueOnError)
	p := fs.Int("port", 0, "port")
	if err := fs.ParseEnv(os.Environ()); err != nil {
		t.Fatalf("ParseEnv failed: %v", err)
	}
	if *p != 7777 {
		t.Fatalf("expected env-applied port 7777 got %d", *p)
	}
}

func TestFlagSetLookupAndTopLevelLookup(t *testing.T) {
	ResetForTesting(nil)
	fs := NewFlagSet("fs", ContinueOnError)
	fs.Bool("verbose", false, "")
	if fs.Lookup("verbose") == nil {
		t.Fatalf("expected fs.Lookup to find flag")
	}
	if Lookup("verbose") != nil {
		t.Fatalf("global Lookup should not see fs flag")
	}
	Bool("verbose", false, "")
	if Lookup("verbose") == nil {
		t.Fatalf("expected global Lookup to find flag after definition")
	}
}

func TestDefaultUsagePath(t *testing.T) {
	fs := NewFlagSet("usage", ContinueOnError)
	fs.Bool("ok", false, "")
	// capture output to force defaultUsage via parse error (-unknown)
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	_ = fs.Parse([]string{"-unknown"}) // cause error
	if !strings.Contains(buf.String(), "unknown") {
		t.Fatalf("expected usage output to mention unknown flag; got %q", buf.String())
	}
}

func TestTopLevelNArgAndArgs(t *testing.T) {
	ResetForTesting(nil)
	// define a dummy flag to avoid help panic
	Bool("x", false, "")
	// capture and ignore output
	var buf bytes.Buffer
	CommandLine.SetOutput(&buf)
	os.Args = []string{"cmd", "-x", "true", "pos1", "pos2"}
	Parse()
	// For boolean flags without = form, the next arg is NOT consumed as the value; thus "true" becomes positional.
	if NArg() != 3 {
		t.Fatalf("expected 3 positional args got %d", NArg())
	}
	if len(Args()) != 3 {
		t.Fatalf("expected Args length 3")
	}
}

func TestIsZeroValueBranches(t *testing.T) {
	ResetForTesting(nil)
	// create flags covering zero values of types
	Bool("zb", false, "")
	Int("zi", 0, "")
	String("zs", "", "")
	// capture PrintDefaults and ensure no default lines appear for zero values
	var buf bytes.Buffer
	CommandLine.SetOutput(&buf)
	PrintDefaults()
	out := buf.String()
	if strings.Contains(out, "default false") || strings.Contains(out, "default 0") || strings.Contains(out, "default \"\"") {
		t.Fatalf("unexpected default annotations in zero values output: %s", out)
	}
}

func TestCommandLinePrintDefaultsFunctionCoverage(t *testing.T) {
	ResetForTesting(nil)
	Bool("cov", true, "boolean defaulting to true")
	// ensure top-level PrintDefaults path executed again with output
	var buf bytes.Buffer
	CommandLine.SetOutput(&buf)
	PrintDefaults()
	if !strings.Contains(buf.String(), "cov") {
		t.Fatalf("expected to see 'cov' in PrintDefaults output")
	}
}

func TestFailfAndUsageDirectly(t *testing.T) {
	fs := NewFlagSet("failset", ContinueOnError)
	var out bytes.Buffer
	fs.SetOutput(&out)
	// Trigger failf via providing unknown flag
	_ = fs.Parse([]string{"-unknown"})
	if !strings.Contains(out.String(), "flag provided but not defined") {
		t.Fatalf("expected failf output, got %q", out.String())
	}
}

// Ensure Var path for CommandLine.Var (top-level Var already used) plus Args helper
func TestVarOnFlagSetDirect(t *testing.T) {
	fs := NewFlagSet("varset", ContinueOnError)
	var s string
	fs.Var(&simpleValue{&s}, "simp", "")
	if err := fs.Parse([]string{"-simp", "val"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if s != "val" {
		t.Fatalf("expected val, got %s", s)
	}
}

// cover SetOutput nil path (out() fallback to stderr) by passing nil
func TestSetOutputNil(t *testing.T) {
	var fs FlagSet
	fs.Init("nil", ContinueOnError)
	fs.SetOutput(nil) // will fall back internally
	// define a flag and call PrintDefaults; we can't easily assert stderr here so just ensure no panic
	fs.Bool("x", false, "")
	fs.PrintDefaults()
}

// cover environment boolean empty value path (treat as true)
func TestParseEnvEmptyBoolean(t *testing.T) {
	ResetForTesting(nil)
	os.Setenv("EMPTYBOOL", "")
	defer os.Unsetenv("EMPTYBOOL")
	b := Bool("emptybool", false, "")
	if err := CommandLine.ParseEnv(os.Environ()); err != nil {
		t.Fatalf("ParseEnv error: %v", err)
	}
	if !*b {
		t.Fatalf("expected empty bool env to set flag true")
	}
}

// cover ParseFile boolean without value (treated as true)
func TestParseFileBooleanNoValue(t *testing.T) {
	// create temp file
	f, err := os.CreateTemp("", "flagfile-*.conf")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	defer os.Remove(f.Name())
	_, _ = io.WriteString(f, "boolflag\n")
	_ = f.Close()
	fs := NewFlagSet("filebool", ContinueOnError)
	bf := fs.Bool("boolflag", false, "")
	if err := fs.ParseFile(f.Name()); err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if !*bf {
		t.Fatalf("expected boolflag true")
	}
}
