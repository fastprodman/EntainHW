package envconf

import (
	"encoding"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"time"
)

var (
	ErrMissingRequired = errors.New("missing required environment variable")
	ErrUnsupportedType = errors.New("unsupported field type")
)

// nolint:gocognit
func Load(dst any) error {
	if dst == nil {
		return errors.New("destination is nil")
	}

	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return errors.New("destination must be a non-nil pointer to a struct")
	}

	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return errors.New("destination must point to a struct")
	}

	t := v.Type()
	for i := range v.NumField() {
		sf := t.Field(i)
		fv := v.Field(i)

		if !sf.IsExported() {
			continue
		}

		tag := sf.Tag.Get("env")

		// No tag: recurse into embedded/nested structs (including pointer-to-struct),
		// except time.Duration which is an int64.
		if tag == "-" || tag == "" {
			if fv.Kind() == reflect.Struct && sf.Type != reflect.TypeOf(time.Duration(0)) {
				err := Load(fv.Addr().Interface())
				if err != nil {
					return fmt.Errorf("load recursively %q: %w", sf.Name, err)
				}

				continue
			}

			if fv.Kind() == reflect.Pointer && fv.Type().Elem().Kind() == reflect.Struct {
				if fv.IsNil() {
					fv.Set(reflect.New(fv.Type().Elem()))
				}

				err := Load(fv.Interface())
				if err != nil {
					return fmt.Errorf("load recursively %q: %w", sf.Name, err)
				}

				continue
			}

			continue
		}

		raw, ok := os.LookupEnv(tag)
		if !ok {
			return fmt.Errorf("%w: %s (field %q)", ErrMissingRequired, tag, sf.Name)
		}

		err := setValue(fv, raw)
		if err != nil {
			return fmt.Errorf("parse %q for field %q: %w", tag, sf.Name, err)
		}
	}

	return nil
}

//nolint:gocognit,cyclop
func setValue(fv reflect.Value, raw string) error {
	if !fv.CanSet() {
		return fmt.Errorf("field not settable: %w", ErrUnsupportedType)
	}

	// encoding.TextUnmarshaler support
	if fv.CanAddr() {
		u, ok := fv.Addr().Interface().(encoding.TextUnmarshaler)
		if ok {
			err := u.UnmarshalText([]byte(raw))
			if err != nil {
				return fmt.Errorf("unmarshal text: %w", err)
			}

			return nil
		}
	}

	switch fv.Kind() {
	case reflect.String:
		fv.SetString(raw)

		return nil
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("parse bool: %w", err)
		}

		fv.SetBool(b)

		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if fv.Type() == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(raw)
			if err != nil {
				return fmt.Errorf("parse duration: %w", err)
			}

			fv.SetInt(int64(d))

			return nil
		}

		i, err := strconv.ParseInt(raw, 10, fv.Type().Bits())
		if err != nil {
			return fmt.Errorf("parse int: %w", err)
		}

		fv.SetInt(i)

		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(raw, 10, fv.Type().Bits())
		if err != nil {
			return fmt.Errorf("parse uint: %w", err)
		}

		fv.SetUint(u)

		return nil
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, fv.Type().Bits())
		if err != nil {
			return fmt.Errorf("parse float: %w", err)
		}

		fv.SetFloat(f)

		return nil
	case reflect.Pointer:
		if fv.IsNil() {
			elem := reflect.New(fv.Type().Elem())

			err := setValue(elem.Elem(), raw)
			if err != nil {
				return fmt.Errorf("parse pointer: %w", err)
			}

			fv.Set(elem)

			return nil
		}

		err := setValue(fv.Elem(), raw)
		if err != nil {
			return fmt.Errorf("parse pointer: %w", err)
		}

		return nil
	default:
		return fmt.Errorf("unsupported type: %w", ErrUnsupportedType)
	}
}
