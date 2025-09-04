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

// prefix stack for nested struct flagPrefix handling
var prefixStack []string

func pushPrefix(p string) {
	if p == "" {
		return
	}
	prefixStack = append(prefixStack, p)
}
func popPrefix() {
	if len(prefixStack) > 0 {
		prefixStack = prefixStack[:len(prefixStack)-1]
	}
}
func currentPrefix() string {
	if len(prefixStack) == 0 {
		return ""
	}
	return strings.Join(prefixStack, ".")
}

/*
    In this file, we are going to define a way of users providing a struct that we can use to resolve flags.
	The idea will be that the user can provide a struct with the following field tags:
	- `flag:"name"`: the name of the flag
	- `default:"value"`: the default value of the flag
	- `help:"message"`: the help message for the flag
	- `required:"true"`: mark the flag as required (in which case the `default` value will be ignored)

	We will expose the function `ParseStruct` which will take a pointer to a struct and parse the flags accordingly.

	If the user has not provided any of the required fields when ParseStruct is called, we will return an error indicating which fields are missing.
*/

// internal validation helpers
func checkMin(v reflect.Value, minTag, name string) error {
	if minTag == "" {
		return nil
	}
	min, err := strconv.ParseFloat(minTag, 64)
	if err != nil {
		return fmt.Errorf("invalid min tag for %s: %v", name, err)
	}
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if float64(v.Int()) < min {
			return fmt.Errorf("flag %s: value %d < min %s", name, v.Int(), minTag)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if float64(v.Uint()) < min {
			return fmt.Errorf("flag %s: value %d < min %s", name, v.Uint(), minTag)
		}
	case reflect.Float32, reflect.Float64:
		if v.Float() < min {
			return fmt.Errorf("flag %s: value %v < min %s", name, v.Float(), minTag)
		}
	case reflect.String, reflect.Slice, reflect.Map:
		if float64(v.Len()) < min {
			return fmt.Errorf("flag %s: length %d < min %s", name, v.Len(), minTag)
		}
	}
	return nil
}
func checkMax(v reflect.Value, maxTag, name string) error {
	if maxTag == "" {
		return nil
	}
	max, err := strconv.ParseFloat(maxTag, 64)
	if err != nil {
		return fmt.Errorf("invalid max tag for %s: %v", name, err)
	}
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if float64(v.Int()) > max {
			return fmt.Errorf("flag %s: value %d > max %s", name, v.Int(), maxTag)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if float64(v.Uint()) > max {
			return fmt.Errorf("flag %s: value %d > max %s", name, v.Uint(), maxTag)
		}
	case reflect.Float32, reflect.Float64:
		if v.Float() > max {
			return fmt.Errorf("flag %s: value %v > max %s", name, v.Float(), maxTag)
		}
	case reflect.String, reflect.Slice, reflect.Map:
		if float64(v.Len()) > max {
			return fmt.Errorf("flag %s: length %d > max %s", name, v.Len(), maxTag)
		}
	}
	return nil
}
func checkPattern(v reflect.Value, pat, name string) error {
	if pat == "" {
		return nil
	}
	if v.Kind() != reflect.String {
		return nil
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return fmt.Errorf("invalid pattern tag for %s: %v", name, err)
	}
	if !re.MatchString(v.String()) {
		return fmt.Errorf("flag %s: value %q does not match pattern %s", name, v.String(), pat)
	}
	return nil
}

// ParseStructOptions controls ParseStruct behavior.
type ParseStructOptions struct{ AutoParse bool }

// ParseStructWithOptions allows disabling automatic final Parse().
func ParseStructWithOptions(s any, opts ParseStructOptions) error {
	return parseStructInternal(s, opts)
}

// ParseStruct preserves legacy behavior (auto parse).
func ParseStruct(s any) error { return parseStructInternal(s, ParseStructOptions{AutoParse: true}) }

func parseStructInternal(s any, opts ParseStructOptions) error {
	v := reflect.ValueOf(s)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("ParseStruct expects a non-nil pointer to a struct, got %T", s)
	}
	if Parsed() && opts.AutoParse {
		return fmt.Errorf("ParseStruct must be called before flag.Parse()")
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("ParseStruct expects a pointer to a struct, got %T", s)
	}
	t := v.Type()
	var requiredFlags []string
	regErr := func(fname string, err error) error { return fmt.Errorf("ParseStruct: field %s: %w", fname, err) }
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		} // unexported
		flagName := field.Tag.Get("flag")
		// Nested struct support: if no flag tag but it's a struct, recurse (without auto-parsing).
		if flagName == "" {
			if field.Type.Kind() == reflect.Struct {
				fv := v.Field(i)
				if fv.Kind() == reflect.Struct && fv.CanAddr() {
					if err := parseStructInternal(fv.Addr().Interface(), ParseStructOptions{AutoParse: false}); err != nil {
						return err
					}
				}
			}
			continue
		}
		help := field.Tag.Get("help")
		required := strings.EqualFold(field.Tag.Get("required"), "true")
		sensitiveTag := strings.EqualFold(field.Tag.Get("sensitive"), "true")
		deprecatedTag := field.Tag.Get("deprecated") // if set, note deprecation after registration
		defTag := field.Tag.Get("default")
		fv := v.Field(i)
		// Build context for registry
		ctx := &StructFieldContext{
			FS:         CommandLine,
			Field:      field,
			Value:      fv,
			FlagName:   flagName,
			Help:       help,
			Required:   required,
			Sensitive:  sensitiveTag,
			Deprecated: deprecatedTag,
			DefaultTag: defTag,
			Tags: map[string]string{
				"layout": field.Tag.Get("layout"),
				"sep":    field.Tag.Get("sep"),
				"enum":   field.Tag.Get("enum"),
			},
		}
		if handled, hErr := tryHandleStructField(ctx); hErr != nil {
			return regErr(field.Name, hErr)
		} else if handled {
			if required {
				requiredFlags = append(requiredFlags, flagName)
			}
			if deprecatedTag != "" {
				Deprecate(flagName, deprecatedTag)
			}
			if sensitiveTag {
				CommandLine.MarkSensitive(flagName)
			}
			goto VALIDATION_TAGS
		}
		// Fallback legacy explicit concrete types first
		switch field.Type {
		case reflect.TypeOf(time.Time{}):
			layout := field.Tag.Get("layout")
			if layout == "" {
				layout = time.RFC3339
			}
			def := fv.Interface().(time.Time)
			if required {
				def = time.Time{}
			} else if defTag != "" {
				tv, err := time.Parse(layout, defTag)
				if err != nil {
					return regErr(field.Name, fmt.Errorf("invalid default time %q: %v", defTag, err))
				}
				def = tv
			}
			TimeVar(fv.Addr().Interface().(*time.Time), flagName, layout, def, help)
		case reflect.TypeOf(decimal.Decimal{}):
			def := fv.Interface().(decimal.Decimal)
			if required {
				def = decimal.Decimal{}
			} else if defTag != "" {
				d, err := decimal.NewFromString(defTag)
				if err != nil {
					return regErr(field.Name, fmt.Errorf("invalid default decimal %q: %v", defTag, err))
				}
				def = d
			}
			DecimalVar(fv.Addr().Interface().(*decimal.Decimal), flagName, def, help)
		case reflect.TypeOf(net.IP(nil)):
			def := fv.Interface().(net.IP)
			if required {
				def = nil
			} else if defTag != "" {
				ip := net.ParseIP(defTag)
				if ip == nil {
					return regErr(field.Name, fmt.Errorf("invalid default ip %q", defTag))
				}
				def = ip
			}
			IPVar(fv.Addr().Interface().(*net.IP), flagName, def, help)
		case reflect.TypeOf(net.IPNet{}):
			def := fv.Interface().(net.IPNet)
			if required {
				def = net.IPNet{}
			} else if defTag != "" {
				_, n, err := net.ParseCIDR(defTag)
				if err != nil {
					return regErr(field.Name, fmt.Errorf("invalid default cidr %q: %v", defTag, err))
				}
				def = *n
			}
			IPNetVar(fv.Addr().Interface().(*net.IPNet), flagName, &def, help)
		case reflect.TypeOf(neturl.URL{}):
			def := fv.Interface().(neturl.URL)
			if required {
				def = neturl.URL{}
			} else if defTag != "" {
				u, err := neturl.Parse(defTag)
				if err != nil {
					return regErr(field.Name, fmt.Errorf("invalid default url %q: %v", defTag, err))
				}
				def = *u
			}
			URLVar(fv.Addr().Interface().(*neturl.URL), flagName, &def, help)
		case reflect.TypeOf(uuid.UUID{}):
			def := fv.Interface().(uuid.UUID)
			if required {
				def = uuid.UUID{}
			} else if defTag != "" {
				id, err := uuid.Parse(defTag)
				if err != nil {
					return regErr(field.Name, fmt.Errorf("invalid default uuid %q: %v", defTag, err))
				}
				def = id
			}
			UUIDVar(fv.Addr().Interface().(*uuid.UUID), flagName, def, help)
		case reflect.TypeOf(ByteSize(0)):
			def := fv.Interface().(ByteSize)
			if required {
				def = 0
			} else if defTag != "" {
				bs, err := parseByteSize(defTag)
				if err != nil {
					return regErr(field.Name, fmt.Errorf("invalid default bytesize %q: %v", defTag, err))
				}
				def = bs
			}
			ByteSizeVar(fv.Addr().Interface().(*ByteSize), flagName, def, help)
		case reflect.TypeOf([]time.Duration(nil)):
			sep := field.Tag.Get("sep")
			if sep == "" {
				sep = ","
			}
			def := fv.Interface().([]time.Duration)
			if required {
				def = nil
			} else if defTag != "" {
				parts := strings.Split(defTag, sep)
				tmp := make([]time.Duration, 0, len(parts))
				for _, p := range parts {
					d, err := time.ParseDuration(strings.TrimSpace(p))
					if err != nil {
						return regErr(field.Name, fmt.Errorf("invalid default duration slice element %q: %v", p, err))
					}
					tmp = append(tmp, d)
				}
				def = tmp
			}
			DurationSliceVar(fv.Addr().Interface().(*[]time.Duration), flagName, sep, def, help)
		case reflect.TypeOf([]string(nil)):
			sep := field.Tag.Get("sep")
			if sep == "" {
				sep = ","
			}
			flagName := field.Tag.Get("flag")
			if flagName != "" {
				if pf := currentPrefix(); pf != "" {
					flagName = pf + "." + flagName
				}
			}
			def := fv.Interface().([]string)
			if required {
				def = nil
			} else if defTag != "" {
				parts := strings.Split(defTag, sep)
				for i := range parts {
					parts[i] = strings.TrimSpace(parts[i])
				}
				def = parts
			}
			StringSliceVar(fv.Addr().Interface().(*[]string), flagName, sep, def, help)
		case reflect.TypeOf(map[string]string(nil)):
			def := fv.Interface().(map[string]string)
			if required {
				def = nil
			} else if defTag != "" {
				m := make(map[string]string)
				for _, pair := range strings.Split(defTag, ",") {
					if strings.TrimSpace(pair) == "" {
						continue
					}
					kv := strings.SplitN(pair, "=", 2)
					if len(kv) != 2 {
						return regErr(field.Name, fmt.Errorf("invalid default map entry %q", pair))
					}
					m[kv[0]] = kv[1]
				}
				def = m
			}
			StringMapVar(fv.Addr().Interface().(*map[string]string), flagName, def, help)
		case reflect.TypeOf(json.RawMessage{}):
			def := fv.Interface().(json.RawMessage)
			if required {
				def = json.RawMessage{}
			} else if defTag != "" {
				jm := json.RawMessage([]byte(defTag))
				var tmp interface{}
				if err := json.Unmarshal(jm, &tmp); err != nil {
					return regErr(field.Name, fmt.Errorf("invalid default json %q: %v", defTag, err))
				}
				def = jm
			}
			JSONVar(fv.Addr().Interface().(*json.RawMessage), flagName, def, help)
		case reflect.TypeOf((*regexp.Regexp)(nil)):
			def := fv.Interface().(*regexp.Regexp)
			if required {
				def = nil
			} else if defTag != "" {
				r, err := regexp.Compile(defTag)
				if err != nil {
					return regErr(field.Name, fmt.Errorf("invalid default regexp %q: %v", defTag, err))
				}
				def = r
			}
			RegexpVar(fv.Addr().Interface().(**regexp.Regexp), flagName, def, help)
		default:
			// Fall back on kind
			switch fv.Kind() {
			case reflect.Bool:
				def := fv.Bool()
				if required {
					def = false
				} else if defTag != "" {
					b, err := strconv.ParseBool(defTag)
					if err != nil {
						return regErr(field.Name, fmt.Errorf("invalid default bool %q: %v", defTag, err))
					}
					def = b
				}
				BoolVar(fv.Addr().Interface().(*bool), flagName, def, help)
			case reflect.Int:
				def := fv.Int()
				if required {
					def = 0
				} else if defTag != "" {
					iv, err := strconv.ParseInt(defTag, 0, 64)
					if err != nil {
						return regErr(field.Name, fmt.Errorf("invalid default int %q: %v", defTag, err))
					}
					def = iv
				}
				IntVar(fv.Addr().Interface().(*int), flagName, int(def), help)
			case reflect.Int64:
				if field.Type == reflect.TypeOf(time.Duration(0)) {
					d := fv.Interface().(time.Duration)
					if required {
						d = 0
					} else if defTag != "" {
						dv, err := time.ParseDuration(defTag)
						if err != nil {
							return regErr(field.Name, fmt.Errorf("invalid default duration %q: %v", defTag, err))
						}
						d = dv
					}
					DurationVar(fv.Addr().Interface().(*time.Duration), flagName, d, help)
				} else {
					def := fv.Int()
					if required {
						def = 0
					} else if defTag != "" {
						iv, err := strconv.ParseInt(defTag, 0, 64)
						if err != nil {
							return regErr(field.Name, fmt.Errorf("invalid default int64 %q: %v", defTag, err))
						}
						def = iv
					}
					Int64Var(fv.Addr().Interface().(*int64), flagName, def, help)
				}
			case reflect.Uint:
				def := fv.Uint()
				if required {
					def = 0
				} else if defTag != "" {
					uv, err := strconv.ParseUint(defTag, 0, 64)
					if err != nil {
						return regErr(field.Name, fmt.Errorf("invalid default uint %q: %v", defTag, err))
					}
					def = uv
				}
				UintVar(fv.Addr().Interface().(*uint), flagName, uint(def), help)
			case reflect.Uint64:
				def := fv.Uint()
				if required {
					def = 0
				} else if defTag != "" {
					uv, err := strconv.ParseUint(defTag, 0, 64)
					if err != nil {
						return regErr(field.Name, fmt.Errorf("invalid default uint64 %q: %v", defTag, err))
					}
					def = uv
				}
				Uint64Var(fv.Addr().Interface().(*uint64), flagName, def, help)
			case reflect.String:
				def := fv.String()
				if enumList := field.Tag.Get("enum"); enumList != "" {
					allowed := strings.Split(enumList, ",")
					for i := range allowed {
						allowed[i] = strings.TrimSpace(allowed[i])
					}
					if required {
						def = ""
					} else if defTag != "" {
						def = defTag
					}
					EnumVar(fv.Addr().Interface().(*string), flagName, def, allowed, help)
				} else {
					if required {
						def = ""
					} else if defTag != "" {
						def = defTag
					}
					StringVar(fv.Addr().Interface().(*string), flagName, def, help)
				}
			case reflect.Float64:
				def := fv.Float()
				if required {
					def = 0
				} else if defTag != "" {
					fv2, err := strconv.ParseFloat(defTag, 64)
					if err != nil {
						return regErr(field.Name, fmt.Errorf("invalid default float64 %q: %v", defTag, err))
					}
					def = fv2
				}
				Float64Var(fv.Addr().Interface().(*float64), flagName, def, help)
			default:
				return regErr(field.Name, fmt.Errorf("unsupported field type %s for flag %q", field.Type.String(), flagName))
			}
		}
		if required {
			requiredFlags = append(requiredFlags, flagName)
		}
		if deprecatedTag != "" {
			Deprecate(flagName, deprecatedTag)
		}
		if sensitiveTag {
			CommandLine.MarkSensitive(flagName)
		}
	VALIDATION_TAGS:
		// validation tag capture
		minTag := field.Tag.Get("min")
		maxTag := field.Tag.Get("max")
		patTag := field.Tag.Get("pattern")
		if minTag != "" || maxTag != "" || patTag != "" {
			fname := flagName
			fvCopy := fv.Addr()
			CommandLine.deferredValidations = append(CommandLine.deferredValidations, func() error {
				var m MultiError
				val := fvCopy.Elem()
				if err := checkMin(val, minTag, fname); err != nil {
					m.Append(err)
				}
				if err := checkMax(val, maxTag, fname); err != nil {
					m.Append(err)
				}
				if err := checkPattern(val, patTag, fname); err != nil {
					m.Append(err)
				}
				if m.HasErrors() {
					return &m
				}
				return nil
			})
		}
	}
	if opts.AutoParse && !Parsed() {
		Parse()
	}
	// run deferred validations only if we auto-parsed (otherwise caller will Parse then call Validate manually).
	if opts.AutoParse && len(CommandLine.deferredValidations) > 0 {
		var all MultiError
		for _, fn := range CommandLine.deferredValidations {
			all.Append(fn())
		}
		if all.HasErrors() {
			return &all
		}
	}
	var missing []string
	for _, name := range requiredFlags {
		if CommandLine.actual == nil || CommandLine.actual[name] == nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required flags: %s", strings.Join(missing, ", "))
	}
	return nil
}
