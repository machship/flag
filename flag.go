// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package flag implements command-line flag parsing.

Usage:

Define flags using flag.String(), Bool(), Int(), etc.

This declares an integer flag, -flagname, stored in the pointer ip, with type *int.

	import "flag"
	var ip = flag.Int("flagname", 1234, "help message for flagname")

If you like, you can bind the flag to a variable using the Var() functions.

	var flagvar int
	func init() {
		flag.IntVar(&flagvar, "flagname", 1234, "help message for flagname")
	}

Or you can create custom flags that satisfy the Value interface (with
pointer receivers) and couple them to flag parsing by

	flag.Var(&flagVal, "name", "help message for flagname")

For such flags, the default value is just the initial value of the variable.

After all flags are defined, call

	flag.Parse()

to parse the command line into the defined flags.

Flags may then be used directly. If you're using the flags themselves,
they are all pointers; if you bind to variables, they're values.

	fmt.Println("ip has value ", *ip)
	fmt.Println("flagvar has value ", flagvar)

After parsing, the arguments following the flags are available as the
slice flag.Args() or individually as flag.Arg(i).
The arguments are indexed from 0 through flag.NArg()-1.

Command line flag syntax:

	-flag
	-flag=x
	-flag x  // non-boolean flags only

One or two minus signs may be used; they are equivalent.
The last form is not permitted for boolean flags because the
meaning of the command

	cmd -x *

will change if there is a file called 0, false, etc.  You must
use the -flag=false form to turn off a boolean flag.

Flag parsing stops just before the first non-flag argument
("-" is a non-flag argument) or after the terminator "--".

Integer flags accept 1234, 0664, 0x1234 and may be negative.
Boolean flags may be:

	1, 0, t, f, T, F, true, false, TRUE, FALSE, True, False

Duration flags accept any input valid for time.ParseDuration.

The default set of command-line flags is controlled by
top-level functions.  The FlagSet type allows one to define
independent sets of flags, such as to implement subcommands
in a command-line interface. The methods of FlagSet are
analogous to the top-level functions for the command-line
flag set.
*/
package flag

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	neturl "net/url"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	decimal "github.com/shopspring/decimal"
)

// ErrHelp is the error returned if the -help or -h flag is invoked
// but no such flag is defined.
var ErrHelp = errors.New("flag: help requested")

// -- bool Value
type boolValue bool

func newBoolValue(val bool, p *bool) *boolValue {
	*p = val
	return (*boolValue)(p)
}

func (b *boolValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	*b = boolValue(v)
	return err
}

func (b *boolValue) Get() interface{} { return bool(*b) }

func (b *boolValue) String() string { return fmt.Sprintf("%v", *b) }

func (b *boolValue) IsBoolFlag() bool { return true }

// optional interface to indicate boolean flags that can be
// supplied without "=value" text
type boolFlag interface {
	Value
	IsBoolFlag() bool
}

// -- int Value
type intValue int

func newIntValue(val int, p *int) *intValue {
	*p = val
	return (*intValue)(p)
}

func (i *intValue) Set(s string) error {
	v, err := strconv.ParseInt(s, 0, 64)
	*i = intValue(v)
	return err
}

func (i *intValue) Get() interface{} { return int(*i) }

func (i *intValue) String() string { return fmt.Sprintf("%v", *i) }

// -- int64 Value
type int64Value int64

func newInt64Value(val int64, p *int64) *int64Value {
	*p = val
	return (*int64Value)(p)
}

func (i *int64Value) Set(s string) error {
	v, err := strconv.ParseInt(s, 0, 64)
	*i = int64Value(v)
	return err
}

func (i *int64Value) Get() interface{} { return int64(*i) }

func (i *int64Value) String() string { return fmt.Sprintf("%v", *i) }

// -- uint Value
type uintValue uint

func newUintValue(val uint, p *uint) *uintValue {
	*p = val
	return (*uintValue)(p)
}

func (i *uintValue) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 64)
	*i = uintValue(v)
	return err
}

func (i *uintValue) Get() interface{} { return uint(*i) }

func (i *uintValue) String() string { return fmt.Sprintf("%v", *i) }

// -- uint64 Value
type uint64Value uint64

func newUint64Value(val uint64, p *uint64) *uint64Value {
	*p = val
	return (*uint64Value)(p)
}

func (i *uint64Value) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 64)
	*i = uint64Value(v)
	return err
}

func (i *uint64Value) Get() interface{} { return uint64(*i) }

func (i *uint64Value) String() string { return fmt.Sprintf("%v", *i) }

// -- string Value
type stringValue string

func newStringValue(val string, p *string) *stringValue {
	*p = val
	return (*stringValue)(p)
}

func (s *stringValue) Set(val string) error {
	*s = stringValue(val)
	return nil
}

func (s *stringValue) Get() interface{} { return string(*s) }

func (s *stringValue) String() string { return fmt.Sprintf("%s", *s) }

// -- float64 Value
type float64Value float64

func newFloat64Value(val float64, p *float64) *float64Value {
	*p = val
	return (*float64Value)(p)
}

func (f *float64Value) Set(s string) error {
	v, err := strconv.ParseFloat(s, 64)
	*f = float64Value(v)
	return err
}

func (f *float64Value) Get() interface{} { return float64(*f) }

func (f *float64Value) String() string { return fmt.Sprintf("%v", *f) }

// -- time.Duration Value
type durationValue time.Duration

func newDurationValue(val time.Duration, p *time.Duration) *durationValue {
	*p = val
	return (*durationValue)(p)
}

func (d *durationValue) Set(s string) error {
	v, err := time.ParseDuration(s)
	*d = durationValue(v)
	return err
}

func (d *durationValue) Get() interface{} { return time.Duration(*d) }

func (d *durationValue) String() string { return (*time.Duration)(d).String() }

// ---- Extended / custom types ----

// ByteSize represents a size in bytes (supports K, M, G, T suffixes incl. KiB style).
type ByteSize int64

func parseByteSize(s string) (ByteSize, error) {
	if s == "" {
		return 0, nil
	}
	// Accept forms: 123, 10k, 5K, 1MB, 2MiB, etc.
	orig := s
	s = strings.TrimSpace(s)
	// Extract numeric prefix
	i := 0
	for i < len(s) && (s[i] == '.' || s[i] == '+' || s[i] == '-' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	if i == 0 {
		return 0, fmt.Errorf("invalid size: %s", orig)
	}
	numStr := s[:i]
	unit := strings.ToUpper(strings.TrimSpace(s[i:]))
	f, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size number %q: %v", numStr, err)
	}
	mult := float64(1)
	switch unit {
	case "", "B":
		mult = 1
	case "K", "KB":
		mult = 1000
	case "KI", "KIB":
		mult = 1024
	case "M", "MB":
		mult = 1000 * 1000
	case "MI", "MIB":
		mult = 1024 * 1024
	case "G", "GB":
		mult = 1000 * 1000 * 1000
	case "GI", "GIB":
		mult = 1024 * 1024 * 1024
	case "T", "TB":
		mult = 1000 * 1000 * 1000 * 1000
	case "TI", "TIB":
		mult = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown size unit in %q", orig)
	}
	return ByteSize(f * mult), nil
}

type byteSizeValue struct{ p *ByteSize }

func newByteSizeValue(val ByteSize, p *ByteSize) *byteSizeValue {
	*p = val
	return &byteSizeValue{p: p}
}
func (b *byteSizeValue) Set(s string) error {
	v, err := parseByteSize(s)
	if err != nil {
		return err
	}
	*b.p = v
	return nil
}
func (b *byteSizeValue) String() string {
	if b.p == nil {
		return "0"
	}
	return fmt.Sprintf("%d", *b.p)
}
func (b *byteSizeValue) Get() interface{} { return *b.p }

// time.Time value with layout
type timeValue struct {
	p      *time.Time
	layout string
}

func newTimeValue(val time.Time, layout string, p *time.Time) *timeValue {
	*p = val
	return &timeValue{p: p, layout: layout}
}
func (tv *timeValue) Set(s string) error {
	t, err := time.Parse(tv.layout, s)
	if err != nil {
		return err
	}
	*tv.p = t
	return nil
}
func (tv *timeValue) String() string {
	if tv.p == nil || tv.p.IsZero() {
		return ""
	}
	return tv.p.Format(tv.layout)
}
func (tv *timeValue) Get() interface{} { return *tv.p }

// decimal.Decimal
type decimalValue struct{ p *decimal.Decimal }

func newDecimalValue(val decimal.Decimal, p *decimal.Decimal) *decimalValue {
	*p = val
	return &decimalValue{p: p}
}
func (dv *decimalValue) Set(s string) error {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return err
	}
	*dv.p = d
	return nil
}
func (dv *decimalValue) String() string {
	if dv.p == nil {
		return "0"
	}
	return dv.p.String()
}
func (dv *decimalValue) Get() interface{} { return *dv.p }

// net.IP
type ipValue struct{ p *net.IP }

func newIPValue(val net.IP, p *net.IP) *ipValue { *p = val; return &ipValue{p: p} }
func (iv *ipValue) Set(s string) error {
	ip := net.ParseIP(s)
	if ip == nil {
		return fmt.Errorf("invalid IP %q", s)
	}
	*iv.p = ip
	return nil
}
func (iv *ipValue) String() string {
	if iv.p == nil || *iv.p == nil {
		return ""
	}
	return iv.p.String()
}
func (iv *ipValue) Get() interface{} { return *iv.p }

// net.IPNet
type ipNetValue struct{ p *net.IPNet }

func newIPNetValue(val *net.IPNet, p *net.IPNet) *ipNetValue {
	if val != nil {
		*p = *val
	}
	return &ipNetValue{p: p}
}
func (nv *ipNetValue) Set(s string) error {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		return err
	}
	*nv.p = *n
	return nil
}
func (nv *ipNetValue) String() string {
	if nv.p == nil || nv.p.IP == nil {
		return ""
	}
	return nv.p.String()
}
func (nv *ipNetValue) Get() interface{} { return *nv.p }

// url.URL
type urlValue struct{ p *neturl.URL }

func newURLValue(val *neturl.URL, p *neturl.URL) *urlValue {
	if val != nil {
		*p = *val
	}
	return &urlValue{p: p}
}
func (uv *urlValue) Set(s string) error {
	u, err := neturl.Parse(s)
	if err != nil {
		return err
	}
	*uv.p = *u
	return nil
}
func (uv *urlValue) String() string {
	if uv.p == nil || uv.p.Host == "" {
		return ""
	}
	return uv.p.String()
}
func (uv *urlValue) Get() interface{} { return *uv.p }

// uuid.UUID
type uuidValue struct{ p *uuid.UUID }

func newUUIDValue(val uuid.UUID, p *uuid.UUID) *uuidValue { *p = val; return &uuidValue{p: p} }
func (uv *uuidValue) Set(s string) error {
	id, err := uuid.Parse(s)
	if err != nil {
		return err
	}
	*uv.p = id
	return nil
}
func (uv *uuidValue) String() string {
	if uv.p == nil {
		return ""
	}
	return uv.p.String()
}
func (uv *uuidValue) Get() interface{} { return *uv.p }

// big.Int
type bigIntValue struct{ p *big.Int }

func newBigIntValue(val *big.Int, p *big.Int) *bigIntValue {
	if val != nil {
		p.Set(val)
	}
	return &bigIntValue{p: p}
}
func (bv *bigIntValue) Set(s string) error {
	if _, ok := bv.p.SetString(s, 0); !ok {
		return fmt.Errorf("invalid big.Int %q", s)
	}
	return nil
}
func (bv *bigIntValue) String() string {
	if bv.p == nil {
		return "0"
	}
	return bv.p.String()
}
func (bv *bigIntValue) Get() interface{} { return *bv.p }

// big.Rat
type bigRatValue struct{ p *big.Rat }

func newBigRatValue(val *big.Rat, p *big.Rat) *bigRatValue {
	if val != nil {
		p.Set(val)
	}
	return &bigRatValue{p: p}
}
func (rv *bigRatValue) Set(s string) error {
	if _, ok := rv.p.SetString(s); !ok {
		return fmt.Errorf("invalid big.Rat %q", s)
	}
	return nil
}
func (rv *bigRatValue) String() string {
	if rv.p == nil {
		return "0"
	}
	return rv.p.RatString()
}
func (rv *bigRatValue) Get() interface{} { return *rv.p }

// regexp
type regexpValue struct{ p **regexp.Regexp }

func newRegexpValue(val *regexp.Regexp, p **regexp.Regexp) *regexpValue {
	*p = val
	return &regexpValue{p: p}
}
func (rv *regexpValue) Set(s string) error {
	r, err := regexp.Compile(s)
	if err != nil {
		return err
	}
	*rv.p = r
	return nil
}
func (rv *regexpValue) String() string {
	if rv.p == nil || *rv.p == nil {
		return ""
	}
	return (*rv.p).String()
}
func (rv *regexpValue) Get() interface{} {
	if rv.p == nil || *rv.p == nil {
		return nil
	}
	return *rv.p
}

// string slice
type stringSliceValue struct {
	p   *[]string
	sep string
}

func newStringSliceValue(val []string, sep string, p *[]string) *stringSliceValue {
	*p = append((*p)[:0], val...)
	return &stringSliceValue{p: p, sep: sep}
}
func (sv *stringSliceValue) Set(s string) error {
	parts := strings.Split(s, sv.sep)
	*sv.p = append((*sv.p)[:0], parts...)
	return nil
}
func (sv *stringSliceValue) String() string {
	if sv.p == nil {
		return ""
	}
	return strings.Join(*sv.p, sv.sep)
}
func (sv *stringSliceValue) Get() interface{} { return *sv.p }

// duration slice
type durationSliceValue struct {
	p   *[]time.Duration
	sep string
}

func newDurationSliceValue(val []time.Duration, sep string, p *[]time.Duration) *durationSliceValue {
	*p = append((*p)[:0], val...)
	return &durationSliceValue{p: p, sep: sep}
}
func (dv *durationSliceValue) Set(s string) error {
	parts := strings.Split(s, dv.sep)
	out := make([]time.Duration, 0, len(parts))
	for _, part := range parts {
		d, err := time.ParseDuration(strings.TrimSpace(part))
		if err != nil {
			return err
		}
		out = append(out, d)
	}
	*dv.p = out
	return nil
}
func (dv *durationSliceValue) String() string {
	if dv.p == nil {
		return ""
	}
	var ss []string
	for _, d := range *dv.p {
		ss = append(ss, d.String())
	}
	return strings.Join(ss, dv.sep)
}
func (dv *durationSliceValue) Get() interface{} { return *dv.p }

// map[string]string (comma separated key=value list)
type stringMapValue struct{ p *map[string]string }

func newStringMapValue(val map[string]string, p *map[string]string) *stringMapValue {
	*p = val
	return &stringMapValue{p: p}
}
func (mv *stringMapValue) Set(s string) error {
	m := make(map[string]string)
	if strings.TrimSpace(s) != "" {
		pairs := strings.Split(s, ",")
		for _, p := range pairs {
			kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
			if len(kv) != 2 {
				return fmt.Errorf("invalid map entry %q", p)
			}
			m[kv[0]] = kv[1]
		}
	}
	*mv.p = m
	return nil
}
func (mv *stringMapValue) String() string {
	if mv.p == nil {
		return ""
	}
	var parts []string
	for k, v := range *mv.p {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}
func (mv *stringMapValue) Get() interface{} { return *mv.p }

// json.RawMessage
type jsonValue struct{ p *json.RawMessage }

func newJSONValue(val json.RawMessage, p *json.RawMessage) *jsonValue {
	*p = val
	return &jsonValue{p: p}
}
func (jv *jsonValue) Set(s string) error {
	var tmp json.RawMessage = json.RawMessage([]byte(s)) // basic validation
	var v interface{}
	if err := json.Unmarshal(tmp, &v); err != nil {
		return err
	}
	*jv.p = tmp
	return nil
}
func (jv *jsonValue) String() string {
	if jv.p == nil {
		return ""
	}
	return string(*jv.p)
}
func (jv *jsonValue) Get() interface{} { return *jv.p }

// enum string wrapper
type enumStringValue struct {
	p       *string
	allowed map[string]struct{}
}

func newEnumStringValue(def string, allowed []string, p *string) *enumStringValue {
	*p = def
	m := make(map[string]struct{}, len(allowed))
	for _, a := range allowed {
		m[a] = struct{}{}
	}
	return &enumStringValue{p: p, allowed: m}
}
func (ev *enumStringValue) Set(s string) error {
	if _, ok := ev.allowed[s]; !ok {
		return fmt.Errorf("invalid value %q (allowed: %s)", s, keys(ev.allowed))
	}
	*ev.p = s
	return nil
}
func (ev *enumStringValue) String() string {
	if ev.p == nil {
		return ""
	}
	return *ev.p
}
func (ev *enumStringValue) Get() interface{} { return *ev.p }

func keys(m map[string]struct{}) string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return strings.Join(ks, ",")
}

// Helper registration methods for extended types
func (f *FlagSet) ByteSizeVar(p *ByteSize, name string, value ByteSize, usage string) {
	f.Var(newByteSizeValue(value, p), name, usage)
}
func ByteSizeVar(p *ByteSize, name string, value ByteSize, usage string) {
	CommandLine.Var(newByteSizeValue(value, p), name, usage)
}

// ByteSizeFlag defines a ByteSize flag and returns a pointer to it.
func (f *FlagSet) ByteSizeFlag(name string, value ByteSize, usage string) *ByteSize {
	p := new(ByteSize)
	f.ByteSizeVar(p, name, value, usage)
	return p
}
func ByteSizeFlag(name string, value ByteSize, usage string) *ByteSize {
	return CommandLine.ByteSizeFlag(name, value, usage)
}

func (f *FlagSet) TimeVar(p *time.Time, name, layout string, value time.Time, usage string) {
	if layout == "" {
		layout = time.RFC3339
	}
	f.Var(newTimeValue(value, layout, p), name, usage)
}
func TimeVar(p *time.Time, name, layout string, value time.Time, usage string) {
	CommandLine.TimeVar(p, name, layout, value, usage)
}
func (f *FlagSet) Time(name, layout string, value time.Time, usage string) *time.Time {
	p := new(time.Time)
	f.TimeVar(p, name, layout, value, usage)
	return p
}
func Time(name, layout string, value time.Time, usage string) *time.Time {
	return CommandLine.Time(name, layout, value, usage)
}

func (f *FlagSet) DecimalVar(p *decimal.Decimal, name string, value decimal.Decimal, usage string) {
	f.Var(newDecimalValue(value, p), name, usage)
}
func DecimalVar(p *decimal.Decimal, name string, value decimal.Decimal, usage string) {
	CommandLine.DecimalVar(p, name, value, usage)
}
func (f *FlagSet) Decimal(name string, value decimal.Decimal, usage string) *decimal.Decimal {
	p := new(decimal.Decimal)
	f.DecimalVar(p, name, value, usage)
	return p
}
func Decimal(name string, value decimal.Decimal, usage string) *decimal.Decimal {
	return CommandLine.Decimal(name, value, usage)
}

func (f *FlagSet) IPVar(p *net.IP, name string, value net.IP, usage string) {
	f.Var(newIPValue(value, p), name, usage)
}
func IPVar(p *net.IP, name string, value net.IP, usage string) {
	CommandLine.IPVar(p, name, value, usage)
}
func (f *FlagSet) IP(name string, value net.IP, usage string) *net.IP {
	p := new(net.IP)
	f.IPVar(p, name, value, usage)
	return p
}
func IP(name string, value net.IP, usage string) *net.IP { return CommandLine.IP(name, value, usage) }

func (f *FlagSet) IPNetVar(p *net.IPNet, name string, value *net.IPNet, usage string) {
	f.Var(newIPNetValue(value, p), name, usage)
}
func IPNetVar(p *net.IPNet, name string, value *net.IPNet, usage string) {
	CommandLine.IPNetVar(p, name, value, usage)
}
func (f *FlagSet) IPNet(name string, value *net.IPNet, usage string) *net.IPNet {
	p := new(net.IPNet)
	f.IPNetVar(p, name, value, usage)
	return p
}
func IPNet(name string, value *net.IPNet, usage string) *net.IPNet {
	return CommandLine.IPNet(name, value, usage)
}

func (f *FlagSet) URLVar(p *neturl.URL, name string, value *neturl.URL, usage string) {
	f.Var(newURLValue(value, p), name, usage)
}
func URLVar(p *neturl.URL, name string, value *neturl.URL, usage string) {
	CommandLine.URLVar(p, name, value, usage)
}
func (f *FlagSet) URL(name string, value *neturl.URL, usage string) *neturl.URL {
	p := new(neturl.URL)
	f.URLVar(p, name, value, usage)
	return p
}
func URL(name string, value *neturl.URL, usage string) *neturl.URL {
	return CommandLine.URL(name, value, usage)
}

func (f *FlagSet) UUIDVar(p *uuid.UUID, name string, value uuid.UUID, usage string) {
	f.Var(newUUIDValue(value, p), name, usage)
}
func UUIDVar(p *uuid.UUID, name string, value uuid.UUID, usage string) {
	CommandLine.UUIDVar(p, name, value, usage)
}
func (f *FlagSet) UUID(name string, value uuid.UUID, usage string) *uuid.UUID {
	p := new(uuid.UUID)
	f.UUIDVar(p, name, value, usage)
	return p
}
func UUID(name string, value uuid.UUID, usage string) *uuid.UUID {
	return CommandLine.UUID(name, value, usage)
}

func (f *FlagSet) BigIntVar(p *big.Int, name string, value *big.Int, usage string) {
	if value == nil {
		value = big.NewInt(0)
	}
	f.Var(newBigIntValue(value, p), name, usage)
}
func BigIntVar(p *big.Int, name string, value *big.Int, usage string) {
	CommandLine.BigIntVar(p, name, value, usage)
}
func (f *FlagSet) BigInt(name string, value *big.Int, usage string) *big.Int {
	p := new(big.Int)
	f.BigIntVar(p, name, value, usage)
	return p
}
func BigInt(name string, value *big.Int, usage string) *big.Int {
	return CommandLine.BigInt(name, value, usage)
}

func (f *FlagSet) BigRatVar(p *big.Rat, name string, value *big.Rat, usage string) {
	if value == nil {
		value = big.NewRat(0, 1)
	}
	f.Var(newBigRatValue(value, p), name, usage)
}
func BigRatVar(p *big.Rat, name string, value *big.Rat, usage string) {
	CommandLine.BigRatVar(p, name, value, usage)
}
func (f *FlagSet) BigRat(name string, value *big.Rat, usage string) *big.Rat {
	p := new(big.Rat)
	f.BigRatVar(p, name, value, usage)
	return p
}
func BigRat(name string, value *big.Rat, usage string) *big.Rat {
	return CommandLine.BigRat(name, value, usage)
}

func (f *FlagSet) RegexpVar(p **regexp.Regexp, name string, value *regexp.Regexp, usage string) {
	f.Var(newRegexpValue(value, p), name, usage)
}
func RegexpVar(p **regexp.Regexp, name string, value *regexp.Regexp, usage string) {
	CommandLine.RegexpVar(p, name, value, usage)
}
func (f *FlagSet) Regexp(name string, value *regexp.Regexp, usage string) **regexp.Regexp {
	p := new(*regexp.Regexp)
	f.RegexpVar(p, name, value, usage)
	return p
}
func Regexp(name string, value *regexp.Regexp, usage string) **regexp.Regexp {
	return CommandLine.Regexp(name, value, usage)
}

func (f *FlagSet) StringSliceVar(p *[]string, name, sep string, value []string, usage string) {
	if sep == "" {
		sep = ","
	}
	f.Var(newStringSliceValue(value, sep, p), name, usage)
}
func StringSliceVar(p *[]string, name, sep string, value []string, usage string) {
	CommandLine.StringSliceVar(p, name, sep, value, usage)
}
func (f *FlagSet) StringSlice(name, sep string, value []string, usage string) *[]string {
	p := new([]string)
	f.StringSliceVar(p, name, sep, value, usage)
	return p
}
func StringSlice(name, sep string, value []string, usage string) *[]string {
	return CommandLine.StringSlice(name, sep, value, usage)
}

func (f *FlagSet) DurationSliceVar(p *[]time.Duration, name, sep string, value []time.Duration, usage string) {
	if sep == "" {
		sep = ","
	}
	f.Var(newDurationSliceValue(value, sep, p), name, usage)
}
func DurationSliceVar(p *[]time.Duration, name, sep string, value []time.Duration, usage string) {
	CommandLine.DurationSliceVar(p, name, sep, value, usage)
}
func (f *FlagSet) DurationSlice(name, sep string, value []time.Duration, usage string) *[]time.Duration {
	p := new([]time.Duration)
	f.DurationSliceVar(p, name, sep, value, usage)
	return p
}
func DurationSlice(name, sep string, value []time.Duration, usage string) *[]time.Duration {
	return CommandLine.DurationSlice(name, sep, value, usage)
}

func (f *FlagSet) StringMapVar(p *map[string]string, name string, value map[string]string, usage string) {
	f.Var(newStringMapValue(value, p), name, usage)
}
func StringMapVar(p *map[string]string, name string, value map[string]string, usage string) {
	CommandLine.StringMapVar(p, name, value, usage)
}
func (f *FlagSet) StringMap(name string, value map[string]string, usage string) *map[string]string {
	p := new(map[string]string)
	f.StringMapVar(p, name, value, usage)
	return p
}
func StringMap(name string, value map[string]string, usage string) *map[string]string {
	return CommandLine.StringMap(name, value, usage)
}

func (f *FlagSet) JSONVar(p *json.RawMessage, name string, value json.RawMessage, usage string) {
	f.Var(newJSONValue(value, p), name, usage)
}
func JSONVar(p *json.RawMessage, name string, value json.RawMessage, usage string) {
	CommandLine.JSONVar(p, name, value, usage)
}
func (f *FlagSet) JSON(name string, value json.RawMessage, usage string) *json.RawMessage {
	p := new(json.RawMessage)
	f.JSONVar(p, name, value, usage)
	return p
}
func JSON(name string, value json.RawMessage, usage string) *json.RawMessage {
	return CommandLine.JSON(name, value, usage)
}

// EnumVar registers an enum string flag restricted to the provided allowed values.
func (f *FlagSet) EnumVar(p *string, name string, value string, allowed []string, usage string) {
	// Normalize allowed list (trim spaces)
	norm := make([]string, 0, len(allowed))
	for _, a := range allowed {
		a = strings.TrimSpace(a)
		if a != "" {
			norm = append(norm, a)
		}
	}
	f.Var(newEnumStringValue(value, norm, p), name, usage)
}
func EnumVar(p *string, name string, value string, allowed []string, usage string) {
	CommandLine.EnumVar(p, name, value, allowed, usage)
}
func (f *FlagSet) Enum(name string, value string, allowed []string, usage string) *string {
	p := new(string)
	f.EnumVar(p, name, value, allowed, usage)
	return p
}
func Enum(name string, value string, allowed []string, usage string) *string {
	return CommandLine.Enum(name, value, allowed, usage)
}

// Value is the interface to the dynamic value stored in a flag.
// (The default value is represented as a string.)
//
// If a Value has an IsBoolFlag() bool method returning true,
// the command-line parser makes -name equivalent to -name=true
// rather than using the next command-line argument.
//
// Set is called once, in command line order, for each flag present.
type Value interface {
	String() string
	Set(string) error
}

// Getter is an interface that allows the contents of a Value to be retrieved.
// It wraps the Value interface, rather than being part of it, because it
// appeared after Go 1 and its compatibility rules. All Value types provided
// by this package satisfy the Getter interface.
type Getter interface {
	Value
	Get() interface{}
}

// Var defines a flag with the specified name and usage string. The type and
// value of the flag are represented by the first argument, of type Value, which
// typically holds a user-defined implementation of Value. For instance, the
// caller could create a flag that turns a comma-separated string into a slice
// of strings by giving the slice the methods of Value; in particular, Set would
// decompose the comma-separated string into the slice.
func (f *FlagSet) Var(value Value, name string, usage string) {
	// Remember the default value as a string; it won't change.
	flag := &Flag{name, usage, value, value.String()}
	_, alreadythere := f.formal[name]
	if alreadythere {
		var msg string
		if f.name == "" {
			msg = fmt.Sprintf("flag redefined: %s", name)
		} else {
			msg = fmt.Sprintf("%s flag redefined: %s", f.name, name)
		}
		fmt.Fprintln(f.out(), msg)
		panic(msg) // Happens only if flags are declared with identical names
	}
	if f.formal == nil {
		f.formal = make(map[string]*Flag)
	}
	f.formal[name] = flag
}

// Var defines a flag with the specified name and usage string. The type and
// value of the flag are represented by the first argument, of type Value, which
// typically holds a user-defined implementation of Value. For instance, the
// caller could create a flag that turns a comma-separated string into a slice
// of strings by giving the slice the methods of Value; in particular, Set would
// decompose the comma-separated string into the slice.
func Var(value Value, name string, usage string) { CommandLine.Var(value, name, usage) }

// failf prints to standard error a formatted error and usage message and
// returns the error.
func (f *FlagSet) failf(format string, a ...interface{}) error {
	err := fmt.Errorf(format, a...)
	fmt.Fprintln(f.out(), err)
	f.usage()
	return err
}

// usage calls the Usage method for the flag set if one is specified,
// or the appropriate default usage function otherwise.
func (f *FlagSet) usage() {
	if f.Usage == nil {
		if f == CommandLine {
			Usage()
		} else {
			defaultUsage(f)
		}
	} else {
		f.Usage()
	}
}

// parseOne parses one flag. It reports whether a flag was seen.
func (f *FlagSet) parseOne() (bool, error) {
	if len(f.args) == 0 {
		return false, nil
	}
	s := f.args[0]
	if len(s) == 0 || s[0] != '-' || len(s) == 1 {
		return false, nil
	}
	numMinuses := 1
	if s[1] == '-' {
		numMinuses++
		if len(s) == 2 { // "--" terminates the flags
			f.args = f.args[1:]
			return false, nil
		}
	}
	name := s[numMinuses:]
	if len(name) == 0 || name[0] == '-' || name[0] == '=' {
		return false, f.failf("bad flag syntax: %s", s)
	}
	// ignore go test flags
	if strings.HasPrefix(name, "test.") {
		return false, nil
	}
	// it's a flag. does it have an argument?
	f.args = f.args[1:]
	hasValue := false
	value := ""
	for i := 1; i < len(name); i++ { // equals cannot be first
		if name[i] == '=' {
			value = name[i+1:]
			hasValue = true
			name = name[0:i]
			break
		}
	}
	m := f.formal
	flag, alreadythere := m[name]
	if !alreadythere {
		if name == "help" || name == "h" {
			f.usage()
			return false, ErrHelp
		}
		return false, f.failf("flag provided but not defined: -%s", name)
	}
	if fv, ok := flag.Value.(boolFlag); ok && fv.IsBoolFlag() { // special case: doesn't need an arg
		if hasValue {
			if err := fv.Set(value); err != nil {
				return false, f.failf("invalid boolean value %q for -%s: %v", value, name, err)
			}
		} else {
			if err := fv.Set("true"); err != nil {
				return false, f.failf("invalid boolean flag %s: %v", name, err)
			}
		}
	} else {
		// It must have a value, which might be the next argument.
		if !hasValue && len(f.args) > 0 {
			hasValue = true
			value, f.args = f.args[0], f.args[1:]
		}
		if !hasValue {
			return false, f.failf("flag needs an argument: -%s", name)
		}
		if err := flag.Value.Set(value); err != nil {
			return false, f.failf("invalid value %q for flag -%s: %v", value, name, err)
		}
	}
	if f.actual == nil {
		f.actual = make(map[string]*Flag)
	}
	f.actual[name] = flag
	return true, nil
}

// Parse parses flag definitions from the argument list, which should not
// include the command name. Must be called after all flags in the FlagSet
// are defined and before flags are accessed by the program.
// The return value will be ErrHelp if -help or -h were set but not defined.
func (f *FlagSet) Parse(arguments []string) error {
	f.parsed = true
	f.args = arguments
	for {
		seen, err := f.parseOne()
		if seen {
			continue
		}
		if err == nil {
			break
		}
		switch f.errorHandling {
		case ContinueOnError:
			return err
		case ExitOnError:
			os.Exit(2)
		case PanicOnError:
			panic(err)
		}
	}
	if err := f.ParseEnv(os.Environ()); err != nil {
		switch f.errorHandling {
		case ContinueOnError:
			return err
		case ExitOnError:
			os.Exit(2)
		case PanicOnError:
			panic(err)
		}
		return err
	}
	var cFile string
	if cf := f.formal[DefaultConfigFlagname]; cf != nil {
		cFile = cf.Value.String()
	}
	if cf := f.actual[DefaultConfigFlagname]; cf != nil {
		cFile = cf.Value.String()
	}
	if cFile != "" {
		if err := f.ParseFile(cFile); err != nil {
			switch f.errorHandling {
			case ContinueOnError:
				return err
			case ExitOnError:
				os.Exit(2)
			case PanicOnError:
				panic(err)
			}
			return err
		}
	}
	return nil
}

// Parsed reports whether f.Parse has been called.
func (f *FlagSet) Parsed() bool { return f.parsed }

// Parse parses the command-line flags from os.Args[1:].  Must be called
// after all flags are defined and before flags are accessed by the program.
func Parse() { CommandLine.Parse(os.Args[1:]) }

// Parsed reports whether the command-line flags have been parsed.
func Parsed() bool { return CommandLine.Parsed() }

// CommandLine is the default set of command-line flags, parsed from os.Args.
// The top-level functions such as BoolVar, Arg, and so on are wrappers for the
// methods of CommandLine.
var CommandLine = NewFlagSet(os.Args[0], ExitOnError)

// NewFlagSet returns a new, empty flag set with the specified name and
// error handling property.
func NewFlagSet(name string, errorHandling ErrorHandling) *FlagSet {
	f := &FlagSet{name: name, errorHandling: errorHandling}
	return f
}

// Init sets the name and error handling property for a flag set.
// By default, the zero FlagSet uses an empty name, EnvironmentPrefix, and the
// ContinueOnError error handling policy.
func (f *FlagSet) Init(name string, errorHandling ErrorHandling) {
	f.name = name
	f.envPrefix = EnvironmentPrefix
	f.errorHandling = errorHandling
}

// ErrorHandling defines how FlagSet.Parse behaves if the parse fails.
type ErrorHandling int

// These constants cause FlagSet.Parse to behave as described if the parse fails.
const (
	ContinueOnError ErrorHandling = iota // Return a descriptive error.
	ExitOnError                          // Call os.Exit(2).
	PanicOnError                         // Call panic with a descriptive error.
)

// A FlagSet represents a set of defined flags. The zero value of a FlagSet
// has no name and has ContinueOnError error handling.
type FlagSet struct {
	// Usage is the function called when an error occurs while parsing flags.
	// The field is a function (not a method) that may be changed to point to
	// a custom error handler.
	Usage func()

	name          string
	parsed        bool
	actual        map[string]*Flag
	formal        map[string]*Flag
	envPrefix     string   // prefix to all env variable names
	args          []string // arguments after flags
	errorHandling ErrorHandling
	output        io.Writer // nil means stderr; use out() accessor
}

// A Flag represents the state of a flag.
type Flag struct {
	Name     string // name as it appears on command line
	Usage    string // help message
	Value    Value  // value as set
	DefValue string // default value (as text); for usage message
}

// sortFlags returns the flags as a slice in lexicographical sorted order.
func sortFlags(flags map[string]*Flag) []*Flag {
	list := make(sort.StringSlice, len(flags))
	i := 0
	for _, f := range flags {
		list[i] = f.Name
		i++
	}
	list.Sort()
	result := make([]*Flag, len(list))
	for i, name := range list {
		result[i] = flags[name]
	}
	return result
}

func (f *FlagSet) out() io.Writer {
	if f.output == nil {
		return os.Stderr
	}
	return f.output
}

// SetOutput sets the destination for usage and error messages.
// If output is nil, os.Stderr is used.
func (f *FlagSet) SetOutput(output io.Writer) {
	f.output = output
}

// VisitAll visits the flags in lexicographical order, calling fn for each.
// It visits all flags, even those not set.
func (f *FlagSet) VisitAll(fn func(*Flag)) {
	for _, flag := range sortFlags(f.formal) {
		fn(flag)
	}
}

// VisitAll visits the command-line flags in lexicographical order, calling
// fn for each. It visits all flags, even those not set.
func VisitAll(fn func(*Flag)) {
	CommandLine.VisitAll(fn)
}

// Visit visits the flags in lexicographical order, calling fn for each.
// It visits only those flags that have been set.
func (f *FlagSet) Visit(fn func(*Flag)) {
	for _, flag := range sortFlags(f.actual) {
		fn(flag)
	}
}

// Visit visits the command-line flags in lexicographical order, calling fn
// for each. It visits only those flags that have been set.
func Visit(fn func(*Flag)) {
	CommandLine.Visit(fn)
}

// Lookup returns the Flag structure of the named flag, returning nil if none exists.
func (f *FlagSet) Lookup(name string) *Flag {
	return f.formal[name]
}

// Lookup returns the Flag structure of the named command-line flag,
// returning nil if none exists.
func Lookup(name string) *Flag {
	return CommandLine.formal[name]
}

// Set sets the value of the named flag.
func (f *FlagSet) Set(name, value string) error {
	flag, ok := f.formal[name]
	if !ok {
		return fmt.Errorf("no such flag -%v", name)
	}
	err := flag.Value.Set(value)
	if err != nil {
		return err
	}
	if f.actual == nil {
		f.actual = make(map[string]*Flag)
	}
	f.actual[name] = flag
	return nil
}

// Set sets the value of the named command-line flag.
func Set(name, value string) error {
	return CommandLine.Set(name, value)
}

// isZeroValue guesses whether the string represents the zero
// value for a flag. It is not accurate but in practice works OK.
func isZeroValue(flag *Flag, value string) bool {
	// Build a zero value of the flag's Value type, and see if the
	// result of calling its String method equals the value passed in.
	// This works unless the Value type is itself an interface type.
	typ := reflect.TypeOf(flag.Value)
	var z reflect.Value
	if typ.Kind() == reflect.Ptr {
		z = reflect.New(typ.Elem())
	} else {
		z = reflect.Zero(typ)
	}
	if value == z.Interface().(Value).String() {
		return true
	}

	switch value {
	case "false":
		return true
	case "":
		return true
	case "0":
		return true
	}
	return false
}

// UnquoteUsage extracts a back-quoted name from the usage
// string for a flag and returns it and the un-quoted usage.
// Given "a `name` to show" it returns ("name", "a name to show").
// If there are no back quotes, the name is an educated guess of the
// type of the flag's value, or the empty string if the flag is boolean.
func UnquoteUsage(flag *Flag) (name string, usage string) {
	// Look for a back-quoted name, but avoid the strings package.
	usage = flag.Usage
	for i := 0; i < len(usage); i++ {
		if usage[i] == '`' {
			for j := i + 1; j < len(usage); j++ {
				if usage[j] == '`' {
					name = usage[i+1 : j]
					usage = usage[:i] + name + usage[j+1:]
					return name, usage
				}
			}
			break // Only one back quote; use type name.
		}
	}
	// No explicit name, so use type if we can find one.
	name = "value"
	switch flag.Value.(type) {
	case boolFlag:
		name = ""
	case *durationValue:
		name = "duration"
	case *float64Value:
		name = "float"
	case *intValue, *int64Value:
		name = "int"
	case *stringValue:
		name = "string"
	case *uintValue, *uint64Value:
		name = "uint"
	}
	return
}

// PrintDefaults prints to standard error the default values of all
// defined command-line flags in the set. See the documentation for
// the global function PrintDefaults for more information.
func (f *FlagSet) PrintDefaults() {
	f.VisitAll(func(flag *Flag) {
		s := fmt.Sprintf("  -%s", flag.Name) // Two spaces before -; see next two comments.
		name, usage := UnquoteUsage(flag)
		if len(name) > 0 {
			s += " " + name
		}
		// Boolean flags of one ASCII letter are so common we
		// treat them specially, putting their usage on the same line.
		if len(s) <= 4 { // space, space, '-', 'x'.
			s += "\t"
		} else {
			// Four spaces before the tab triggers good alignment
			// for both 4- and 8-space tab stops.
			s += "\n    \t"
		}
		s += usage
		if !isZeroValue(flag, flag.DefValue) {
			if _, ok := flag.Value.(*stringValue); ok {
				// put quotes on the value
				s += fmt.Sprintf(" (default %q)", flag.DefValue)
			} else {
				s += fmt.Sprintf(" (default %v)", flag.DefValue)
			}
		}
		fmt.Fprint(f.out(), s, "\n")
	})
}

// PrintDefaults prints, to standard error unless configured otherwise,
// a usage message showing the default settings of all defined
// command-line flags.
// For an integer valued flag x, the default output has the form
//
//	-x int
//		usage-message-for-x (default 7)
//
// The usage message will appear on a separate line for anything but
// a bool flag with a one-byte name. For bool flags, the type is
// omitted and if the flag name is one byte the usage message appears
// on the same line. The parenthetical default is omitted if the
// default is the zero value for the type. The listed type, here int,
// can be changed by placing a back-quoted name in the flag's usage
// string; the first such item in the message is taken to be a parameter
// name to show in the message and the back quotes are stripped from
// the message when displayed. For instance, given
//
//	flag.String("I", "", "search `directory` for include files")
//
// the output will be
//
//	-I directory
//		search directory for include files.
func PrintDefaults() {
	CommandLine.PrintDefaults()
}

// defaultUsage is the default function to print a usage message.
func defaultUsage(f *FlagSet) {
	if f.name == "" {
		fmt.Fprintf(f.out(), "Usage:\n")
	} else {
		fmt.Fprintf(f.out(), "Usage of %s:\n", f.name)
	}
	f.PrintDefaults()
}

// NOTE: Usage is not just defaultUsage(CommandLine)
// because it serves (via godoc flag Usage) as the example
// for how to write your own usage function.

// Usage prints to standard error a usage message documenting all defined command-line flags.
// It is called when an error occurs while parsing flags.
// The function is a variable that may be changed to point to a custom function.
// By default it prints a simple header and calls PrintDefaults; for details about the
// format of the output and how to control it, see the documentation for PrintDefaults.
var Usage = func() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	PrintDefaults()
}

// NFlag returns the number of flags that have been set.
func (f *FlagSet) NFlag() int { return len(f.actual) }

// NFlag returns the number of command-line flags that have been set.
func NFlag() int { return len(CommandLine.actual) }

// Arg returns the i'th argument. Arg(0) is the first remaining argument
// after flags have been processed. Arg returns an empty string if the
// requested element does not exist.
func (f *FlagSet) Arg(i int) string {
	if i < 0 || i >= len(f.args) {
		return ""
	}
	return f.args[i]
}

// Arg returns the i'th command-line argument. Arg(0) is the first remaining argument
// after flags have been processed. Arg returns an empty string if the
// requested element does not exist.
func Arg(i int) string {
	return CommandLine.Arg(i)
}

// NArg is the number of arguments remaining after flags have been processed.
func (f *FlagSet) NArg() int { return len(f.args) }

// NArg is the number of arguments remaining after flags have been processed.
func NArg() int { return len(CommandLine.args) }

// Args returns the non-flag arguments.
func (f *FlagSet) Args() []string { return f.args }

// Args returns the non-flag command-line arguments.
func Args() []string { return CommandLine.args }

// BoolVar defines a bool flag with specified name, default value, and usage string.
// The argument p points to a bool variable in which to store the value of the flag.
func (f *FlagSet) BoolVar(p *bool, name string, value bool, usage string) {
	f.Var(newBoolValue(value, p), name, usage)
}

// BoolVar defines a bool flag with specified name, default value, and usage string.
// The argument p points to a bool variable in which to store the value of the flag.
func BoolVar(p *bool, name string, value bool, usage string) {
	CommandLine.Var(newBoolValue(value, p), name, usage)
}

// Bool defines a bool flag with specified name, default value, and usage string.
// The return value is the address of a bool variable that stores the value of the flag.
func (f *FlagSet) Bool(name string, value bool, usage string) *bool {
	p := new(bool)
	f.BoolVar(p, name, value, usage)
	return p
}

// Bool defines a bool flag with specified name, default value, and usage string.
// The return value is the address of a bool variable that stores the value of the flag.
func Bool(name string, value bool, usage string) *bool {
	return CommandLine.Bool(name, value, usage)
}

// IntVar defines an int flag with specified name, default value, and usage string.
// The argument p points to an int variable in which to store the value of the flag.
func (f *FlagSet) IntVar(p *int, name string, value int, usage string) {
	f.Var(newIntValue(value, p), name, usage)
}

// IntVar defines an int flag with specified name, default value, and usage string.
// The argument p points to an int variable in which to store the value of the flag.
func IntVar(p *int, name string, value int, usage string) {
	CommandLine.Var(newIntValue(value, p), name, usage)
}

// Int defines an int flag with specified name, default value, and usage string.
// The return value is the address of an int variable that stores the value of the flag.
func (f *FlagSet) Int(name string, value int, usage string) *int {
	p := new(int)
	f.IntVar(p, name, value, usage)
	return p
}

// Int defines an int flag with specified name, default value, and usage string.
// The return value is the address of an int variable that stores the value of the flag.
func Int(name string, value int, usage string) *int {
	return CommandLine.Int(name, value, usage)
}

// Int64Var defines an int64 flag with specified name, default value, and usage string.
// The argument p points to an int64 variable in which to store the value of the flag.
func (f *FlagSet) Int64Var(p *int64, name string, value int64, usage string) {
	f.Var(newInt64Value(value, p), name, usage)
}

// Int64Var defines an int64 flag with specified name, default value, and usage string.
// The argument p points to an int64 variable in which to store the value of the flag.
func Int64Var(p *int64, name string, value int64, usage string) {
	CommandLine.Var(newInt64Value(value, p), name, usage)
}

// Int64 defines an int64 flag with specified name, default value, and usage string.
// The return value is the address of an int64 variable that stores the value of the flag.
func (f *FlagSet) Int64(name string, value int64, usage string) *int64 {
	p := new(int64)
	f.Int64Var(p, name, value, usage)
	return p
}

// Int64 defines an int64 flag with specified name, default value, and usage string.
// The return value is the address of an int64 variable that stores the value of the flag.
func Int64(name string, value int64, usage string) *int64 {
	return CommandLine.Int64(name, value, usage)
}

// UintVar defines a uint flag with specified name, default value, and usage string.
// The argument p points to a uint variable in which to store the value of the flag.
func (f *FlagSet) UintVar(p *uint, name string, value uint, usage string) {
	f.Var(newUintValue(value, p), name, usage)
}

// UintVar defines a uint flag with specified name, default value, and usage string.
// The argument p points to a uint  variable in which to store the value of the flag.
func UintVar(p *uint, name string, value uint, usage string) {
	CommandLine.Var(newUintValue(value, p), name, usage)
}

// Uint defines a uint flag with specified name, default value, and usage string.
// The return value is the address of a uint  variable that stores the value of the flag.
func (f *FlagSet) Uint(name string, value uint, usage string) *uint {
	p := new(uint)
	f.UintVar(p, name, value, usage)
	return p
}

// Uint defines a uint flag with specified name, default value, and usage string.
// The return value is the address of a uint  variable that stores the value of the flag.
func Uint(name string, value uint, usage string) *uint {
	return CommandLine.Uint(name, value, usage)
}

// Uint64Var defines a uint64 flag with specified name, default value, and usage string.
// The argument p points to a uint64 variable in which to store the value of the flag.
func (f *FlagSet) Uint64Var(p *uint64, name string, value uint64, usage string) {
	f.Var(newUint64Value(value, p), name, usage)
}

// Uint64Var defines a uint64 flag with specified name, default value, and usage string.
// The argument p points to a uint64 variable in which to store the value of the flag.
func Uint64Var(p *uint64, name string, value uint64, usage string) {
	CommandLine.Var(newUint64Value(value, p), name, usage)
}

// Uint64 defines a uint64 flag with specified name, default value, and usage string.
// The return value is the address of a uint64 variable that stores the value of the flag.
func (f *FlagSet) Uint64(name string, value uint64, usage string) *uint64 {
	p := new(uint64)
	f.Uint64Var(p, name, value, usage)
	return p
}

// Uint64 defines a uint64 flag with specified name, default value, and usage string.
// The return value is the address of a uint64 variable that stores the value of the flag.
func Uint64(name string, value uint64, usage string) *uint64 {
	return CommandLine.Uint64(name, value, usage)
}

// StringVar defines a string flag with specified name, default value, and usage string.
// The argument p points to a string variable in which to store the value of the flag.
func (f *FlagSet) StringVar(p *string, name string, value string, usage string) {
	f.Var(newStringValue(value, p), name, usage)
}

// StringVar defines a string flag with specified name, default value, and usage string.
// The argument p points to a string variable in which to store the value of the flag.
func StringVar(p *string, name string, value string, usage string) {
	CommandLine.Var(newStringValue(value, p), name, usage)
}

// String defines a string flag with specified name, default value, and usage string.
// The return value is the address of a string variable that stores the value of the flag.
func (f *FlagSet) String(name string, value string, usage string) *string {
	p := new(string)
	f.StringVar(p, name, value, usage)
	return p
}

// String defines a string flag with specified name, default value, and usage string.
// The return value is the address of a string variable that stores the value of the flag.
func String(name string, value string, usage string) *string {
	return CommandLine.String(name, value, usage)
}

// Float64Var defines a float64 flag with specified name, default value, and usage string.
// The argument p points to a float64 variable in which to store the value of the flag.
func (f *FlagSet) Float64Var(p *float64, name string, value float64, usage string) {
	f.Var(newFloat64Value(value, p), name, usage)
}

// Float64Var defines a float64 flag with specified name, default value, and usage string.
// The argument p points to a float64 variable in which to store the value of the flag.
func Float64Var(p *float64, name string, value float64, usage string) {
	CommandLine.Var(newFloat64Value(value, p), name, usage)
}

// Float64 defines a float64 flag with specified name, default value, and usage string.
// The return value is the address of a float64 variable that stores the value of the flag.
func (f *FlagSet) Float64(name string, value float64, usage string) *float64 {
	p := new(float64)
	f.Float64Var(p, name, value, usage)
	return p
}

// Float64 defines a float64 flag with specified name, default value, and usage string.
// The return value is the address of a float64 variable that stores the value of the flag.
func Float64(name string, value float64, usage string) *float64 {
	return CommandLine.Float64(name, value, usage)
}

// DurationVar defines a time.Duration flag with specified name, default value, and usage string.
// The argument p points to a time.Duration variable in which to store the value of the flag.
// The flag accepts a value acceptable to time.ParseDuration.
func (f *FlagSet) DurationVar(p *time.Duration, name string, value time.Duration, usage string) {
	f.Var(newDurationValue(value, p), name, usage)
}

// DurationVar defines a time.Duration flag with specified name, default value, and usage string.
// The argument p points to a time.Duration variable in which to store the value of the flag.
// The flag accepts a value acceptable to time.ParseDuration.
func DurationVar(p *time.Duration, name string, value time.Duration, usage string) {
	CommandLine.Var(newDurationValue(value, p), name, usage)
}

// Duration defines a time.Duration flag with specified name, default value, and usage string.
// The return value is the address of a time.Duration variable that stores the value of the flag.
// The flag accepts a value acceptable to time.ParseDuration.
func (f *FlagSet) Duration(name string, value time.Duration, usage string) *time.Duration {
	p := new(time.Duration)
	f.DurationVar(p, name, value, usage)
	return p
}

// Duration defines a time.Duration flag with specified name, default value, and usage string.
// The return value is the address of a time.Duration variable that stores the value of the flag.
// The flag accepts a value acceptable to time.ParseDuration.
func Duration(name string, value time.Duration, usage string) *time.Duration {
	return CommandLine.Duration(name, value, usage)
}
