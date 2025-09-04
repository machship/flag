package flag

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseSecretDirAndAtFile(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	var pass string
	var token string
	var debug bool
	var nested string
	var timeout time.Duration
	fs.StringVar(&pass, "db-password", "", "db pass")
	fs.StringVar(&token, "api-token", "", "api token")
	fs.BoolVar(&debug, "debug", false, "debug mode")
	fs.StringVar(&nested, "nested", "", "nested indirection")
	fs.DurationVar(&timeout, "timeout", 0, "timeout")

	dir := t.TempDir()
	// Simple secret
	if err := os.WriteFile(filepath.Join(dir, "db-password"), []byte("s3cr3t\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Underscore variant -> maps to api-token
	if err := os.WriteFile(filepath.Join(dir, "API_TOKEN"), []byte("tok123"), 0600); err != nil {
		t.Fatal(err)
	}
	// Boolean with empty content should set true
	if err := os.WriteFile(filepath.Join(dir, "debug"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	// Indirection via @file (create inner file)
	inner := filepath.Join(dir, "inner.txt")
	if err := os.WriteFile(inner, []byte("inner-value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested"), []byte("@"+inner), 0600); err != nil {
		t.Fatal(err)
	}
	// Duration secret
	if err := os.WriteFile(filepath.Join(dir, "timeout"), []byte("5s"), 0600); err != nil {
		t.Fatal(err)
	}

	// No flags set yet; parse secrets
	if err := fs.ParseSecretDir(dir); err != nil {
		t.Fatalf("ParseSecretDir error: %v", err)
	}

	if pass != "s3cr3t" {
		t.Fatalf("expected db-password 's3cr3t', got %q", pass)
	}
	if token != "tok123" {
		t.Fatalf("expected api-token 'tok123', got %q", token)
	}
	if !debug {
		t.Fatalf("expected debug true")
	}
	if nested != "inner-value" {
		t.Fatalf("expected nested expanded to 'inner-value', got %q", nested)
	}
	if timeout != 5*time.Second {
		t.Fatalf("expected timeout 5s got %v", timeout)
	}

	// Ensure precedence: set via CLI then secret dir should not override
	if err := fs.Set("db-password", "override"); err != nil {
		t.Fatal(err)
	}
	if err := fs.ParseSecretDir(dir); err != nil {
		t.Fatal(err)
	}
	if pass != "override" {
		t.Fatalf("secret dir overwrote existing value: %q", pass)
	}
}

func TestExpandAtFileEscapes(t *testing.T) {
	val, err := expandAtFile("plain")
	if err == nil || err == nil && val != "" {
		t.Fatalf("expected errNoAtExpansion for non @ value")
	}
	// Escaped @@
	res, err := expandAtFile("@@abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "@abc" {
		t.Fatalf("expected '@abc', got %q", res)
	}
}
