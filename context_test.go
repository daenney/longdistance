package longdistance_test

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"strings"
	"testing"

	ld "sourcery.dny.nu/longdistance"
)

func TestCacheableContext(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "propagate false",
			in:   "longdistance/context-cache/cacheable/0001/in.jsonld",
			want: "set to false",
		},
		{
			name: "relative vocab",
			in:   "longdistance/context-cache/cacheable/0002/in.jsonld",
			want: "is not absolute",
		},
		{
			name: "relative vocab in a scoped context",
			in:   "longdistance/context-cache/cacheable/0003/in.jsonld",
			want: "is not absolute",
		},
		{
			name: "nullified scoped context",
			in:   "longdistance/context-cache/cacheable/0004/in.jsonld",
			want: "is nullified",
		},
		{
			name: "anything but an object",
			in:   "longdistance/context-cache/cacheable/0005/in.jsonld",
			want: "not a context definition",
		},
		{
			name: "empty object",
			in:   "longdistance/context-cache/cacheable/0006/in.jsonld",
			want: "no terms",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("expected panic, got nothing")
					return
				}

				if msg := fmt.Sprint(r); !strings.Contains(msg, tc.want) {
					t.Errorf("expected panic with %q, got: %s", tc.want, msg)
				}
			}()

			p := ld.NewProcessor()
			p.CacheableContext(t.Context(), "https://example.test/u", bytes.NewBuffer(LoadData(t, tc.in)))
		})
	}
}

func TestProcessedContextIsUsed(t *testing.T) {
	const (
		remoteURL  = "https://example.test/remote"
		remoteURL2 = "https://example.test/remote2"
	)

	tests := []struct {
		name          string
		in            string
		out           string
		contexts      []string
		expandContext string
		err           error
	}{
		{
			name:     "single remote context",
			in:       "longdistance/context-cache/processed-context/0001/in.jsonld",
			contexts: []string{remoteURL, "longdistance/context-cache/processed-context/0001/context-1.jsonld"},
			out:      "longdistance/context-cache/processed-context/0001/out.jsonld",
		},
		{
			name: "two remote contexts",
			in:   "longdistance/context-cache/processed-context/0002/in.jsonld",
			contexts: []string{
				remoteURL, "longdistance/context-cache/processed-context/0002/context-1.jsonld",
				remoteURL2, "longdistance/context-cache/processed-context/0002/context-2.jsonld",
			},
			out: "longdistance/context-cache/processed-context/0002/out.jsonld",
		},
		{
			name:     "remote context with a local extension",
			in:       "longdistance/context-cache/processed-context/0003/in.jsonld",
			contexts: []string{remoteURL, "longdistance/context-cache/processed-context/0003/context-1.jsonld"},
			out:      "longdistance/context-cache/processed-context/0003/out.jsonld",
		},
		{
			name:     "local context before a remote one",
			in:       "longdistance/context-cache/processed-context/0004/in.jsonld",
			contexts: []string{remoteURL, "longdistance/context-cache/processed-context/0004/context-1.jsonld"},
			out:      "longdistance/context-cache/processed-context/0004/out.jsonld",
		},
		{
			name:          "remote context as the expand context",
			in:            "longdistance/context-cache/processed-context/0005/in.jsonld",
			contexts:      []string{remoteURL, "longdistance/context-cache/processed-context/0005/context-1.jsonld"},
			expandContext: "longdistance/context-cache/processed-context/0005/expand-context.jsonld",
			out:           "longdistance/context-cache/processed-context/0005/out.jsonld",
		},
		{
			name:     "protected term redefined locally",
			in:       "longdistance/context-cache/processed-context/0006/in.jsonld",
			contexts: []string{remoteURL, "longdistance/context-cache/processed-context/0006/context-1.jsonld"},
			err:      ld.ErrProtectedTermRedefinition,
		},
		{
			name: "remote contexts defining the same protected term",
			in:   "longdistance/context-cache/processed-context/0007/in.jsonld",
			contexts: []string{
				remoteURL, "longdistance/context-cache/processed-context/0007/context-1.jsonld",
				remoteURL2, "longdistance/context-cache/processed-context/0007/context-2.jsonld",
			},
			out: "longdistance/context-cache/processed-context/0007/out.jsonld",
		},
		{
			name: "remote contexts defining conflicting protected term",
			in:   "longdistance/context-cache/processed-context/0008/in.jsonld",
			contexts: []string{
				remoteURL, "longdistance/context-cache/processed-context/0008/context-1.jsonld",
				remoteURL2, "longdistance/context-cache/processed-context/0008/context-2.jsonld",
			},
			err: ld.ErrProtectedTermRedefinition,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var opts []ld.ProcessorOption
			if tc.expandContext != "" {
				opts = append(opts, ld.WithExpandContext(LoadData(t, tc.expandContext)))
			}

			for i := 0; i < len(tc.contexts); i += 2 {
				iri, file := tc.contexts[i], tc.contexts[i+1]
				opts = append(opts, ld.WithProcessedContext(
					iri, CacheableContext(t, iri, LoadData(t, file))))
			}

			p := ld.NewProcessor(opts...)

			nodes, err := p.Expand(t.Context(), bytes.NewBuffer(LoadData(t, tc.in)), "")
			if errors.Is(err, ld.ErrLoadingRemoteContext) {
				t.Fatal("expected processed context to be used")
			}

			if !errors.Is(err, tc.err) {
				t.Fatalf("expected error: %v, got: %v", tc.err, err)
			}

			if tc.err != nil {
				return
			}

			DiffExpanded(t, LoadData(t, tc.out), nodes)
		})
	}
}

func TestProcessedContextMode(t *testing.T) {
	entry := ld.WithProcessedContext("https://example.test/remote", CacheableContext(t,
		"https://example.test/remote",
		jsontext.Value(`{"content": "https://example.org#content"}`),
		ld.With10Processing(true)))

	tests := []struct {
		name string
		ld10 bool
		out  jsontext.Value
		err  error
	}{
		{
			name: "same processing mode",
			ld10: true,
			out:  jsontext.Value(`[{"https://example.org#content": [{"@value": "hi"}]}]`),
		},
		{
			name: "distinct processing mode",
			err:  ld.ErrLoadingRemoteContext,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p := ld.NewProcessor(entry, ld.With10Processing(tc.ld10))

			nodes, err := p.Expand(t.Context(), strings.NewReader(
				`{"@context": "https://example.test/remote", "content": "hi"}`), "")
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected error: %v, got: %v", tc.err, err)
			}

			if tc.err != nil {
				return
			}

			DiffExpanded(t, tc.out, nodes)
		})
	}
}
