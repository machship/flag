package flag

import (
	"os"
	"regexp"
	"testing"
	"time"
)

// helper to reset and set args
func withArgsRaw(args []string, fn func()) {
	old := os.Args
	os.Args = append([]string{"cmd"}, args...)
	defer func() { os.Args = old }()
	fn()
}

func TestSensitiveMaskingAndSources(t *testing.T) {
	ResetForTesting(nil)
	type C struct {
		Password string `flag:"password" required:"true" sensitive:"true"`
		Token    string `flag:"token" default:"abc" sensitive:"true"`
		Mode     string `flag:"mode" default:"fast"`
	}
	var c C
	// provide token via env, password via cli
	os.Setenv("TOKEN", "envtoken")
	defer os.Unsetenv("TOKEN")
	withArgsRaw([]string{"-password", "supersecret", "-mode", "slow"}, func() {
		if err := ParseStruct(&c); err != nil {
			t.Fatalf("parse: %v", err)
		}
	})
	if c.Password != "supersecret" {
		t.Fatalf("expected password value set")
	}
	metas := Introspect()
	var pwMeta, tokenMeta, modeMeta FlagMeta
	for _, m := range metas {
		switch m.Name {
		case "password":
			pwMeta = m
		case "token":
			tokenMeta = m
		case "mode":
			modeMeta = m
		}
	}
	if pwMeta.Value != "******" || !pwMeta.Set || pwMeta.Source != "cli" || !pwMeta.Sensitive {
		t.Fatalf("unexpected pw meta: %+v", pwMeta)
	}
	if tokenMeta.Source != "env" || tokenMeta.Value != "******" || tokenMeta.Default != "******" {
		t.Fatalf("unexpected token meta: %+v", tokenMeta)
	}
	if modeMeta.Source != "cli" {
		t.Fatalf("expected mode source cli got %+v", modeMeta)
	}
}

func TestValidationTags_MinMaxPattern(t *testing.T) {
	ResetForTesting(nil)
	type C struct {
		Port int    `flag:"port" default:"10" min:"1" max:"20"`
		Name string `flag:"name" default:"svc1" pattern:"^[a-z0-9]+$"`
	}
	var c C
	withArgsRaw([]string{"-port", "15", "-name", "alpha1"}, func() {
		if err := ParseStruct(&c); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
	})
	// invalid: port too low, name fails pattern
	ResetForTesting(nil)
	var bad C
	withArgsRaw([]string{"-port", "0", "-name", "Alpha!"}, func() {
		err := ParseStruct(&bad)
		if err == nil {
			t.Fatalf("expected validation errors")
		}
		if !regexp.MustCompile(`value 0 < min`).MatchString(err.Error()) || !regexp.MustCompile(`does not match pattern`).MatchString(err.Error()) {
			t.Fatalf("unexpected combined error: %v", err)
		}
	})
}

func TestAutoParseFalseFlow(t *testing.T) {
	ResetForTesting(nil)
	type C struct {
		X int `flag:"x" default:"5" min:"3"`
	}
	var c C
	withArgsRaw([]string{"-x", "4"}, func() {
		if err := ParseStructWithOptions(&c, ParseStructOptions{AutoParse: false}); err != nil {
			t.Fatalf("register: %v", err)
		}
		// Not parsed yet
		if Parsed() {
			t.Fatalf("expected not parsed yet")
		}
		Parse()
		if err := Validate(); err != nil {
			t.Fatalf("validate: %v", err)
		}
	})
}

func TestNestedStruct(t *testing.T) {
	ResetForTesting(nil)
	type Inner struct {
		Rate time.Duration `flag:"rate" default:"2s"`
	}
	type Outer struct {
		Inner Inner
		Mode  string `flag:"mode" default:"x"`
	}
	var o Outer
	withArgsRaw([]string{"-mode", "y"}, func() {
		if err := ParseStruct(&o); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
	if o.Inner.Rate != 2*time.Second {
		t.Fatalf("expected nested default rate 2s got %v", o.Inner.Rate)
	}
	if o.Mode != "y" {
		t.Fatalf("mode y got %v", o.Mode)
	}
}
