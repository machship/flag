package flag_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	decimal "github.com/shopspring/decimal"

	. "github.com/machship/flag"
)

// TestWrapperFunctions exercise all top-level wrapper helpers to raise coverage.
func TestWrapperFunctions(t *testing.T) {
	ResetForTesting(nil)
	// basic primitives
	b := Bool("b", true, "")
	i := Int("i", 5, "")
	i64 := Int64("i64", 6, "")
	u := Uint("u", 7, "")
	u64 := Uint64("u64", 8, "")
	s := String("s", "str", "")
	f := Float64("f", 1.23, "")
	d := Duration("d", 2*time.Second, "")
	_ = b
	_ = i
	_ = i64
	_ = u
	_ = u64
	_ = s
	_ = f
	_ = d
	// extended types
	bs := ByteSizeFlag("bs", 1024, "")
	if *bs != 1024 {
		t.Fatalf("bytesize default mismatch")
	}
	var tme = Time("t", time.RFC3339, time.Time{}, "")
	dec := Decimal("dec", decimalFromString(t, "12.3"), "")
	ip := IP("ip", nil, "")
	ipn := IPNet("ipn", nil, "")
	url := URL("url", nil, "")
	uid := UUID("uid", uuidZeros(t), "")
	bigint := BigInt("bigint", nil, "")
	bigrat := BigRat("bigrat", nil, "")
	rx := Regexp("rx", nil, "")
	ss := StringSlice("ss", ",", []string{"a", "b"}, "")
	ds := DurationSlice("ds", ",", []time.Duration{time.Second}, "")
	sm := StringMap("sm", map[string]string{"k": "v"}, "")
	jm := JSON("jm", jsonRaw(t, `{"x":1}`), "")
	enum := Enum("enm", "apple", []string{"apple", "banana"}, "")
	_ = tme
	_ = dec
	_ = ip
	_ = ipn
	_ = url
	_ = uid
	_ = bigint
	_ = bigrat
	_ = rx
	_ = ss
	_ = ds
	_ = sm
	_ = jm
	_ = enum
}

// helpers (duplicate minimal impl so we don't import unexported parts)
func decimalFromString(t *testing.T, s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}
func uuidZeros(t *testing.T) uuid.UUID               { return uuid.UUID{} }
func jsonRaw(t *testing.T, s string) json.RawMessage { return json.RawMessage([]byte(s)) }
