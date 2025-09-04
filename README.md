Flag
====

An extended, drop‑in replacement for Go's standard `flag` package originally forked from [namsral/flag] and enhanced with:

* Multi-source configuration with layered precedence
* Automatic struct-based flag registration (`ParseStruct`)
* Secret directory ingestion (`-secret-dir`) and `@file` indirection for values
* Extended types (time, decimal, UUID, IP/CIDR, URL, ByteSize, slices, maps, regex, JSON raw, enums, duration & time slices, etc.)
* Environment variable prefix support
* Enum validation via struct tags
* Zero external runtime dependencies beyond a few well-known libraries (uuid, decimal)

If you follow the [twelve-factor app methodology][] this package supports the third factor: store config in the environment—while also adding secure secret file loading and ergonomics for complex configuration surfaces.

[twelve-factor app methodology]: http://12factor.net
[namsral/flag]: https://github.com/namsral/flag

---

## Quick Start

```go
package main

import (
    "fmt"
    "log"
    flag "github.com/machship/flag"
)

func main() {
    var age int
    flag.IntVar(&age, "age", 0, "age of gopher")
    flag.Parse()
    fmt.Println("age:", age)
}
```

```bash
go run main.go -age 1   # CLI
export AGE=2; go run main.go   # ENV
echo 'age 3' > cfg.conf; go run main.go -config cfg.conf   # File
```

## Precedence (highest wins)
1. Command line flags
2. Environment variables
3. Secret directory files (`-secret-dir` if set)
4. Configuration file (`-config` if set)
5. Declared / struct defaults (or zero values)

## ParseStruct: Declarative Flag Registration

`ParseStruct(ptr)` reflects over a struct and auto-registers flags based on field tags. After registration it calls the global `Parse()`, applying the same layered precedence, then validates required flags.

Supported struct tags:

| Tag        | Purpose | Example |
|------------|---------|---------|
| `flag`     | Flag name (required to participate) | ``Host string `flag:"host"` `` |
| `default`  | Default value (ignored if `required:"true"`) | ``Port int `flag:"port" default:"8080"` `` |
| `help`     | Usage/help text | ``Debug bool `flag:"debug" help:"enable debug"` `` |
| `required` | Mark as required (`true`/`false`) | ``APIKey string `flag:"api-key" required:"true"` `` |
| `enum`     | Comma list of allowed values (string only) | ``Mode string `flag:"mode" enum:"dev,staging,prod"` `` |
| `sep`      | Separator for slice flags (default ",") | ``List []string `flag:"list" default:"a,b" sep:";"` `` |
| `layout`   | `time.Time` parse layout (default RFC3339) | ``Start time.Time `flag:"start" layout:"2006-01-02"` `` |
| `sensitive`| Mask value in usage, errors, introspection | ``Password string `flag:"password" sensitive:"true"` `` |
| `min`      | Minimum numeric value or min length (string/slice/map) | ``Retries int `flag:"retries" min:"1"` `` |
| `max`      | Maximum numeric value or max length (string/slice/map) | ``Retries int `flag:"retries" max:"10"` `` |
| `pattern`  | Regular expression a string must match | ``Name string `flag:"name" pattern:"^[a-z0-9_-]+$"` `` |
| `deprecated` | Mark deprecated; value is replacement name or message | ``Old string `flag:"old" deprecated:"new"` `` |

Example:

```go
type Config struct {
        SecretDir string            `flag:"secret-dir" default:"/run/secrets" help:"directory of secret files"`
        Config    string            `flag:"config" help:"optional config file"`
        Host      string            `flag:"host" default:"localhost"`
        Port      int               `flag:"port" default:"8080"`
        Debug     bool              `flag:"debug" required:"true"`
        Mode      string            `flag:"mode" enum:"dev,staging,prod" default:"dev"`
        Timeout   time.Duration     `flag:"timeout" default:"5s"`
        Start     time.Time         `flag:"start" layout:"2006-01-02" default:"2025-09-04"`
        Tags      []string          `flag:"tags" default:"alpha,beta" sep:","`
        Delays    []time.Duration   `flag:"delays" default:"100ms,250ms"`
        Meta      map[string]string `flag:"meta" default:"region=us,team=core"`
        Pattern   *regexp.Regexp    `flag:"pattern" default:"^user_[0-9]+$"`
        JSONBlob  json.RawMessage   `flag:"json" default:"{\"enabled\":true}"`
        ID        uuid.UUID         `flag:"id" default:"00000000-0000-0000-0000-000000000000"`
        Price     decimal.Decimal   `flag:"price" default:"12.99"`
        CIDR      net.IPNet         `flag:"cidr" default:"10.0.0.0/24"`
        URL       neturl.URL        `flag:"endpoint" default:"https://api.example.com"`
        Limit     ByteSize          `flag:"limit" default:"10MB" help:"memory limit"`
}

var cfg Config
if err := flag.ParseStruct(&cfg); err != nil { log.Fatal(err) }
```

### Supported Types

Primitive & standard: bool, int, int64, uint, uint64, float64, string, time.Duration

Extended:
* `time.Time` (with `layout` tag)
* `[]time.Time` (with `layout` tag & optional `sep`; see Time Slice Flags)
* `decimal.Decimal` (github.com/shopspring/decimal)
* `uuid.UUID`
* `net.IP`, `net.IPNet` (CIDR)
* `net/url`.URL
* `ByteSize` (human sizes: 512B, 10KB, 1MiB, 2G, 5GiB ...)
* `big.Int`, `big.Rat`
* `[]string`, `[]time.Duration`
* `map[string]string` (default string like `k=v,k2=v2`)
* `json.RawMessage` (validated on default parse)
* `*regexp.Regexp`
* String enums via `enum:"a,b,c"`
* Validated strings via `pattern:"^regex$"`

Unsupported types trigger an error referencing the field.

## Secret Directory Support (`-secret-dir`)

If a flag named `secret-dir` (or the value of `flag.DefaultSecretDirFlagname`) is set (CLI, env, or default), every regular file in that directory is considered a potential flag value.

Filename → flag name resolution tries:
1. Lower-case filename
2. Lower-case with underscores converted to dashes

Rules:
* Existing values (set by CLI or env) are NOT overridden
* Empty file for a bool flag sets it to `true`
* Contents are trimmed of one trailing newline
* A value starting with `@path` is replaced by the referenced file's contents (use `@@` to escape a literal `@`)

Example layout:
```
/run/secrets/
    db-user        => "alice"\n
    db-pass        => "@/run/secure/pass.txt"   (indirection)
    DEBUG          => ""  (sets -debug)
```

```go
type C struct {
    SecretDir string `flag:"secret-dir" default:"/run/secrets"`
    DBUser    string `flag:"db-user" required:"true"`
    DBPass    string `flag:"db-pass" required:"true"`
    Debug     bool   `flag:"debug"`
}
var c C
if err := flag.ParseStruct(&c); err != nil { log.Fatal(err) }
```

## `@file` Indirection

Anywhere a value is accepted (CLI, env, config file, secret file) you can supply `@/path/to/file` to load the value from that file. Use `@@` to escape.

| Input | Example | Result |
|-------|---------|--------|
| CLI | `-password @/run/secret/pass` | flag value becomes file contents |
| Env | `PASSWORD=@/run/secret/pass` | same |
| Config file | `password @/run/secret/pass` | same |
| Secret file | file contains `@/path` | nested expansion |

## ByteSize Type

Human-friendly sizes with decimal (KB=1000) or binary (KiB=1024) units.

Examples: `512B`, `128K`, `10KB`, `12MiB`, `1G`, `2GiB`, `5TB`, `3TiB`.

```go
var limit flag.ByteSize
flag.ByteSizeVar(&limit, "limit", 0, "memory limit")
```

## Enum Flags

```go
type C struct {
    Mode string `flag:"mode" enum:"dev,staging,prod" default:"dev"`
}
```
Invalid values produce an error listing allowed values.

## Validation Tags

Validation is deferred until after all precedence layers are applied, so the final value (from CLI, env, secret, config or default) is checked.

Supported tags:

* `min` / `max` – apply to numeric types OR length (string, slice, map)
* `pattern` – Go regexp applied to string value

Multiple failures aggregate into a single error (joined with `; `) via an internal multi-error collector (`MultiError`).

```go
type C struct {
    Port int    `flag:"port" default:"8080" min:"1" max:"65535"`
    Name string `flag:"name" pattern:"^[a-z0-9_-]{3,32}$"`
}
```

## Sensitive Values

Mark secrets so they are masked in:
* Usage output (default values show as `******`)
* Error messages (actual provided secret value suppressed)
* Introspection metadata

```go
type Secrets struct {
    Password string `flag:"password" required:"true" sensitive:"true"`
}
```

You can also mark flags programmatically: `flag.MarkSensitive("password")`.

## Introspection API

Programmatically inspect all registered flags and their provenance:

```go
metas := flag.Introspect()
for _, m := range metas {
    fmt.Printf("%s: set=%v source=%s value=%q sensitive=%v\n", m.Name, m.Set, m.Source, m.Value, m.Sensitive)
}
```

`Source` is one of: `cli`, `env`, `secret`, `config`, or `default`.
Sensitive values are masked as `******` (value & default).

## Disabling Auto Parse

`ParseStruct` automatically calls `flag.Parse()` after registration. To decouple registration and parsing (e.g., to add more flags manually, or defer to a subcommand decision) use:

```go
var cfg Config
if err := flag.ParseStructWithOptions(&cfg, flag.ParseStructOptions{AutoParse:false}); err != nil { log.Fatal(err) }
// later
flag.Parse()
if err := flag.Validate(); err != nil { log.Fatal(err) }
```

`Validate()` executes deferred validations and returns aggregated errors (if any).

## Nested Structs & Prefixing

`ParseStruct` recurses into exported nested struct fields even when they lack a `flag` tag; their inner fields with `flag` tags are registered.

You can apply a prefix to all flags in a nested struct using a `flagPrefix` tag on the nested struct field. Prefixes are concatenated with dots.

```go
type DBConfig struct {
    Host string `flag:"host" default:"localhost"`
    Port int    `flag:"port" default:"5432"`
}
type App struct {
    DB DBConfig `flagPrefix:"db"`
    Cache struct {
        Size int `flag:"size" default:"128"`
    } `flagPrefix:"cache"`
}
```

Flags registered:
```
 -db.host   (default localhost)
 -db.port   (default 5432)
 -cache.size (default 128)
```

Nested prefixes compose; an inner struct with `flagPrefix:"inner"` under a parent with `flagPrefix:"outer"` yields flags like `-outer.inner.field`.

## Provenance / Sources

Each flag stores the source that supplied its final value. This is exposed via introspection; you can build diagnostics or config dumps that omit secrets but still show origin.

Example (with a sensitive password read from env):

```
password: set=true source=env value="******" sensitive=true
host: set=false source=default value="localhost" sensitive=false
```

## Error Aggregation

When multiple validation errors occur they are combined into a single returned error (implementing `error`). The concrete type is `*flag.MultiError` which also implements:

* `Errors() []error` – returns the individual errors
* `Unwrap() []error` – Go's multi-error unwrapping, allowing `errors.Is` / `errors.As` to match any constituent error

Example:

```go
if err := flag.ParseStruct(&cfg); err != nil {
    var me *flag.MultiError
    if errors.As(err, &me) {
        for _, e := range me.Errors() { fmt.Println("validation:", e) }
    }
    return err
}
```

## Deprecation

Mark flags as deprecated with a struct tag or programmatically:

```go
type Opts struct {
    Old string `flag:"old" deprecated:"new"` // suggests replacement
    Legacy int  `flag:"legacy" deprecated:"(will be removed)"`
}
```

First use prints a one-time warning:

```
warning: flag -old is deprecated, use -new instead
warning: flag -legacy is deprecated (will be removed)
```

Programmatic: `flag.Deprecate("old", "new")`.

## Time Slice Flags

`[]time.Time` flags parse comma (or custom sep) separated values. Layout defaults to RFC3339 unless `layout` tag provided.

```go
type Schedule struct {
    Windows []time.Time `flag:"windows" layout:"2006-01-02" default:"2025-09-05,2025-09-06"`
}
```

CLI: `-windows 2025-09-05,2025-09-06`

## Cross-field / Post-parse Validation (Deferred)

Use `flag.Deferred(func() error { ... })` inside custom handlers or after manual registration to run logic after all precedence layers (CLI > env > secret > config > defaults) have settled. This is useful for:

* Cross-field consistency checks (e.g. `start < end`)
* Decoding multi-part derived values
* Expensive validations you only want once

Example:

```go
type Range struct {
    Start time.Time `flag:"start" layout:"2006-01-02" required:"true"`
    End   time.Time `flag:"end" layout:"2006-01-02" required:"true"`
}
var r Range
if err := flag.ParseStruct(&r); err != nil { log.Fatal(err) }
flag.Deferred(func() error { // or register before ParseStruct with AutoParse:false
    if !r.End.After(r.Start) { return fmt.Errorf("end must be after start") }
    return nil
})
if err := flag.Validate(); err != nil { log.Fatal(err) }
```

If you use `ParseStruct` with default `AutoParse:true`, deferred funcs added during struct handling execute automatically; any you add afterwards require calling `flag.Validate()`.

## Programmatic API Summary

Beyond the standard library-compatible surface, the following helpers are provided:

* Sources / layering: `ParseStruct`, `ParseStructWithOptions`, `Validate`
* Introspection: `Introspect()` -> `[]FlagMeta`
* Sensitivity: `MarkSensitive(names...)`
* Deprecation: `Deprecate(name, replacement)` (also via struct tag `deprecated`)
* Hot reload: `StartWatcher(secretDir, configFile)`, `StopWatcher()`, `OnChange(flagName, func(string))`
* Custom struct types: `RegisterStructHandler(reflect.Type, FieldHandler)` + `StructFieldContext`
* Deferred post-parse hooks: `Deferred(func() error)`
* Extended type registration helpers: `TimeVar`, `TimeSliceVar`, `ByteSizeVar`, `DecimalVar`, `IPVar`, `IPNetVar`, `URLVar`, `UUIDVar`, `BigIntVar`, `BigRatVar`, `RegexpVar`, `StringSliceVar`, `DurationSliceVar`, `TimeSliceVar`, `StringMapVar`, `JSONVar`, `EnumVar`
* Environment prefix: `NewFlagSetWithEnvPrefix(name, prefix, handling)` (uses `APP_` style names)

All of these augment (not replace) the original `flag` package patterns; you can mix and match incrementally.

## Struct Field Handler Registry

Extend `ParseStruct` with custom types by registering a handler:

```go
type Base64String string

func init() {
    flag.RegisterStructHandler(reflect.TypeOf(Base64String("")), func(ctx *flag.StructFieldContext) (bool, error) {
        def := ctx.Value.String()
        if ctx.DefaultTag != "" { def = ctx.DefaultTag }
        var raw string = def
        flag.StringVar(&raw, ctx.FlagName, raw, ctx.Help)
        // deferred decode after precedence layers settled
        flag.Deferred(func() error {
            if raw == "" { return nil }
            b, err := base64.StdEncoding.DecodeString(raw)
            if err != nil { return fmt.Errorf("invalid base64 for -%s: %w", ctx.FlagName, err) }
            *(*Base64String)(ctx.Value.Addr().Interface().(*Base64String)) = Base64String(b)
            return nil
        })
        return true, nil
    })
}
```

Handler API:
* `RegisterStructHandler(reflect.Type, FieldHandler)`
* `FieldHandler` returns `(handled bool, err error)`
* `StructFieldContext` carries tags (`DefaultTag`, `Deprecated`, etc.)

## Generic Numeric Values

All numeric flag types now share a generic implementation internally; public APIs (`IntVar`, `Uint64Var`, etc.) are unchanged. Avoid reflecting on internal unexported Value concrete types.

## Hot Reload & OnChange Callbacks

You can watch a secret directory and/or a config file for changes and react when specific flag values change.

```go
flag.StartWatcher("/run/secrets", "/app/config.conf")
flag.OnChange("db-password", func(v string) {
    // v is the new string value (be careful with sensitive data)
    reloadDB(v)
})
defer flag.StopWatcher()
```

Behavior:
* Secret dir watch: any file modification/add triggers re-read of that directory via `ParseSecretDir` (existing CLI/env values still win and are not overridden).
* Config file watch: file change triggers re-parse of config file layer (only flags originally sourced from config layer or still unset are updated).
* Only differences dispatch callbacks (per flag). Callbacks run in watcher goroutine; they are recovered on panic.
* Sensitive flags are passed in plain form to callbacks; handle securely.

Limitations / Notes:
* Environment variable changes are not automatically detected (exported env cannot be watched portably).
* Debouncing is not currently implemented—rapid successive writes may emit multiple callbacks.
* CLI-provided values are never overwritten by hot reload.

## Slices & Maps

| Type | Declaration | Default Tag Example |
|------|-------------|---------------------|
| `[]string` | ``Tags []string `flag:"tags" sep:","` `` | `default:"a,b,c"` |
| `[]time.Duration` | ``Delays []time.Duration `flag:"delays"` `` | `default:"100ms,1s"` |
| `[]time.Time` | ``Times []time.Time `flag:"times" layout:"2006-01-02T15:04:05Z07:00"` `` | `default:"2025-09-05T10:00:00Z,2025-09-05T12:00:00Z"` |
| `map[string]string` | ``Meta map[string]string `flag:"meta"` `` | `default:"k=v,team=core"` |

`sep` controls splitting for slices (default ","). Map defaults expect `k=v` comma separated pairs.

## Configuration File Format

Plain text, one flag per line:

```
key value
key=value
booleanFlag
# comments and blank lines ignored
```

## Environment Variables

Name is the flag name upper-cased, dashes replaced by underscores. Optional prefix via `NewFlagSetWithEnvPrefix`.

```go
fs := flag.NewFlagSetWithEnvPrefix(os.Args[0], "APP", flag.ContinueOnError)
fs.String("db-host", "localhost", "db host")
// Parses APP_DB_HOST
```

## Extended Example End-to-End

```go
type Config struct {
    SecretDir string        `flag:"secret-dir" default:"./secrets"`
    Config    string        `flag:"config"`
    Host      string        `flag:"host" default:"localhost"`
    Port      int           `flag:"port" default:"8080"`
    Mode      string        `flag:"mode" enum:"dev,staging,prod" default:"dev"`
    Timeout   time.Duration `flag:"timeout" default:"5s"`
    Limits    []time.Duration `flag:"limits" default:"100ms,200ms,500ms"`
    SizeLimit flag.ByteSize `flag:"limit" default:"256MiB"`
}

var cfg Config
if err := flag.ParseStruct(&cfg); err != nil { log.Fatal(err) }
```

Populate via (in ascending precedence):
* Default tag values
* `config` file (if provided)
* Secret dir files (if `secret-dir` set)
* Environment variables (e.g. `PORT=9000`)
* CLI flags (`-port 9000`)

## Migration From namsral/flag

Most code will continue to compile unchanged. New features are opt‑in:
* Add `ParseStruct` instead of manually declaring many flags
* Add a `secret-dir` flag for secret ingestion
* Use `@file` syntax to externalize sensitive values
* Leverage additional types and enums without extra boilerplate

## Examples

See the [examples](./examples) directory for simple usage patterns. Tests in the repository exercise advanced features (secret dir, struct parsing, indirection).

## License
---


Copyright (c) 2012 The Go Authors. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google Inc. nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
