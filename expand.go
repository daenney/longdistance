package longdistance

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"sourcery.dny.nu/longdistance/internal/iri"
	"sourcery.dny.nu/longdistance/internal/json"
)

type expandOptions struct {
	frameExpansion bool
	fromMap        bool
}

func (e expandOptions) withoutFromMap() expandOptions {
	return expandOptions{
		frameExpansion: e.frameExpansion,
	}
}

// Expand transforms a JSON document into JSON-LD expanded document form.
//
// If the document was retrieved from a URL, pass it as the second argument.
// Otherwise an empty string.
func (p *Processor) Expand(document io.Reader, url string) ([]Node, error) {
	opts := expandOptions{}
	baseIRI := cmp.Or(p.baseIRI, url)

	var ctx *Context

	if p.expandContext == nil {
		ctx = newContext(baseIRI)
	} else {
		var obj json.Object
		if err := json.Unmarshal(p.expandContext, &obj); err != nil {
			return nil, ErrInvalidLocalContext
		}
		var rawctx json.RawMessage
		if v, ok := obj[KeywordContext]; ok {
			rawctx = v
		} else {
			rawctx = p.expandContext
		}

		var err error
		dec := json.NewDecoder(bytes.NewReader(rawctx))
		ctx, err = p.context(nil, dec, "", newCtxProcessingOpts())
		if err != nil {
			return nil, err
		}
	}

	dec := json.NewDecoder(document)
	res, err := p.expand(ctx, "", dec, url, opts)
	if err != nil {
		return nil, err
	}

	if _, derr := dec.Token(); derr != io.EOF {
		return nil, errors.Join(err, fmt.Errorf("trailing garbage in JSON"))
	}

	if res == nil {
		return []Node{}, nil
	}

	// 19)
	if len(res) == 1 && res[0].IsSimpleGraph() {
		res = res[0].Graph
	}

	result := make([]Node, 0, len(res))
	for _, obj := range res {
		if obj.IsZero() {
			continue
		}

		if obj.IsValue() {
			continue
		}

		if obj.Has(KeywordID) && obj.Len() == 1 {
			continue
		}

		result = append(result, obj)
	}

	return result, nil
}

func (p *Processor) expand(
	activeCtx *Context,
	activeProp string,
	dec *json.Decoder,
	baseURL string,
	opts expandOptions,
) ([]Node, error) {
	// 2)
	if activeProp == KeywordDefault {
		opts.frameExpansion = false
	}

	// bail out on frame expansion since we don't do that
	if opts.frameExpansion {
		return nil, ErrFrameExpansionUnsupported
	}

	termDef := activeCtx.defs[activeProp]

	// 3)
	// If there was no term definition, then .Context is nil.
	propContext := termDef.Context

	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}

	// Handle based on token type
	switch t := tok.(type) {
	case nil:
		// 1)
		return nil, nil
	case json.Delim:
		// 5)
		if t == '[' {
			// array expansion
			return p.expandArray(activeCtx, activeProp, dec, baseURL, opts, termDef)
		}

		if t == '{' {
			// object expansion
			return p.expandObject(activeCtx, activeProp, dec, baseURL, opts, termDef, propContext)
		}

		return nil, ErrInvalidLocalContext
	default:
		// 4) scalar (string, number, or boolean)
		if activeProp == "" || activeProp == KeywordGraph {
			return nil, nil
		}

		if propContext != nil {
			ctxDec := json.NewDecoder(bytes.NewReader(propContext))
			nctx, err := p.context(activeCtx, ctxDec, termDef.BaseIRI, newCtxProcessingOpts())
			if err != nil {
				return nil, err
			}
			activeCtx = nctx
		}

		res, err := p.expandValue(activeCtx, activeProp, tok)
		if err != nil {
			return nil, err
		}
		return []Node{res}, nil
	}
}

func (p *Processor) expandArray(
	activeCtx *Context,
	activeProp string,
	dec *json.Decoder,
	baseURL string,
	opts expandOptions,
	termDef Term,
) ([]Node, error) {
	if !dec.More() {
		if _, err := dec.Token(); err != nil {
			return nil, err
		}

		if slices.Contains(termDef.Container, KeywordList) {
			return []Node{{List: []Node{}}}, nil
		}

		return []Node{}, nil
	}

	// 5.1)
	result := make([]Node, 0, 8)
	first := true

	// 5.2)
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}

		var res []Node
		isMap := false

		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{':
				isMap = true
				res, err = p.expandObject(activeCtx, activeProp, dec, baseURL, opts, termDef, termDef.Context)
			case '[':
				res, err = p.expandArray(activeCtx, activeProp, dec, baseURL, opts, termDef)
			default:
				return nil, ErrInvalidLocalContext
			}
		case nil:
			res = nil
		default:
			if activeProp != "" && activeProp != KeywordGraph {
				ctx := activeCtx

				if termDef.Context != nil {
					ctxDec := json.NewDecoder(bytes.NewReader(termDef.Context))
					ctx, err = p.context(ctx, ctxDec, termDef.BaseIRI, newCtxProcessingOpts())
					if err != nil {
						return nil, err
					}
				}

				node, err := p.expandValue(ctx, activeProp, tok)
				if err != nil {
					return nil, err
				}

				res = []Node{node}
			}
		}

		if err != nil {
			return nil, err
		}

		// 5.2.3)
		if !slices.Contains(termDef.Container, KeywordList) {
			result = append(result, res...)
			first = false
			continue
		}

		// 5.2.2)
		if first {
			if isMap && len(res) == 1 && len(res[0].List) > 0 {
				result = res
			} else {
				result = append(result, Node{List: res})
			}
			first = false
		} else {
			result[0].List = append(result[0].List, res...)
		}
	}

	if _, err := dec.Token(); err != nil {
		return nil, err
	}

	// 5.3)
	return result, nil
}

func (p *Processor) expandRaw(
	activeCtx *Context,
	activeProp string,
	value json.RawMessage,
	baseURL string,
	opts expandOptions,
) ([]Node, error) {
	if len(value) == 0 || json.IsNull(value) {
		return nil, nil
	}

	return p.expand(activeCtx, activeProp, json.NewDecoder(bytes.NewReader(value)), baseURL, opts)
}

func (p *Processor) expandObject(
	activeCtx *Context,
	activeProp string,
	dec *json.Decoder,
	baseURL string,
	opts expandOptions,
	termDef Term,
	propContext json.RawMessage,
) ([]Node, error) {
	// this is a bit unfortunate, but we have to go through all keys in the
	// object for the @value/@type lookup after. We can't avoid collecting
	// everything here.
	obj := make(json.Object, 8)

	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}

		key := tok.(string)

		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}

		obj[key] = value
	}

	if _, err := dec.Token(); err != nil {
		return nil, err
	}

	// 7)
	if activeCtx.previousCtx != nil && !opts.fromMap {
		hasValue := p.expandsToKeyword(activeCtx, KeywordValue, maps.Keys(obj))
		hasID := p.expandsToKeyword(activeCtx, KeywordID, maps.Keys(obj))
		if !hasValue && !(len(obj) == 1 && hasID) {
			activeCtx = activeCtx.previousCtx
		}
	}

	// 8)
	if propContext != nil {
		ropts := newCtxProcessingOpts()
		ropts.override = true
		nctx, err := p.context(activeCtx, json.NewDecoder(bytes.NewReader(propContext)), termDef.BaseIRI, ropts)
		if err != nil {
			return nil, err
		}

		activeCtx = nctx
	}

	// 9)
	if rawCtx, ok := obj[KeywordContext]; ok {
		nctx, err := p.context(activeCtx, json.NewDecoder(bytes.NewReader(rawCtx)), baseURL, newCtxProcessingOpts())
		if err != nil {
			return nil, err
		}

		activeCtx = nctx
	}

	// 10)
	typContext := activeCtx

	// 11) Find @type key and process type-scoped contexts
	var typeVal json.RawMessage
	for k, v := range obj {
		u, err := p.expandIRI(activeCtx, k, false, true, nil, nil)
		if err != nil {
			continue
		}
		if u == KeywordType {
			typeVal = v
			break
		}
	}

	var stringTerms []string
	if len(typeVal) > 0 {
		if err := json.Unmarshal(json.MakeArray(typeVal), &stringTerms); err != nil {
			return nil, ErrInvalidTypeValue
		}

		slices.Sort(stringTerms)

		for _, term := range stringTerms {
			if tscopeDef, ok := typContext.defs[term]; ok && tscopeDef.Context != nil {
				adef := activeCtx.defs[term]
				ropts := newCtxProcessingOpts()
				ropts.propagate = false

				nctx, err := p.context(activeCtx, json.NewDecoder(bytes.NewReader(tscopeDef.Context)), adef.BaseIRI, ropts)
				if err != nil {
					return nil, err
				}

				activeCtx = nctx
			}
		}
	}

	// 12)
	result := &Node{
		Properties: make(Properties, len(obj)),
	}

	nests := Properties{}

	var inputType string
	if len(stringTerms) > 0 {
		lastTerm := stringTerms[len(stringTerms)-1]

		u, err := p.expandIRI(activeCtx, lastTerm, false, true, nil, nil)
		if err != nil {
			return nil, err
		}

		inputType = u
	}

	// 13) and 14)
	if err := p.expandObjectKeys(
		result,
		nests,
		activeCtx,
		typContext,
		activeProp,
		inputType,
		baseURL,
		obj,
		opts,
	); err != nil {
		return nil, err
	}

	// 15)
	if result.Has(KeywordValue) {
		if !result.IsValue() {
			return nil, ErrInvalidValueObject
		}

		if result.Has(KeywordType) && (result.Has(KeywordLanguage) || result.Has(KeywordDirection)) {
			return nil, ErrInvalidValueObject
		}

		if !slices.Equal(result.Type, []string{KeywordJSON}) {
			if json.IsNull(result.Value) {
				return nil, nil
			}

			if result.Has(KeywordLanguage) && !json.IsString(result.Value) {
				return nil, ErrInvalidLanguageTaggedValue
			}

			if len(result.Type) > 1 || (len(result.Type) == 1 && !iri.IsAbsolute(result.Type[0])) {
				return nil, ErrInvalidTypedValue
			}
		}
	}

	// 17)
	if result.Has(KeywordSet) || result.Has(KeywordList) {
		if len(result.propsWithout(KeywordIndex, KeywordList, KeywordSet)) != 0 {
			return nil, ErrInvalidSetOrListObject
		}

		if result.Has(KeywordSet) {
			return result.Set, nil
		}

		return []Node{*result}, nil
	}

	// 18)
	if result.Has(KeywordLanguage) && result.Len() == 1 {
		return nil, nil
	}

	// 19)
	if activeProp == "" || activeProp == KeywordGraph {
		if result.Len() == 0 ||
			result.Has(KeywordList) ||
			result.Has(KeywordValue) ||
			(result.Len() == 1 && result.Has(KeywordID)) {
			return nil, nil
		}
	}

	return []Node{*result}, nil
}

func (p *Processor) expandObjectKeys(
	result *Node,
	nests Properties,
	activeCtx *Context,
	typContext *Context,
	activeProp string,
	inputType string,
	baseURL string,
	obj json.Object,
	opts expandOptions,
) error {
	// 13)
mainLoop:
	for key, value := range obj {
		// 13.1)
		if key == KeywordContext {
			continue
		}

		// 13.2)
		expProp, err := p.expandIRI(activeCtx, key, false, true, nil, nil)
		if err != nil {
			return err
		}

		// 13.3)
		if expProp == "" { // "null"
			continue
		}

		if !(isKeyword(expProp) || strings.Contains(expProp, ":")) {
			continue
		}

		// 13.4)
		if isKeyword(expProp) {
			// 13.4.1)
			if activeProp == KeywordReverse {
				return ErrInvalidReversePropertyMap
			}

			// 13.4.2)
			if result.Has(expProp) && (p.modeLD10 || (expProp != KeywordIncluded && expProp != KeywordType)) {
				return ErrCollidingKeywords
			}

			switch expProp {
			case KeywordID:
				// 13.4.3)
				if json.IsNull(value) {
					return ErrInvalidIDValue
				}

				var s string
				if err := json.Unmarshal(value, &s); err != nil {
					// 13.4.3.1)
					return ErrInvalidIDValue
				}

				if s == "" {
					return ErrInvalidIDValue
				}

				iri, err := p.expandIRI(activeCtx, s, true, false, nil, nil)
				if err != nil {
					return err
				}

				if iri == "" {
					// This is theoretically against spec, as empty string is
					// moonlighting for null and in theory we should output an
					// expanded form document with `id: null`. However, that's
					// invalid JSON-LD, so instead we error out here because
					// if someone does that it's BS or shenanigans.
					return ErrInvalidIDValue
				}

				// 13.4.3.2)
				result.ID = iri
			case KeywordType:
				// 13.4.4)
				if !json.IsString(value) && !json.IsArray(value) {
					// 13.4.4.1)
					return ErrInvalidTypeValue
				}

				// 13.4.4.2) 13.4.4.3) skipped because frame expansion

				// 13.4.4.4)
				value = json.MakeArray(value)

				var vals []string
				if err := json.Unmarshal(value, &vals); err != nil {
					return err
				}

				iris := make([]string, 0, len(vals))
				for _, v := range vals {
					u, err := p.expandIRI(typContext, v, true, true, nil, nil)
					if err != nil {
						return err
					}
					iris = append(iris, u)
				}

				// 13.4.4.5)
				result.Type = append(result.Type, iris...)
			case KeywordGraph:
				// 13.4.5)
				res, err := p.expandRaw(activeCtx, KeywordGraph, value, baseURL, opts.withoutFromMap())
				if err != nil {
					return err
				}
				result.Graph = res
			case KeywordIncluded:
				// 13.4.6)
				if p.modeLD10 {
					// 13.4.6.1)
					continue mainLoop
				}

				if !json.IsMap(value) && !json.IsArray(value) {
					return ErrInvalidIncludedValue
				}

				// 13.4.6.2)
				res, err := p.expandRaw(
					activeCtx,
					"",
					value,
					baseURL,
					opts.withoutFromMap(),
				)
				if err != nil {
					return err
				}

				// 13.4.6.3)
				if res == nil {
					return ErrInvalidIncludedValue
				}

				for _, elem := range res {
					if !elem.isNode() {
						return ErrInvalidIncludedValue
					}
				}
				result.Included = append(result.Included, res...)
			case KeywordValue:
				// 13.4.7)

				if inputType == KeywordJSON {
					// 13.4.7.1)
					if p.modeLD10 {
						return ErrInvalidValueObjectValue
					}
					result.Value = value
					continue mainLoop
				}

				// 13.4.7.2)
				if !json.IsScalar(value) && !json.IsNull(value) {
					return ErrInvalidValueObjectValue
				}

				// 13.4.7.3) // 13.4.7.4)
				result.Value = value
			case KeywordLanguage:
				// 13.4.8)
				var l string
				if err := json.Unmarshal(value, &l); err != nil {
					// 13.4.8.1)
					return ErrInvalidLanguageTaggedString
				}

				// 13.4.8.2)
				result.Language = strings.ToLower(l)
			case KeywordDirection:
				// 13.4.9)
				if p.modeLD10 {
					// 13.4.9.1)
					continue mainLoop
				}

				var d string
				if err := json.Unmarshal(value, &d); err != nil {
					return ErrInvalidBaseDirection
				}

				// 13.4.9.2)
				switch d {
				case DirectionLTR, DirectionRTL:
				default:
					return ErrInvalidBaseDirection
				}

				// 13.4.9.3)
				result.Direction = d
			case KeywordIndex:
				// 13.4.10)
				var i string
				if err := json.Unmarshal(value, &i); err != nil {
					// 13.4.10.1)
					return ErrInvalidIndexValue
				}

				// 13.4.10.2)
				result.Index = i
			case KeywordList:
				// 13.4.11)
				if activeProp == "" || activeProp == KeywordGraph {
					// 13.4.11.1)
					continue mainLoop
				}

				if json.IsEmptyArray(value) {
					result.List = make([]Node, 0)
				} else {
					// 13.4.11.2)
					res, err := p.expandRaw(
						activeCtx,
						activeProp,
						value,
						baseURL,
						opts.withoutFromMap(),
					)
					if err != nil {
						return err
					}
					result.List = res
				}
			case KeywordSet:
				// 13.4.12)
				res, err := p.expandRaw(
					activeCtx,
					activeProp,
					value,
					baseURL,
					opts.withoutFromMap(),
				)
				if err != nil {
					return err
				}
				result.Set = res
			case KeywordReverse:
				// 13.4.13)
				if !json.IsMap(value) {
					// 13.4.13.1)
					return ErrInvalidReverseValue
				}

				// 13.4.13.2)	}
				res, err := p.expandRaw(
					activeCtx,
					KeywordReverse,
					value,
					baseURL,
					opts.withoutFromMap(),
				)
				if err != nil {
					return err
				}

				for _, obj := range res {
					// 13.4.13.3)
					for k, v := range obj.Reverse {
						result.Properties[k] = append(result.Properties[k], v...)
					}

					// 13.4.13.4), 13.4.13.4.2)
					for k, v := range obj.Properties {
						if !result.Has(KeywordReverse) {
							result.Reverse = make(Properties, 8)
						}

						// 13.4.13.4.2.1
						for _, item := range v {
							// 13.4.13.4.2.1.1)
							if item.IsValue() || item.IsList() {
								return ErrInvalidReversePropertyValue
							}

							// 13.4.13.4.2.1.2)
							result.Reverse[k] = append(result.Reverse[k], item)
						}
					}
				}

				// 13.4.13.5)
				continue mainLoop
			case KeywordNest:
				// 13.4.14)
				if _, ok := nests[key]; !ok {
					nests[key] = []Node{}
				}

				continue mainLoop
			default:
				p.logger.Warn("unhandled property", slog.String("proprety", expProp))
			}

			// 13.4.15) skip because frame expansion
			// 13.4.16) 13.4.17) we've already been doing this implicitly at each step
			continue mainLoop
		}

		// 13.5)
		termDef := activeCtx.defs[key]
		cnt := termDef.Container
		expVal := []Node{}

		if termDef.Type == KeywordJSON {
			// 13.6)
			expVal = append(expVal, Node{Value: value, Type: []string{KeywordJSON}})
		} else if slices.Contains(cnt, KeywordLanguage) && json.IsMap(value) {
			// 13.7)
			var langMap json.Object
			if err := json.Unmarshal(value, &langMap); err != nil {
				return err
			}

			// 13.7.1)
			langPairs := make([]Node, 0, len(langMap))

			// 13.7.2)
			dir := cmp.Or(termDef.Direction, activeCtx.defaultDirection)

			// 13.7.4)
			for langKey, langValue := range langMap {
				// 13.7.4.1)
				langValue = json.MakeArray(langValue)

				var langValues json.Array
				if err := json.Unmarshal(langValue, &langValues); err != nil {
					return err
				}

				// 13.7.4.2)
				for _, item := range langValues {
					// 13.7.4.2.1)
					if json.IsNull(item) {
						continue
					}

					// 13.7.4.2.2)
					if !json.IsString(item) {
						return ErrInvalidLanguageMapValue
					}

					obj := Node{
						Value: item,
					}

					// 13.7.4.2.3)
					if ldef := activeCtx.defs[langKey]; ldef.IRI != KeywordNone && langKey != KeywordNone {
						// 13.7.4.2.4)
						obj.Language = langKey
					}

					// 13.7.4.2.5)
					if dir != "" && dir != KeywordNull {
						obj.Direction = dir
					}

					langPairs = append(langPairs, obj)
				}
			}
			// 13.7.4.2.6)
			expVal = langPairs
		} else if (slices.Contains(cnt, KeywordIndex) ||
			slices.Contains(cnt, KeywordType) ||
			slices.Contains(cnt, KeywordID)) &&
			json.IsMap(value) {
			// 13.8)

			var objVal json.Object
			if err := json.Unmarshal(value, &objVal); err != nil {
				return err
			}

			// 13.8.1) implicit, we've already initialised expVal

			// 13.8.2)
			idxKey := cmp.Or(termDef.Index, KeywordIndex)

			// 13.8.3)
			for idx, idxVal := range objVal {
				// 13.8.3.1) 13.8.3.3)
				mapCtx := activeCtx

				if (slices.Contains(cnt, KeywordID) ||
					slices.Contains(cnt, KeywordType)) &&
					activeCtx.previousCtx != nil {
					mapCtx = activeCtx.previousCtx
				}

				// 13.8.3.2)
				if slices.Contains(cnt, KeywordType) {
					if def, ok := mapCtx.defs[idx]; ok && def.Context != nil {
						dec := json.NewDecoder(bytes.NewReader(def.Context))
						nctx, err := p.context(
							mapCtx,
							dec,
							def.BaseIRI,
							newCtxProcessingOpts(),
						)
						if err != nil {
							return err
						}
						mapCtx = nctx
					}
				}

				// 13.8.3.4)
				expIdx, err := p.expandIRI(activeCtx, idx, false, true, nil, nil)
				if err != nil {
					return err
				}

				// 13.8.3.5)
				idxVal = json.MakeArray(idxVal)

				// 13.8.3.6)
				expIdxVals, err := p.expandRaw(
					mapCtx,
					key,
					idxVal,
					baseURL,
					expandOptions{fromMap: true, frameExpansion: opts.frameExpansion},
				)
				if err != nil {
					return err
				}

				// 13.8.3.7)
				for _, item := range expIdxVals {
					// 13.8.3.7.1)
					if slices.Contains(cnt, KeywordGraph) && item.Graph == nil {
						item = Node{Graph: []Node{item}}
					}

					if expIdx != KeywordNone {
						if slices.Contains(cnt, KeywordIndex) && idxKey != KeywordIndex {
							// 13.8.3.7.2)

							// 13.8.3.7.2.1)
							rexpIdx, err := p.expandValue(
								activeCtx,
								idxKey,
								idx,
							)
							if err != nil {
								return err
							}

							// 13.8.3.7.2.2)
							expIdxKey, err := p.expandIRI(activeCtx, idxKey, false, true, nil, nil)
							if err != nil {
								return err
							}

							// 13.8.3.7.2.3)
							rexpPropVals := []Node{rexpIdx}
							rexpPropVals = append(rexpPropVals, item.Properties[expIdxKey]...)

							// 13.8.3.7.2.4)
							if item.Properties == nil {
								item.Properties = make(Properties, 4)
							}
							item.Properties[expIdxKey] = rexpPropVals

							// 13.8.3.7.2.5)
							if item.Has(KeywordValue) && !item.IsValue() {
								return ErrInvalidValueObject
							}
						} else if slices.Contains(cnt, KeywordIndex) && !item.Has(KeywordIndex) {
							// 13.8.3.7.3)
							item.Index = idx
						} else if slices.Contains(cnt, KeywordID) && !item.Has(KeywordID) {
							// 13.8.3.7.4)
							idx, err := p.expandIRI(activeCtx,
								idx, true, false, nil, nil)
							if err != nil {
								return err
							}
							item.ID = idx
						} else if slices.Contains(cnt, KeywordType) {
							// 13.8.3.7.5)
							item.Type = append([]string{expIdx}, item.Type...)
						}
					}
					// 13.8.3.7.6)
					expVal = append(expVal, item)
				}
			}
		} else {
			// 13.9)
			var expErr error
			expVal, expErr = p.expandRaw(
				activeCtx,
				key,
				value,
				baseURL,
				opts.withoutFromMap(),
			)
			if expErr != nil {
				return expErr
			}
		}

		// 13.10)
		// check for nil and not len()>0 because a slice of 0 elements still
		// needs to be retained for sets. expand will return nil if the
		// element should be dropped.
		if expVal == nil {
			continue mainLoop
		}

		// 13.11)
		if slices.Contains(termDef.Container, KeywordList) {
			if len(expVal) != 1 || !expVal[0].IsList() {
				expVal = []Node{{List: expVal}}
			}
		}

		// 13.12)
		if slices.Contains(cnt, KeywordGraph) && !slices.Contains(cnt, KeywordID) && !slices.Contains(cnt, KeywordIndex) {
			res := make([]Node, 0, len(expVal))
			for _, obj := range expVal {
				res = append(res, Node{Graph: []Node{obj}})
			}
			expVal = res
		}

		// 13.13)
		if termDef.Reverse {
			// 13.13.1)
			if !result.Has(KeywordReverse) {
				result.Reverse = make(Properties, len(expVal))
			}

			// 13.13.2) can reference result.Reverse directly
			// 13.13.3) already is an array

			// 13.13.4)
			for _, obj := range expVal {
				// 13.13.4.1)
				if obj.IsValue() || obj.IsList() {
					return ErrInvalidReversePropertyValue
				}
				// 13.13.4.3)
				if result.Reverse[expProp] == nil {
					result.Reverse[expProp] = make([]Node, 0, len(obj.Properties)+2)
				}
				result.Reverse[expProp] = append(result.Reverse[expProp], obj)
			}
		} else {
			// 13.14)
			// explicitly initialise the expProp in case the first time
			// we encounter expProp expVal is an empty set because
			// appending with len(expVal)==0 does nothing but we need
			// to retain the fact that we got an empty array
			if !result.Has(expProp) {
				result.Properties[expProp] = expVal
			} else {
				result.Properties[expProp] = append(result.Properties[expProp], expVal...)
			}
		}
	}

	// 14)
	for k := range nests {
		// 14.1)
		nestData := json.MakeArray(obj[k])

		var nestValues []json.Object
		if err := json.Unmarshal(nestData, &nestValues); err != nil {
			return ErrInvalidNestValue
		}

		for _, nestValue := range nestValues {
			if p.expandsToKeyword(
				activeCtx,
				KeywordValue,
				maps.Keys(nestValue),
			) {
				// 14.2.1)
				return ErrInvalidNestValue
			}
			// 14.2.2)
			nestCtx := activeCtx
			if termDef := activeCtx.defs[k]; termDef.Context != nil {
				ropts := newCtxProcessingOpts()
				ropts.override = true

				nctx, err := p.context(activeCtx, json.NewDecoder(bytes.NewReader(termDef.Context)), termDef.BaseIRI, ropts)
				if err != nil {
					return err
				}

				nestCtx = nctx
			}
			if err := p.expandObjectKeys(
				result,
				nests,
				nestCtx,
				typContext,
				k,
				inputType,
				baseURL,
				nestValue,
				opts,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Processor) expandsToKeyword(
	activeContext *Context,
	keyword string,
	elems iter.Seq[string],
) bool {
	for k := range elems {
		res, err := p.expandIRI(
			activeContext,
			k, false, true, nil, nil,
		)

		if err != nil {
			return false
		}

		if res == keyword {
			return true
		}
	}

	return false
}

func (p *Processor) expandValue(
	ctx *Context,
	property string,
	value any,
) (Node, error) {
	def := ctx.defs[property]
	result := Node{}

	switch def.Type {
	case KeywordID, KeywordVocab:
		// 1) 2)
		val, ok := value.(string)
		if !ok || val == "" {
			break // don't coerce types of some other value
		}

		u, err := p.expandIRI(ctx, val, true, def.Type == KeywordVocab, nil, nil)
		if err != nil {
			return result, err
		}

		result.ID = u
		return result, nil
	case KeywordNone, "":
		// 4)
	default:
		// 4)
		result.Type = []string{def.Type}
	}

	// 3)
	raw, _ := json.Marshal(value)
	result.Value = raw

	// 5)
	if _, ok := value.(string); ok {
		// 5.1)
		lang := cmp.Or(def.Language, ctx.defaultLang)

		// 5.2)
		dir := cmp.Or(def.Direction, ctx.defaultDirection)

		// 5.3)
		if lang != KeywordNull {
			result.Language = lang
		}

		// 5.4)
		if dir != KeywordNull {
			result.Direction = dir
		}
	}

	return result, nil
}
