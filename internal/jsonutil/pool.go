package jsonutil

import (
	"bytes"
	"encoding/json/jsontext"
	"sync"
)

// Decoder is a pooled [jsontext.Decoder] reading from the buffer.
type Decoder struct {
	jsontext.Decoder
	buf bytes.Buffer
}

var decoderPool = sync.Pool{
	New: func() any { return new(Decoder) },
}

// NewDecoder returns a [Decoder] from the pool reading from in.
//
// When true, validated disables duplicate name checking. This is safe
// to do when the value came from a jsontext.Decoder that did enforce
// the duplicate name checks.
//
// Call [Decoder.Release] to return it to the pool.
func NewDecoder(in []byte, validated bool) *Decoder {
	d := decoderPool.Get().(*Decoder)
	d.buf = *bytes.NewBuffer(in) // avoid this escaping by stashing it on the decoder
	d.Decoder.Reset(&d.buf, jsontext.AllowDuplicateNames(validated))

	return d
}

func (d *Decoder) Release() {
	d.buf = bytes.Buffer{} // don't keep the buffer alive
	d.Decoder.Reset(&d.buf)
	decoderPool.Put(d)
}
