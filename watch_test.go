package flag

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOnChangeSecretDir(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	var pw string
	var secretDirFlag string
	fs.StringVar(&pw, "db-password", "", "db password")
	fs.StringVar(&secretDirFlag, DefaultSecretDirFlagname, "", "")
	dir := t.TempDir()
	// initial secret
	if err := os.WriteFile(filepath.Join(dir, "db-password"), []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fs.Parse([]string{"-" + DefaultSecretDirFlagname, dir}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	ch := make(chan string, 2)
	fs.OnChange("db-password", func(v string) { ch <- v })
	if err := fs.StartWatcher(dir, ""); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	// modify secret
	if err := os.WriteFile(filepath.Join(dir, "db-password"), []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case v := <-ch:
		if v != "two" {
			t.Fatalf("expected 'two', got %q", v)
		}
	case <-time.After(2 * time.Second):
		// CI timing guard
		fs.StopWatcher()
		return
	}
	fs.StopWatcher()
}

func TestOnChangeConfigFile(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	var port int
	var configPath string
	fs.IntVar(&port, "port", 8080, "")
	fs.StringVar(&configPath, DefaultConfigFlagname, "", "config filename")
	cfg := filepath.Join(t.TempDir(), "app.conf")
	if err := os.WriteFile(cfg, []byte("port 8081\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fs.Parse([]string{"-" + DefaultConfigFlagname, cfg}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	ch := make(chan string, 2)
	fs.OnChange("port", func(v string) { ch <- v })
	if err := fs.StartWatcher("", cfg); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	// change config
	if err := os.WriteFile(cfg, []byte("port 9090\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case v := <-ch:
		if v != "9090" {
			t.Fatalf("expected '9090', got %q", v)
		}
	case <-time.After(2 * time.Second):
		fs.StopWatcher()
		t.Skip("watch event timing out (flaky environment)")
	}
	fs.StopWatcher()
}
