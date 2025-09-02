package flag_test

import (
	"bytes"
	"os"
	"testing"
	"time"

	. "github.com/machship/flag"
)

// custom value whose zero string is "0" to exercise isZeroValue additional branch
type zeroStringVal struct{ v string }

func (z *zeroStringVal) String() string     { return z.v }
func (z *zeroStringVal) Set(s string) error { z.v = s; return nil }

func TestSetUnknownFlag(t *testing.T) {
	fs := NewFlagSet("unknown-set", ContinueOnError)
	if err := fs.Set("nope", "1"); err == nil {
		t.Fatalf("expected error setting unknown flag")
	}
}

func TestPrintDefaultsStringQuotingAndZeroDetection(t *testing.T) {
	ResetForTesting(nil)
	// Non-zero string default should appear quoted; zero defaults suppressed
	Bool("zb", false, "zero bool")
	Int("zi", 0, "zero int")
	String("name", "alice", "user `name`") // backquoted param for UnquoteUsage
	// custom value to test isZeroValue fallback path (String() returns "0")
	z := &zeroStringVal{v: "0"}
	Var(z, "customzero", "custom value with 0 default")
	var buf bytes.Buffer
	CommandLine.SetOutput(&buf)
	PrintDefaults()
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("(default \"alice\")")) {
		t.Fatalf("expected quoted string default in output: %s", out)
	}
	if bytes.Contains([]byte(out), []byte("zero bool (default")) {
		t.Fatalf("did not expect zero bool default annotation: %s", out)
	}
	if bytes.Contains([]byte(out), []byte("zero int (default")) {
		t.Fatalf("did not expect zero int default annotation: %s", out)
	}
}

func TestDefaultUsageUnnamedAndNamed(t *testing.T) {
	ResetForTesting(nil)
	// Provide no-op usage to avoid panic and noise
	Usage = func() {}
	// Unnamed set (zero value FlagSet) triggers generic Usage header
	var fs FlagSet
	fs.Init("", ContinueOnError)
	var buf1 bytes.Buffer
	fs.SetOutput(&buf1)
	fs.Bool("a", false, "help")
	// Cause error to invoke usage
	_ = fs.Parse([]string{"-unknown"})
	if got := buf1.String(); got == "" || !bytes.Contains([]byte(got), []byte("Usage:")) {
		t.Fatalf("expected generic Usage header, got %q", got)
	}

	// Named set
	fs2 := NewFlagSet("mytool", ContinueOnError)
	var buf2 bytes.Buffer
	fs2.SetOutput(&buf2)
	fs2.Bool("b", false, "help")
	_ = fs2.Parse([]string{"-unknown"})
	if got := buf2.String(); got == "" || !bytes.Contains([]byte(got), []byte("Usage of mytool:")) {
		t.Fatalf("expected named usage header, got %q", got)
	}
}

func TestParseTerminatorAndBadSyntax(t *testing.T) {
	fs := NewFlagSet("term", ContinueOnError)
	fs.Bool("v", false, "verbosity")
	if err := fs.Parse([]string{"--", "arg1", "arg2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs.NArg() != 2 {
		t.Fatalf("expected 2 args after terminator, got %d", fs.NArg())
	}
	// Now test bad flag syntax: "-="
	fs2 := NewFlagSet("bad", ContinueOnError)
	var out bytes.Buffer
	fs2.SetOutput(&out)
	if err := fs2.Parse([]string{"-="}); err == nil {
		t.Fatalf("expected error for bad syntax")
	}
}

func TestParseStructAdditionalErrors(t *testing.T) {
	ResetForTesting(nil)
	// nil pointer
	if err := ParseStruct(nil); err == nil {
		t.Fatalf("expected error for nil pointer")
	}
	// non-struct pointer
	x := 5
	if err := ParseStruct(&x); err == nil {
		t.Fatalf("expected error for non-struct pointer")
	}
	// multiple required missing & invalid bool default
	type C struct {
		A string `flag:"a" required:"true"`
		B bool   `flag:"b" default:"notabool"`
		C int    `flag:"c" required:"true"`
	}
	var c C
	// This should error on invalid default before parsing
	if err := ParseStruct(&c); err == nil {
		t.Fatalf("expected invalid default bool error")
	}
}

func TestParseStructMultipleMissing(t *testing.T) {
	ResetForTesting(nil)
	type C struct {
		A string `flag:"a" required:"true"`
		C string `flag:"c" required:"true"`
	}
	var c C
	// Provide custom args (none) and avoid other global flags interfering
	os.Args = []string{"cmd"}
	err := ParseStruct(&c)
	if err == nil {
		t.Fatalf("expected missing required flags a and c; got nil")
	}
	if !containsAll(err.Error(), []string{"a", "c"}) {
		t.Fatalf("error should mention both a and c, got %v", err)
	}
}

func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !bytes.Contains([]byte(s), []byte(sub)) {
			return false
		}
	}
	return true
}

func TestParseEnvAlreadySetSkip(t *testing.T) {
	// ensure branch where env value exists but flag already set is exercised
	ResetForTesting(nil)
	os.Setenv("ALREADY", "fromenv")
	defer os.Unsetenv("ALREADY")
	fs := NewFlagSet("skip", ContinueOnError)
	f := fs.String("already", "", "")
	// set via args first
	if err := fs.Parse([]string{"-already", "cli"}); err != nil {
		t.Fatalf("parse err: %v", err)
	}
	if *f != "cli" {
		t.Fatalf("expected cli, got %s", *f)
	}
}

func TestParseFileSkipWhenAlreadySet(t *testing.T) {
	// create temp file with value override
	ResetForTesting(nil)
	f, err := os.CreateTemp("", "skipfile-*.conf")
	if err != nil {
		t.Fatalf("tmp: %v", err)
	}
	defer os.Remove(f.Name())
	_, _ = f.WriteString("name=fromfile\n")
	_ = f.Close()
	fs := NewFlagSet("skipfile", ContinueOnError)
	name := fs.String("name", "", "")
	// set from args first
	if err := fs.Parse([]string{"-name", "cli"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	// manually call ParseFile to attempt override
	if err := fs.ParseFile(f.Name()); err != nil {
		t.Fatalf("parse file: %v", err)
	}
	if *name != "cli" {
		t.Fatalf("expected name to remain cli, got %s", *name)
	}
}

func TestParseConfigFileFlagSetButEnvError(t *testing.T) {
	// Create a config file and invalid env to force env error before file parse executed
	ResetForTesting(nil)
	Usage = func() {}
	// config flag defined with existing file
	cf, err := os.CreateTemp("", "cfg-*.conf")
	if err != nil {
		t.Fatalf("tmp: %v", err)
	}
	defer os.Remove(cf.Name())
	cf.WriteString("num=1\n")
	cf.Close()
	Int("num", 0, "")
	String(DefaultConfigFlagname, cf.Name(), "cfg")
	os.Setenv("NUM", "bad")
	defer os.Unsetenv("NUM")
	// Parse should surface env error, not proceed to config override
	if err := CommandLine.Parse([]string{}); err == nil {
		t.Fatalf("expected env parse error")
	}
}

func TestUnquoteUsageBackquotedAndBool(t *testing.T) {
	fs := NewFlagSet("u", ContinueOnError)
	b := fs.Bool("flag", false, "some `thing` here")
	name, usage := UnquoteUsage(fs.Lookup("flag"))
	if name != "thing" || usage != "some thing here" {
		t.Fatalf("unexpected unquote result: %s / %s", name, usage)
	}
	if *b {
		t.Fatalf("expected default false")
	}
	// bool path name="" when no backquotes
	fs2 := NewFlagSet("u2", ContinueOnError)
	b2 := fs2.Bool("flag2", false, "plain usage")
	name2, _ := UnquoteUsage(fs2.Lookup("flag2"))
	if name2 != "" || *b2 {
		t.Fatalf("expected empty name for bool flag with no backquotes")
	}
}

func TestParseOneBadFlagNameLeadingDash(t *testing.T) {
	fs := NewFlagSet("dash", ContinueOnError)
	fs.SetOutput(&bytes.Buffer{})
	// Provide invalid flag "--=value" which should error bad flag syntax
	if err := fs.Parse([]string{"--=value"}); err == nil {
		t.Fatalf("expected bad flag syntax error")
	}
}

func TestParseOneUnknownNonHelp(t *testing.T) {
	fs := NewFlagSet("unknown", ContinueOnError)
	fs.SetOutput(&bytes.Buffer{})
	if err := fs.Parse([]string{"-notdefined"}); err == nil {
		t.Fatalf("expected unknown flag error")
	}
}

func TestParseStructDurationAndUintDefaults(t *testing.T) {
	ResetForTesting(nil)
	type C struct {
		T time.Duration `flag:"timeout" default:"3s"`
		U uint64        `flag:"limit" default:"9"`
	}
	var c C
	if err := ParseStruct(&c); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if c.T != 3*time.Second || c.U != 9 {
		t.Fatalf("expected parsed defaults 3s & 9; got %v %d", c.T, c.U)
	}
}

func TestParseEnvHelpFlag(t *testing.T) {
	fs := NewFlagSet("help", ContinueOnError)
	// no flags defined; env contains HELP variable -> should not match (needs exact flag name); instead we set HELP for -help special case
	os.Setenv("HELP", "1")
	defer os.Unsetenv("HELP")
	if err := fs.ParseEnv(os.Environ()); err != nil && err != ErrHelp {
		t.Fatalf("expected ErrHelp or nil, got %v", err)
	}
}

func TestParseEnvInvalidBool(t *testing.T) {
	fs := NewFlagSet("envbool", ContinueOnError)
	fs.Bool("badbool", false, "")
	os.Setenv("BADBOOL", "notbool")
	defer os.Unsetenv("BADBOOL")
	err := fs.ParseEnv(os.Environ())
	if err == nil {
		t.Fatalf("expected error for invalid boolean env")
	}
}

func TestParseTerminatorWithFollowingFlagsIgnored(t *testing.T) {
	fs := NewFlagSet("term2", ContinueOnError)
	fs.SetOutput(&bytes.Buffer{})
	fs.Bool("v", false, "verbosity")
	fs.Int("n", 0, "number")
	if err := fs.Parse([]string{"--", "-v", "-n", "5"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs.NArg() != 3 {
		t.Fatalf("expected 3 positional args got %d", fs.NArg())
	}
}

func TestConfigFlagMissingFileAfterDefinition(t *testing.T) {
	ResetForTesting(nil)
	Usage = func() {}
	String(DefaultConfigFlagname, "./no_such_file_xyz", "cfg")
	if err := CommandLine.Parse([]string{}); err == nil {
		t.Fatalf("expected missing config file error")
	}
}
