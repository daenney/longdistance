package jsonutil

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"fmt"
)

func DecodeString(dec *jsontext.Decoder) (string, bool, error) {
	switch dec.PeekKind() {
	case jsontext.KindNull:
		return "", true, dec.SkipValue()
	case jsontext.KindString:
		value, err := dec.ReadToken()
		if err != nil {
			return "", false, err
		}

		return value.String(), false, nil
	default:
		return "", false, errors.Join(
			fmt.Errorf("unexpected token"),
			dec.SkipValue(),
		)
	}
}

func DecodeFloat64(dec *jsontext.Decoder) (float64, bool, error) {
	switch dec.PeekKind() {
	case jsontext.KindNull:
		return 0, true, dec.SkipValue()
	case jsontext.KindNumber:
		tok, err := dec.ReadToken()
		if err != nil {
			return 0, false, err
		}

		res, err := tok.Float()
		if err != nil {
			return 0, false, err
		}

		return res, false, nil
	default:
		return 0, false, errors.Join(
			fmt.Errorf("unexpected token"),
			dec.SkipValue(),
		)
	}
}

func DecodeBool(dec *jsontext.Decoder) (bool, bool, error) {
	switch dec.PeekKind() {
	case jsontext.KindNull:
		return false, true, dec.SkipValue()
	case jsontext.KindFalse, jsontext.KindTrue:
		tok, err := dec.ReadToken()
		if err != nil {
			return false, false, err
		}

		return tok.Bool(), false, nil
	default:
		return false, false, errors.Join(
			fmt.Errorf("unexpected token"),
			dec.SkipValue(),
		)
	}
}

func DecodeStringSlice(dec *jsontext.Decoder) ([]string, bool, error) {
	switch dec.PeekKind() {
	case jsontext.KindNull:
		return nil, true, dec.SkipValue()
	case jsontext.KindString:
		tok, err := dec.ReadToken()
		if err != nil {
			return nil, false, err
		}

		return []string{tok.String()}, false, nil
	case jsontext.KindBeginArray:
		_, err := dec.ReadToken()
		if err != nil {
			return nil, false, err
		}

		var out []string
		for dec.PeekKind() != jsontext.KindEndArray {
			res, _, err := DecodeString(dec)
			if err != nil {
				return nil, false, err
			}

			out = append(out, res)
		}

		if _, err := dec.ReadToken(); err != nil { // consume ']'
			return nil, false, err
		}

		return out, false, nil
	default:
		return nil, false, errors.Join(
			fmt.Errorf("unexpected token"),
			dec.SkipValue(),
		)
	}
}

func ReadObject(in jsontext.Value) (Object, error) {
	if in.Kind() != jsontext.KindBeginObject {
		return nil, fmt.Errorf("unexpected token")
	}

	dec := jsontext.NewDecoder(bytes.NewReader(in))
	if _, err := dec.ReadToken(); err != nil { // consume '{'
		return nil, err
	}

	obj := make(Object)
	for dec.PeekKind() != jsontext.KindEndObject {
		tok, err := dec.ReadToken()
		if err != nil {
			return nil, err
		}

		key := tok.String()
		val, err := dec.ReadValue()
		if err != nil {
			return nil, err
		}

		obj[key] = val.Clone()
	}

	return obj, nil
}

func ReadStringSlice(in jsontext.Value) ([]string, error) {
	if len(in) == 0 {
		return nil, nil
	}

	res, _, err := DecodeStringSlice(
		jsontext.NewDecoder(bytes.NewBuffer(in)),
	)

	return res, err
}

func ReadValueSlice(in jsontext.Value) ([]jsontext.Value, error) {
	if len(in) == 0 {
		return nil, nil
	}

	dec := jsontext.NewDecoder(bytes.NewBuffer(in))
	switch dec.PeekKind() {
	case jsontext.KindNull:
		return nil, nil
	case jsontext.KindBeginArray:
		if _, err := dec.ReadToken(); err != nil {
			return nil, err
		}

		var out []jsontext.Value
		for dec.PeekKind() != jsontext.KindEndArray {
			val, err := dec.ReadValue()
			if err != nil {
				return nil, err
			}

			out = append(out, val.Clone())
		}

		if _, err := dec.ReadToken(); err != nil {
			return nil, err
		}

		return out, nil
	default:
		val, err := dec.ReadValue()
		return []jsontext.Value{val}, err
	}
}

func ReadString(in jsontext.Value) (string, error) {
	if IsNull(in) {
		return "", nil
	}

	if !IsString(in) {
		return "", fmt.Errorf("unexpected tokenn")
	}

	if IsEmptyString(in) {
		return "", nil
	}

	b, err := jsontext.AppendUnquote(nil, in)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
