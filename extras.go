// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flag

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvironmentPrefix defines a string that will be implicitely prefixed to a
// flag name before looking it up in the environment variables.
var EnvironmentPrefix = ""

// ParseEnv parses flags from environment variables.
// Flags already set will be ignored.
func (f *FlagSet) ParseEnv(environ []string) error {

	m := f.formal

	env := make(map[string]string)
	for _, s := range environ {
		i := strings.Index(s, "=")
		if i < 1 {
			continue
		}
		env[s[0:i]] = s[i+1 : len(s)]
	}

	for _, flag := range m {
		name := flag.Name
		_, set := f.actual[name]
		if set {
			continue
		}

		flag, alreadythere := m[name]
		if !alreadythere {
			if name == "help" || name == "h" { // special case for nice help message.
				f.usage()
				return ErrHelp
			}
			return f.failf("environment variable provided but not defined: %s", name)
		}

		envKey := strings.ToUpper(flag.Name)
		if f.envPrefix != "" {
			envKey = f.envPrefix + "_" + envKey
		}
		envKey = strings.Replace(envKey, "-", "_", -1)

		value, isSet := env[envKey]
		if !isSet {
			continue
		}

		hasValue := false
		if len(value) > 0 {
			hasValue = true
		}

		if fv, ok := flag.Value.(boolFlag); ok && fv.IsBoolFlag() { // special case: doesn't need an arg
			if hasValue {
				if expanded, err := expandAtFile(value); err == nil {
					value = expanded
				} else if !errors.Is(err, errNoAtExpansion) {
					return f.failf("invalid value %q for environment variable %s: %v", value, name, err)
				}
				if err := fv.Set(value); err != nil {
					return f.failf("invalid boolean value %q for environment variable %s: %v", value, name, err)
				}
			} else {
				fv.Set("true")
			}
		} else {
			if expanded, err := expandAtFile(value); err == nil {
				value = expanded
			} else if !errors.Is(err, errNoAtExpansion) {
				return f.failf("invalid value %q for environment variable %s: %v", value, name, err)
			}
			if err := flag.Value.Set(value); err != nil {
				return f.failf("invalid value %q for environment variable %s: %v", value, name, err)
			}
		}

		// update f.actual
		if f.actual == nil {
			f.actual = make(map[string]*Flag)
		}
		f.actual[name] = flag

	}
	return nil
}

// NewFlagSetWithEnvPrefix returns a new empty flag set with the specified name,
// environment variable prefix, and error handling property.
func NewFlagSetWithEnvPrefix(name string, prefix string, errorHandling ErrorHandling) *FlagSet {
	f := NewFlagSet(name, errorHandling)
	f.envPrefix = prefix
	return f
}

// DefaultConfigFlagname defines the flag name of the optional config file
// path. Used to lookup and parse the config file when a default is set and
// available on disk.
var DefaultConfigFlagname = "config"

// DefaultSecretDirFlagname defines an optional flag name whose value, if set,
// points to a directory containing secret files (each filename = flag name or
// underscore variant). If present, it is processed after environment variables
// and before the config file.
var DefaultSecretDirFlagname = "secret-dir"

// ParseFile parses flags from the file in path.
// Same format as commandline argumens, newlines and lines beginning with a
// "#" charater are ignored. Flags already set will be ignored.
func (f *FlagSet) ParseFile(path string) error {

	// Extract arguments from file
	fp, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fp.Close()

	scanner := bufio.NewScanner(fp)
	for scanner.Scan() {
		line := scanner.Text()

		// Ignore empty lines
		if len(line) == 0 {
			continue
		}

		// Ignore comments
		if line[:1] == "#" {
			continue
		}

		// Match `key=value` and `key value`
		var name, value string
		hasValue := false
		for i, v := range line {
			if v == '=' || v == ' ' {
				hasValue = true
				name, value = line[:i], line[i+1:]
				break
			}
		}

		if hasValue == false {
			name = line
		}

		// Ignore flag when already set; arguments have precedence over file
		if f.actual[name] != nil {
			continue
		}

		m := f.formal
		flag, alreadythere := m[name]
		if !alreadythere {
			if name == "help" || name == "h" { // special case for nice help message.
				f.usage()
				return ErrHelp
			}
			return f.failf("configuration variable provided but not defined: %s", name)
		}

		if fv, ok := flag.Value.(boolFlag); ok && fv.IsBoolFlag() { // special case: doesn't need an arg
			if hasValue {
				if expanded, err := expandAtFile(value); err == nil {
					value = expanded
				} else if !errors.Is(err, errNoAtExpansion) {
					return f.failf("invalid boolean value %q for configuration variable %s: %v", value, name, err)
				}
				if err := fv.Set(value); err != nil {
					return f.failf("invalid boolean value %q for configuration variable %s: %v", value, name, err)
				}
			} else {
				fv.Set("true")
			}
		} else {
			if expanded, err := expandAtFile(value); err == nil {
				value = expanded
			} else if !errors.Is(err, errNoAtExpansion) {
				return f.failf("invalid value %q for configuration variable %s: %v", value, name, err)
			}
			if err := flag.Value.Set(value); err != nil {
				return f.failf("invalid value %q for configuration variable %s: %v", value, name, err)
			}
		}

		// update f.actual
		if f.actual == nil {
			f.actual = make(map[string]*Flag)
		}
		f.actual[name] = flag
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

// --- Secret directory & @file support ---

var errNoAtExpansion = errors.New("no @file expansion")

// expandAtFile supports indirection syntax: a value beginning with '@path' will be
// replaced by the file contents (trimmed of a single trailing newline). '@@' escapes
// to a literal leading '@'. Returns errNoAtExpansion if no expansion occurred.
func expandAtFile(val string) (string, error) {
	if len(val) == 0 || val[0] != '@' {
		return "", errNoAtExpansion
	}
	if strings.HasPrefix(val, "@@") {
		return val[1:], nil
	} // escaped
	path := val[1:]
	if path == "" {
		return "", fmt.Errorf("invalid @file reference: empty path")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	// Trim a single trailing newline / CR
	s := string(b)
	s = strings.TrimRight(s, "\r\n")
	return s, nil
}

// ParseSecretDir ingests secret values from a directory where each file's name
// maps to a flag name (case-insensitive). Filename transformations tried in order:
// 1. raw lower-case filename
// 2. lower-case with '_' replaced by '-'
// Existing (already set) flags are not overridden. Subdirectories are ignored.
func (f *FlagSet) ParseSecretDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		candidates := []string{lower, strings.ReplaceAll(lower, "_", "-")}
		var target *Flag
		for _, cand := range candidates {
			if fl := f.formal[cand]; fl != nil {
				target = fl
				break
			}
		}
		if target == nil {
			continue
		}
		if f.actual != nil && f.actual[target.Name] != nil {
			continue
		} // respect precedence
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		val := strings.TrimRight(string(data), "\r\n")
		if fv, ok := target.Value.(boolFlag); ok && fv.IsBoolFlag() && (val == "" || strings.EqualFold(val, "true")) {
			// Empty or 'true' sets boolean true
			if err := fv.Set("true"); err != nil {
				return err
			}
		} else {
			if expanded, err := expandAtFile(val); err == nil {
				val = expanded
			} // nested @ optional
			if err := target.Value.Set(val); err != nil {
				return fmt.Errorf("secret file %s invalid for -%s: %w", name, target.Name, err)
			}
		}
		if f.actual == nil {
			f.actual = make(map[string]*Flag)
		}
		f.actual[target.Name] = target
	}
	return nil
}
