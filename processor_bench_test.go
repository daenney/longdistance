package longdistance_test

import (
	"encoding/json"
	"testing"

	ld "sourcery.dny.nu/longdistance"
)

func BenchmarkContextProcessing(b *testing.B) {
	ctx := json.RawMessage(`"https://www.w3.org/ns/activitystreams"`)
	b.Run("without processed context", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(ctx)))

		p := ld.NewProcessor(
			ld.WithRemoteContextLoader(StaticLoader(b, "as.jsonld")),
		)

		for b.Loop() {
			_, err := p.Context(ctx, "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("with processed context", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(ctx)))

		p := ld.NewProcessor(
			ld.WithProcessedContext(ASURL, ProcessContext(b, LoadData(b, "as.jsonld"), ASURL)),
		)

		for b.Loop() {
			_, err := p.Context(ctx, "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCompact(b *testing.B) {
	doc := LoadData(b, "observatory/createnote.json")

	p := ld.NewProcessor(
		ld.WithRemoteContextLoader(StaticLoader(b, "as.jsonld")),
	)

	exp, err := p.Expand(doc, "")
	if err != nil {
		b.Fatal(err)
	}

	compCtx := LoadData(b, "observatory/context.jsonld")

	b.Run("without processed context", func(b *testing.B) {
		b.ReportAllocs()

		p := ld.NewProcessor(
			ld.WithRemoteContextLoader(StaticLoader(b, "as.jsonld")),
		)

		for b.Loop() {
			_, err := p.Compact(compCtx, exp, "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("with processed context", func(b *testing.B) {
		b.ReportAllocs()

		p := ld.NewProcessor(
			ld.WithProcessedContext(ASURL, ProcessContext(b, LoadData(b, "as.jsonld"), ASURL)),
		)

		for b.Loop() {
			_, err := p.Compact(compCtx, exp, "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkExpand(b *testing.B) {
	doc := LoadData(b, "observatory/createnote.json")

	b.Run("without processed context", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(doc)))

		p := ld.NewProcessor(
			ld.WithRemoteContextLoader(StaticLoader(b, "as.jsonld")),
		)

		for b.Loop() {
			_, err := p.Expand(doc, "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("with processed context", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(doc)))

		p := ld.NewProcessor(
			ld.WithProcessedContext(ASURL, ProcessContext(b, LoadData(b, "as.jsonld"), ASURL)),
		)

		for b.Loop() {
			_, err := p.Expand(doc, "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
