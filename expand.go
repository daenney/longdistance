package longdistance

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io"
	"iter"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"sourcery.dny.nu/longdistance/internal/iri"
	"sourcery.dny.nu/longdistance/internal/jsonutil"
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
func (p *Processor) Expand(
	ctx context.Context,
	document io.Reader, url string) ([]Node, error) {
	opts := expandOptions{}
	baseIRI := cmp.Or(p.baseIRI, url)

	var ldCtx *Context

	if p.expandContext == nil {
		ldCtx = newContext(baseIRI)
	} else {
		var obj jsonutil.Object
		if err := json.Unmarshal(p.expandContext, &obj); err != nil {
			return nil, ErrInvalidLocalContext
		}
		var rawctx jsontext.Value
		if v, ok := obj[KeywordContext]; ok {
			rawctx = v
		} else {
			rawctx = p.expandContext
		}

		var err error
		ldCtx, err = p.context(ctx, nil, jsontext.NewDecoder(bytes.NewReader(rawctx)), "", newCtxProcessingOpts())
		if err != nil {
			return nil, err
		}
	}

	dec := jsontext.NewDecoder(document,
		jsontext.AllowDuplicateNames(false),
		jsontext.AllowInvalidUTF8(false),
	)

	res, err := p.expand(ctx, ldCtx, "", dec, url, opts)
	if err != nil {
		return nil, err
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
	ctx context.Context,
	activeCtx *Context,
	activeProp string,
	dec *jsontext.Decoder,
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

	switch dec.PeekKind() {
	case jsontext.KindNull:
		// 1)
		_, err := dec.ReadToken()
		return nil, err
	case jsontext.KindBeginArray:
		// 5) array expansion
		return p.expandArray(ctx, activeCtx, activeProp, dec, baseURL, opts, termDef)
	case jsontext.KindBeginObject:
		// 5) object expansion
		return p.expandObject(ctx, activeCtx, activeProp, dec, baseURL, opts, termDef, propContext)
	case jsontext.KindFalse, jsontext.KindTrue, jsontext.KindString, jsontext.KindNumber:
		value, _ := dec.ReadValue()
		if !value.IsValid() {
			return nil, ErrInvalidLocalContext
		}

		// 4) scalar (string, number, or boolean)
		return p.expandScalar(ctx, activeCtx, activeProp, value, termDef)
	default:
		return nil, ErrInvalidLocalContext
	}
}

func (p *Processor) expandArray(
	ctx context.Context,
	activeCtx *Context,
	activeProp string,
	dec *jsontext.Decoder,
	baseURL string,
	opts expandOptions,
	termDef Term,
) ([]Node, error) {
	// consume opening '['
	_, err := dec.ReadToken()
	if err != nil {
		return nil, err
	}

	// check it's not an empty array
	if dec.PeekKind() == jsontext.KindEndArray {
		_, err := dec.ReadToken()
		if err != nil {
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
LOOP:
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var res []Node
		isMap := false

		switch dec.PeekKind() {
		case jsontext.KindNull:
			_, err := dec.ReadToken()
			if err != nil {
				return nil, err
			}
			res = nil
		case jsontext.KindEndArray:
			_, err := dec.ReadToken()
			if err != nil {
				return nil, err
			}

			break LOOP
		case jsontext.KindBeginArray:
			res, err = p.expandArray(ctx, activeCtx, activeProp, dec, baseURL, opts, termDef)
		case jsontext.KindBeginObject:
			isMap = true
			res, err = p.expandObject(ctx, activeCtx, activeProp, dec, baseURL, opts, termDef, termDef.Context)
		case jsontext.KindFalse, jsontext.KindTrue, jsontext.KindNumber, jsontext.KindString:
			value, verr := dec.ReadValue()
			if verr != nil {
				return nil, ErrInvalidLocalContext
			}

			res, err = p.expandScalar(ctx, activeCtx, activeProp, value, termDef)
		default:
			return nil, ErrInvalidLocalContext
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

	// 5.3)
	return result, nil
}

func (p *Processor) expandRaw(
	ctx context.Context,
	activeCtx *Context,
	activeProp string,
	value jsontext.Value,
	baseURL string,
	opts expandOptions,
) ([]Node, error) {
	if len(value) == 0 || value.Kind() == jsontext.KindNull {
		return nil, nil
	}

	return p.expand(ctx, activeCtx, activeProp, jsontext.NewDecoder(bytes.NewReader(value)), baseURL, opts)
}

func (p *Processor) expandObject(
	ctx context.Context,
	activeCtx *Context,
	activeProp string,
	dec *jsontext.Decoder,
	baseURL string,
	opts expandOptions,
	termDef Term,
	propContext jsontext.Value,
) ([]Node, error) {
	// consume opening '{'
	_, err := dec.ReadToken()
	if err != nil {
		return nil, err
	}

	// check for empty object
	if dec.PeekKind() == jsontext.KindEndObject {
		_, err := dec.ReadToken()
		return nil, err
	}

	// this is a bit unfortunate, but we have to go through all keys in the
	// object for the @value/@type lookup after. We can't avoid collecting
	// everything here.
	obj := make(jsonutil.Object, 8)

LOOP:
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if dec.PeekKind() == jsontext.KindEndObject {
			_, err := dec.ReadToken()
			if err != nil {
				return nil, err
			}

			break LOOP
		}

		// read key
		tok, err := dec.ReadToken()
		if err != nil {
			return nil, err
		}

		key := tok.String()

		// read value
		value, err := dec.ReadValue()
		if err != nil {
			return nil, err
		}

		obj[key] = value.Clone()
	}

	// 7)
	if activeCtx.previousCtx != nil && !opts.fromMap {
		hasValue := p.expandsToKeyword(ctx, activeCtx, KeywordValue, maps.Keys(obj))
		hasID := p.expandsToKeyword(ctx, activeCtx, KeywordID, maps.Keys(obj))
		if !hasValue && !(len(obj) == 1 && hasID) {
			activeCtx = activeCtx.previousCtx
		}
	}

	// 8)
	if propContext != nil {
		ropts := newCtxProcessingOpts()
		ropts.override = true
		nctx, err := p.context(ctx, activeCtx, jsontext.NewDecoder(bytes.NewReader(propContext)), termDef.BaseIRI, ropts)
		if err != nil {
			return nil, err
		}

		activeCtx = nctx
	}

	// 9)
	if rawCtx, ok := obj[KeywordContext]; ok {
		nctx, err := p.context(ctx, activeCtx, jsontext.NewDecoder(bytes.NewReader(rawCtx)), baseURL, newCtxProcessingOpts())
		if err != nil {
			return nil, err
		}

		activeCtx = nctx
	}

	// 10)
	typContext := activeCtx

	// 11) Find @type key and process type-scoped contexts
	var typeVal jsontext.Value
	for k, v := range obj {
		u, err := p.expandIRI(ctx, activeCtx, k, false, true, nil, nil)
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
		if err := json.Unmarshal(jsonutil.MakeArray(typeVal), &stringTerms); err != nil {
			return nil, ErrInvalidTypeValue
		}

		slices.Sort(stringTerms)

		for _, term := range stringTerms {
			if tscopeDef, ok := typContext.defs[term]; ok && tscopeDef.Context != nil {
				adef := activeCtx.defs[term]
				ropts := newCtxProcessingOpts()
				ropts.propagate = false

				nctx, err := p.context(ctx, activeCtx, jsontext.NewDecoder(bytes.NewReader(tscopeDef.Context)), adef.BaseIRI, ropts)
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

		u, err := p.expandIRI(ctx, activeCtx, lastTerm, false, true, nil, nil)
		if err != nil {
			return nil, err
		}

		inputType = u
	}

	// 13) and 14)
	if err := p.expandObjectKeys(
		ctx,
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
			if jsonutil.IsNull(result.Value) {
				return nil, nil
			}

			if result.Has(KeywordLanguage) && !jsonutil.IsString(result.Value) {
				return nil, ErrInvalidLanguageTaggedValue
			}

			if len(result.Type) > 1 || (len(result.Type) == 1 && !iri.IsAbsolute(result.Type[0])) {
				return nil, ErrInvalidTypedValue
			}
		}
	}

	// 17)
	if result.Has(KeywordSet) || result.Has(KeywordList) {
		if result.propsWithout(KeywordIndex, KeywordList, KeywordSet) != 0 {
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
	ctx context.Context,
	result *Node,
	nests Properties,
	activeCtx *Context,
	typContext *Context,
	activeProp string,
	inputType string,
	baseURL string,
	obj jsonutil.Object,
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
		expProp, err := p.expandIRI(ctx, activeCtx, key, false, true, nil, nil)
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
			if _, ok := p.disallowedKeys[expProp]; ok {
				return ErrDisallowedKeyword
			}

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
				if jsonutil.IsNull(value) {
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

				iri, err := p.expandIRI(ctx, activeCtx, s, true, false, nil, nil)
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
				if !jsonutil.IsString(value) && !jsonutil.IsArray(value) {
					// 13.4.4.1)
					return ErrInvalidTypeValue
				}

				// 13.4.4.2) 13.4.4.3) skipped because frame expansion

				// 13.4.4.4)
				value = jsonutil.MakeArray(value)

				var vals []string
				if err := json.Unmarshal(value, &vals); err != nil {
					return err
				}

				iris := make([]string, 0, len(vals))
				for _, v := range vals {
					u, err := p.expandIRI(ctx, typContext, v, true, true, nil, nil)
					if err != nil {
						return err
					}
					iris = append(iris, u)
				}

				// 13.4.4.5)
				result.Type = append(result.Type, iris...)
			case KeywordGraph:
				// 13.4.5)
				res, err := p.expandRaw(ctx, activeCtx, KeywordGraph, value, baseURL, opts.withoutFromMap())
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

				if !jsonutil.IsMap(value) && !jsonutil.IsArray(value) {
					return ErrInvalidIncludedValue
				}

				// 13.4.6.2)
				res, err := p.expandRaw(
					ctx,
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
				if !jsonutil.IsScalar(value) && !jsonutil.IsNull(value) {
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

				if jsonutil.IsEmptyArray(value) {
					result.List = make([]Node, 0)
				} else {
					// 13.4.11.2)
					res, err := p.expandRaw(
						ctx,
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
					ctx,
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
				if !jsonutil.IsMap(value) {
					// 13.4.13.1)
					return ErrInvalidReverseValue
				}

				// 13.4.13.2)	}
				res, err := p.expandRaw(
					ctx,
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
		} else if slices.Contains(cnt, KeywordLanguage) && jsonutil.IsMap(value) {
			// 13.7)
			var langMap jsonutil.Object
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
				langValue = jsonutil.MakeArray(langValue)

				var langValues jsonutil.Array
				if err := json.Unmarshal(langValue, &langValues); err != nil {
					return err
				}

				// 13.7.4.2)
				for _, item := range langValues {
					// 13.7.4.2.1)
					if jsonutil.IsNull(item) {
						continue
					}

					// 13.7.4.2.2)
					if !jsonutil.IsString(item) {
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
			jsonutil.IsMap(value) {
			// 13.8)

			var objVal jsonutil.Object
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
						nctx, err := p.context(
							ctx,
							mapCtx,
							jsontext.NewDecoder(bytes.NewReader(def.Context)),
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
				expIdx, err := p.expandIRI(ctx, activeCtx, idx, false, true, nil, nil)
				if err != nil {
					return err
				}

				// 13.8.3.5)
				idxVal = jsonutil.MakeArray(idxVal)

				// 13.8.3.6)
				expIdxVals, err := p.expandRaw(
					ctx,
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
								ctx,
								activeCtx,
								idxKey,
								jsontext.Value(`"`+idx+`"`),
							)
							if err != nil {
								return err
							}

							// 13.8.3.7.2.2)
							expIdxKey, err := p.expandIRI(ctx, activeCtx, idxKey, false, true, nil, nil)
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
							idx, err := p.expandIRI(ctx, activeCtx,
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
				ctx,
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
		nestData := jsonutil.MakeArray(obj[k])

		var nestValues []jsonutil.Object
		if err := json.Unmarshal(nestData, &nestValues); err != nil {
			return ErrInvalidNestValue
		}

		for _, nestValue := range nestValues {
			if p.expandsToKeyword(
				ctx,
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

				nctx, err := p.context(ctx, activeCtx, jsontext.NewDecoder(bytes.NewReader(termDef.Context)), termDef.BaseIRI, ropts)
				if err != nil {
					return err
				}

				nestCtx = nctx
			}
			if err := p.expandObjectKeys(
				ctx,
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
	ctx context.Context,
	activeContext *Context,
	keyword string,
	elems iter.Seq[string],
) bool {
	for k := range elems {
		res, err := p.expandIRI(
			ctx,
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

func (p *Processor) expandScalar(
	ctx context.Context,
	activeCtx *Context,
	activeProp string,
	value jsontext.Value,
	termDef Term,
) ([]Node, error) {
	// 4) scalar (string, number, or boolean)
	if activeProp == "" || activeProp == KeywordGraph {
		return nil, nil
	}

	if termDef.Context != nil {
		nctx, err := p.context(ctx, activeCtx,
			jsontext.NewDecoder(bytes.NewReader(termDef.Context)),
			termDef.BaseIRI, newCtxProcessingOpts())
		if err != nil {
			return nil, err
		}
		activeCtx = nctx
	}

	node, err := p.expandValue(ctx, activeCtx, activeProp, value)
	if err != nil {
		return nil, err
	}
	return []Node{node}, nil
}

func (p *Processor) expandValue(
	ctx context.Context,
	ldContext *Context,
	property string,
	value jsontext.Value,
) (Node, error) {
	result := Node{}

	kind := value.Kind()
	def := ldContext.defs[property]

	switch def.Type {
	case KeywordID, KeywordVocab:
		// 1) 2)
		if kind != jsontext.KindString {
			break // don't coerce types of some other value
		}

		var v string
		if err := json.Unmarshal(value, &v); err != nil {
			return Node{}, ErrInvalidLocalContext
		}

		if v == "" {
			break
		}

		u, err := p.expandIRI(ctx, ldContext, v, true, def.Type == KeywordVocab, nil, nil)
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
	result.Value = value.Clone()

	// 5)
	if result.Type == nil && kind == jsontext.KindString {
		// 5.1)
		lang := cmp.Or(def.Language, ldContext.defaultLang)

		// 5.2)
		dir := cmp.Or(def.Direction, ldContext.defaultDirection)

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
