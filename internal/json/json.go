package json

import (
	"bytes"
	"encoding/json"
)

type RawMessage = json.RawMessage
type Object = map[string]RawMessage
type Array = []RawMessage
type Decoder = json.Decoder
type Token = json.Token
type Delim = json.Delim

var NewDecoder = json.NewDecoder
var NewEncoder = json.NewEncoder
var Marshal = json.Marshal
var Unmarshal = json.Unmarshal

var (
	beginArray  = byte('[')
	beginObject = byte('{')
	beginString = byte('"')
	null        = RawMessage(`null`)

	emptyArray = []byte(`[]`)
)

func IsNull(in RawMessage) bool {
	return bytes.Equal(in, null)
}

func IsArray(in RawMessage) bool {
	if len(in) < 2 {
		return false
	}

	return in[0] == beginArray
}

func IsEmptyArray(in RawMessage) bool {
	return bytes.Equal(in, emptyArray)
}

func IsMap(in RawMessage) bool {
	if len(in) < 2 {
		return false
	}

	return in[0] == beginObject
}

func IsString(in RawMessage) bool {
	if len(in) < 2 {
		return false
	}

	return in[0] == beginString
}

func IsScalar(in RawMessage) bool {
	return !IsArray(in) && !IsMap(in) && !IsNull(in)
}

func MakeArray(in RawMessage) RawMessage {
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
