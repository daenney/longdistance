package longdistance

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"sourcery.dny.nu/longdistance/internal/json"
	"sourcery.dny.nu/longdistance/internal/url"
)

func (p *Processor) compactIRI(
	activeContext *Context,
	key string,
	value any,
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

	object, isObject := value.(Node)

	// 4)
	if _, ok := inverse[key]; ok && vocab {
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
			if !isObject || (isObject && !object.Has(KeywordIndex)) {
				containers = append(containers,
					KeywordIndex,
					KeywordIndex+KeywordSet)
			}
			// 4.12)
			if isObject && object.IsValue() && len(object.PropertySet()) == 1 {
				containers = append(containers,
					KeywordLanguage,
					KeywordLanguage+KeywordSet)
			}
		}

		// 4.13)
		if typeLanguageValue == "" {
			typeLanguageValue = KeywordNull
		}

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
				object.ID,
				nil, true, false,
			)
			if err != nil {
				return "", err
			}

			cdef, cok := activeContext.defs[c]
			if cok && cdef.IRI == object.ID {
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
	for term, def := range activeContext.defs {
		if def.IRI == "" || def.IRI == key || !strings.HasPrefix(
			key, def.IRI) || !def.Prefix {
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
	for term, def := range activeContext.defs {
		if u.Scheme == term && def.Prefix && u.Host == "" {
			return "", ErrIRIConfusedWithPrefix
		}
	}

	// 10)
	if !vocab && activeContext.currentBaseIRI != "" {
		res, err := url.Relative(activeContext.currentBaseIRI, key)
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

func (p *Processor) compactValue(
	ctx *Context,
	prop string,
	value *Node,
) (any, error) {
	// 1) 2) and 3) aren't needed

	// 4)
	language := ctx.defs[prop].Language
	if language == "" && ctx.defaultLang != "" {
		language = ctx.defaultLang
	}

	// 5)
	direction := ctx.defs[prop].Direction
	if direction == "" && ctx.defaultDirection != "" {
		direction = ctx.defaultDirection
	}

	allProps := value.PropertySet()
	allPropsLen := len(allProps)
	def, defOK := ctx.defs[prop]

	if (value.Has(KeywordID) && allPropsLen == 1) || (value.Has(KeywordID) && value.Has(KeywordIndex) && allPropsLen == 2) {
		// 6)
		if defOK && def.Type != "" {
			var res string
			var err error

			switch def.Type {
			case KeywordID:
				res, err = p.compactIRI(ctx,
					value.ID,
					nil,
					false, false)
			case KeywordVocab:
				res, err = p.compactIRI(ctx,
					value.ID,
					nil,
					true, false)
			default:
				return nil, nil
			}
			return res, err
		} else {
			return nil, nil
		}
	} else if defOK && value.Has(KeywordType) && slices.Contains(value.Type, def.Type) {
		// 7)
		return value.Value, nil
	} else if (defOK && def.Type == KeywordNone) || value.Has(KeywordType) && !slices.Contains(value.Type, def.Type) {
		// 8) don't need to do anything here
		return nil, nil
	} else if value.IsValue() && !json.IsString(value.Value) {
		// 9)
		if (value.Has(KeywordIndex) && slices.Contains(def.Container, KeywordIndex)) || !value.Has(KeywordIndex) {
			// 9.1)
			return value.Value, nil
		}
	} else if value.IsValue() && (((value.Has(KeywordLanguage) && language != "" && language != KeywordNull && strings.EqualFold(value.Language, language)) || (!value.Has(KeywordLanguage) && language != "" && language == KeywordNull) || (!value.Has(KeywordLanguage) && language == "")) && ((value.Has(KeywordDirection) && direction != "" && direction != KeywordNull && strings.EqualFold(value.Direction, direction)) || (!value.Has(KeywordDirection) && direction != "" && direction == KeywordNull) || (!value.Has(KeywordDirection) && direction == ""))) {
		// 10)
		if !value.Has(KeywordIndex) || (value.Has(KeywordIndex) && defOK && slices.Contains(def.Container, KeywordIndex)) {
			// 10.1)
			return value.Value, nil
		}

	}

	// 11) doesn't seem necessary
	return nil, nil
}

func (p *Processor) Compact(
	compactionCtx json.RawMessage,
	document []Node,
	documentURL string,
) (json.RawMessage, error) {
	ctx, err := p.Context(compactionCtx, documentURL)
	if err != nil {
		return nil, err
	}

	if len(document) == 0 {
		return json.RawMessage(`{}`), nil
	}

	if ctx == nil {
		return json.Marshal(document)
	}

	res, err := p.compact(
		ctx,
		"",
		document,
		p.compactArrays,
		p.ordered,
	)

	if err != nil {
		return nil, err
	}

	if res == nil {
		return json.RawMessage(`{}`), nil
	}

	if v, isObject := res.(map[string]any); isObject && p.compactArrays {
		if len(compactionCtx) > 2 {
			v[KeywordContext] = compactionCtx
		}
		return json.Marshal(v)
	}

	alias, err := p.compactIRI(ctx, KeywordGraph, nil, true, false)
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		alias: res,
	}

	if len(compactionCtx) > 2 {
		result[KeywordContext] = compactionCtx
	}

	return json.Marshal(result)
}

func (p *Processor) compact(
	activeContext *Context,
	activeProperty string,
	element any,
	compactArrays bool,
	ordered bool,
) (any, error) {
	var activeTermDefinition Term
	if activeProperty != "" {
		activeTermDefinition = activeContext.defs[activeProperty]
	}

	// 1)
	typeScopedContext := activeContext

	// 2)
	elemArray, isArray := element.([]Node)
	object, isObject := element.(Node)
	if !isArray && !isObject {
		return element, nil
	}

	// 3)
	if isArray {
		// 3.1)
		result := make([]any, 0, len(elemArray))

		// 3.2)
		for _, elem := range elemArray {
			// 3.2.1)
			compactedItem, err := p.compact(activeContext, activeProperty, elem, compactArrays, ordered)
			if err != nil {
				return nil, err
			}
			// 3.2.2)
			if compactedItem != nil {
				result = append(result, compactedItem)
			}
		}

		// 3.3)
		if len(result) != 1 || !compactArrays || activeProperty == KeywordGraph || activeProperty == KeywordSet {
			return result, nil
		}

		asList := slices.Contains(activeTermDefinition.Container, KeywordList)
		asSet := slices.Contains(activeTermDefinition.Container, KeywordSet)
		if asList || asSet {
			return result, nil
		}

		// 3.4)
		return result[0], nil
	}

	// 4)
	if !isObject {
		return fmt.Errorf("what the fuck"), nil
	}

	// 5)
	if activeContext.previousContext != nil {
		if !object.Has(KeywordValue) && (!object.Has(KeywordID) || len(object.PropertySet()) > 1) {
			activeContext = activeContext.previousContext
		}
	}

	// 6)
	if activeTermDefinition.Context != nil {
		opts := newCtxProcessingOpts()
		opts.override = true
		nctx, err := p.context(activeContext, activeTermDefinition.Context, activeTermDefinition.BaseIRI, opts)
		if err != nil {
			return nil, err
		}
		activeContext = nctx
		activeTermDefinition = activeContext.defs[activeProperty]
	}

	// 7)
	if object.Has(KeywordValue) || object.Has(KeywordID) {
		if activeTermDefinition.Type == KeywordJSON {
			return object.Value, nil
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
		return p.compact(
			activeContext,
			activeProperty,
			object.List,
			compactArrays,
			ordered,
		)
	}

	// 9)
	insideReverse := activeProperty == KeywordReverse

	// 10)
	result := map[string]any{}

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
					activeContext,
					cdef.Context,
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
	expandedProperties := slices.Collect(maps.Keys(object.PropertySet()))
	if ordered {
		slices.Sort(expandedProperties)
	}

	for _, expandedProperty := range expandedProperties {
		// 12.1)
		if expandedProperty == KeywordID {
			// 12.1.1)
			cv, err := p.compactIRI(activeContext, object.ID, nil, false, false)
			if err != nil {
				return nil, err
			}
			// 12.1.2)
			alias, err := p.compactIRI(activeContext, KeywordID, nil, true, false)
			if err != nil {
				return nil, err
			}
			// 12.1.3)
			result[alias] = cv
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
			if asArray || len(vt) > 1 {
				result[alias] = vt
			} else {
				result[alias] = vt[0]
			}

			// 12.2.6)
			continue
		}

		// 12.3)
		if expandedProperty == KeywordReverse {
			// 12.3.1)
			res := make([]any, 0, len(object.Reverse))
			for k, elem := range object.Reverse {
				compactedValue, err := p.compact(
					activeContext,
					KeywordReverse,
					Node{Properties: Properties{
						k: elem,
					}},
					compactArrays,
					ordered,
				)
				if err != nil {
					return nil, err
				}

				obj, objOK := compactedValue.(map[string]any)

				// 12.3.2)
				if objOK {
					for prop, val := range obj {
						if rdef, rok := activeContext.defs[prop]; rok && rdef.Reverse {
							asArray := !compactArrays
							if slices.Contains(rdef.Container, KeywordSet) {
								asArray = true
							}

							valArray, valIsArray := val.([]any)
							if asArray {
								if valIsArray {
									result[prop] = valArray
								} else {
									result[prop] = []any{val}
								}
							} else {
								result[prop] = val
							}
							delete(obj, prop)
						}
					}

					if len(obj) != 0 {
						res = append(res, obj)
					}
				}
			}

			if len(res) == 0 {
				continue
			}

			final := res[0].(map[string]any)
			for _, elem := range res[1:] {
				maps.Copy(final, elem.(map[string]any))
			}

			// 12.3.3)
			alias, err := p.compactIRI(activeContext, KeywordReverse, nil, true, false)
			if err != nil {
				return nil, err
			}
			result[alias] = final

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
			var value any
			switch expandedProperty {
			case KeywordDirection:
				value = object.Direction
			case KeywordIndex:
				value = object.Index
			case KeywordLanguage:
				value = object.Language
			case KeywordValue:
				value = object.Value
			}
			result[alias] = value
			continue
		}

		var expandedValue []Node
		if expandedProperty == KeywordList {
			expandedValue = object.List
		} else if expandedProperty == KeywordGraph {
			expandedValue = object.Graph
		} else if expandedProperty == KeywordIncluded {
			expandedValue = object.Included
		} else {
			expandedValue = object.Properties[expandedProperty]
		}

		// 12.7
		if len(expandedValue) == 0 {
			itemActiveProperty, err := p.compactIRI(
				activeContext,
				expandedProperty,
				expandedValue,
				true, insideReverse,
			)
			if err != nil {
				return nil, err
			}

			var nestResult map[string]any

			if edef, eok := activeContext.defs[itemActiveProperty]; eok && edef.Nest != "" {
				// 12.7.2)
				term, err := p.expandIRI(activeContext, edef.Nest, false, true, nil, nil)
				if err != nil {
					return nil, err
				}
				// 12.7.2.1)
				if term != KeywordNest {
					return nil, ErrInvalidNestValue
				}

				term = edef.Nest

				// 12.7.2.2)
				if _, ok := result[term]; !ok {
					result[term] = map[string]any{}
				}

				// 12.7.2.3)
				nestResult = result[term].(map[string]any)
			} else {
				// 12.7.3)
				nestResult = result
			}

			// 12.7.4)
			nestResult[itemActiveProperty] = []any{}
		}

		// 12.8)

		for _, expandedItem := range expandedValue {
			// 12.8.1)
			itemActiveProperty, err := p.compactIRI(
				activeContext,
				expandedProperty,
				expandedItem,
				true, insideReverse,
			)
			if err != nil {
				return nil, err
			}

			// 12.8.2)
			var nestResult map[string]any

			if edef, eok := activeContext.defs[itemActiveProperty]; eok && edef.Nest != "" {
				// 12.8.2.1)
				term, err := p.expandIRI(activeContext, edef.Nest, false, true, nil, nil)
				if err != nil {
					return nil, err
				}

				if term != KeywordNest {
					return nil, ErrInvalidNestValue
				}

				term = edef.Nest

				// 12.8.2.2)
				if _, ok := result[term]; !ok {
					result[term] = map[string]any{}
				}

				// 12.8.2.3)
				nestResult = result[term].(map[string]any)
			} else {
				// 12.8.3)
				nestResult = result
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
			var itemToCompact any
			if expandedItem.IsList() {
				itemToCompact = expandedItem.List
			} else if expandedItem.IsGraph() {
				itemToCompact = expandedItem.Graph
			} else {
				itemToCompact = expandedItem
			}

			compactedItem, err := p.compact(
				activeContext,
				itemActiveProperty,
				itemToCompact,
				compactArrays,
				ordered,
			)
			if err != nil {
				return nil, err
			}

			// 12.8.7)
			if expandedItem.IsList() {
				_, isArray := compactedItem.([]any)
				// 12.8.7.1)
				if !isArray {
					compactedItem = []any{compactedItem}
				}

				// 12.8.7.2)
				if !slices.Contains(container, KeywordList) {
					// 12.8.7.2.1)
					compactedMap := map[string]any{}
					alias, err := p.compactIRI(
						activeContext,
						KeywordList,
						nil, true, false,
					)
					if err != nil {
						return nil, err
					}
					compactedMap[alias] = compactedItem

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
						compactedMap[iAlias] = expandedItem.Index
					}
					// 12.8.7.2.3)
					if v, ok := nestResult[itemActiveProperty]; ok {
						vlist, vok := v.([]any)
						if !vok {
							vlist = []any(vlist)
						}
						vlist = append(vlist, compactedMap)
						nestResult[itemActiveProperty] = vlist
					} else {
						if asArray {
							nestResult[itemActiveProperty] = []any{compactedMap}
						} else {
							nestResult[itemActiveProperty] = compactedMap
						}
					}
				} else {
					// 12.8.7.3)
					nestResult[itemActiveProperty] = compactedItem
				}
			} else if expandedItem.IsGraph() {
				// 12.8.8)
				if slices.Contains(container, KeywordGraph) &&
					slices.Contains(container, KeywordID) {
					// 12.8.8.1)
					mapObject, ok := nestResult[itemActiveProperty].(map[string]any)
					if !ok {
						// 12.8.8.1.1)
						mapObject = map[string]any{}
					}

					// 12.8.8.1.2)
					vocab := true
					key := cmp.Or(expandedItem.ID, KeywordNone)
					if expandedItem.Has(KeywordID) {
						vocab = false
					}
					alias, err := p.compactIRI(activeContext, key, nil, vocab, false)
					if err != nil {
						return nil, err
					}

					// 12.8.8.1.3)
					if v, ok := mapObject[alias]; ok {
						vlist, vok := v.([]any)
						if !vok {
							vlist = []any{v}
						}
						vlist = append(vlist, compactedItem)
						mapObject[alias] = vlist
					} else {
						if asArray {
							if _, isArray := compactedItem.([]any); isArray {
								mapObject[alias] = compactedItem
							} else {
								mapObject[alias] = []any{compactedItem}
							}
						} else {
							mapObject[alias] = compactedItem
						}
					}
					nestResult[itemActiveProperty] = mapObject
				} else if slices.Contains(container, KeywordGraph) &&
					slices.Contains(container, KeywordIndex) && expandedItem.IsSimpleGraph() {
					// 12.8.8.2)

					mapObject, ok := nestResult[itemActiveProperty].(map[string]any)
					if !ok {
						// 12.8.8.2.1)
						mapObject = map[string]any{}
					}

					// 12.8.8.2.2)
					key := cmp.Or(expandedItem.Index, KeywordNone)

					// 12.8.8.2.3)
					if v, ok := mapObject[key]; ok {
						vlist, vok := v.([]any)
						if !vok {
							vlist = []any{v}
						}
						vlist = append(vlist, compactedItem)
						mapObject[key] = vlist
					} else {
						if asArray {
							if _, isArray := compactedItem.([]any); isArray {
								mapObject[key] = compactedItem
							} else {
								mapObject[key] = []any{compactedItem}
							}
						} else {
							mapObject[key] = compactedItem
						}
					}
					nestResult[itemActiveProperty] = mapObject
				} else if slices.Contains(container, KeywordGraph) && expandedItem.IsSimpleGraph() {
					// 12.8.8.3)
					clist, cok := compactedItem.([]any)

					// 12.8.8.3.1)
					if cok && len(clist) > 1 {
						alias, err := p.compactIRI(activeContext, KeywordIncluded, nil, true, false)
						if err != nil {
							return nil, err
						}
						compactedItem = map[string]any{
							alias: compactedItem,
						}
					}

					// 12.8.8.3.2)
					if v, ok := nestResult[itemActiveProperty]; ok {
						vlist, vok := v.([]any)
						if !vok {
							vlist = []any{v}
						}
						if cok {
							vlist = append(vlist, clist...)
						} else {
							vlist = append(vlist, compactedItem)
						}
						nestResult[itemActiveProperty] = vlist
					} else {
						_, ncok := compactedItem.([]any)
						if asArray && !ncok {
							nestResult[itemActiveProperty] = []any{compactedItem}
						} else {
							nestResult[itemActiveProperty] = compactedItem
						}
					}
				} else {
					// 12.8.8.4)
					alias, err := p.compactIRI(activeContext, KeywordGraph, nil, true, false)
					if err != nil {
						return nil, err
					}

					// 12.8.8.4.1)
					compactedItem := map[string]any{
						alias: compactedItem,
					}

					// 12.8.8.4.2)
					if expandedItem.Has(KeywordID) {
						alias, err := p.compactIRI(activeContext, KeywordID, nil, true, false)
						if err != nil {
							return nil, err
						}
						val, err := p.compactIRI(activeContext, expandedItem.ID, nil, false, false)
						if err != nil {
							return nil, err
						}
						compactedItem[alias] = val
					}

					// 12.8.8.4.3)
					if expandedItem.Has(KeywordIndex) {
						alias, err := p.compactIRI(activeContext, KeywordIndex, nil, true, false)
						if err != nil {
							return nil, err
						}
						compactedItem[alias] = expandedItem.Index
					}

					// 12.8.8.4.4)
					if v, ok := nestResult[itemActiveProperty]; ok {
						vlist, vok := v.([]any)
						if !vok {
							vlist = []any{v}
						}
						vlist = append(vlist, compactedItem)
						nestResult[itemActiveProperty] = vlist
					} else {
						if asArray {
							nestResult[itemActiveProperty] = []any{compactedItem}
						} else {
							nestResult[itemActiveProperty] = compactedItem
						}
					}
				}
			} else if !slices.Contains(container, KeywordGraph) && (slices.Contains(container, KeywordLanguage) ||
				slices.Contains(container, KeywordIndex) ||
				slices.Contains(container, KeywordID) ||
				slices.Contains(container, KeywordType)) {
				// 12.8.9)
				mapObject, ok := nestResult[itemActiveProperty].(map[string]any)
				if !ok {
					// 12.8.9.1)
					mapObject = map[string]any{}
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
					compactedItem = expandedItem.Value
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
					expIdx, err := p.expandIRI(activeContext, indexKey, false, false, nil, nil)
					if err != nil {
						return nil, err
					}
					containerKey, err = p.compactIRI(activeContext, expIdx, nil, true, false)
					if err != nil {
						return nil, err
					}

					// 12.8.9.6.2)
					if compactedObject, isObject := compactedItem.(map[string]any); isObject {
						if value, vok := compactedObject[containerKey]; vok {
							if nv, nok := value.(json.RawMessage); nok {
								var m string
								if err := json.Unmarshal(nv, &m); err != nil {
									return nil, err
								}
								mapKey = m
								delete(compactedObject, containerKey)
							}
							if nv, nok := value.(string); nok {
								mapKey = nv
								delete(compactedObject, containerKey)
							}
							if nv, nok := value.([]any); nok {
								if v, vok := nv[0].(json.RawMessage); vok {
									var m string
									if err := json.Unmarshal(v, &m); err != nil {
										return nil, err
									}
									mapKey = m
								}

								if v, vok := nv[0].(string); vok {
									mapKey = v
								}

								lnv := len(nv)
								if lnv == 2 {
									compactedObject[containerKey] = nv[1]
								} else if lnv > 2 {
									compactedObject[containerKey] = nv[1:]
								} else {
									delete(compactedObject, containerKey)
								}
							}
						}
						compactedItem = compactedObject
					}
				} else if slices.Contains(container, KeywordID) {
					// 12.8.9.7)
					if compactedObject, ok := compactedItem.(map[string]any); ok {
						if value, vok := compactedObject[containerKey]; vok {
							mapKey = value.(string)
							delete(compactedObject, containerKey)
						}
						compactedItem = compactedObject
					}
				} else if slices.Contains(container, KeywordType) {
					// 12.8.9.8)

					if compactedObject, isObject := compactedItem.(map[string]any); isObject {
						// 12.8.9.8.1)
						if value, vok := compactedObject[containerKey]; vok {
							if vlist, lok := value.([]string); lok {
								mapKey = vlist[0]
								if len(vlist) == 2 {
									compactedObject[containerKey] = vlist[1]
								} else if len(vlist) > 2 {
									compactedObject[containerKey] = vlist[1:]
								}
							}
							if s, sok := value.(string); sok {
								mapKey = s
								// 12.8.9.8.2)
								delete(compactedObject, containerKey)
							}
						}

						// 12.8.9.8.4)
						if len(compactedObject) == 1 {
							for k := range compactedObject {
								expIri, err := p.expandIRI(activeContext, k, false, true, nil, nil)
								if err != nil {
									return nil, err
								}

								if expIri == KeywordID {
									res, err := p.compact(
										activeContext,
										itemActiveProperty,
										Node{ID: expandedItem.ID},
										false, false,
									)
									if err != nil {
										return nil, err
									}
									compactedItem = res
								}
							}
						} else {
							compactedItem = compactedObject
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

				if v, ok := mapObject[mapKey]; ok {
					vlist, ok := v.([]any)
					if !ok {
						vlist = []any{v}
					}
					vlist = append(vlist, compactedItem)
					mapObject[mapKey] = vlist
				} else {
					if asArray {
						mapObject[mapKey] = []any{compactedItem}
					} else {
						mapObject[mapKey] = compactedItem
					}
				}

				nestResult[itemActiveProperty] = mapObject
			} else {
				// 12.8.10)
				if v, ok := nestResult[itemActiveProperty]; ok {
					vlist, ok := v.([]any)
					if !ok {
						vlist = []any{v}
					}
					vlist = append(vlist, compactedItem)
					nestResult[itemActiveProperty] = vlist
				} else {
					if asArray {
						if itemDef.Type == KeywordJSON && json.IsArray(expandedItem.Value) {
							nestResult[itemActiveProperty] = compactedItem
						} else {
							nestResult[itemActiveProperty] = []any{compactedItem}
						}
					} else {
						nestResult[itemActiveProperty] = compactedItem
					}
				}
			}
		}
	}
	return result, nil
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
