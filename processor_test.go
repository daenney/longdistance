package longdistance_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	ld "sourcery.dny.nu/longdistance"
	"sourcery.dny.nu/longdistance/internal/json"
)

func TestExcludeIRIsFromCompaction(t *testing.T) {
	excl := "https://www.w3.org/ns/activitystreams#Public"

	proc := ld.NewProcessor(
		ld.WithExcludeIRIsFromCompaction(excl),
	)

	graph := ld.Node{
		ID:   "https://example.com",
		Type: []string{"https://www.w3.org/ns/activitystreams#Create"},
		Properties: ld.Properties{
			"https://www.w3.org/ns/activitystreams#to": []ld.Node{
				{ID: excl},
			},
		},
	}

	var dst bytes.Buffer
	err := proc.Compact(&dst,
		json.RawMessage(`{"as": "https://www.w3.org/ns/activitystreams#"}`),
		[]ld.Node{graph},
		"",
	)

	if err != nil {
		t.Fatal(err.Error())
	}

	want := json.RawMessage(`{"@context":{"as": "https://www.w3.org/ns/activitystreams#"},"@id": "https://example.com", "@type": "as:Create", "as:to": {"@id": "https://www.w3.org/ns/activitystreams#Public"}}`)

	if diff := cmp.Diff(want, json.RawMessage(dst.Bytes()), JSONDiff()); diff != "" {
		t.Errorf("compaction mismatch (-want +got):\n%s", diff)
	}
}

func TestRemapPrefixIRIs(t *testing.T) {
	proc := ld.NewProcessor(
		ld.WithRemapPrefixIRIs("http://schema.org#", "http://schema.org/"),
	)

	compacted := json.RawMessage(`{"@context":{"schema":"http://schema.org#"}, "@id":"https://example.com", "schema:name": "Alice"}`)

	nodes, err := proc.Expand(bytes.NewReader(compacted), "")
	if err != nil {
		t.Fatal(err.Error())
	}

	if _, ok := nodes[0].Properties["http://schema.org/name"]; !ok {
		t.Logf("%#v\n", nodes[0])
		t.Fatal("expected IRI to remap.")
	}

	var dst bytes.Buffer
	err = proc.Compact(&dst, json.RawMessage(`{"schema":"http://schema.org#"}`), nodes, "")
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Contains(dst.Bytes(), []byte(`schema.org/`)) {
		t.Fatal("remap did not apply to compact")
	}
}

func TestValidateContextFunc(t *testing.T) {
	proc := ld.NewProcessor(
		ld.WithValidateContext(func(ctx *ld.Context) bool {
			defs := ctx.TermMap()
			if def, ok := defs["test"]; ok {
				return def.IRI == "https://example.com/test"
			}

			return true
		}),
	)

	compacted := json.RawMessage(`{"@context":{"test": "https://example.com/different"}, "test": "value"}`)

	_, err := proc.Expand(bytes.NewReader(compacted), "")
	if !errors.Is(err, ld.ErrInvalid) {
		t.Fatalf("expected: %s, got: %s", ld.ErrInvalid, err)
	}
}
