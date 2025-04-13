package longdistance_test

import (
	"testing"

	ld "code.dny.dev/longdistance"
	"code.dny.dev/longdistance/internal/json"
	"github.com/google/go-cmp/cmp"
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

	compacted, err := proc.Compact(
		json.RawMessage(`{"as": "https://www.w3.org/ns/activitystreams#"}`),
		[]ld.Node{graph},
		"",
	)

	if err != nil {
		t.Fatal(err.Error())
	}

	want := json.RawMessage(`{"@context":{"as": "https://www.w3.org/ns/activitystreams#"},"@id": "https://example.com", "@type": "as:Create", "as:to": {"@id": "https://www.w3.org/ns/activitystreams#Public"}}`)

	if diff := cmp.Diff(want, json.RawMessage(compacted), JSONDiff()); diff != "" {
		t.Errorf("compaction mismatch (-want +got):\n%s", diff)
	}
}

func TestRemapPrefixIRIs(t *testing.T) {
	proc := ld.NewProcessor(
		ld.WithRemapPrefixIRIs("http://schema.org#", "http://schema.org/"),
	)

	compacted := json.RawMessage(`{"@context":{"schema":"http://schema.org#"}, "@id":"https://example.com", "schema:name": "Alice"}`)

	nodes, err := proc.Expand(compacted, "")
	if err != nil {
		t.Fatal(err.Error())
	}

	if _, ok := nodes[0].Properties["http://schema.org/name"]; !ok {
		t.Logf("%#v\n", nodes[0])
		t.Fatal("expected IRI to remap.")
	}
}
