package iri

import (
	"net/url"
	"testing"
)

func TestRelative(t *testing.T) {
	tests := []struct {
		name      string
		base, iri string
		want      string
		mustErr   bool
	}{
		{
			name: "base equal to iri without path",
			base: "http://example.com",
			iri:  "http://example.com",
			want: "",
		},
		{
			name: "base equal to iri with empty path",
			base: "http://example.com/",
			iri:  "http://example.com/",
			want: "",
		},
		{
			name: "iri with path in base without path",
			base: "http://example.com",
			iri:  "http://example.com/foo",
			want: "foo",
		},
		{
			name: "iri with path in base with empty path",
			base: "http://example.com/",
			iri:  "http://example.com/foo",
			want: "foo",
		},
		{
			name: "iri equal to base with path",
			base: "http://example.com/a/b",
			iri:  "http://example.com/a/b",
			want: "b",
		},
		{
			name: "sibling resource",
			base: "http://example.com/a/b",
			iri:  "http://example.com/a/c",
			want: "c",
		},
		{
			name: "child of base with path",
			base: "http://example.com/a/",
			iri:  "http://example.com/a/b",
			want: "b",
		},
		{
			name: "same document with fragment",
			base: "http://example.com/a/b",
			iri:  "http://example.com/a/b#frag",
			want: "#frag",
		},
		{
			name: "iri within base but distinct paths",
			base: "http://example.com/a/b",
			iri:  "http://example.com/x/y",
			want: "../x/y",
		},
		{
			name:    "different host is an error",
			base:    "http://a.example.com/x",
			iri:     "http://b.example.com/x",
			mustErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := Relative(tc.base, tc.iri)
			if tc.mustErr {
				if err == nil {
					t.Fatalf("expect an error, got: %s", got)
				}

				return
			}

			if err != nil {
				t.Fatalf("expected no error, got: %s", err)
			}

			if got != tc.want {
				t.Fatalf("expected: %s, got: %s", tc.want, got)
			}
		})
	}
}

func TestAbsolute(t *testing.T) {
	tests := []struct {
		iri string
		abs bool
	}{
		{iri: "http://example.com", abs: true},
		{iri: "http://example.com/path", abs: true},
		{iri: "http://example.com#fragment", abs: true},
		{iri: "http://example.com/#fragment", abs: true},
		{iri: "http://example.com?q=a&b=c", abs: true},
		{iri: "http://example.com/?query", abs: true},
		{iri: "http://example.com/path#fragment", abs: true},
		{iri: "http://127.0.0.1", abs: true},
		{iri: "coap+tcp://example.com/x", abs: true},
		{iri: "http://example.com/context.json", abs: true},
		{iri: "http://example.com/a-b", abs: true},

		// cases that should be false on the fast path
		{iri: ""},
		{iri: "http://[::1]"},
		{iri: "urn:isbn:484848484"},
		{iri: "mailto:alice@example.com"},
		{iri: "http://example.com/a#b#c"},
		{iri: "http://example.com:8443"},
		{iri: "http://user:password@example.com"},
		{iri: "http://example.com/über"},
		{iri: "Term"},
		{iri: "a/b/c"},
		{iri: "#fragment"},
		{iri: "/path"},
		{iri: "?query"},
		{iri: ":localhost"},
		{iri: "_:b0"},
	}

	for _, tc := range tests {
		t.Run(tc.iri, func(t *testing.T) {
			t.Parallel()
			if res := absolute(tc.iri); res != tc.abs {
				t.Fatalf("expected: %v, got: %v", tc.abs, res)
			}
		})
	}
}

func FuzzIsAbsolute(f *testing.F) {
	for _, tc := range []string{
		"http://example.com",
		"http://example.com/path",
		"http://example.com#fragment",
		"http://example.com/#fragment",
		"http://example.com?q=a&b=c",
		"http://example.com/?query",
		"http://example.com/path#fragment",
		"http://example.com:8443",
		"http://user:password@example.com",
		"http://127.0.0.1",
		"coap+tcp://example.com/x",
		"http://example.com/context.json",
		"http://example.com/a-b",
		"",
		"http://[::1]",
		"urn:isbn:484848484",
		"mailto:alice@example.com",
		"http://example.com/über",
		"Term",
		"a/b/c",
		"#fragment",
		"/path",
		"?query",
		":localhost",
		"_:b0",
		"http://example.com/a#b#c",
	} {
		f.Add(tc)
	}

	parseAbs := func(s string) bool {
		u, err := url.Parse(s)
		return err == nil &&
			u.IsAbs() &&
			(u.RawPath == "" || u.RawPath == u.EscapedPath()) &&
			(u.RawFragment == "" || u.RawFragment == u.EscapedFragment())
	}

	f.Fuzz(func(t *testing.T, s string) {
		if got, want := IsAbsolute(s), parseAbs(s); got != want {
			t.Fatalf("input: %q, ours: %v, stdlib: %v", s, got, want)
		}
	})
}
