package longdistance

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io"
	"net/url"
	"slices"
	"strings"

	"sourcery.dny.nu/longdistance/internal/iri"
	"sourcery.dny.nu/longdistance/internal/jsonutil"
)

// compact represents a compacted node.
//
// The kind of a compact is determined by which field is populated:
//   - A leaf holds a Value.
//   - An object populates Properties, with each element having Attr set.
//   - An array populates Members.
//
// IsID and IsType record whether a property is the alias of @id or @type, and
// Context holds the raw @context and is only set on the root compact.
type compact struct {
	Attr       string
	Value      jsontext.Value
	Properties []compact
	Members    []compact

	IsID    bool
	IsType  bool
	Context jsontext.Value
}

func (c *compact) get(key string) (*compact, bool) {
	for i := range c.Properties {
		if c.Properties[i].Attr == key {
			return &c.Properties[i], true
		}
	}
	return nil, false
}

func (c *compact) del(key string) {
	for i := range c.Properties {
		if c.Properties[i].Attr == key {
			c.Properties = slices.Delete(c.Properties, i, i+1)
			return
		}
	}
}

func (c *compact) asString() (string, bool) {
	if c.Value.Kind() != jsontext.KindString {
		return "", false
	}

	s, err := jsontext.AppendUnquote(nil, c.Value)
	if err != nil {
		return "", false
	}

	return string(s), true
}

// addValue adds item for key to object.
//
// If key already exists it becomes an array. Otherwise arrayification is
// dictated by asArray.
func (c *compact) addValue(key string, item compact, asArray bool) {
	if e, ok := c.get(key); ok {
		if e.Members == nil {
			*e = compact{Attr: e.Attr, Members: []compact{*e}}
		}

		e.Members = append(e.Members, item)
		return
	}

	if asArray && item.Members == nil {
		c.Properties = append(c.Properties, compact{
			Attr:    key,
			Members: []compact{item},
		})
		return
	}

	item.Attr = key
	c.Properties = append(c.Properties, item)
}

// MarshalJSONTo marshals the compacted document.
//
// For any object, the keys are emitted in the following order, if present:
//   - @context.
//   - @type, or its aliassed key.
//   - @id, or its aliassed key.
//   - Any remaining properties, including other JSON-LD keywords or their
//     aliasses, are sorted lexicographically with shortest keys first.
//
// With @context, @type and @id being present first, the document can be processed
// by a receiver in a fully streaming manner.
func (c *compact) MarshalJSONTo(enc *jsontext.Encoder) error {
	switch {
	case c.Members != nil:
		if err := enc.WriteToken(jsontext.BeginArray); err != nil {
			return err
		}

		for i := range c.Members {
			if err := c.Members[i].MarshalJSONTo(enc); err != nil {
				return err
			}
		}

		return enc.WriteToken(jsontext.EndArray)
	case c.Properties != nil:
		if err := enc.WriteToken(jsontext.BeginObject); err != nil {
			return err
		}

		if c.Context != nil {
			if err := cmp.Or(
				enc.WriteToken(jsontext.String(KeywordContext)),
				enc.WriteValue(c.Context),
			); err != nil {
				return err
			}
		}

		rank := func(p compact) int {
			switch {
			case p.IsID:
				return 1
			case p.IsType:
				return 0
			default:
				return 2
			}
		}

		slices.SortFunc(c.Properties, func(a, b compact) int {
			if r := rank(a) - rank(b); r != 0 {
				return r
			}
			return sortedLeast(a.Attr, b.Attr)
		})

		for i := range c.Properties {
			if err := cmp.Or(
				enc.WriteToken(jsontext.String(c.Properties[i].Attr)),
				c.Properties[i].MarshalJSONTo(enc),
			); err != nil {
				return err
			}
		}

		return enc.WriteToken(jsontext.EndObject)
	default:
		return enc.WriteValue(c.Value)
	}
}

func (p *Processor) compactIRI(
	activeContext *Context,
	key string,
	value *Node,
	vocab bool,
	reverse bool,
) (string, error) {
	if value != nil || !vocab || reverse {
		return p.compactIRIUncached(activeContext, key, value, vocab, reverse)
	}

	if res, ok := activeContext.iriCache[key]; ok {
		return res, nil
	}

	res, err := p.compactIRIUncached(activeContext, key, value, vocab, reverse)
	if err != nil {
		return res, err
	}

	if activeContext.iriCache == nil {
		activeContext.iriCache = make(map[string]string)
	}
	activeContext.iriCache[key] = res

	return res, nil
}

func (p *Processor) compactIRIUncached(
	activeContext *Context,
	key string,
	value *Node,
	vocab bool,
	reverse bool,
) (string, error) {
	// 1)
	if key == "" {
		return "", nil
	}

	// this should be done as the first thing in step 10
	// but we can avoid a ton of work by doing it early here
	if strings.HasPrefix(key, BlankNode) {
		return key, nil
	}

	if slices.Contains(p.excludeIRIsFromCompaction, key) {
		return key, nil
	}

	// 2)
	activeContext.initInverse()

	// 3)
	inverse := activeContext.inverse

	var object Node
	isObject := value != nil
	if isObject {
		object = *value
	}

	// 4)
	if _, ok := inverse.get(key); ok && vocab {
		// 4.1)
		defaultLanguage := KeywordNone // 4.1.2)
		if activeContext.defaultDirection != "" {
			// 4.1.1)
			defaultLanguage = activeContext.defaultLang + "_" + activeContext.defaultDirection
		} else if activeContext.defaultLang != "" {
			// 4.1.2)
			defaultLanguage = "_" + activeContext.defaultLang
		}

		// 4.2) we don't have @preserve
		// if value != nil && value.Preserve != nil {}

		// 4.3)
		containers := make([]string, 0, 4)

		// 4.4)
		typeLanguage := KeywordLanguage
		typeLanguageValue := KeywordNull

		// 4.5)
		if isObject && object.Has(KeywordIndex) && !object.IsGraph() {
			containers = append(containers,
				KeywordIndex,
				KeywordIndex+KeywordSet,
			)
		}

		if reverse {
			// 4.6)
			typeLanguage = KeywordType
			typeLanguageValue = KeywordReverse
			containers = append(containers, KeywordSet)
		} else if isObject && object.IsList() {
			// 4.7)

			// 4.7.1)
			if !object.Has(KeywordIndex) {
				containers = append(containers, KeywordList)
			}

			// 4.7.2) don't need it

			// 4.7.3)
			var commonLanguage *string
			var commonType *string

			if len(object.List) == 0 {
				commonLanguage = &defaultLanguage
			}

			// 4.7.4)
			for _, item := range object.List {
				// 4.7.4.1)
				itemLanguage := KeywordNone
				itemType := KeywordNone

				// 4.7.4.2)
				if item.IsValue() {
					if item.Has(KeywordDirection) {
						if item.Has(KeywordLanguage) {
							itemLanguage = item.Language + "_" + item.Direction
						} else {
							itemLanguage = "_" + item.Direction
						}
					} else if item.Has(KeywordLanguage) {
						itemLanguage = item.Language
					} else if item.Has(KeywordType) {
						itemType = item.Type[0]
					} else {
						itemLanguage = KeywordNull
					}
				} else {
					// 4.7.4.3)
					itemType = KeywordID
				}

				if commonLanguage == nil {
					// 4.7.4.4)
					commonLanguage = &itemLanguage
				} else if itemLanguage != *commonLanguage &&
					isObject && object.Value != nil {
					// 4.7.4.5)
					*commonLanguage = KeywordNone
				}

				if commonType == nil {
					// 4.7.4.6)
					commonType = &itemType
				} else if itemType != *commonType {
					// 4.7.4.7)
					*commonType = KeywordNone
				}
				// 4.7.4.8)
				if commonLanguage != nil && commonType != nil &&
					*commonLanguage == KeywordNone &&
					*commonType == KeywordNone {
					break
				}
			}

			// 4.7.5)
			if commonLanguage == nil {
				commonLanguage = new(string)
				*commonLanguage = KeywordNone
			}

			// 4.7.6)
			if commonType == nil {
				commonType = new(string)
				*commonType = KeywordNone
			}

			if *commonType != KeywordNone {
				// 4.7.7)
				typeLanguage = KeywordType
				typeLanguageValue = *commonType
			} else {
				// 4.7.8)
				typeLanguageValue = *commonLanguage
			}
		} else if isObject && object.IsGraph() {
			// 4.8)
			if object.Has(KeywordIndex) {
				// 4.8.1)
				containers = append(containers,
					KeywordGraph+KeywordIndex,
					KeywordGraph+KeywordIndex+KeywordSet,
				)
			}

			if object.Has(KeywordID) {
				// 4.8.2)
				containers = append(containers,
					KeywordGraph+KeywordID,
					KeywordGraph+KeywordID+KeywordSet,
				)
			}

			// 4.8.3)
			containers = append(containers,
				KeywordGraph,
				KeywordGraph+KeywordSet,
				KeywordSet,
			)

			if !object.Has(KeywordIndex) {
				// 4.8.4)
				containers = append(containers,
					KeywordGraph+KeywordIndex,
					KeywordGraph+KeywordIndex+KeywordSet,
				)
			}

			if !object.Has(KeywordID) {
				// 4.8.5)
				containers = append(containers,
					KeywordGraph+KeywordID,
					KeywordGraph+KeywordID+KeywordSet,
				)
			}

			// 4.8.6)
			containers = append(containers,
				KeywordIndex,
				KeywordIndex+KeywordSet,
			)

			typeLanguage = KeywordType
			typeLanguageValue = KeywordID
		} else {
			// 4.9)
			if isObject && object.IsValue() {
				// 4.9.1)
				if object.Has(KeywordDirection) && !object.Has(KeywordIndex) {
					if object.Has(KeywordLanguage) {
						typeLanguageValue = object.Language + "_" + object.Direction
					} else {
						typeLanguageValue = "_" + object.Direction
					}
					containers = append(containers,
						KeywordLanguage,
						KeywordLanguage+KeywordSet)
				} else if object.Has(KeywordLanguage) && !object.Has(KeywordIndex) {
					typeLanguageValue = object.Language
					containers = append(containers,
						KeywordLanguage,
						KeywordLanguage+KeywordSet)
				} else if object.Has(KeywordType) {
					typeLanguage = KeywordType
					typeLanguageValue = object.Type[0]
				}
			} else {
				// 4.9.3)
				typeLanguage = KeywordType
				typeLanguageValue = KeywordID
				containers = append(containers,
					KeywordID,
					KeywordID+KeywordSet,
					KeywordType,
					KeywordSet+KeywordType,
				)
			}
			// 4.9.3)
			containers = append(containers, KeywordSet)
		}
		// 4.10)
		containers = append(containers, KeywordNone)

		if !p.modeLD10 {
			// 4.11)
			if !isObject || !object.Has(KeywordIndex) {
				containers = append(containers,
					KeywordIndex,
					KeywordIndex+KeywordSet)
			}
			// 4.12)
			if isObject && object.IsValue() && object.Len() == 1 {
				containers = append(containers,
					KeywordLanguage,
					KeywordLanguage+KeywordSet)
			}
		}

		// 4.13)
		typeLanguageValue = cmp.Or(
			typeLanguageValue,
			KeywordNull,
		)

		// 4.14)
		preferredValues := make([]string, 0, 4)

		// 4.15)
		if typeLanguageValue == KeywordReverse {
			preferredValues = append(preferredValues, KeywordReverse)
		}

		if isObject && object.Has(KeywordID) && (typeLanguageValue == KeywordID || typeLanguageValue == KeywordReverse) {
			// 4.16)
			c, err := p.compactIRI(
				activeContext,
				object.ID.Value,
				nil, true, false,
			)
			if err != nil {
				return "", err
			}

			cdef, cok := activeContext.defs[c]
			if cok && cdef.IRI == object.ID.Value {
				// 4.16.1)
				preferredValues = append(preferredValues,
					KeywordVocab,
					KeywordID,
					KeywordNone)
			} else {
				// 4.16.2)
				preferredValues = append(preferredValues,
					KeywordID,
					KeywordVocab,
					KeywordNone)
			}
		} else {
			// 4.17)
			preferredValues = append(preferredValues,
				typeLanguageValue,
				KeywordNone)
			if isObject && object.IsList() && len(object.List) == 0 {
				typeLanguage = KeywordAny
			}
		}

		// 4.18)
		preferredValues = append(preferredValues, KeywordAny)

		// 4.19)
		for _, p := range preferredValues[:] {
			idx := strings.Index(p, "_")
			if idx == -1 {
				continue
			}
			preferredValues = append(preferredValues, p[idx:])
		}

		// 4.20)
		term := selectTerm(
			activeContext,
			key,
			containers,
			typeLanguage,
			preferredValues,
		)

		// 4.21)
		if term != "" {
			return term, nil
		}
	}

	// 5)
	vocabMapping := activeContext.vocabMapping
	if vocab && vocabMapping != "" {
		if strings.HasPrefix(key, vocabMapping) && len(key) > len(vocabMapping) {
			// 5.1)
			suffix := strings.TrimPrefix(key, vocabMapping)
			if _, ok := activeContext.defs[suffix]; !ok {
				return suffix, nil
			}
		}
	}

	// 6)
	compactIRI := ""

	// 7)
	for term := range activeContext.prefixes {
		def := activeContext.defs[term]
		if def.IRI == "" || def.IRI == key || !strings.HasPrefix(
			key, def.IRI) {
			// 7.1)
			continue
		}

		// 7.2)
		candidate := term + ":" + strings.TrimPrefix(
			key, def.IRI)

		// 7.3)
		cdef, cok := activeContext.defs[candidate]

		if !cok && (compactIRI == "" || sortedLeast(candidate, compactIRI) < 0) {
			compactIRI = candidate
		} else if cok && cdef.IRI == key && value == nil {
			compactIRI = candidate
		}
	}

	// 8)
	if compactIRI != "" {
		return compactIRI, nil
	}

	// 9)
	u, err := url.Parse(key)
	if err != nil {
		return "", err
	}

	if _, isPrefix := activeContext.prefixes[u.Scheme]; isPrefix && u.Host == "" {
		return "", ErrIRIConfusedWithPrefix
	}

	// 10)
	if !vocab && activeContext.currentBaseIRI != "" {
		res, err := iri.Relative(activeContext.currentBaseIRI, key)
		if err == nil {
			if looksLikeKeyword(res) {
				res = "./" + res
			}
			key = res
		}
	}

	// 11)
	return key, nil
}

func langDirMatch(keyword string, value *Node, expected string) bool {
	var comp string
	switch keyword {
	case KeywordLanguage:
		comp = value.Language
	case KeywordDirection:
		comp = value.Direction
	}

	if value.Has(keyword) {
		return expected != "" && expected != KeywordNull &&
			strings.EqualFold(comp, expected)
	}

	return expected == "" || expected == KeywordNull
}

func (p *Processor) compactValue(
	ctx *Context,
	prop string,
	value *Node,
) (*compact, error) {
	// 1) 2) and 3) aren't needed

	def, defOK := ctx.defs[prop]

	// 4)
	language := cmp.Or(
		def.Language,
		ctx.defaultLang,
	)

	// 5)
	direction := cmp.Or(
		def.Direction,
		ctx.defaultDirection,
	)

	if value.Has(KeywordID) &&
		(value.Len() == 1 ||
			(value.Len() == 2 && value.Has(KeywordIndex))) {
		// 6)
		if defOK && def.Type != "" {
			var res string
			var err error

			switch def.Type {
			case KeywordID:
				res, err = p.compactIRI(ctx,
					value.ID.Value,
					nil,
					false, false)
			case KeywordVocab:
				res, err = p.compactIRI(ctx,
					value.ID.Value,
					nil,
					true, false)
			default:
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			q, err := jsontext.AppendQuote(nil, res)
			if err != nil {
				return nil, err
			}
			return &compact{Value: q}, nil
		} else {
			return nil, nil
		}
	} else if defOK && value.Has(KeywordType) && slices.Contains(value.Type, def.Type) {
		// 7)
		return &compact{Value: value.Value}, nil
	} else if (defOK && def.Type == KeywordNone) || value.Has(KeywordType) && !slices.Contains(value.Type, def.Type) {
		// 8) don't need to do anything here
		return nil, nil
	} else if value.IsValue() && !jsonutil.IsString(value.Value) {
		// 9)
		if !value.Has(KeywordIndex) || slices.Contains(def.Container, KeywordIndex) {
			// 9.1)
			return &compact{Value: value.Value}, nil
		}
	} else if value.IsValue() && langDirMatch(KeywordLanguage, value, language) && langDirMatch(KeywordDirection, value, direction) {
		// 10)
		if !value.Has(KeywordIndex) || (defOK && slices.Contains(def.Container, KeywordIndex)) {
			// 10.1)
			return &compact{Value: value.Value}, nil
		}
	}

	// 11) doesn't seem necessary
	return nil, nil
}

func (p *Processor) Compact(
	ctx context.Context,
	dst io.Writer,
	compactionCtx jsontext.Value,
	document []Node,
	documentURL string,
) error {
	ldCtx, err := p.context(ctx, nil, jsontext.NewDecoder(bytes.NewReader(compactionCtx)), documentURL, newCtxProcessingOpts())
	if err != nil {
		return err
	}

	if len(document) == 0 {
		_, err := dst.Write([]byte(`{}`))
		return err
	}

	if ldCtx == nil {
		return json.MarshalWrite(dst, document)
	}

	root, err := p.compactArray(
		ctx,
		ldCtx,
		"",
		document,
		p.compactArrays,
	)

	if err != nil {
		return err
	}

	if root == nil {
		_, err := dst.Write([]byte(`{}`))
		return err
	}

	out := root
	if root.Properties == nil || !p.compactArrays {
		alias, err := p.compactIRI(ldCtx, KeywordGraph, nil, true, false)
		if err != nil {
			return err
		}

		graph := *root
		graph.Attr = alias
		out = &compact{Properties: []compact{graph}}
	}

	if len(compactionCtx) > 2 {
		out.Context = compactionCtx
	}

	return out.MarshalJSONTo(jsontext.NewEncoder(dst))
}

func (p *Processor) compactArray(
	ctx context.Context,
	activeContext *Context,
	activeProperty string,
	elems []Node,
	compactArrays bool,
) (*compact, error) {
	var activeTermDefinition Term
	if activeProperty != "" {
		activeTermDefinition = activeContext.defs[activeProperty]
	}

	// 3.1)
	result := make([]compact, 0, len(elems))

	// 3.2)
	for _, elem := range elems {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// 3.2.1)
		compactedItem, err := p.compactNode(ctx, activeContext, activeProperty, elem, compactArrays)
		if err != nil {
			return nil, err
		}
		// 3.2.2)
		if compactedItem != nil {
			result = append(result, *compactedItem)
		}
	}

	// 3.3)
	if len(result) != 1 || !compactArrays || activeProperty == KeywordGraph || activeProperty == KeywordSet {
		return &compact{Members: result}, nil
	}

	if slices.Contains(activeTermDefinition.Container, KeywordList) ||
		slices.Contains(activeTermDefinition.Container, KeywordSet) {
		return &compact{Members: result}, nil
	}

	// 3.4)
	elem := result[0]
	return &elem, nil
}

func (p *Processor) compactNode(
	ctx context.Context,
	activeContext *Context,
	activeProperty string,
	object Node,
	compactArrays bool,
) (*compact, error) {
	var activeTermDefinition Term
	if activeProperty != "" {
		activeTermDefinition = activeContext.defs[activeProperty]
	}

	// 1)
	typeScopedContext := activeContext

	// 4) We're guaranteed to have an object here.

	// 5)
	if activeContext.previousCtx != nil &&
		!object.Has(KeywordValue) &&
		(!object.Has(KeywordID) || object.Len() > 1) {
		activeContext = activeContext.previousCtx
	}

	// 6)
	if activeTermDefinition.Context != nil {
		opts := newCtxProcessingOpts()
		opts.override = true
		nctx, err := p.context(ctx, activeContext, jsontext.NewDecoder(bytes.NewReader(activeTermDefinition.Context)), activeTermDefinition.BaseIRI, opts)
		if err != nil {
			return nil, err
		}
		activeContext = nctx
		activeTermDefinition = activeContext.defs[activeProperty]
	}

	// 7)
	if object.Has(KeywordValue) || object.Has(KeywordID) {
		if activeTermDefinition.Type == KeywordJSON {
			return &compact{Value: object.Value}, nil
		}

		value, err := p.compactValue(activeContext, activeProperty, &object)
		if err != nil {
			return nil, err
		}

		if value != nil {
			return value, nil
		}
	}

	// 8)
	if object.IsList() &&
		slices.Contains(activeTermDefinition.Container, KeywordList) {
		return p.compactArray(
			ctx,
			activeContext,
			activeProperty,
			object.List,
			compactArrays,
		)
	}

	// 9)
	insideReverse := activeProperty == KeywordReverse

	// 10)
	result := &compact{Properties: []compact{}}

	// 11)
	if object.Has(KeywordType) {
		compactedTypes := make([]string, 0, len(object.Type))
		for _, t := range object.Type {
			res, err := p.compactIRI(activeContext, t, nil, true, false)
			if err != nil {
				return nil, err
			}
			compactedTypes = append(compactedTypes, res)
		}

		slices.Sort(compactedTypes)

		// 11.1)
		for _, t := range compactedTypes {
			if cdef, cok := typeScopedContext.defs[t]; cok && cdef.Context != nil {
				opts := newCtxProcessingOpts()
				opts.propagate = false
				nctx, err := p.context(
					ctx,
					activeContext,
					jsontext.NewDecoder(bytes.NewReader(cdef.Context)),
					cdef.BaseIRI,
					opts,
				)
				if err != nil {
					return nil, err
				}
				activeContext = nctx
			}
		}
	}

	// 12)
	for expandedProperty := range object.PropertyKeys() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// 12.1)
		if expandedProperty == KeywordID {
			// 12.1.1)
			cv, err := p.compactIRI(activeContext, object.ID.Value, nil, false, false)
			if err != nil {
				return nil, err
			}
			// 12.1.2)
			alias, err := p.compactIRI(activeContext, KeywordID, nil, true, false)
			if err != nil {
				return nil, err
			}
			// 12.1.3)
			q, err := jsontext.AppendQuote(nil, cv)
			if err != nil {
				return nil, err
			}
			result.Properties = append(result.Properties, compact{
				Attr:  alias,
				Value: q,
				IsID:  true,
			})
			continue
		}

		if expandedProperty == KeywordType {
			// 12.2.1) 12.2.2)
			vt := make([]string, 0, len(object.Type))
			for _, t := range object.Type {
				res, err := p.compactIRI(typeScopedContext, t, nil, true, false)
				if err != nil {
					return nil, err
				}
				vt = append(vt, res)
			}

			// 12.2.3)
			alias, err := p.compactIRI(activeContext, KeywordType, nil, true, false)
			if err != nil {
				return nil, err
			}

			// 12.2.4)
			asArray := !compactArrays
			if tdef, tok := activeContext.defs[alias]; tok && slices.Contains(tdef.Container, KeywordSet) && !p.modeLD10 {
				asArray = true
			}

			// 12.2.5)
			entry := compact{Attr: alias, IsType: true}
			if asArray || len(vt) > 1 {
				entry.Members = make([]compact, 0, len(vt))
				for _, t := range vt {
					q, err := jsontext.AppendQuote(nil, t)
					if err != nil {
						return nil, err
					}
					entry.Members = append(entry.Members, compact{Value: q})
				}
			} else {
				q, err := jsontext.AppendQuote(nil, vt[0])
				if err != nil {
					return nil, err
				}
				entry.Value = q
			}
			result.Properties = append(result.Properties, entry)

			// 12.2.6)
			continue
		}

		// 12.3)
		if expandedProperty == KeywordReverse {
			// 12.3.1)
			final := &compact{Properties: []compact{}}
			for k, elem := range object.Reverse {
				compactedValue, err := p.compactNode(
					ctx,
					activeContext,
					KeywordReverse,
					Node{Properties: Properties{
						k: elem,
					}},
					compactArrays,
				)
				if err != nil {
					return nil, err
				}

				// 12.3.2)
				if compactedValue == nil || compactedValue.Properties == nil {
					continue
				}

				for _, entry := range compactedValue.Properties {
					prop := entry.Attr
					if rdef, rok := activeContext.defs[prop]; rok && rdef.Reverse {
						asArray := !compactArrays
						if slices.Contains(rdef.Container, KeywordSet) {
							asArray = true
						}

						rev := entry
						if asArray && rev.Members == nil {
							rev = compact{Attr: entry.Attr, Members: []compact{rev}}
						}

						if e, ok := result.get(prop); ok {
							*e = rev
						} else {
							result.Properties = append(result.Properties, rev)
						}
						continue
					}

					if e, ok := final.get(prop); ok {
						*e = entry
					} else {
						final.Properties = append(final.Properties, entry)
					}
				}
			}

			if len(final.Properties) == 0 {
				continue
			}

			// 12.3.3)
			alias, err := p.compactIRI(activeContext, KeywordReverse, nil, true, false)
			if err != nil {
				return nil, err
			}
			result.Properties = append(result.Properties, compact{
				Attr:       alias,
				Properties: final.Properties,
			})

			// 12.3.4)
			continue
		}

		// 12.4)
		if expandedProperty == KeywordPreserve {
			return nil, ErrPreserveUnsupported
		}

		// 12.5)
		if slices.Contains(activeTermDefinition.Container, KeywordIndex) && expandedProperty == KeywordIndex {
			continue
		} else if expandedProperty == KeywordDirection ||
			expandedProperty == KeywordIndex ||
			expandedProperty == KeywordLanguage ||
			expandedProperty == KeywordValue {
			// 12.6)

			// 12.6.1)
			alias, err := p.compactIRI(activeContext, expandedProperty, nil, true, false)
			if err != nil {
				return nil, err
			}

			// 12.6.2)
			var value jsontext.Value
			switch expandedProperty {
			case KeywordValue:
				value = object.Value
			default:
				var s string
				switch expandedProperty {
				case KeywordDirection:
					s = object.Direction
				case KeywordIndex:
					s = object.Index
				case KeywordLanguage:
					s = object.Language
				}
				q, err := jsontext.AppendQuote(nil, s)
				if err != nil {
					return nil, err
				}
				value = q
			}
			result.Properties = append(result.Properties, compact{
				Attr:  alias,
				Value: value,
			})
			continue
		}

		var expandedValue []Node
		switch expandedProperty {
		case KeywordList:
			expandedValue = object.List
		case KeywordGraph:
			expandedValue = object.Graph
		case KeywordIncluded:
			expandedValue = object.Included
		default:
			expandedValue = object.Properties[expandedProperty]
		}

		// 12.7
		if len(expandedValue) == 0 {
			itemActiveProperty, err := p.compactIRI(
				activeContext,
				expandedProperty,
				&Node{}, // Needs a zero-value, not a nil. See TestCompactCustom.
				true, insideReverse,
			)
			if err != nil {
				return nil, err
			}

			// 12.7.2) 12.7.3)
			nestResult, err := p.nestFor(ctx, activeContext, result, itemActiveProperty)
			if err != nil {
				return nil, err
			}

			// 12.7.4)
			nestResult.Properties = append(nestResult.Properties, compact{
				Attr:    itemActiveProperty,
				Members: []compact{},
			})
		}

		// 12.8)
		for _, expandedItem := range expandedValue {
			// 12.8.1)
			itemActiveProperty, err := p.compactIRI(
				activeContext,
				expandedProperty,
				&expandedItem,
				true, insideReverse,
			)
			if err != nil {
				return nil, err
			}

			// 12.8.2) 12.8.3)
			nestResult, err := p.nestFor(ctx, activeContext, result, itemActiveProperty)
			if err != nil {
				return nil, err
			}

			itemDef := activeContext.defs[itemActiveProperty]

			// 12.8.4)
			container := itemDef.Container

			// 12.8.5)
			asArray := !compactArrays
			if itemActiveProperty == KeywordList || itemActiveProperty == KeywordGraph || slices.Contains(container, KeywordSet) {
				asArray = true
			}

			// 12.8.6)
			var compactedItem *compact
			if expandedItem.IsList() {
				compactedItem, err = p.compactArray(ctx, activeContext, itemActiveProperty, expandedItem.List, compactArrays)
			} else if expandedItem.IsGraph() {
				compactedItem, err = p.compactArray(ctx, activeContext, itemActiveProperty, expandedItem.Graph, compactArrays)
			} else {
				compactedItem, err = p.compactNode(ctx, activeContext, itemActiveProperty, expandedItem, compactArrays)
			}
			if err != nil {
				return nil, err
			}

			if compactedItem == nil {
				continue
			}

			// 12.8.7)
			if expandedItem.IsList() {
				// 12.8.7.1)
				if compactedItem.Members == nil {
					compactedItem = &compact{Members: []compact{*compactedItem}}
				}

				// 12.8.7.2)
				if !slices.Contains(container, KeywordList) {
					// 12.8.7.2.1)
					alias, err := p.compactIRI(
						activeContext,
						KeywordList,
						nil, true, false,
					)
					if err != nil {
						return nil, err
					}
					compactedMap := compact{Properties: []compact{{
						Attr:    alias,
						Members: compactedItem.Members,
					}}}

					// 12.8.7.2.2)
					if expandedItem.Has(KeywordIndex) {
						iAlias, err := p.compactIRI(
							activeContext,
							KeywordIndex,
							nil, true, false,
						)
						if err != nil {
							return nil, err
						}
						q, err := jsontext.AppendQuote(nil, expandedItem.Index)
						if err != nil {
							return nil, err
						}
						compactedMap.Properties = append(compactedMap.Properties, compact{
							Attr:  iAlias,
							Value: q,
						})
					}
					// 12.8.7.2.3)
					nestResult.addValue(itemActiveProperty, compactedMap, asArray)
				} else {
					// 12.8.7.3)
					entry := *compactedItem
					entry.Attr = itemActiveProperty
					if e, ok := nestResult.get(itemActiveProperty); ok {
						*e = entry
					} else {
						nestResult.Properties = append(nestResult.Properties, entry)
					}
				}
			} else if expandedItem.IsGraph() {
				// 12.8.8)
				if slices.Contains(container, KeywordGraph) &&
					slices.Contains(container, KeywordID) {
					// 12.8.8.1)
					mapObject, ok := nestResult.get(itemActiveProperty)
					if !ok {
						// 12.8.8.1.1)
						nestResult.Properties = append(nestResult.Properties, compact{
							Attr:       itemActiveProperty,
							Properties: []compact{},
						})
						mapObject = &nestResult.Properties[len(nestResult.Properties)-1]
					}

					// 12.8.8.1.2)
					vocab := true
					key := cmp.Or(expandedItem.ID.Value, KeywordNone)
					if expandedItem.Has(KeywordID) {
						vocab = false
					}
					alias, err := p.compactIRI(activeContext, key, nil, vocab, false)
					if err != nil {
						return nil, err
					}

					// 12.8.8.1.3)
					mapObject.addValue(alias, *compactedItem, asArray)
				} else if slices.Contains(container, KeywordGraph) &&
					slices.Contains(container, KeywordIndex) && expandedItem.IsSimpleGraph() {
					// 12.8.8.2)

					mapObject, ok := nestResult.get(itemActiveProperty)
					if !ok {
						// 12.8.8.2.1)
						nestResult.Properties = append(nestResult.Properties, compact{
							Attr:       itemActiveProperty,
							Properties: []compact{},
						})
						mapObject = &nestResult.Properties[len(nestResult.Properties)-1]
					}

					// 12.8.8.2.2)
					key := cmp.Or(expandedItem.Index, KeywordNone)

					// 12.8.8.2.3)
					mapObject.addValue(key, *compactedItem, asArray)
				} else if slices.Contains(container, KeywordGraph) && expandedItem.IsSimpleGraph() {
					// 12.8.8.3)
					cok := compactedItem.Members != nil
					clist := compactedItem.Members

					// 12.8.8.3.1)
					if cok && len(clist) > 1 {
						alias, err := p.compactIRI(activeContext, KeywordIncluded, nil, true, false)
						if err != nil {
							return nil, err
						}
						compactedItem = &compact{Properties: []compact{{
							Attr:    alias,
							Members: clist,
						}}}
					}

					// 12.8.8.3.2)
					if e, ok := nestResult.get(itemActiveProperty); ok {
						if e.Members == nil {
							*e = compact{Attr: e.Attr, Members: []compact{*e}}
						}
						if cok {
							e.Members = append(e.Members, clist...)
						} else {
							e.Members = append(e.Members, *compactedItem)
						}
					} else {
						if asArray && compactedItem.Members == nil {
							nestResult.Properties = append(nestResult.Properties, compact{
								Attr:    itemActiveProperty,
								Members: []compact{*compactedItem},
							})
						} else {
							ci := *compactedItem
							ci.Attr = itemActiveProperty
							nestResult.Properties = append(nestResult.Properties, ci)
						}
					}
				} else {
					// 12.8.8.4)
					alias, err := p.compactIRI(activeContext, KeywordGraph, nil, true, false)
					if err != nil {
						return nil, err
					}

					// 12.8.8.4.1)
					graphVal := *compactedItem
					graphVal.Attr = alias
					newItem := compact{Properties: []compact{graphVal}}

					// 12.8.8.4.2)
					if expandedItem.Has(KeywordID) {
						idAlias, err := p.compactIRI(activeContext, KeywordID, nil, true, false)
						if err != nil {
							return nil, err
						}
						val, err := p.compactIRI(activeContext, expandedItem.ID.Value, nil, false, false)
						if err != nil {
							return nil, err
						}
						q, err := jsontext.AppendQuote(nil, val)
						if err != nil {
							return nil, err
						}
						newItem.Properties = append(newItem.Properties, compact{
							Attr:  idAlias,
							Value: q,
							IsID:  true,
						})
					}

					// 12.8.8.4.3)
					if expandedItem.Has(KeywordIndex) {
						idxAlias, err := p.compactIRI(activeContext, KeywordIndex, nil, true, false)
						if err != nil {
							return nil, err
						}
						q, err := jsontext.AppendQuote(nil, expandedItem.Index)
						if err != nil {
							return nil, err
						}
						newItem.Properties = append(newItem.Properties, compact{
							Attr:  idxAlias,
							Value: q,
						})
					}

					// 12.8.8.4.4)
					nestResult.addValue(itemActiveProperty, newItem, asArray)
				}
			} else if !slices.Contains(container, KeywordGraph) && (slices.Contains(container, KeywordLanguage) ||
				slices.Contains(container, KeywordIndex) ||
				slices.Contains(container, KeywordID) ||
				slices.Contains(container, KeywordType)) {
				// 12.8.9)
				mapObject, ok := nestResult.get(itemActiveProperty)
				if !ok {
					// 12.8.9.1)
					nestResult.Properties = append(nestResult.Properties, compact{
						Attr:       itemActiveProperty,
						Properties: []compact{},
					})
					mapObject = &nestResult.Properties[len(nestResult.Properties)-1]
				}

				key := KeywordNull // this is invalid so we'll immediate see bugs
				if slices.Contains(container, KeywordLanguage) {
					key = KeywordLanguage
				} else if slices.Contains(container, KeywordIndex) {
					key = KeywordIndex
				} else if slices.Contains(container, KeywordID) {
					key = KeywordID
				} else if slices.Contains(container, KeywordType) {
					key = KeywordType
				}

				// 12.8.9.2)
				containerKey, err := p.compactIRI(activeContext,
					key, nil, true, false)
				if err != nil {
					return nil, err
				}

				// 12.8.9.3)
				indexKey := KeywordIndex
				if idef, iok := activeContext.defs[itemActiveProperty]; iok && idef.Index != "" {
					indexKey = idef.Index
				}

				mapKey := ""

				// 12.8.9.4)
				if expandedItem.IsValue() && slices.Contains(container, KeywordLanguage) {
					compactedItem = &compact{Value: expandedItem.Value}
					if expandedItem.Has(KeywordLanguage) {
						mapKey = expandedItem.Language
					}
				} else if slices.Contains(container, KeywordIndex) && indexKey == KeywordIndex {
					// 12.8.9.5)
					if expandedItem.Has(KeywordIndex) {
						mapKey = expandedItem.Index
					}
				} else if slices.Contains(container, KeywordIndex) && indexKey != KeywordIndex {
					// 12.8.9.6)

					// 12.8.9.6.1)
					expIdx, err := p.expandIRI(ctx, activeContext, indexKey, false, false, nil, nil)
					if err != nil {
						return nil, err
					}
					containerKey, err = p.compactIRI(activeContext, expIdx, nil, true, false)
					if err != nil {
						return nil, err
					}

					// 12.8.9.6.2)
					if compactedItem.Properties != nil {
						if value, vok := compactedItem.get(containerKey); vok {
							if value.Members != nil {
								if s, ok := value.Members[0].asString(); ok {
									mapKey = s
								}
								switch {
								case len(value.Members) == 2:
									nv := value.Members[1]
									nv.Attr = containerKey
									*value = nv
								case len(value.Members) > 2:
									value.Members = value.Members[1:]
								default:
									compactedItem.del(containerKey)
								}
							} else if s, ok := value.asString(); ok {
								mapKey = s
								compactedItem.del(containerKey)
							}
						}
					}
				} else if slices.Contains(container, KeywordID) {
					// 12.8.9.7)
					if compactedItem.Properties != nil {
						if value, vok := compactedItem.get(containerKey); vok {
							if s, ok := value.asString(); ok {
								mapKey = s
							}
							compactedItem.del(containerKey)
						}
					}
				} else if slices.Contains(container, KeywordType) {
					// 12.8.9.8)

					if compactedItem.Properties != nil {
						// 12.8.9.8.1)
						if value, vok := compactedItem.get(containerKey); vok {
							if value.Members != nil {
								if s, ok := value.Members[0].asString(); ok {
									mapKey = s
								}
								switch {
								case len(value.Members) == 2:
									nv := value.Members[1]
									nv.Attr = containerKey
									*value = nv
								case len(value.Members) > 2:
									value.Members = value.Members[1:]
								}
							} else if s, ok := value.asString(); ok {
								mapKey = s
								// 12.8.9.8.2)
								compactedItem.del(containerKey)
							}
						}

						// 12.8.9.8.4)
						if len(compactedItem.Properties) == 1 {
							k := compactedItem.Properties[0].Attr
							expIri, err := p.expandIRI(ctx, activeContext, k, false, true, nil, nil)
							if err != nil {
								return nil, err
							}

							if expIri == KeywordID {
								res, err := p.compactNode(
									ctx,
									activeContext,
									itemActiveProperty,
									Node{ID: expandedItem.ID},
									false,
								)
								if err != nil {
									return nil, err
								}
								if res != nil {
									compactedItem = res
								}
							}
						}
					}
				}

				// 12.8.9.9
				if mapKey == "" {
					alias, err := p.compactIRI(activeContext, KeywordNone, nil, true, false)
					if err != nil {
						return nil, err
					}
					mapKey = alias
				}

				// 12.8.9.10)
				mapObject.addValue(mapKey, *compactedItem, asArray)
			} else {
				// 12.8.10)
				if e, ok := nestResult.get(itemActiveProperty); ok {
					if e.Members == nil {
						*e = compact{Attr: e.Attr, Members: []compact{*e}}
					}
					e.Members = append(e.Members, *compactedItem)
				} else {
					if asArray {
						if itemDef.Type == KeywordJSON && jsonutil.IsArray(expandedItem.Value) {
							ci := *compactedItem
							ci.Attr = itemActiveProperty
							nestResult.Properties = append(nestResult.Properties, ci)
						} else {
							nestResult.Properties = append(nestResult.Properties, compact{
								Attr:    itemActiveProperty,
								Members: []compact{*compactedItem},
							})
						}
					} else {
						ci := *compactedItem
						ci.Attr = itemActiveProperty
						nestResult.Properties = append(nestResult.Properties, ci)
					}
				}
			}
		}
	}
	return result, nil
}

func (p *Processor) nestFor(
	ctx context.Context,
	activeContext *Context,
	result *compact,
	itemActiveProperty string,
) (*compact, error) {
	edef, eok := activeContext.defs[itemActiveProperty]
	if !eok || edef.Nest == "" {
		return result, nil
	}

	// 12.8.2.1)
	term, err := p.expandIRI(ctx, activeContext, edef.Nest, false, true, nil, nil)
	if err != nil {
		return nil, err
	}
	if term != KeywordNest {
		return nil, ErrInvalidNestValue
	}

	term = edef.Nest

	// 12.8.2.2) 12.8.2.3)
	if e, ok := result.get(term); ok {
		return e, nil
	}
	result.Properties = append(result.Properties, compact{
		Attr:       term,
		Properties: []compact{},
	})
	return &result.Properties[len(result.Properties)-1], nil
}

// sortedLeast sorts strings based on smallest first and if they're
// equal, then by string comparison.
func sortedLeast(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}
