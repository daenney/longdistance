package json

import (
	"bytes"
	"encoding/json"
)

type RawMessage = json.RawMessage
type Object map[string]RawMessage
type Array []RawMessage

var Compact = json.Compact
var Marshal = json.Marshal
var MarshalIndent = json.MarshalIndent
var Valid = json.Valid
var Unmarshal = json.Unmarshal

var (
	beginArray  = byte('[')
	beginObject = byte('{')
	beginString = byte('"')
	null        = RawMessage(`null`)
)

func IsNull(in RawMessage) bool {
	return bytes.Equal(in, null)
}

func IsArray(in RawMessage) bool {
	if len(in) == 0 {
		return false
	}
	return in[0] == beginArray
}

func IsMap(in RawMessage) bool {
	if len(in) == 0 {
		return false
	}
	return in[0] == beginObject
}

func IsString(in RawMessage) bool {
	if len(in) == 0 {
		return false
	}
	return in[0] == beginString
}

func IsScalar(in RawMessage) bool {
	return !IsArray(in) && !IsMap(in) && !IsNull(in)
}

func MakeArray(in RawMessage) RawMessage {
	if len(in) == 0 {
		return json.RawMessage(`[]`)
	}

	if IsArray(in) {
		return in
	}

	return bytes.Join([][]byte{
		[]byte(`[`),
		in,
		[]byte(`]`),
	}, nil)
}
