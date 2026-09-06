package longdistance_test

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	ld "sourcery.dny.nu/longdistance"
)

func BenchmarkContextProcessing(b *testing.B) {
	ctx := json.RawMessage(`"https://www.w3.org/ns/activitystreams"`)

	b.ReportAllocs()
	b.SetBytes(int64(len(ctx)))

	p := ld.NewProcessor(
		ld.WithRemoteContextLoader(StaticLoader(b, "as.jsonld")),
	)

	for b.Loop() {
		_, err := p.Context(b.Context(), bytes.NewReader(ctx), "")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkContextProcessingCached(b *testing.B) {
	ctx := json.RawMessage(`"https://www.w3.org/ns/activitystreams"`)

	b.ReportAllocs()
	b.SetBytes(int64(len(ctx)))

	p := ld.NewProcessor(
		ld.WithProcessedContext(ASURL, ProcessContext(b, LoadData(b, "as.jsonld"), ASURL)),
	)

	for b.Loop() {
		_, err := p.Context(b.Context(), bytes.NewReader(ctx), "")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompact(b *testing.B) {
	compCtx := LoadData(b, "observatory/context.jsonld")
	var dst bytes.Buffer
	var exp []ld.Node

	{
		p := ld.NewProcessor(
			ld.WithRemoteContextLoader(StaticLoader(b, "as.jsonld")),
		)

		var err error
		exp, err = p.Expand(b.Context(), bytes.NewReader(LoadData(b, "observatory/createnote.json")), "")
		if err != nil {
			b.Fatal(err)
		}

		if err := p.Compact(b.Context(), &dst, compCtx, exp, ""); err != nil {
			b.Fatal(err)
		}
	}

	b.ReportAllocs()
	b.SetBytes(int64(dst.Len()))

	p := ld.NewProcessor(
		ld.WithRemoteContextLoader(StaticLoader(b, "as.jsonld")),
	)

	for b.Loop() {
		err := p.Compact(b.Context(), io.Discard, compCtx, exp, "")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompactCached(b *testing.B) {
	compCtx := LoadData(b, "observatory/context.jsonld")
	var dst bytes.Buffer
	var exp []ld.Node

	{
		p := ld.NewProcessor(
			ld.WithProcessedContext(ASURL, ProcessContext(b, LoadData(b, "as.jsonld"), ASURL)),
		)

		var err error
		exp, err = p.Expand(b.Context(), bytes.NewReader(LoadData(b, "observatory/createnote.json")), "")
		if err != nil {
			b.Fatal(err)
		}

		if err := p.Compact(b.Context(), &dst, compCtx, exp, ""); err != nil {
			b.Fatal(err)
		}
	}

	b.ReportAllocs()
	b.SetBytes(int64(dst.Len()))

	p := ld.NewProcessor(
		ld.WithProcessedContext(ASURL, ProcessContext(b, LoadData(b, "as.jsonld"), ASURL)),
	)

	for b.Loop() {
		err := p.Compact(b.Context(), io.Discard, compCtx, exp, "")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExpand(b *testing.B) {
	doc := LoadData(b, "observatory/createnote.json")

	b.ReportAllocs()
	b.SetBytes(int64(len(doc)))

	p := ld.NewProcessor(
		ld.WithRemoteContextLoader(StaticLoader(b, "as.jsonld")),
	)

	for b.Loop() {
		_, err := p.Expand(b.Context(), bytes.NewReader(doc), "")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExpandCached(b *testing.B) {
	doc := LoadData(b, "observatory/createnote.json")

	b.ReportAllocs()
	b.SetBytes(int64(len(doc)))

	p := ld.NewProcessor(
		ld.WithProcessedContext(ASURL, ProcessContext(b, LoadData(b, "as.jsonld"), ASURL)),
	)

	for b.Loop() {
		_, err := p.Expand(b.Context(), bytes.NewReader(doc), "")
		if err != nil {
			b.Fatal(err)
		}
	}
}
