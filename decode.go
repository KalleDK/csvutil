package csvutil

import (
	"encoding"
	"encoding/base64"
	"reflect"
	"strconv"
)

var (
	textUnmarshaler = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
	csvUnmarshaler  = reflect.TypeOf((*Unmarshaler)(nil)).Elem()
)

var intDecoders = map[int]decodeFunc{
	8:  decodeIntN(8),
	16: decodeIntN(16),
	32: decodeIntN(32),
	64: decodeIntN(64),
}

var uintDecoders = map[int]decodeFunc{
	8:  decodeUintN(8),
	16: decodeUintN(16),
	32: decodeUintN(32),
	64: decodeUintN(64),
}

var (
	decodeFloat32 = decodeFloatN(32)
	decodeFloat64 = decodeFloatN(64)
)

type DecodeFunc func(s string, v interface{}) error

type decodeFunc func(s string, v reflect.Value) error

func decodeString(s string, v reflect.Value) error {
	v.SetString(s)
	return nil
}

func decodeIntN(bits int) decodeFunc {
	return func(s string, v reflect.Value) error {
		n, err := strconv.ParseInt(s, 10, bits)
		if err != nil {
			return &UnmarshalTypeError{Value: s, Type: v.Type()}
		}
		v.SetInt(n)
		return nil
	}
}

func decodeUintN(bits int) decodeFunc {
	return func(s string, v reflect.Value) error {
		n, err := strconv.ParseUint(s, 10, bits)
		if err != nil {
			return &UnmarshalTypeError{Value: s, Type: v.Type()}
		}
		v.SetUint(n)
		return nil
	}
}

func decodeFloatN(bits int) decodeFunc {
	return func(s string, v reflect.Value) error {
		n, err := strconv.ParseFloat(s, bits)
		if err != nil {
			return &UnmarshalTypeError{Value: s, Type: v.Type()}
		}
		v.SetFloat(n)
		return nil
	}
}

func decodeBool(s string, v reflect.Value) error {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return &UnmarshalTypeError{Value: s, Type: v.Type()}
	}
	v.SetBool(b)
	return nil
}

func decodePtrTextUnmarshaler(s string, v reflect.Value) error {
	return decodeTextUnmarshaler(s, v.Addr())
}

func decodeTextUnmarshaler(s string, v reflect.Value) error {
	return v.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(s))
}

func decodePtrFieldUnmarshaler(s string, v reflect.Value) error {
	return decodeFieldUnmarshaler(s, v.Addr())
}

func decodeFieldUnmarshaler(s string, v reflect.Value) error {
	return v.Interface().(Unmarshaler).UnmarshalCSV([]byte(s))
}

func decodePtr(typ reflect.Type) (decodeFunc, error) {
	next, err := decodeFn(typ.Elem())
	if err != nil {
		return nil, err
	}

	return func(s string, v reflect.Value) error {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return next(s, v.Elem())
	}, nil
}

func decodeInterface(s string, v reflect.Value) error {
	if v.NumMethod() != 0 {
		return &UnmarshalTypeError{
			Value: s,
			Type:  v.Type(),
		}
	}

	if v.IsNil() {
		v.Set(reflect.ValueOf(s))
		return nil
	}

	el := walkValue(v)
	if !el.CanSet() {
		v.Set(reflect.ValueOf(s))
		return nil
	}

	fn, err := decodeFn(el.Type())
	if err != nil {
		return err
	}
	return fn(s, el)
}

func decodeBytes(s string, v reflect.Value) error {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	v.SetBytes(b)
	return nil
}

func decodeFn(typ reflect.Type) (decodeFunc, error) {
	if reflect.PtrTo(typ).Implements(csvUnmarshaler) {
		return decodePtrFieldUnmarshaler, nil
	}
	if reflect.PtrTo(typ).Implements(textUnmarshaler) {
		return decodePtrTextUnmarshaler, nil
	}

	switch typ.Kind() {
	case reflect.Ptr:
		return decodePtr(typ)
	case reflect.Interface:
		return decodeInterface, nil
	case reflect.String:
		return decodeString, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intDecoders[typ.Bits()], nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return uintDecoders[typ.Bits()], nil
	case reflect.Float32:
		return decodeFloat32, nil
	case reflect.Float64:
		return decodeFloat64, nil
	case reflect.Bool:
		return decodeBool, nil
	case reflect.Slice:
		if typ.Elem().Kind() == reflect.Uint8 {
			return decodeBytes, nil
		}
	}

	return nil, &UnsupportedTypeError{Type: typ}
}
