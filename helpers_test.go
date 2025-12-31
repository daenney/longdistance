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

// JSONDiff should be used when diffing JSON-LD documents.
//
// It implements the JSON-LD object comparison algorithm from
// https://w3c.github.io/json-ld-api/tests/#json-ld-object-comparison
func JSONDiff() cmp.Option {
	return cmp.Options{
		cmp.FilterValues(func(x, y json.RawMessage) bool {
			return json.Valid(x) && json.Valid(y)
		}, cmp.Comparer(func(x, y json.RawMessage) bool {
			var xv, yv any
			if err := json.Unmarshal(x, &xv); err != nil {
				return false
			}

			if err := json.Unmarshal(y, &yv); err != nil {
				return false
			}

			return jsonLDEqual(xv, yv)
		})),
	}
}

// jsonLDEqual compares two JSON values using JSON-LD comparison semantics.
func jsonLDEqual(x, y any) bool {
	switch xv := x.(type) {
	case map[string]any:
		yv, ok := y.(map[string]any)
		if !ok || len(xv) != len(yv) {
			return false
		}

		for k, xVal := range xv {
			yVal, ok := yv[k]
			if !ok {
				return false
			}

			switch k {
			case ld.KeywordLanguage:
				if !equalLanguage(xVal, yVal) {
					return false
				}
			case ld.KeywordList:
				if !equalOrdered(xVal, yVal) {
					return false
				}
			default:
				if !jsonLDEqual(xVal, yVal) {
					return false
				}
			}
		}

		return true
	case []any:
		yv, ok := y.([]any)
		if !ok || len(xv) != len(yv) {
			return false
		}

		return equalUnordered(xv, yv)
	case string:
		ys, ok := y.(string)
		return ok && xv == ys
	case float64:
		yf, ok := y.(float64)
		return ok && xv == yf
	case bool:
		yb, ok := y.(bool)
		return ok && xv == yb
	case nil:
		return y == nil
	default:
		return false
	}
}

func equalLanguage(x, y any) bool {
	if x == nil && y == nil {
		return true
	}

	xs, xok := x.(string)
	ys, yok := y.(string)
	return xok && yok && strings.EqualFold(xs, ys)
}

func equalOrdered(x, y any) bool {
	xv, xok := x.([]any)
	yv, yok := y.([]any)
	if !xok || !yok || len(xv) != len(yv) {
		return false
	}

	for i := range xv {
		if !jsonLDEqual(xv[i], yv[i]) {
			return false
		}
	}

	return true
}

func equalUnordered(xv, yv []any) bool {
	for _, xe := range xv {
		found := false
		for _, ye := range yv {
			if jsonLDEqual(xe, ye) {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}
