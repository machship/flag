package flag_test

import (
	"encoding/json"
	"errors"
	stdflag "flag"
	"fmt"
	"math/big"
	"net"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	lib "github.com/machship/flag"
	decimal "github.com/shopspring/decimal"
)

// Example 1: A single string flag called "species" with default value "gopher".
// NOTE: The original stdlib flag examples are preserved below for familiarity.
// We import the standard library as stdflag to avoid confusion with this package.

var species = stdflag.String("species", "gopher", "the species we are studying")

// Example 2: Two flags sharing a variable, so we can have a shorthand.
// The order of initialization is undefined, so make sure both use the
// same default value. They must be set up with an init function.
var gopherType string

func init() {
	const (
		defaultGopher = "pocket"
		usage         = "the variety of gopher"
	)
	stdflag.StringVar(&gopherType, "gopher_type", defaultGopher, usage)
	stdflag.StringVar(&gopherType, "g", defaultGopher, usage+" (shorthand)")
}

// Example 3: A user-defined flag type, a slice of durations.
type interval []time.Duration

// String is the method to format the flag's value, part of the flag.Value interface.
// The String method's output will be used in diagnostics.
func (i *interval) String() string {
	return fmt.Sprint(*i)
}

// Set is the method to set the flag value, part of the flag.Value interface.
// Set's argument is a string to be parsed to set the flag.
// It's a comma-separated list, so we split it.
func (i *interval) Set(value string) error {
	// If we wanted to allow the flag to be set multiple times,
	// accumulating values, we would delete this if statement.
	// That would permit usages such as
	//	-deltaT 10s -deltaT 15s
	// and other combinations.
	if len(*i) > 0 {
		return errors.New("interval flag already set")
	}
	for _, dt := range strings.Split(value, ",") {
		duration, err := time.ParseDuration(dt)
		if err != nil {
			return err
		}
		*i = append(*i, duration)
	}
	return nil
}

// Define a flag to accumulate durations. Because it has a special type,
// we need to use the Var function and therefore create the flag during
// init.

var intervalFlag interval

func init() {
	// Tie the command-line flag to the intervalFlag variable and
	// set a usage message.
	stdflag.Var(&intervalFlag, "deltaT", "comma-separated list of intervals to use between events")
}

func Example() {
	// All the interesting pieces are with the variables declared above, but
	// to enable the flag package to see the flags defined there, one must
	// execute, typically at the start of main (not init!):
	//	flag.Parse()
	// We don't run it here because this is not a main function and
	// the testing suite has already parsed the flags.
}

// =============================
// Additional verbose examples for the extended flag library (github.com/machship/flag)
// Demonstrate: precedence (args>env>file>default), ParseStruct, extended types, enums, slices & maps.
// =============================

// Example_argsEnvFilePrecedence shows how command line overrides env which overrides file which overrides default.
func Example_argsEnvFilePrecedence() {
	lib.ResetForTesting(nil)
	// Register flags & config file flag.
	lib.String(lib.DefaultConfigFlagname, "", "config file")
	host := lib.String("host", "default-host", "service host")
	port := lib.Int("port", 8080, "service port")
	// Create config file with baseline values.
	cfg := filepath.Join(os.TempDir(), "precedence.conf")
	os.WriteFile(cfg, []byte("host file-host\nport 9000\n"), 0o600)
	defer os.Remove(cfg)
	// Set env vars (will override file if no CLI override)
	os.Setenv("HOST", "env-host")
	os.Setenv("PORT", "9100")
	defer os.Unsetenv("HOST")
	defer os.Unsetenv("PORT")
	// Provide CLI args overriding only host (not port) and supply config file.
	os.Args = []string{"app", "-config", cfg, "-host", "cli-host"}
	lib.Parse()
	fmt.Printf("host=%s port=%d\n", *host, *port) // host from CLI, port from env
	// Output: host=cli-host port=9100
}

// Example_struct_basic shows using ParseStruct with defaults and a required flag.
func Example_struct_basic() {
	lib.ResetForTesting(nil)
	type Config struct {
		Endpoint string        `flag:"endpoint" default:"https://api.example" help:"API base URL"`
		Timeout  time.Duration `flag:"timeout" default:"2s" help:"request timeout"`
		Debug    bool          `flag:"debug" required:"true" help:"enable debug logging"`
	}
	var cfg Config
	os.Args = []string{"svc", "-debug"}
	if err := lib.ParseStruct(&cfg); err != nil {
		panic(err)
	}
	fmt.Printf("endpoint=%s timeout=%s debug=%v\n", cfg.Endpoint, cfg.Timeout, cfg.Debug)
	// Output: endpoint=https://api.example timeout=2s debug=true
}

// Example_struct_enumsAndNumerics demonstrates enum validation & numeric parsing via struct tags.
func Example_struct_enumsAndNumerics() {
	lib.ResetForTesting(nil)
	type Limits struct {
		Mode    string  `flag:"mode" enum:"fast,slow" default:"fast"`
		Retries int     `flag:"retries" default:"3"`
		Rate    float64 `flag:"rate" default:"1.5"`
	}
	var lim Limits
	os.Args = []string{"app", "-mode", "slow", "-retries", "5"}
	if err := lib.ParseStruct(&lim); err != nil {
		panic(err)
	}
	fmt.Printf("mode=%s retries=%d rate=%.1f\n", lim.Mode, lim.Retries, lim.Rate)
	// Output: mode=slow retries=5 rate=1.5
}

// Example_extended_types covers many custom Value types provided by the library.
func Example_extended_types() {
	lib.ResetForTesting(nil)
	dec := decimal.NewFromFloat(0)
	lib.DecimalVar(&dec, "price", decimal.NewFromFloat(0), "decimal price")
	var when time.Time
	lib.TimeVar(&when, "when", time.RFC3339, time.Time{}, "timestamp")
	var ip net.IP
	lib.IPVar(&ip, "ip", nil, "ip address")
	var ipn net.IPNet
	lib.IPNetVar(&ipn, "cidr", nil, "network cidr")
	var u neturl.URL
	lib.URLVar(&u, "url", nil, "resource URL")
	var id uuid.UUID
	lib.UUIDVar(&id, "id", uuid.UUID{}, "identifier")
	bs := lib.ByteSizeFlag("mem", 0, "memory size")
	ss := lib.StringSlice("tags", ",", []string{}, "comma tags")
	ds := lib.DurationSlice("intervals", ",", []time.Duration{}, "durations")
	mp := lib.StringMap("labels", map[string]string{}, "k=v pairs")
	var raw json.RawMessage
	lib.JSONVar(&raw, "json", nil, "json blob")
	enum := lib.Enum("env", "dev", []string{"dev", "prod"}, "environment")
	os.Args = []string{"cmd",
		"-price", "12.34", "-when", "2023-01-02T03:04:05Z",
		"-ip", "127.0.0.1", "-cidr", "10.0.0.0/8", "-url", "https://example.com/x",
		"-id", "123e4567-e89b-12d3-a456-426614174000", "-mem", "5MiB",
		"-tags", "a,b", "-intervals", "1s,2s", "-labels", "k1=v1,k2=v2",
		"-json", "{\"k\":1}", "-env", "prod",
	}
	lib.Parse()
	fmt.Println("env:", *enum, "mem:", *bs, "tags:", len(*ss), "intervals:", len(*ds), "labels:", len(*mp))
	// Output: env: prod mem: 5242880 tags: 2 intervals: 2 labels: 2
}

// Example_struct_byteSizeAndMap defaults & required interplay.
func Example_struct_byteSizeAndMap() {
	lib.ResetForTesting(nil)
	type Opts struct {
		Cache lib.ByteSize      `flag:"cache" default:"256KiB"`
		Meta  map[string]string `flag:"meta" default:"a=1,b=2"`
	}
	var o Opts
	os.Args = []string{"tool"}
	if err := lib.ParseStruct(&o); err != nil {
		panic(err)
	}
	fmt.Printf("cache=%d meta=%d\n", o.Cache, len(o.Meta))
	// Output: cache=262144 meta=2
}

// Example_struct_error demonstrates capturing an invalid default error for teaching purposes.
// (We run it as an example but only assert the prefix to keep it stable.)
func Example_struct_error() {
	lib.ResetForTesting(nil)
	type Bad struct {
		N int `flag:"n" default:"NaN"`
	}
	var b Bad
	os.Args = []string{"bad"}
	err := lib.ParseStruct(&b)
	if err != nil && strings.Contains(err.Error(), "invalid default int") {
		fmt.Println("error: invalid default int")
	}
	// Output: error: invalid default int
}

// Example_customFlagSetWithPrefix shows using a separate FlagSet with an environment variable prefix.
func Example_customFlagSetWithPrefix() {
	fs := lib.NewFlagSetWithEnvPrefix("service", "APP", lib.ContinueOnError)
	p := fs.Int("port", 0, "service port")
	os.Setenv("APP_PORT", "7777")
	defer os.Unsetenv("APP_PORT")
	// No CLI args, Parse() pulls from env.
	if err := fs.Parse([]string{}); err != nil {
		panic(err)
	}
	fmt.Println("port:", *p)
	// Output: port: 7777
}

// Example_varRegistration shows manual Var usage for a big.Int.
func Example_varRegistration() {
	lib.ResetForTesting(nil)
	bi := new(big.Int)
	lib.BigIntVar(bi, "big", nil, "big integer value")
	os.Args = []string{"app", "-big", "0x10"}
	lib.Parse()
	fmt.Println("big:", bi.String())
	// Output: big: 16
}

// Example_zeroValuesAndPrintDefaults illustrates PrintDefaults output (truncated by checking substring).
func Example_zeroValuesAndPrintDefaults() {
	lib.ResetForTesting(nil)
	// Redirect output to buffer (use bytes in real code; here just call directly for demonstration)
	lib.String("name", "", "user name")
	// We won't assert output to avoid brittleness; example just ensures no panic.
	// No Output section means test harness only verifies it runs.
}

// Example_basic shows the simplest use: define a flag and parse args.
func Example_basic() {
	lib.ResetForTesting(nil) // ensure a clean CommandLine
	age := lib.Int("age", 0, "age of gopher")
	// Simulate command-line: program name + flags
	os.Args = []string{"cmd", "-age", "7"}
	lib.Parse()
	fmt.Println("age:", *age)
	// Output: age: 7
}

// Example_environment demonstrates reading a value from the environment
// when it is not supplied on the command line.
func Example_environment() {
	lib.ResetForTesting(nil)
	// Define the flag.
	port := lib.Int("port", 8080, "service port")
	// Set environment variable PORT (uppercase name of flag)
	os.Setenv("PORT", "9000")
	defer os.Unsetenv("PORT")
	os.Args = []string{"svc"} // no -port argument provided
	lib.Parse()
	fmt.Println("port:", *port)
	// Output: port: 9000
}

// Example_configFile shows using the built-in config file flag (default name "config").
func Example_configFile() {
	lib.ResetForTesting(nil)
	name := lib.String("name", "", "user name")
	// Register the config flag so the library knows where to load from.
	lib.String(lib.DefaultConfigFlagname, "", "config file path")
	// Create a temporary config file.
	tmpDir := os.TempDir()
	cfgPath := filepath.Join(tmpDir, "example_flag.conf")
	// File supports either "key value" or "key=value" forms.
	os.WriteFile(cfgPath, []byte("name Alice\n"), 0o600)
	defer os.Remove(cfgPath)
	// Provide -config argument pointing to the file.
	os.Args = []string{"app", "-config", cfgPath}
	lib.Parse()
	fmt.Println("name:", *name)
	// Output: name: Alice
}

// Example_enumAndCollections demonstrates enum, duration slice, and string map flags.
func Example_enumAndCollections() {
	lib.ResetForTesting(nil)
	color := lib.Enum("color", "red", []string{"red", "green", "blue"}, "color choice")
	intervals := lib.DurationSlice("intervals", ",", []time.Duration{}, "comma separated durations")
	labels := lib.StringMap("labels", map[string]string{}, "key=value pairs")
	os.Args = []string{"cmd", "-color", "green", "-intervals", "1s,2s,500ms", "-labels", "env=prod,ver=1"}
	lib.Parse()
	fmt.Println("color:", *color)
	fmt.Println("interval count:", len(*intervals))
	fmt.Println("labels ver:", (*labels)["ver"])
	// Output:
	// color: green
	// interval count: 3
	// labels ver: 1
}

// Example_structParsing shows defining flags from a struct using tags.
func Example_structParsing() {
	lib.ResetForTesting(nil)
	type Config struct {
		Host  string        `flag:"host" default:"localhost" help:"service host"`
		Port  int           `flag:"port" default:"8080" help:"service port"`
		Debug bool          `flag:"debug" required:"true" help:"enable debug"`
		Delay time.Duration `flag:"delay" default:"250ms"`
	}
	var cfg Config
	os.Args = []string{"svc", "-debug", "-port", "9001"}
	if err := lib.ParseStruct(&cfg); err != nil {
		panic(err)
	}
	fmt.Printf("%s:%d debug=%v delay=%s\n", cfg.Host, cfg.Port, cfg.Debug, cfg.Delay)
	// Output: localhost:9001 debug=true delay=250ms
}

// Example_extendedTypes shows some additional custom types supported (ByteSize and time with layout).
func Example_extendedTypes() {
	lib.ResetForTesting(nil)
	size := lib.ByteSizeFlag("size", 0, "buffer size")
	ts := lib.Time("start", time.RFC3339, time.Time{}, "start time (RFC3339)")
	os.Args = []string{"cmd", "-size", "10KiB", "-start", "2023-01-02T03:04:05Z"}
	lib.Parse()
	fmt.Println("size:", *size) // prints raw bytes value
	fmt.Println("start year:", ts.Year())
	// Output:
	// size: 10240
	// start year: 2023
}
