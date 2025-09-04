package flag

import (
	"encoding/json"
	"fmt"
	"net"
	neturl "net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	decimal "github.com/shopspring/decimal"
)

// FieldHandler registers a flag for a struct field. It should apply the default value
// (considering required/default tags) and call the appropriate Var/XXXVar function.
// It returns true if it handled the field type.
type FieldHandler func(ctx *StructFieldContext) (handled bool, err error)

// StructFieldContext provides information & helpers for a struct field registration.
type StructFieldContext struct {
	FS         *FlagSet
	Field      reflect.StructField
	Value      reflect.Value
	FlagName   string
	Help       string
	Required   bool
	Sensitive  bool
	Deprecated string
	DefaultTag string
	Tags       map[string]string // raw tag values (layout, sep, enum, etc.)
}

var (
	structTypeHandlers = make(map[reflect.Type]FieldHandler)
)

// RegisterStructHandler allows users to plug in custom struct field handling for
// ParseStruct. The handler is invoked before built-in logic. If it returns
// (handled=true) no further processing occurs for that field.
//
// Typical usage (example: base64-decoded string field):
//
//	type B64String string
//
//	func init() {
//	    flag.RegisterStructHandler(reflect.TypeOf(B64String("")), func(ctx *flag.StructFieldContext) (bool, error) {
//	        def := string(ctx.Value.String())
//	        if ctx.DefaultTag != "" { def = ctx.DefaultTag }
//	        // store raw default first
//	        if err := flag.CommandLine.Set(ctx.FlagName, def); err != nil { return true, err }
//	        // but ParseStruct registers via XXXVar helpers; emulate:
//	        p := (*string)(ctx.Value.Addr().Interface().(*B64String))
//	        flag.StringVar(p, ctx.FlagName, def, ctx.Help)
//	        return true, nil
//	    })
//	}
//
// Handlers run before legacy switch/case fallback. If multiple handlers are
// registered for the same concrete type, the last wins.
func RegisterStructHandler(t reflect.Type, h FieldHandler) { structTypeHandlers[t] = h }

// tryHandleStructField attempts to locate a handler for the field's concrete type.
func tryHandleStructField(ctx *StructFieldContext) (bool, error) {
	if h, ok := structTypeHandlers[ctx.Field.Type]; ok {
		return h(ctx)
	}
	return false, nil
}

// init registers built-in handlers replicating existing ParseStruct switch logic.
func init() {
	// time.Time
	RegisterStructHandler(reflect.TypeOf(time.Time{}), func(ctx *StructFieldContext) (bool, error) {
		layout := ctx.Tags["layout"]
		if layout == "" {
			layout = time.RFC3339
		}
		def := ctx.Value.Interface().(time.Time)
		if ctx.Required {
			def = time.Time{}
		} else if ctx.DefaultTag != "" {
			v, err := time.Parse(layout, ctx.DefaultTag)
			if err != nil {
				return true, fmt.Errorf("invalid default time %q: %v", ctx.DefaultTag, err)
			}
			def = v
		}
		TimeVar(ctx.Value.Addr().Interface().(*time.Time), ctx.FlagName, layout, def, ctx.Help)
		return true, nil
	})
	// decimal.Decimal
	RegisterStructHandler(reflect.TypeOf(decimal.Decimal{}), func(ctx *StructFieldContext) (bool, error) {
		def := ctx.Value.Interface().(decimal.Decimal)
		if ctx.Required {
			def = decimal.Decimal{}
		} else if ctx.DefaultTag != "" {
			d, err := decimal.NewFromString(ctx.DefaultTag)
			if err != nil {
				return true, fmt.Errorf("invalid default decimal %q: %v", ctx.DefaultTag, err)
			}
			def = d
		}
		DecimalVar(ctx.Value.Addr().Interface().(*decimal.Decimal), ctx.FlagName, def, ctx.Help)
		return true, nil
	})
	// net.IP
	RegisterStructHandler(reflect.TypeOf(net.IP(nil)), func(ctx *StructFieldContext) (bool, error) {
		def := ctx.Value.Interface().(net.IP)
		if ctx.Required {
			def = nil
		} else if ctx.DefaultTag != "" {
			ip := net.ParseIP(ctx.DefaultTag)
			if ip == nil {
				return true, fmt.Errorf("invalid default ip %q", ctx.DefaultTag)
			}
			def = ip
		}
		IPVar(ctx.Value.Addr().Interface().(*net.IP), ctx.FlagName, def, ctx.Help)
		return true, nil
	})
	// net.IPNet
	RegisterStructHandler(reflect.TypeOf(net.IPNet{}), func(ctx *StructFieldContext) (bool, error) {
		def := ctx.Value.Interface().(net.IPNet)
		if ctx.Required {
			def = net.IPNet{}
		} else if ctx.DefaultTag != "" {
			_, n, err := net.ParseCIDR(ctx.DefaultTag)
			if err != nil {
				return true, fmt.Errorf("invalid default cidr %q: %v", ctx.DefaultTag, err)
			}
			def = *n
		}
		IPNetVar(ctx.Value.Addr().Interface().(*net.IPNet), ctx.FlagName, &def, ctx.Help)
		return true, nil
	})
	// url.URL
	RegisterStructHandler(reflect.TypeOf(neturl.URL{}), func(ctx *StructFieldContext) (bool, error) {
		def := ctx.Value.Interface().(neturl.URL)
		if ctx.Required {
			def = neturl.URL{}
		} else if ctx.DefaultTag != "" {
			u, err := neturl.Parse(ctx.DefaultTag)
			if err != nil {
				return true, fmt.Errorf("invalid default url %q: %v", ctx.DefaultTag, err)
			}
			def = *u
		}
		URLVar(ctx.Value.Addr().Interface().(*neturl.URL), ctx.FlagName, &def, ctx.Help)
		return true, nil
	})
	// uuid.UUID
	RegisterStructHandler(reflect.TypeOf(uuid.UUID{}), func(ctx *StructFieldContext) (bool, error) {
		def := ctx.Value.Interface().(uuid.UUID)
		if ctx.Required {
			def = uuid.UUID{}
		} else if ctx.DefaultTag != "" {
			id, err := uuid.Parse(ctx.DefaultTag)
			if err != nil {
				return true, fmt.Errorf("invalid default uuid %q: %v", ctx.DefaultTag, err)
			}
			def = id
		}
		UUIDVar(ctx.Value.Addr().Interface().(*uuid.UUID), ctx.FlagName, def, ctx.Help)
		return true, nil
	})
	// ByteSize
	RegisterStructHandler(reflect.TypeOf(ByteSize(0)), func(ctx *StructFieldContext) (bool, error) {
		def := ctx.Value.Interface().(ByteSize)
		if ctx.Required {
			def = 0
		} else if ctx.DefaultTag != "" {
			bs, err := parseByteSize(ctx.DefaultTag)
			if err != nil {
				return true, fmt.Errorf("invalid default bytesize %q: %v", ctx.DefaultTag, err)
			}
			def = bs
		}
		ByteSizeVar(ctx.Value.Addr().Interface().(*ByteSize), ctx.FlagName, def, ctx.Help)
		return true, nil
	})
	// []time.Duration
	RegisterStructHandler(reflect.TypeOf([]time.Duration(nil)), func(ctx *StructFieldContext) (bool, error) {
		sep := ctx.Tags["sep"]
		if sep == "" {
			sep = ","
		}
		def := ctx.Value.Interface().([]time.Duration)
		if ctx.Required {
			def = nil
		} else if ctx.DefaultTag != "" {
			parts := strings.Split(ctx.DefaultTag, sep)
			tmp := make([]time.Duration, 0, len(parts))
			for _, p := range parts {
				d, err := time.ParseDuration(strings.TrimSpace(p))
				if err != nil {
					return true, fmt.Errorf("invalid default duration slice element %q: %v", p, err)
				}
				tmp = append(tmp, d)
			}
			def = tmp
		}
		DurationSliceVar(ctx.Value.Addr().Interface().(*[]time.Duration), ctx.FlagName, sep, def, ctx.Help)
		return true, nil
	})
	// []string
	RegisterStructHandler(reflect.TypeOf([]string(nil)), func(ctx *StructFieldContext) (bool, error) {
		sep := ctx.Tags["sep"]
		if sep == "" {
			sep = ","
		}
		def := ctx.Value.Interface().([]string)
		if ctx.Required {
			def = nil
		} else if ctx.DefaultTag != "" {
			parts := strings.Split(ctx.DefaultTag, sep)
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}
			def = parts
		}
		StringSliceVar(ctx.Value.Addr().Interface().(*[]string), ctx.FlagName, sep, def, ctx.Help)
		return true, nil
	})
	// map[string]string
	RegisterStructHandler(reflect.TypeOf(map[string]string(nil)), func(ctx *StructFieldContext) (bool, error) {
		def := ctx.Value.Interface().(map[string]string)
		if ctx.Required {
			def = nil
		} else if ctx.DefaultTag != "" {
			m := make(map[string]string)
			for _, pair := range strings.Split(ctx.DefaultTag, ",") {
				if strings.TrimSpace(pair) == "" {
					continue
				}
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) != 2 {
					return true, fmt.Errorf("invalid default map entry %q", pair)
				}
				m[kv[0]] = kv[1]
			}
			def = m
		}
		StringMapVar(ctx.Value.Addr().Interface().(*map[string]string), ctx.FlagName, def, ctx.Help)
		return true, nil
	})
	// json.RawMessage
	RegisterStructHandler(reflect.TypeOf(json.RawMessage{}), func(ctx *StructFieldContext) (bool, error) {
		def := ctx.Value.Interface().(json.RawMessage)
		if ctx.Required {
			def = json.RawMessage{}
		} else if ctx.DefaultTag != "" {
			jm := json.RawMessage([]byte(ctx.DefaultTag))
			var tmp interface{}
			if err := json.Unmarshal(jm, &tmp); err != nil {
				return true, fmt.Errorf("invalid default json %q: %v", ctx.DefaultTag, err)
			}
			def = jm
		}
		JSONVar(ctx.Value.Addr().Interface().(*json.RawMessage), ctx.FlagName, def, ctx.Help)
		return true, nil
	})
	// *regexp.Regexp (represented as pointer type in struct)
	RegisterStructHandler(reflect.TypeOf((*regexp.Regexp)(nil)), func(ctx *StructFieldContext) (bool, error) {
		def := ctx.Value.Interface().(*regexp.Regexp)
		if ctx.Required {
			def = nil
		} else if ctx.DefaultTag != "" {
			r, err := regexp.Compile(ctx.DefaultTag)
			if err != nil {
				return true, fmt.Errorf("invalid default regexp %q: %v", ctx.DefaultTag, err)
			}
			def = r
		}
		RegexpVar(ctx.Value.Addr().Interface().(**regexp.Regexp), ctx.FlagName, def, ctx.Help)
		return true, nil
	})
	// numeric & primitive kinds registered via exact type mapping
	RegisterStructHandler(reflect.TypeOf(true), func(ctx *StructFieldContext) (bool, error) {
		def := ctx.Value.Bool()
		if ctx.Required {
			def = false
		} else if ctx.DefaultTag != "" {
			b, err := strconv.ParseBool(ctx.DefaultTag)
			if err != nil {
				return true, fmt.Errorf("invalid default bool %q: %v", ctx.DefaultTag, err)
			}
			def = b
		}
		BoolVar(ctx.Value.Addr().Interface().(*bool), ctx.FlagName, def, ctx.Help)
		return true, nil
	})
	RegisterStructHandler(reflect.TypeOf(int(0)), func(ctx *StructFieldContext) (bool, error) {
		def := ctx.Value.Int()
		if ctx.Required {
			def = 0
		} else if ctx.DefaultTag != "" {
			iv, err := strconv.ParseInt(ctx.DefaultTag, 0, 64)
			if err != nil {
				return true, fmt.Errorf("invalid default int %q: %v", ctx.DefaultTag, err)
			}
			def = iv
		}
		IntVar(ctx.Value.Addr().Interface().(*int), ctx.FlagName, int(def), ctx.Help)
		return true, nil
	})
	RegisterStructHandler(reflect.TypeOf(int64(0)), func(ctx *StructFieldContext) (bool, error) {
		// time.Duration is handled separately by type; this catch-all covers other int64 fields
		if ctx.Field.Type == reflect.TypeOf(time.Duration(0)) { // handled as duration
			d := ctx.Value.Interface().(time.Duration)
			if ctx.Required {
				d = 0
			} else if ctx.DefaultTag != "" {
				dv, err := time.ParseDuration(ctx.DefaultTag)
				if err != nil {
					return true, fmt.Errorf("invalid default duration %q: %v", ctx.DefaultTag, err)
				}
				d = dv
			}
			DurationVar(ctx.Value.Addr().Interface().(*time.Duration), ctx.FlagName, d, ctx.Help)
			return true, nil
		}
		def := ctx.Value.Int()
		if ctx.Required {
			def = 0
		} else if ctx.DefaultTag != "" {
			iv, err := strconv.ParseInt(ctx.DefaultTag, 0, 64)
			if err != nil {
				return true, fmt.Errorf("invalid default int64 %q: %v", ctx.DefaultTag, err)
			}
			def = iv
		}
		Int64Var(ctx.Value.Addr().Interface().(*int64), ctx.FlagName, def, ctx.Help)
		return true, nil
	})
	RegisterStructHandler(reflect.TypeOf(uint(0)), func(ctx *StructFieldContext) (bool, error) {
		def := ctx.Value.Uint()
		if ctx.Required {
			def = 0
		} else if ctx.DefaultTag != "" {
			uv, err := strconv.ParseUint(ctx.DefaultTag, 0, 64)
			if err != nil {
				return true, fmt.Errorf("invalid default uint %q: %v", ctx.DefaultTag, err)
			}
			def = uv
		}
		UintVar(ctx.Value.Addr().Interface().(*uint), ctx.FlagName, uint(def), ctx.Help)
		return true, nil
	})
	RegisterStructHandler(reflect.TypeOf(uint64(0)), func(ctx *StructFieldContext) (bool, error) {
		def := ctx.Value.Uint()
		if ctx.Required {
			def = 0
		} else if ctx.DefaultTag != "" {
			uv, err := strconv.ParseUint(ctx.DefaultTag, 0, 64)
			if err != nil {
				return true, fmt.Errorf("invalid default uint64 %q: %v", ctx.DefaultTag, err)
			}
			def = uv
		}
		Uint64Var(ctx.Value.Addr().Interface().(*uint64), ctx.FlagName, def, ctx.Help)
		return true, nil
	})
	RegisterStructHandler(reflect.TypeOf(""), func(ctx *StructFieldContext) (bool, error) {
		def := ctx.Value.String()
		if enumList := ctx.Tags["enum"]; enumList != "" {
			allowed := strings.Split(enumList, ",")
			for i := range allowed {
				allowed[i] = strings.TrimSpace(allowed[i])
			}
			if ctx.Required {
				def = ""
			} else if ctx.DefaultTag != "" {
				def = ctx.DefaultTag
			}
			EnumVar(ctx.Value.Addr().Interface().(*string), ctx.FlagName, def, allowed, ctx.Help)
			return true, nil
		}
		if ctx.Required {
			def = ""
		} else if ctx.DefaultTag != "" {
			def = ctx.DefaultTag
		}
		StringVar(ctx.Value.Addr().Interface().(*string), ctx.FlagName, def, ctx.Help)
		return true, nil
	})
	RegisterStructHandler(reflect.TypeOf(float64(0)), func(ctx *StructFieldContext) (bool, error) {
		def := ctx.Value.Float()
		if ctx.Required {
			def = 0
		} else if ctx.DefaultTag != "" {
			fv, err := strconv.ParseFloat(ctx.DefaultTag, 64)
			if err != nil {
				return true, fmt.Errorf("invalid default float64 %q: %v", ctx.DefaultTag, err)
			}
			def = fv
		}
		Float64Var(ctx.Value.Addr().Interface().(*float64), ctx.FlagName, def, ctx.Help)
		return true, nil
	})
}
