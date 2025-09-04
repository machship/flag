package flag_test

import (
	"encoding/json"
	"math/big"
	"net"
	urlpkg "net/url"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	. "github.com/machship/flag"
	decimal "github.com/shopspring/decimal"
)

// TestWrapperVarFunctions ensures top-level *Var helpers execute.
func TestWrapperVarFunctions(t *testing.T) {
	ResetForTesting(nil)
	var bs ByteSize
	ByteSizeVar(&bs, "bs", 0, "")
	var tm time.Time
	TimeVar(&tm, "tm", time.RFC3339, time.Time{}, "")
	dec := decimal.NewFromInt(0)
	DecimalVar(&dec, "dec", dec, "")
	var ip net.IP
	IPVar(&ip, "ip", nil, "")
	var ipn net.IPNet
	IPNetVar(&ipn, "ipn", nil, "")
	var u urlpkg.URL
	URLVar(&u, "url", nil, "")
	var id uuid.UUID
	UUIDVar(&id, "uuid", uuid.New(), "")
	bi := new(big.Int)
	BigIntVar(bi, "bigint", nil, "")
	br := new(big.Rat)
	BigRatVar(br, "bigrat", nil, "")
	var rx *regexp.Regexp
	RegexpVar(&rx, "rx", nil, "")
	var ss []string
	StringSliceVar(&ss, "ss", ",", nil, "")
	var ds []time.Duration
	DurationSliceVar(&ds, "ds", ",", nil, "")
	mp := map[string]string{}
	StringMapVar(&mp, "mp", nil, "")
	var jm json.RawMessage
	JSONVar(&jm, "js", nil, "")
	var enum string
	EnumVar(&enum, "enum", "one", []string{"one", "two"}, "")
	// basic sanity: ensure flags registered
	if Lookup("enum") == nil {
		t.Fatalf("expected enum flag registered")
	}
}
