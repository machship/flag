package flag

import (
	"os"
	"path/filepath"
	"testing"
)

// Test precedence: CLI > Env > SecretDir > Config (secret dir parsed after env)
func TestParse_SecretDirIntegrationPrecedence(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "username"), []byte("secretUser\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "password"), []byte("secretPass"), 0600); err != nil {
		t.Fatal(err)
	}

	// 1. No env, expect secret dir values
	{
		fs := NewFlagSet("test1", ContinueOnError)
		var user, pass string
		fs.StringVar(&user, "username", "", "user")
		fs.StringVar(&pass, "password", "", "pass")
		fs.String(DefaultSecretDirFlagname, "", "secret directory")
		if err := fs.Parse([]string{"-" + DefaultSecretDirFlagname, dir}); err != nil {
			t.Fatalf("parse (stage1) error: %v", err)
		}
		if user != "secretUser" {
			t.Fatalf("stage1 expected username 'secretUser', got %q", user)
		}
		if pass != "secretPass" {
			t.Fatalf("stage1 expected password 'secretPass', got %q", pass)
		}
	}

	// 2. Env set should override secret dir
	if err := os.Setenv("USERNAME", "envUser"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("PASSWORD", "envPass"); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("USERNAME")
	defer os.Unsetenv("PASSWORD")
	{
		fs := NewFlagSet("test2", ContinueOnError)
		var user, pass string
		fs.StringVar(&user, "username", "", "user")
		fs.StringVar(&pass, "password", "", "pass")
		fs.String(DefaultSecretDirFlagname, "", "secret directory")
		if err := fs.Parse([]string{"-" + DefaultSecretDirFlagname, dir}); err != nil {
			t.Fatalf("parse (stage2) error: %v", err)
		}
		if user != "envUser" {
			t.Fatalf("stage2 expected username from env 'envUser', got %q", user)
		}
		if pass != "envPass" {
			t.Fatalf("stage2 expected password from env 'envPass', got %q", pass)
		}
	}

	// 3. CLI overrides both env and secret dir
	{
		fs := NewFlagSet("test3", ContinueOnError)
		var user, pass string
		fs.StringVar(&user, "username", "", "user")
		fs.StringVar(&pass, "password", "", "pass")
		fs.String(DefaultSecretDirFlagname, "", "secret directory")
		if err := fs.Parse([]string{"-username", "cliUser", "-password", "cliPass", "-" + DefaultSecretDirFlagname, dir}); err != nil {
			t.Fatalf("parse (stage3) error: %v", err)
		}
		if user != "cliUser" {
			t.Fatalf("stage3 expected username 'cliUser', got %q", user)
		}
		if pass != "cliPass" {
			t.Fatalf("stage3 expected password 'cliPass', got %q", pass)
		}
	}
}
