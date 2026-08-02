package iri

import (
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
