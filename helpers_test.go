package longdistance_test

import (
	"bytes"
	"flag"
	"os"
	"testing"

	"code.dny.dev/longdistance/internal/json"
	"github.com/google/go-cmp/cmp"
)

var dump = flag.Bool("dump", false, "dump the compacted or expanded JSON on test failure")

func LoadData(t *testing.T, file string) json.RawMessage {
	t.Helper()

	data, err := os.ReadFile(file)

	if err != nil {
		t.Fatalf("failed to load %s: %s", file, err)
	}

	var res bytes.Buffer
	err = json.Compact(&res, data)
	if err != nil {
		t.Fatalf("invalid JSON in %s: %s", file, err)
	}
	return res.Bytes()
}

// JSONDiff should be used when diffing JSON documents.
func JSONDiff() cmp.Option {
	return cmp.Options{
		cmp.FilterValues(func(x, y json.RawMessage) bool {
			return json.Valid(x) && json.Valid(y)
		}, cmp.Transformer("ParseJSON", func(in json.RawMessage) (out any) {
			if err := json.Unmarshal(in, &out); err != nil {
				panic(err) // should never occur given previous filter to ensure valid JSON
			}
			return out
		})),
	}
}
