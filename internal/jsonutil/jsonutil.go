package jsonutil

import (
	"bytes"
	"encoding/json/jsontext"
)

type Object = map[string]jsontext.Value
type Array = []jsontext.Value

var (
	emptyArray  = jsontext.Value(`[]`)
	emptyString = jsontext.Value(`""`)
)

func IsNull(in jsontext.Value) bool {
	return in.Kind() == jsontext.KindNull
}

func IsArray(in jsontext.Value) bool {
	return in.Kind() == jsontext.KindBeginArray
}

func IsEmptyArray(in jsontext.Value) bool {
	return bytes.Equal(in, emptyArray)
}

func IsEmptyString(in jsontext.Value) bool {
	return bytes.Equal(in, emptyString)
}

func IsMap(in jsontext.Value) bool {
	return in.Kind() == jsontext.KindBeginObject
}

func IsString(in jsontext.Value) bool {
	return in.Kind() == jsontext.KindString
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

func MakeString(in string) jsontext.Value {
	if len(in) == 0 {
		return emptyString
	}

	buf, _ := jsontext.AppendQuote(make([]byte, 0, len(in)+2), in)

	return buf
}
