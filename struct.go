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

func ParseStruct(s any) error {
	v := reflect.ValueOf(s)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("ParseStruct expects a non-nil pointer to a struct, got %T", s)
	}
	if Parsed() {
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
		if flagName == "" {
			continue
		}
		help := field.Tag.Get("help")
		required := strings.EqualFold(field.Tag.Get("required"), "true")
		defTag := field.Tag.Get("default")
		fv := v.Field(i)
		// Explicit concrete types first
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
	}
	if !Parsed() {
		Parse()
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
