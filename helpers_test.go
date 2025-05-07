package longdistance_test

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ld "sourcery.dny.nu/longdistance"
	"sourcery.dny.nu/longdistance/internal/json"
	"sourcery.dny.nu/longdistance/internal/url"
	"github.com/google/go-cmp/cmp"
)

var dump = flag.Bool("dump", false, "dump the compacted or expanded JSON on test failure")

func FileLoader(t *testing.T) ld.RemoteContextLoaderFunc {
	t.Helper()

	return func(_ context.Context, s string) (ld.Document, error) {
		u, err := url.Parse(s)
		if err != nil {
			return ld.Document{}, err
		}

		if u.Scheme != "http" && u.Scheme != "https" {
			return ld.Document{}, ld.ErrLoadingRemoteContext
		}

		data := LoadData(t, filepath.Join(
			filepath.Join("testdata", "w3c"),
			filepath.Join(strings.Split(u.Path, "/")[3:]...),
		))

		var obj map[string]json.RawMessage
		if err := json.Unmarshal(data, &obj); err != nil {
			return ld.Document{}, ld.ErrInvalidRemoteContext
		}

		return ld.Document{
			URL:     s,
			Context: obj[ld.KeywordContext],
		}, nil
	}
}

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
