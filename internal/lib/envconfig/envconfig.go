package envconfig

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var durationType = reflect.TypeOf(time.Duration(0))

func Load(dst any) error {
	return LoadFrom(os.LookupEnv, dst)
}

func LoadFrom(lookup func(string) (string, bool), dst any) error {
	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		return errors.New("envconfig: destination must be a pointer to a struct")
	}
	var problems []error
	fill(lookup, v.Elem(), &problems)
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("invalid environment: %w", errors.Join(problems...))
}

func fill(lookup func(string) (string, bool), v reflect.Value, problems *[]error) {
	t := v.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		name := field.Tag.Get("env")
		if name == "" {
			if field.Type.Kind() == reflect.Struct && field.Type != durationType {
				fill(lookup, v.Field(i), problems)
			}
			continue
		}
		raw, ok := lookup(name)
		if !ok || raw == "" {
			def, hasDefault := field.Tag.Lookup("default")
			switch {
			case hasDefault:
				raw = def
			case field.Tag.Get("required") == "true":
				*problems = append(*problems, fmt.Errorf("%s is required", name))
				continue
			default:
				continue
			}
		}
		if err := set(v.Field(i), raw); err != nil {
			*problems = append(*problems, fmt.Errorf("%s: %w", name, err))
		}
	}
}

func set(f reflect.Value, raw string) error {
	if f.Type() == durationType {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return fmt.Errorf("invalid duration %q", raw)
		}
		f.SetInt(int64(d))
		return nil
	}
	switch f.Kind() {
	case reflect.String:
		f.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("invalid bool %q", raw)
		}
		f.SetBool(b)
	case reflect.Int, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, f.Type().Bits())
		if err != nil {
			return fmt.Errorf("invalid integer %q", raw)
		}
		f.SetInt(n)
	case reflect.Float64:
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Errorf("invalid number %q", raw)
		}
		f.SetFloat(n)
	case reflect.Slice:
		if f.Type().Elem().Kind() != reflect.String {
			return fmt.Errorf("unsupported type %s", f.Type())
		}
		var items []string
		for _, part := range strings.Split(raw, ",") {
			if part = strings.TrimSpace(part); part != "" {
				items = append(items, part)
			}
		}
		f.Set(reflect.ValueOf(items))
	default:
		return fmt.Errorf("unsupported type %s", f.Type())
	}
	return nil
}
