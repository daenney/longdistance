package longdistance

import (
	"fmt"

	"sourcery.dny.nu/longdistance/internal/json"
)

func Example() {
	p := NewProcessor()

	incoming := json.RawMessage(`{
		"@context": {
			"ex": "https://example.org#",
			"id": "@id",
			"type": "@type",
			"name": "ex:name"
		},
		"id": "https://example.org/id",
		"name": "Alice",
		"type": "https://example.org/type"
	}`)

	doc, err := p.Expand(incoming, "")
	if err != nil {
		panic(err)
	}

	fmt.Println("Entries:", len(doc))

	entry := doc[0]

	fmt.Println("Object ID:", entry.ID)
	fmt.Println("Object Type:", entry.Type)
	fmt.Println("Property lookup:", string(entry.Properties["https://example.org#name"][0].Value))

	exp, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}

	fmt.Println("Expanded form:", string(exp))

	compacted, err := p.Compact(
		json.RawMessage(`{
			"ex": "https://example.org#",
			"id": "@id",
			"type": "@type",
			"name": "ex:name"
		}`),
		doc,
		"",
	)

	if err != nil {
		panic(err)
	}

	fmt.Println("Compacted form:", string(compacted))

	// Output:
	// Entries: 1
	// Object ID: https://example.org/id
	// Object Type: [https://example.org/type]
	// Property lookup: "Alice"
	// Expanded form: [{"@id":"https://example.org/id","@type":["https://example.org/type"],"https://example.org#name":[{"@value":"Alice"}]}]
	// Compacted form: {"@context":{"ex":"https://example.org#","id":"@id","type":"@type","name":"ex:name"},"id":"https://example.org/id","name":"Alice","type":"https://example.org/type"}
}
