package longdistance

import (
	"bytes"
	"cmp"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"sourcery.dny.nu/longdistance/internal/json"
	"sourcery.dny.nu/longdistance/internal/url"
)

type expandOptions struct {
	frameExpansion bool
	ordered        bool
	fromMap        bool
}

func (e expandOptions) clone() expandOptions {
	return expandOptions{
		frameExpansion: e.frameExpansion,
		ordered:        e.ordered,
		fromMap:        e.fromMap,
	}
}

func newExpandOptions() expandOptions {
	return expandOptions{}
}

// Expand transforms a JSON document into JSON-LD expanded document form.
//
// If the document was retrieved from a URL, pass it as the second argument.
// Otherwise an empty string.
func (p *Processor) Expand(document json.RawMessage, url string) ([]Node, error) {
	xopts := newExpandOptions()
	xopts.ordered = p.ordered
	baseIRI := cmp.Or(p.baseIRI, url)

	ctx := newContext(baseIRI)
	if p.expandContext != nil {
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
		nctx, err := p.context(ctx, rawctx, ctx.originalBaseIRI, newCtxProcessingOpts())
		if err != nil {
			return nil, err
		}
		ctx = nctx
	}

	res, err := p.expand(ctx, "", document, url, xopts)
	if err != nil {
		return res, err
	}

	if res == nil {
		return []Node{}, nil
	}

	// 19)
	if len(res) == 1 {
		r := res[0]
		if r.IsSimpleGraph() {
			res = r.Graph
		}
	}

	result := make([]Node, 0, len(res))
	for _, obj := range res {
		if obj.IsZero() {
			continue
		}

		if obj.IsValue() {
			continue
		}

		if obj.Has(KeywordID) && len(obj.PropertySet()) == 1 {
			continue
		}

		result = append(result, obj)
	}

	return result, nil
}

func (p *Processor) expand(
	activeContext *Context,
	activeProperty string,
	element json.RawMessage,
	baseURL string,
	opts expandOptions,
) ([]Node, error) {
	// 1)
	if len(element) == 0 || json.IsNull(element) {
		return nil, nil
	}

	// 2)
	if activeProperty == KeywordDefault {
		opts.frameExpansion = false
	}

	// bail out on frame expansion since we don't do that
	if opts.frameExpansion {
		return nil, ErrFrameExpansionUnsupported
	}

	termDef, hasDef := activeContext.defs[activeProperty]

	// 3)
	var propContext json.RawMessage
	if hasDef {
		if termDef.Context != nil {
			propContext = termDef.Context
		}
	}

	switch element[0] { // null was handled at the function start
	case '[':
		// 5)
		var elems json.Array
		if err := json.Unmarshal(element, &elems); err != nil {
			return nil, err
		}

		if len(elems) == 0 {
			if slices.Contains(termDef.Container, KeywordList) {
				return []Node{{List: []Node{}}}, nil
			}
			return make([]Node, 0), nil
		}

		// 5.1)
		result := make([]Node, 0, len(elems))

		// 5.2)
		for _, elem := range elems {
			// 5.2.1)
			res, err := p.expand(
				activeContext,
				activeProperty,
				elem,
				baseURL,
				opts.clone(),
			)
			if err != nil {
				return nil, err
			}

			// 5.2.2)
			if slices.Contains(termDef.Container, KeywordList) {
				if len(elems) > 1 {
					if len(result) == 0 {
						result = append(result, Node{List: res})
					} else {
						result[0].List = append(result[0].List, res...)
					}
				} else {
					if json.IsMap(elem) && len(res[0].List) != 0 {
						result = append(result, res...)
					} else {
						result = append(result, Node{List: res})
					}
				}
			} else {
				// 5.2.3)
				result = append(result, res...)
			}
		}
		// 5.3)
		return result, nil
	case '{':
		// happens after the switch
	default:
		// 4)
		// 4.1)
		if activeProperty == "" || activeProperty == KeywordGraph {
			return nil, nil
		}

		// 4.2)
		if propContext != nil {
			def := activeContext.defs[activeProperty]
			nctx, err := p.context(
				activeContext,
				propContext,
				def.BaseIRI,
				newCtxProcessingOpts(),
			)
			if err != nil {
				return nil, err
			}
			activeContext = nctx
		}

		// 4.3)
		res, err := p.expandValue(
			activeContext,
			activeProperty,
			element,
		)
		if err != nil {
			return nil, err
		}
		return []Node{res}, nil
	}

	// 6)
	var elemObj json.Object
	if err := json.Unmarshal(element, &elemObj); err != nil {
		return nil, err
	}

	elemKeys := slices.Collect(maps.Keys(elemObj))

	// 7)
	if activeContext.previousContext != nil && !opts.fromMap {
		hasValue := p.expandsToKeyword(
			activeContext,
			KeywordValue,
			elemKeys,
		)
		hasID := p.expandsToKeyword(
			activeContext,
			KeywordID,
			elemKeys,
		)
		if !hasValue && !(len(elemObj) == 1 && hasID) {
			activeContext = activeContext.previousContext
		}
	}

	// 8)
	if propContext != nil {
		ropts := newCtxProcessingOpts()
		ropts.override = true
		nctx, err := p.context(
			activeContext,
			propContext,
			termDef.BaseIRI,
			ropts,
		)
		if err != nil {
			return nil, err
		}
		activeContext = nctx
	}

	// 9)
	if ctx, ok := elemObj[KeywordContext]; ok {
		nctx, err := p.context(
			activeContext,
			ctx,
			baseURL,
			newCtxProcessingOpts(),
		)
		if err != nil {
			return nil, err
		}
		activeContext = nctx
	}

	// 10)
	typContext := activeContext

	// 11)
	objKeys := slices.Collect(maps.Keys(elemObj))
	slices.Sort(objKeys)

	for _, k := range objKeys {
		u, err := p.expandIRI(activeContext, k, false, true, nil, nil)
		if err != nil {
			return nil, err
		}

		if u != KeywordType {
			continue
		}

		val := json.MakeArray(elemObj[k])
		var values []json.RawMessage
		if err := json.Unmarshal(val, &values); err != nil {
			return nil, err
		}
		stringTerms := make([]string, 0, len(values))
		for _, term := range values {
			var s string
			if err := json.Unmarshal(term, &s); err != nil {
				return nil, ErrInvalidTypeValue
			}
			stringTerms = append(stringTerms, s)
		}
		slices.Sort(stringTerms)
		for _, term := range stringTerms {
			if tscopeDef, ok := typContext.defs[term]; ok && tscopeDef.Context != nil {
				adef := activeContext.defs[term]
				ropts := newCtxProcessingOpts()
				ropts.propagate = false
				nctx, err := p.context(
					activeContext,
					tscopeDef.Context,
					adef.BaseIRI,
					ropts,
				)
				if err != nil {
					return nil, err
				}
				activeContext = nctx
			}
		}
	}

	// 12)
	result := &Node{
		Properties: make(Properties, len(objKeys)),
	}
	nests := Properties{}
	inputType := ""

	entry := ""
	for _, k := range objKeys {
		u, err := p.expandIRI(activeContext, k, false, true, nil, nil)
		if err != nil {
			return nil, err
		}
		if u == KeywordType {
			entry = k
			break
		}
	}

	if entry != "" {
		vals := json.MakeArray(elemObj[entry])
		var valElems json.Array
		if err := json.Unmarshal(vals, &valElems); err != nil {
			return nil, err
		}
		last := valElems[len(valElems)-1]
		var s string
		if err := json.Unmarshal(last, &s); err != nil {
			return nil, err
		}
		u, err := p.expandIRI(activeContext, s, false, true, nil, nil)
		if err != nil {
			return nil, err
		}
		inputType = u
	}

	// 13) and 14)
	if err := p.expandElement(
		result,
		nests,
		activeContext,
		typContext,
		activeProperty,
		inputType,
		baseURL,
		elemObj,
		opts.clone(),
	); err != nil {
		return nil, err
	}

	// 15)
	if result.Has(KeywordValue) {
		// 15.1)
		if !result.IsValue() {
			return nil, ErrInvalidValueObject
		}

		if result.Has(KeywordType) && (result.Has(KeywordLanguage) || result.Has(KeywordDirection)) {
			return nil, ErrInvalidValueObject
		}

		if slices.Equal(result.Type, []string{KeywordJSON}) {
			// 15.2)
		} else if json.IsNull(result.Value) {
			// 15.3)
			return nil, nil
		} else if result.Has(KeywordLanguage) && !json.IsString(result.Value) {
			// 15.4)
			return nil, ErrInvalidLanguageTaggedValue
		} else if len(result.Type) > 1 {
			// 15.5)
			return nil, ErrInvalidTypedValue
		} else if len(result.Type) == 1 {
			// 15.5)
			if !url.IsIRI(result.Type[0]) {
				return nil, ErrInvalidTypedValue
			}
		}
	}

	// 16) implicit since [Object.Type] is always an array

	// 17)
	if result.Has(KeywordSet) || result.Has(KeywordList) {
		// 17.1)
		if len(result.propsWithout(
			KeywordIndex,
			KeywordList,
			KeywordSet,
		)) != 0 {
			return nil, ErrInvalidSetOrListObject
		}

		// 17.2)
		if result.Has(KeywordSet) {
			return result.Set, nil
		}

		return []Node{*result}, nil
	}

	// 18)
	if result.Has(KeywordLanguage) && len(result.PropertySet()) == 1 {
		return nil, nil
	}

	// 19)
	if activeProperty == "" || activeProperty == KeywordGraph {
		props := result.PropertySet()
		if len(props) == 0 || result.Has(KeywordList) || result.Has(KeywordValue) {
			return nil, nil
		} else if len(props) == 1 && result.Has(KeywordID) {
			return nil, nil
		}
	}

	return []Node{*result}, nil
}

func (p *Processor) expandNestedElement(
	result *Node,
	nests Properties,
	activeContext *Context,
	typContext *Context,
	activeProperty string,
	inputType string,

	baseURL string,
	element json.Object,
	opts expandOptions,
) error {
	termDef, hasDef := activeContext.defs[activeProperty]
	// 3)
	var propContext json.RawMessage
	if hasDef {
		if termDef.Context != nil {
			propContext = termDef.Context
		}
	}

	// 8)
	if propContext != nil {
		ropts := newCtxProcessingOpts()
		ropts.override = true
		nctx, err := p.context(
			activeContext,
			propContext,
			termDef.BaseIRI,
			ropts,
		)
		if err != nil {
			return err
		}
		activeContext = nctx
	}

	return p.expandElement(result, nests, activeContext, typContext, activeProperty, inputType, baseURL, element, opts)
}

func (p *Processor) expandElement(
	result *Node,
	nests Properties,
	activeContext *Context,
	typContext *Context,
	activeProperty string,
	inputType string,

	baseURL string,
	element json.Object,
	opts expandOptions,
) error {
	// 13)
	objKeys := slices.Collect(maps.Keys(element))
	if opts.ordered {
		slices.Sort(objKeys)
	}

mainLoop:
	for _, key := range objKeys {
		// 13.1)
		if key == KeywordContext {
			continue
		}

		// 13.2)
		expProp, err := p.expandIRI(activeContext, key, false, true, nil, nil)
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

		value := element[key]

		// 13.4)
		if isKeyword(expProp) {
			// 13.4.1)
			if activeProperty == KeywordReverse {
				return ErrInvalidReversePropertyMap
			}

			// 13.4.2)
			if result.Has(expProp) {
				switch expProp {
				case KeywordIncluded, KeywordType:
					if p.modeLD10 {
						return ErrCollidingKeywords
					}
				default:
					return ErrCollidingKeywords
				}
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

				iri, err := p.expandIRI(activeContext, s, true, false, nil, nil)
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
				xopts := newExpandOptions()
				xopts.frameExpansion = opts.frameExpansion
				xopts.ordered = opts.ordered
				res, err := p.expand(activeContext, KeywordGraph, value, baseURL, xopts)
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

				xopts := newExpandOptions()
				xopts.frameExpansion = opts.frameExpansion
				xopts.ordered = opts.ordered

				// 13.4.6.2)
				res, err := p.expand(
					activeContext,
					"",
					value,
					baseURL,
					xopts,
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
				if activeProperty == "" || activeProperty == KeywordGraph {
					// 13.4.11.1)
					continue mainLoop
				}

				if json.IsArray(value) && bytes.Equal(value, []byte(`[]`)) {
					result.List = make([]Node, 0)
				} else {
					// 13.4.11.2)
					xopts := newExpandOptions()
					xopts.frameExpansion = opts.frameExpansion
					xopts.ordered = opts.ordered
					res, err := p.expand(
						activeContext,
						activeProperty,
						value,
						baseURL,
						xopts,
					)
					if err != nil {
						return err
					}
					result.List = res
				}
			case KeywordSet:
				// 13.4.12)
				xopts := newExpandOptions()
				xopts.frameExpansion = opts.frameExpansion
				xopts.ordered = opts.ordered
				res, err := p.expand(
					activeContext,
					activeProperty,
					value,
					baseURL,
					xopts,
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

				// 13.4.13.2)
				xopts := newExpandOptions()
				xopts.frameExpansion = opts.frameExpansion
				xopts.ordered = opts.ordered
				res, err := p.expand(
					activeContext,
					KeywordReverse,
					value,
					baseURL,
					xopts,
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
						// 13.4.13.4.2.1
						for _, item := range v {
							// 13.4.13.4.2.1.1)
							if item.IsValue() || item.IsList() {
								return ErrInvalidReversePropertyValue
							}
							if !result.Has(KeywordReverse) {
								result.Reverse = make(Properties, 8)
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
		termDef := activeContext.defs[key]
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
			dir := activeContext.defaultDirection
			// 13.7.3)
			if termDef.Direction != "" {
				dir = termDef.Direction
			}

			langKeys := slices.Collect(maps.Keys(langMap))
			if opts.ordered {
				slices.Sort(langKeys)
			}

			// 13.7.4)
			for _, langKey := range langKeys {
				langValue := langMap[langKey]
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
					if langKey != KeywordNone {
						if ldef := activeContext.defs[langKey]; ldef.IRI != KeywordNone {
							// 13.7.4.2.4)
							obj.Language = langKey
						}
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
			idxKeys := slices.Collect(maps.Keys(objVal))
			if opts.ordered {
				slices.Sort(idxKeys)
			}

			for _, idx := range idxKeys {
				idxVal := objVal[idx]

				var mapCtx *Context

				// 13.8.3.1)
				if slices.Contains(cnt, KeywordID) || slices.Contains(cnt, KeywordType) {
					if activeContext.previousContext != nil {
						mapCtx = activeContext.previousContext
					} else {
						mapCtx = activeContext
					}

					// 13.8.3.2)
					if slices.Contains(cnt, KeywordType) {
						if mapCtx == nil {
							mapCtx = newContext("")
						}

						if def, ok := mapCtx.defs[idx]; ok && def.Context != nil {
							nctx, err := p.context(
								mapCtx,
								def.Context,
								def.BaseIRI,
								newCtxProcessingOpts(),
							)
							if err != nil {
								return err
							}
							mapCtx = nctx
						}
					}
				} else {
					// 13.8.3.3)
					mapCtx = activeContext
				}

				// 13.8.3.4)
				expIdx, err := p.expandIRI(activeContext, idx, false, true, nil, nil)
				if err != nil {
					return err
				}

				// 13.8.3.5)
				idxVal = json.MakeArray(idxVal)

				// 13.8.3.6)
				xopts := newExpandOptions()
				xopts.fromMap = true
				xopts.frameExpansion = opts.frameExpansion
				xopts.ordered = opts.ordered
				expIdxVals, err := p.expand(
					mapCtx,
					key,
					idxVal,
					baseURL,
					xopts,
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

					if slices.Contains(cnt, KeywordIndex) && idxKey != KeywordIndex && expIdx != KeywordNone {
						// 13.8.3.7.2)

						// 13.8.3.7.2.1)
						rexpIdx, err := p.expandValue(
							activeContext,
							idxKey,
							[]byte(`"`+idx+`"`), // we know idx is a string so we cheat a little
						)
						if err != nil {
							return err
						}

						// 13.8.3.7.2.2)
						expIdxKey, err := p.expandIRI(activeContext, idxKey, false, true, nil, nil)
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
					} else if slices.Contains(cnt, KeywordIndex) && !item.Has(KeywordIndex) && expIdx != KeywordNone {
						// 13.8.3.7.3)
						item.Index = idx
					} else if slices.Contains(cnt, KeywordID) && !item.Has(KeywordID) && expIdx != KeywordNone {
						// 13.8.3.7.4)
						idx, err := p.expandIRI(activeContext,
							idx, true, false, nil, nil)
						if err != nil {
							return err
						}
						item.ID = idx
					} else if slices.Contains(cnt, KeywordType) && expIdx != KeywordNone {
						// 13.8.3.7.5)
						item.Type = append([]string{expIdx}, item.Type...)
					}
					// 13.8.3.7.6)
					expVal = append(expVal, item)
				}
			}
		} else {
			// 13.9)
			xopts := newExpandOptions()
			xopts.frameExpansion = opts.frameExpansion
			xopts.ordered = opts.ordered
			var expErr error
			expVal, expErr = p.expand(
				activeContext,
				key,
				value,
				baseURL,
				xopts,
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
			switch len(expVal) {
			case 1:
				if !expVal[0].IsList() {
					expVal = []Node{{List: expVal}}
				}
			default:
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
	nestKeys := slices.Collect(maps.Keys(nests))
	if opts.ordered {
		slices.Sort(nestKeys)
	}

	for _, k := range nestKeys {
		// 14.1)
		nestData := element[k]

		// 14.2)
		nestData = json.MakeArray(nestData)

		var nestValues []json.Object
		if err := json.Unmarshal(nestData, &nestValues); err != nil {
			return ErrInvalidNestValue
		}

		for _, nestValue := range nestValues {
			if p.expandsToKeyword(
				activeContext,
				KeywordValue,
				slices.Collect(maps.Keys(nestValue)),
			) {
				// 14.2.1)
				return ErrInvalidNestValue
			}
			// 14.2.2)
			if err := p.expandNestedElement(
				result,
				nests,
				activeContext,
				typContext,
				k,
				inputType,
				baseURL,
				nestValue,
				opts.clone(),
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
	elems []string,
) bool {
	for _, k := range elems {
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
	value json.RawMessage,
) (Node, error) {

	def := ctx.defs[property]
	result := Node{}

	switch def.Type {
	case KeywordID:
		// 1)
		if json.IsNull(value) {
			break
		}

		var val string
		if err := json.Unmarshal(value, &val); err != nil {
			break // don't coerce types of some other value
		}

		if val != "" {
			u, err := p.expandIRI(ctx, val, true, false, nil, nil)
			if err != nil {
				return result, err
			}
			result.ID = u
			return result, nil
		}
	case KeywordVocab:
		// 2)
		if json.IsNull(value) {
			break
		}

		var val string
		if err := json.Unmarshal(value, &val); err != nil {
			break // don't coerce types of some other value
		}

		if val != "" {
			u, err := p.expandIRI(ctx, val, true, true, nil, nil)
			if err != nil {
				return result, err
			}
			result.ID = u
			return result, nil
		}
	case KeywordNone, "":
		// 4)
	default:
		// 4)
		result.Type = []string{def.Type}
	}

	// 3)
	result.Value = value

	// 5)
	if json.IsString(value) {
		// 5.1)
		lang := ctx.defs[property].Language
		if lang == "" && ctx.defaultLang != "" {
			lang = ctx.defaultLang
		}

		// 5.2)
		dir := ctx.defs[property].Direction
		if dir == "" && ctx.defaultDirection != "" {
			dir = ctx.defaultDirection
		}

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
