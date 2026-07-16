package jsonutil

import (
	"bytes"
	"encoding/json/jsontext"
)

type Object = map[string]jsontext.Value
type Array = []jsontext.Value

var (
	emptyArray = []byte(`[]`)
)

func IsNull(in jsontext.Value) bool {
	if len(in) < 4 {
		return false
	}

	return in[0] == byte(jsontext.KindNull)
}

func IsArray(in jsontext.Value) bool {
	if len(in) < 2 {
		return false
	}

	return in[0] == byte(jsontext.KindBeginArray)
}

func IsEmptyArray(in jsontext.Value) bool {
	return bytes.Equal(in, emptyArray)
}

func IsMap(in jsontext.Value) bool {
	if len(in) < 2 {
		return false
	}

	return in[0] == byte(jsontext.KindBeginObject)
}

func IsString(in jsontext.Value) bool {
	if len(in) < 2 {
		return false
	}

	return in[0] == byte(jsontext.KindString)
}

func IsScalar(in jsontext.Value) bool {
	return !IsArray(in) && !IsMap(in) && !IsNull(in)
}

func MakeArray(in jsontext.Value) jsontext.Value {
	if len(in) == 0 {
		return emptyArray
	}

	if IsArray(in) {
		return in
	}

	buf := make([]byte, 0, len(in)+2)
	buf = append(buf, '[')
	buf = append(buf, in...)
	buf = append(buf, ']')

	return buf
}
