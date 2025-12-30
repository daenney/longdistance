package longdistance_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	ld "sourcery.dny.nu/longdistance"
)

var dump = flag.Bool("dump", false, "dump the compacted or expanded JSON on test failure")

const ASURL = "https://www.w3.org/ns/activitystreams"

func ProcessContext(tb testing.TB, lctx json.RawMessage, iri string) *ld.Context {
	tb.Helper()
	p := ld.NewProcessor()

	ctx, err := p.Context(bytes.NewReader(lctx), iri)
	if err != nil {
		tb.Fatal(err)
	}

	return ctx
}

func StaticLoader(tb testing.TB, file string) ld.RemoteContextLoaderFunc {
	data := LoadData(tb, file)

	return func(ctx context.Context, s string) (ld.Document, error) {
		if s == ASURL {
			return ld.Document{
				URL:     ASURL,
				Context: data,
			}, nil
		}

		tb.Fatal("unknown remote context")
		return ld.Document{}, nil
	}
}

func FileLoader(tb testing.TB) ld.RemoteContextLoaderFunc {
	tb.Helper()

	return func(_ context.Context, s string) (ld.Document, error) {
		u, err := url.Parse(s)
		if err != nil {
			return ld.Document{}, err
		}

		if u.Scheme != "http" && u.Scheme != "https" {
			return ld.Document{}, ld.ErrLoadingRemoteContext
		}

		data := LoadData(tb, filepath.Join(
			"w3c",
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

func LoadData(t testing.TB, file string) json.RawMessage {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", file))

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
		cmp.FilterValues(func(x, y hjson.RawMessage) bool {
			return json.Valid(x) && json.Valid(y)
		}, cmp.Transformer("ParseJSON", func(in hjson.RawMessage) (out any) {
			if err := hjson.Unmarshal(in, &out); err != nil {
				panic(err) // should never occur given previous filter to ensure valid JSON
			}
			return out
		})),
	}
}
