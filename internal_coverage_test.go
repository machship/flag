package flag

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// errorValue implements Value and always returns an error from Set to hit FlagSet.Set error branch.
type errorValue struct{}

func (e errorValue) String() string   { return "" }
func (e errorValue) Set(string) error { return errors.New("set error") }

// nonPtrValue implements Value with a non-pointer concrete type to exercise isZeroValue non-pointer allocation path.
type nonPtrValue struct{}

func (n nonPtrValue) String() string   { return "" }
func (n nonPtrValue) Set(string) error { return nil }

// errBoolFlag implements boolFlag whose Set returns an error when set to "true" to trigger parseOne boolean Set error branch.
type errBoolFlag struct{}

func (e *errBoolFlag) String() string { return "false" }
func (e *errBoolFlag) Set(s string) error {
	if s == "true" {
		return errors.New("boom")
	}
	return nil
}
func (e *errBoolFlag) IsBoolFlag() bool { return true }

// singleBacktickUsage exercises the UnquoteUsage path where only one backtick exists causing a break.
func TestUnquoteUsageSingleBacktick(t *testing.T) {
	iv := intValue(0)
	f := &Flag{Name: "x", Usage: "some `unterminated", Value: &iv, DefValue: "0"}
	name, usage := UnquoteUsage(f)
	if name == "x" { // name should be inferred type "int" not flag name
		t.Fatalf("unexpected inferred name equals flag name")
	}
	if !strings.Contains(usage, "unterminated") {
		t.Fatalf("expected usage to contain original text, got %q", usage)
	}
}

func TestUnquoteUsageDurationType(t *testing.T) {
	d := newDurationValue(0, new(time.Duration))
	f := &Flag{Name: "d", Usage: "duration flag", Value: d, DefValue: d.String()}
	name, _ := UnquoteUsage(f)
	if name != "duration" {
		t.Fatalf("expected inferred duration name, got %q", name)
	}
}

func TestFlagSetSetErrorPath(t *testing.T) {
	var fs FlagSet
	fs.Init("seterr", ContinueOnError)
	fs.Var(errorValue{}, "bad", "")
	if err := fs.Set("bad", "x"); err == nil || !strings.Contains(err.Error(), "set error") {
		t.Fatalf("expected set error, got %v", err)
	}
}

func TestIsZeroValueNonPointerAndSwitchCases(t *testing.T) {
	// Register a non-pointer Value to trigger non-pointer branch inside isZeroValue.
	var fs FlagSet
	fs.Init("np", ContinueOnError)
	fs.Var(nonPtrValue{}, "npv", "non pointer value")
	// Capture PrintDefaults to drive isZeroValue; should not show default annotation.
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	fs.PrintDefaults()
	out := buf.String()
	if strings.Contains(out, "default") { // zero value so no default expected
		t.Fatalf("did not expect default annotation in %q", out)
	}
}

func TestVarRedefinitionPanicUnnamedFlagSet(t *testing.T) {
	// Unnamed FlagSet (zero value + Init with empty name) to cover panic message when f.name=="".
	var fs FlagSet
	fs.Init("", ContinueOnError)
	fs.Var(nonPtrValue{}, "dup", "")
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic redefining flag on unnamed set")
		}
	}()
	fs.Var(nonPtrValue{}, "dup", "")
}

func TestParseOneInvalidBooleanValue(t *testing.T) {
	fs := NewFlagSet("boolerr", ContinueOnError)
	fs.Bool("b", false, "")
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	if err := fs.Parse([]string{"-b=notbool"}); err == nil {
		t.Fatalf("expected error for invalid boolean value")
	}
	if !strings.Contains(buf.String(), "invalid boolean value") {
		t.Fatalf("expected invalid boolean value message")
	}
}

func TestParseOneBooleanSetError(t *testing.T) {
	fs := NewFlagSet("boolseterr", ContinueOnError)
	fs.Var(&errBoolFlag{}, "eb", "")
	fs.SetOutput(&bytes.Buffer{})
	if err := fs.Parse([]string{"-eb"}); err == nil || !strings.Contains(err.Error(), "invalid boolean flag") {
		t.Fatalf("expected invalid boolean flag error, got %v", err)
	}
}

func TestParseStructInvalidDefaultsOthers(t *testing.T) {
	ResetForTesting(nil)
	type C struct {
		D   time.Duration `flag:"d" default:"notdur"`
		I64 int64         `flag:"i64" default:"notint64"`
		U   uint          `flag:"u" default:"notuint"`
		U64 uint64        `flag:"u64" default:"notuint64"`
		F   float64       `flag:"f" default:"notfloat"`
	}
	var c C
	os.Args = []string{"cmd"}
	err := ParseStruct(&c)
	if err == nil || !strings.Contains(err.Error(), "invalid default") {
		t.Fatalf("expected invalid default error, got %v", err)
	}
}

func TestParseStructRequiredNumericsDuration(t *testing.T) {
	ResetForTesting(nil)
	type C struct {
		A  int           `flag:"a" required:"true"`
		B  int64         `flag:"b" required:"true"`
		Cc uint          `flag:"c" required:"true"`
		D  uint64        `flag:"d" required:"true"`
		E  float64       `flag:"e" required:"true"`
		T  time.Duration `flag:"t" required:"true"`
	}
	var c C
	os.Args = []string{"cmd"}
	err := ParseStruct(&c)
	if err == nil {
		t.Fatalf("expected missing required flags error")
	}
	// ensure all names present
	for _, name := range []string{"a", "b", "c", "d", "e", "t"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("expected error to contain %s, got %v", name, err)
		}
	}
}

func TestParseFileBlankAndCommentAndInvalidBool(t *testing.T) {
	fs := NewFlagSet("fileExtras", ContinueOnError)
	fs.Bool("b", false, "")
	tmp := filepath.Join(os.TempDir(), "file_extras.conf")
	content := "\n#comment line\nb=notbool\n" // leading blank, comment, then invalid boolean assignment
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	defer os.Remove(tmp)
	fs.SetOutput(&bytes.Buffer{})
	if err := fs.ParseFile(tmp); err == nil || !strings.Contains(err.Error(), "invalid boolean value") {
		t.Fatalf("expected invalid boolean value error, got %v", err)
	}
}

func TestParseFileHelpErr(t *testing.T) {
	fs := NewFlagSet("fileHelp", ContinueOnError)
	tmp := filepath.Join(os.TempDir(), "file_help.conf")
	if err := os.WriteFile(tmp, []byte("help\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	defer os.Remove(tmp)
	fs.SetOutput(&bytes.Buffer{})
	if err := fs.ParseFile(tmp); err != ErrHelp {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
}

func TestParseFileScannerError(t *testing.T) {
	fs := NewFlagSet("fileScanErr", ContinueOnError)
	tmp := filepath.Join(os.TempDir(), "file_scan_err.conf")
	// create an overly long line >64K to trigger bufio.Scanner token too long error
	big := strings.Repeat("a", 70_000)
	if err := os.WriteFile(tmp, []byte(big), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	defer os.Remove(tmp)
	if err := fs.ParseFile(tmp); err == nil {
		t.Fatalf("expected scanner error, got nil")
	}
}
