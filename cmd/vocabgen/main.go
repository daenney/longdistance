package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"iter"
	"maps"
	"os"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	ld "code.dny.dev/longdistance"
)

func main() {
	doc := flag.String("context", "", "context file")
	docIRI := flag.String("document.iri", "", "remote context IRI for this file")
	ns := flag.String("namespace", "", "namespace used by terms in this context")
	pkgName := flag.String("package.name", "vocab", "Go package name")
	flag.Parse()

	if *doc == "" {
		panic("need a context file to read")
	}

	if *ns == "" {
		panic("need a namespace")
	}

	data, err := os.ReadFile(*doc)
	if err != nil {
		panic(err)
	}

	var rawCtx map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawCtx); err != nil {
		panic(err)
	}

	proc := ld.NewProcessor()

	res, err := proc.Context(rawCtx[ld.KeywordContext], *docIRI)
	if err != nil {
		panic(err)
	}

	var result bytes.Buffer
	result.WriteString("package " + *pkgName + "\n\n")

	result.WriteString("// IRI is the remote context IRI.\n")
	result.WriteString("const IRI = \"" + *docIRI + "\"\n\n")

	result.WriteString("// Namespace is the IRI prefix used for terms defined in this context that don't\n// map to a different namespace.\n")
	if strings.HasPrefix(*ns, *docIRI) {
		result.WriteString("const Namespace = IRI + \"" + strings.TrimPrefix(*ns, *docIRI) + "\"\n\n")
	} else {
		result.WriteString("const Namespace = \"" + *ns + "\"\n\n")
	}

	terms := makeTerms(proc, *docIRI, *ns, res.Terms())
	slices.Sort(terms)
	result.WriteString("const (\n")
	for _, v := range terms {
		result.WriteString(v)
	}
	result.WriteString(")\n")
	fmt.Println(result.String())
}

func makeTerms(
	proc *ld.Processor,
	documentURL string,
	namespace string,
	terms iter.Seq2[string, ld.Term],
) []string {
	texts := make([]string, 0, 100)
	scoped := make(map[string]ld.Term, 20)

	for term, def := range terms {
		if def.Prefix {
			continue
		}

		value := def.IRI
		if strings.HasPrefix(value, "@") {
			continue
		}

		goTerm := goName(term, def.Context != nil)
		text := "\t// " + goTerm + " "
		if strings.HasPrefix(goTerm, "Type") {
			text = text + "is a possible value for the type property.\n"
		} else if strings.HasPrefix(goTerm, "Relationship") && goTerm != "Relationship" {
			text = text + "is a possible value for a relationship property.\n"
		} else if def.Type == ld.KeywordID {
			text = text + "is an IRI, either as a string or as an object with an\n// id property.\n"
		} else if def.Type != "" && def.Context == nil {
			xsdPrefix1 := "http://www.w3.org/2001/XMLSchema#"
			xsdPrefix2 := "https://www.w3.org/2001/XMLSchema#"
			if strings.HasPrefix(def.Type, xsdPrefix1) || strings.HasPrefix(def.Type, xsdPrefix2) {
				typ := strings.TrimPrefix(def.Type, xsdPrefix1)
				typ = strings.TrimPrefix(typ, xsdPrefix2)
				switch typ {
				case "float":
					typ = typ + ", an IEEE single-precision 32-bit floating point\n// value equivalent to a Go float32"
				case "integer":
					typ = typ + ", an \"infinite size\" integer. The\n// XML specification requires you to at least accept numbers with up to\n// 16 digits. A Go int64 may be sufficient depending on your usage.\n// Remember that you can only safely express up to 53-bit precision\n// integers this way since JSON treats integers as floats. For bigger\n// values you'll need a string"
				case "nonNegativeInteger":
					typ = typ + ", an \"infinite size\" integer. The\n// XML specification requires you to at least accept numbers with up to\n// 16 digits. A Go uint64 may be sufficient depending on your usage.\n// Remember that you can only safely express up to 53-bit precision\n// integers this way since JSON treats integers as floats. For bigger\n// values you'll need a string"
				case "dateTime":
					typ = typ + ", equivalent to a time.Date in RFC3339Nano"
				case "duration":
					typ = typ + " and does not have a Go equivalent, but\n// can be handled as a string"
				case "@json":
					typ = "JSON"
				}
				text = text + "is an xml:" + typ + ".\n"
			} else if def.Type == "@json" {
				text = text + "is a JSON value that will be left untouched.\n"
			} else {
				text = text + "is a " + def.Type + ".\n"
			}
		} else {
			if def.Context != nil {
				text = text + "is an object.\n"
			} else {
				text = text + "is a string.\n"
			}
		}
		if strings.HasPrefix(value, namespace) {
			texts = append(texts, text+"\t"+goTerm+" = Namespace + \""+strings.TrimPrefix(value, namespace)+"\"\n")
		} else {
			texts = append(texts, text+"\t"+goTerm+" = \""+value+"\"\n")
		}

		if def.Context != nil {
			nctx, err := proc.Context(def.Context, documentURL)
			if err != nil {
				panic(err)
			}
			if nctx != nil {
				maps.Insert(scoped, nctx.Terms())
			}
		}
	}

	if len(scoped) != 0 {
		texts = append(texts, makeTerms(proc, documentURL, namespace, maps.All(scoped))...)
	}
	return texts
}

func isUpper(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		return unicode.IsUpper(r)
	}
	return false
}

func goName(s string, isObject bool) string {
	if len(s) == 0 {
		return ""
	}

	mapped := s
	if strings.HasPrefix(mapped, "id") || strings.HasPrefix(mapped, "Id") {
		mapped = "ID" + mapped[2:]
	}
	if strings.HasSuffix(mapped, "id") || strings.HasSuffix(mapped, "Id") {
		mapped = mapped[:len(s)-2] + "ID"
	}

	mapped = strings.ReplaceAll(mapped, "url", "URL")
	mapped = strings.ReplaceAll(mapped, "Url", "URL")
	mapped = strings.ReplaceAll(mapped, "ttl", "TTL")

	if isUpper(s) && !isObject {
		prefix := "Type"
		if strings.HasPrefix(s, "Is") {
			prefix = "Relationship"
		}
		return fmt.Sprintf("%s%s", prefix, mapped)
	}

	r, _ := utf8.DecodeRuneInString(mapped)
	r = unicode.ToTitle(r)
	return string(r) + mapped[1:]
}
