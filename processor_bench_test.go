package longdistance_test

import (
	"bytes"
	"encoding/json/jsontext"
	"io"
	"testing"

	ld "sourcery.dny.nu/longdistance"
	"sourcery.dny.nu/longdistance/internal/jsonutil"
)

func BenchmarkContextProcessing(b *testing.B) {
	b.Run("context=AS", func(b *testing.B) {
		ctx := jsontext.Value(jsonutil.MakeString(ASURL))

		b.ReportAllocs()
		b.SetBytes(int64(len(ctx)))

		p := ld.NewProcessor(
			ld.WithRemoteContextLoader(StaticLoader(b, ASURL, "as.jsonld")),
		)

		for b.Loop() {
			_, err := p.Context(b.Context(), bytes.NewBuffer(ctx), "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("context=AS+SecV1", func(b *testing.B) {
		ctx := jsonutil.MakeArray(bytes.Join([][]byte{
			jsonutil.MakeString(ASURL),
			jsonutil.MakeString(Secv1URL),
		},
			[]byte(`,`)))

		b.ReportAllocs()
		b.SetBytes(int64(len(ctx)))

		p := ld.NewProcessor(
			ld.WithRemoteContextLoader(
				StaticLoader(b,
					ASURL, "as.jsonld",
					Secv1URL, "securityv1.jsonld",
				),
			),
		)

		for b.Loop() {
			_, err := p.Context(b.Context(), bytes.NewBuffer(ctx), "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkContextProcessingCached(b *testing.B) {
	b.Run("context=AS", func(b *testing.B) {
		ctx := jsontext.Value(jsonutil.MakeString(ASURL))

		b.ReportAllocs()
		b.SetBytes(int64(len(ctx)))

		p := ld.NewProcessor(
			ld.WithProcessedContext(ASURL, ProcessContext(b, LoadData(b, "as.jsonld"), ASURL)),
		)

		for b.Loop() {
			_, err := p.Context(b.Context(), bytes.NewBuffer(ctx), "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("context=AS+SecV1", func(b *testing.B) {
		ctx := jsonutil.MakeArray(bytes.Join([][]byte{
			jsonutil.MakeString(ASURL),
			jsonutil.MakeString(Secv1URL),
		},
			[]byte(`,`)))

		b.ReportAllocs()
		b.SetBytes(int64(len(ctx)))

		p := ld.NewProcessor(
			ld.WithProcessedContext(ASURL, ProcessContext(b, LoadData(b, "as.jsonld"), ASURL)),
			ld.WithRemoteContextLoader(
				StaticLoader(b,
					Secv1URL, "securityv1.jsonld",
				),
			),
		)

		for b.Loop() {
			_, err := p.Context(b.Context(), bytes.NewBuffer(ctx), "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCompact(b *testing.B) {
	b.Run("activity=create-note", func(b *testing.B) {
		compCtx := LoadData(b, "observatory/create-note/context.jsonld")
		var dst bytes.Buffer
		var exp []ld.Node

		{
			p := ld.NewProcessor(
				ld.WithRemoteContextLoader(StaticLoader(b, ASURL, "as.jsonld")),
			)

			var err error
			exp, err = p.Expand(b.Context(), bytes.NewBuffer(LoadData(b, "observatory/create-note/in.jsonld")), "")
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
			ld.WithRemoteContextLoader(StaticLoader(b, ASURL, "as.jsonld")),
		)

		for b.Loop() {
			err := p.Compact(b.Context(), io.Discard, compCtx, exp, "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("activity=update-person", func(b *testing.B) {
		compCtx := LoadData(b, "observatory/update-person/context.jsonld")
		var dst bytes.Buffer
		var exp []ld.Node

		{
			p := ld.NewProcessor(
				ld.WithRemoteContextLoader(
					StaticLoader(b,
						ASURL, "as.jsonld",
						Secv1URL, "securityv1.jsonld",
					),
				),
			)

			var err error
			exp, err = p.Expand(b.Context(), bytes.NewBuffer(LoadData(b, "observatory/update-person/in.jsonld")), "")
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
			ld.WithRemoteContextLoader(
				StaticLoader(b,
					ASURL, "as.jsonld",
					Secv1URL, "securityv1.jsonld",
				),
			),
		)

		for b.Loop() {
			err := p.Compact(b.Context(), io.Discard, compCtx, exp, "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCompactCached(b *testing.B) {
	b.Run("activity=create-note", func(b *testing.B) {
		compCtx := LoadData(b, "observatory/create-note/context.jsonld")
		var dst bytes.Buffer
		var exp []ld.Node

		{
			p := ld.NewProcessor(
				ld.WithProcessedContext(ASURL, ProcessContext(b, LoadData(b, "as.jsonld"), ASURL)),
			)

			var err error
			exp, err = p.Expand(b.Context(), bytes.NewBuffer(LoadData(b, "observatory/create-note/in.jsonld")), "")
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
	})

	b.Run("activity=update-person", func(b *testing.B) {
		compCtx := LoadData(b, "observatory/update-person/context.jsonld")
		var dst bytes.Buffer
		var exp []ld.Node

		{

			p := ld.NewProcessor(
				ld.WithProcessedContext(ASURL, ProcessContext(b, LoadData(b, "as.jsonld"), ASURL)),
				ld.WithRemoteContextLoader(
					StaticLoader(b,
						Secv1URL, "securityv1.jsonld",
					),
				),
			)

			var err error
			exp, err = p.Expand(b.Context(), bytes.NewBuffer(LoadData(b, "observatory/create-note/in.jsonld")), "")
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
			ld.WithRemoteContextLoader(
				StaticLoader(b,
					Secv1URL, "securityv1.jsonld",
				),
			),
		)

		for b.Loop() {
			err := p.Compact(b.Context(), io.Discard, compCtx, exp, "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkExpand(b *testing.B) {
	b.Run("activity=create-note", func(b *testing.B) {
		doc := LoadData(b, "observatory/create-note/in.jsonld")

		b.ReportAllocs()
		b.SetBytes(int64(len(doc)))

		p := ld.NewProcessor(
			ld.WithRemoteContextLoader(StaticLoader(b, ASURL, "as.jsonld")),
		)

		for b.Loop() {
			_, err := p.Expand(b.Context(), bytes.NewBuffer(doc), "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("activity=update-person", func(b *testing.B) {
		doc := LoadData(b, "observatory/update-person/in.jsonld")

		b.ReportAllocs()
		b.SetBytes(int64(len(doc)))

		p := ld.NewProcessor(
			ld.WithRemoteContextLoader(
				StaticLoader(b,
					ASURL, "as.jsonld",
					Secv1URL, "securityv1.jsonld",
				),
			),
		)

		for b.Loop() {
			_, err := p.Expand(b.Context(), bytes.NewBuffer(doc), "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkExpandCached(b *testing.B) {
	b.Run("activity=create-note", func(b *testing.B) {
		doc := LoadData(b, "observatory/create-note/in.jsonld")

		b.ReportAllocs()
		b.SetBytes(int64(len(doc)))

		p := ld.NewProcessor(
			ld.WithProcessedContext(ASURL, ProcessContext(b, LoadData(b, "as.jsonld"), ASURL)),
		)

		for b.Loop() {
			_, err := p.Expand(b.Context(), bytes.NewBuffer(doc), "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("activity=update-person", func(b *testing.B) {
		doc := LoadData(b, "observatory/update-person/in.jsonld")

		b.ReportAllocs()
		b.SetBytes(int64(len(doc)))

		p := ld.NewProcessor(
			ld.WithProcessedContext(ASURL, ProcessContext(b, LoadData(b, "as.jsonld"), ASURL)),
			ld.WithRemoteContextLoader(
				StaticLoader(b,
					Secv1URL, "securityv1.jsonld",
				),
			),
		)

		for b.Loop() {
			_, err := p.Expand(b.Context(), bytes.NewBuffer(doc), "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
