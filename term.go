package longdistance

import (
	"bytes"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"code.dny.dev/longdistance/internal/json"
	"code.dny.dev/longdistance/internal/url"
)

// Term represents a term definition in a JSON-LD context.
type Term struct {
	IRI       Null[string]
	Prefix    bool
	Protected bool
	Reverse   bool

	BaseIRI   Null[string]
	Context   json.RawMessage
	Container []string
	Direction Null[string]
	Index     string
	Language  Null[string]
	Nest      Null[string]
	Type      string
}

func (t *Term) equalWithoutProtected(ot *Term) bool {
	if t == nil && ot == nil {
		return true
	}
	if t == nil || ot == nil {
		return false
	}
	if !t.IRI.Equal(&ot.IRI) {
		return false
	}
	if t.Prefix != ot.Prefix {
		return false
	}
	if t.Reverse != ot.Reverse {
		return false
	}
	if !t.BaseIRI.Equal(&ot.BaseIRI) {
		return false
	}
	if !bytes.Equal(t.Context, ot.Context) {
		return false
	}
	if !slices.Equal(t.Container, ot.Container) {
		return false
	}
	if !t.Direction.Equal(&ot.Direction) {
		return false
	}
	if t.Index != ot.Index {
		return false
	}
	if !t.Language.Equal(&ot.Language) {
		return false
	}
	if !t.Nest.Equal(&ot.Nest) {
		return false
	}
	if t.Type != ot.Type {
		return false
	}
	return true
}

func (t *Term) IsZero() bool {
	if t == nil {
		return true
	}
	return !t.IRI.Set && !t.Prefix && !t.Protected &&
		!t.Reverse && !t.BaseIRI.Set && t.Context == nil &&
		t.Container == nil && !t.Direction.Set &&
		t.Index == "" && !t.Language.Set && !t.Nest.Set &&
		t.Type == ""
}

type createTermOptions struct {
	baseURL   string
	protected bool
	override  bool
	remotes   []string
	validate  bool
}

func newCreateTermOptions() createTermOptions {
	return createTermOptions{
		validate: true,
	}
}

func (p *Processor) createTerm(
	activeContext *Context,
	localContext map[string]json.RawMessage,
	term string,
	defined map[string]*bool,
	opts createTermOptions,
) error {
	// 1)
	if v := defined[term]; v != nil {
		if *v {
			return nil
		}
		return ErrCyclicIRIMapping
	}

	// 2)
	if term == "" {
		return ErrInvalidTermDefinition
	} else {
		b := false
		defined[term] = &b
	}

	// 3)
	value := localContext[term]

	// 4)
	if term == KeywordType {
		if p.modeLD10 {
			return ErrKeywordRedefinition
		}

		var obj map[string]json.RawMessage
		if err := json.Unmarshal(value, &obj); err != nil {
			return ErrKeywordRedefinition
		}

		if len(obj) == 0 {
			return ErrKeywordRedefinition
		}

		objCopy := maps.Clone(obj)
		delete(objCopy, KeywordContainer)
		delete(objCopy, KeywordProtected)
		if len(objCopy) != 0 {
			return ErrKeywordRedefinition
		}

		if v, ok := obj[KeywordContainer]; ok {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return ErrKeywordRedefinition
			}
			if s != KeywordSet {
				return ErrKeywordRedefinition
			}
		}
		if v, ok := obj[KeywordProtected]; ok {
			var b bool
			if err := json.Unmarshal(v, &b); err != nil {
				return ErrKeywordRedefinition
			}
		}
	} else {
		// 5)
		if isKeyword(term) {
			return ErrKeywordRedefinition
		}

		if looksLikeKeyword(term) {
			p.logger.Warn("keyword lookalike term encountered", slog.String("term", term))
			return nil
		}
	}

	// 6)
	oldDef, oldDefOK := activeContext.defs[term]
	delete(activeContext.defs, term)
	if !oldDefOK {
		// check for aliasses
		for _, def := range activeContext.defs {
			if !def.IRI.Null && def.IRI.Value == term {
				oldDef = def
				oldDefOK = true
				delete(activeContext.defs, term)
				break
			}
		}
	}

	simpleTerm := false
	var valueObj map[string]json.RawMessage

	// 7) 8)
	if json.IsNull(value) || json.IsString(value) {
		// 8)
		if json.IsString(value) {
			simpleTerm = true
		}

		// 7)
		value = bytes.Join([][]byte{
			[]byte(`{"@id":`),
			value,
			[]byte(`}`),
		}, nil)
	}

	// 9)
	if err := json.Unmarshal(value, &valueObj); err != nil {
		return ErrInvalidTermDefinition
	}

	// 10)
	termDef := Term{
		Prefix:    false,
		Protected: opts.protected,
		Reverse:   false,
	}

	// 11)
	if prot, ok := valueObj[KeywordProtected]; ok {
		if p.modeLD10 {
			return ErrInvalidTermDefinition
		}

		var b bool
		if err := json.Unmarshal(prot, &b); err != nil {
			return ErrInvalidProtectedValue
		}
		termDef.Protected = b
	}

	// at this point protected is finalised, so add the
	// term to the protected set on activeContext
	if termDef.Protected {
		activeContext.protected[term] = struct{}{}
	}

	// 12)
	if typ, ok := valueObj[KeywordType]; ok {
		var s Null[string]
		// 12.1)
		if err := json.Unmarshal(typ, &s); err != nil {
			return ErrInvalidTypeMapping
		}

		if s.Null {
			return ErrInvalidTypeMapping
		}

		// 12.2)
		u, err := p.expandIRI(activeContext, s.Value, false, true, localContext, defined)
		if err != nil {
			return ErrInvalidTypeMapping
		}

		// 12.3
		if p.modeLD10 {
			if u == KeywordNone || u == KeywordJSON {
				return ErrInvalidTypeMapping
			}
		}

		// 12.4)
		switch u {
		case KeywordID, KeywordJSON, KeywordNone, KeywordVocab:
		default:
			if !url.IsIRI(u) {
				return ErrInvalidTypeMapping
			}
		}

		// 12.5)
		termDef.Type = u
	}

	// prep for branch 14)
	id, idOK := valueObj[KeywordID]
	var idStr Null[string]
	idErr := json.Unmarshal(id, &idStr)

	// 13)
	if rev, ok := valueObj[KeywordReverse]; ok {
		_, hasID := valueObj[KeywordID]
		_, hasNest := valueObj[KeywordNest]
		// 13.1)
		if hasID || hasNest {
			return ErrInvalidReverseProperty
		}

		// 13.2)
		var s Null[string]
		if err := json.Unmarshal(rev, &s); err != nil {
			return ErrInvalidIRIMapping
		}

		if s.Null {
			return ErrInvalidIRIMapping
		}

		// 13.3)
		if looksLikeKeyword(s.Value) {
			p.logger.Warn("keyword lookalike value encountered",
				slog.String("value", s.Value))
			return nil
		}

		// 13.4)
		u, err := p.expandIRI(activeContext, s.Value, false, true, localContext, defined)
		if err != nil {
			return ErrInvalidIRIMapping
		}

		if !url.IsIRI(u) && u != BlankNode {
			return ErrInvalidIRIMapping
		}

		termDef.IRI = Null[string]{Set: true, Value: u}

		// 13.5)
		if v, ok := valueObj[KeywordContainer]; ok {
			var c Null[string]
			if err := json.Unmarshal(v, &c); err != nil {
				return ErrInvalidReverseProperty
			}

			if !c.Null && c.Value != KeywordSet && c.Value != KeywordIndex {
				return ErrInvalidReverseProperty
			}

			if c.Null {
				termDef.Container = nil
			} else {
				termDef.Container = []string{c.Value}
			}
		}

		// 13.6)
		termDef.Reverse = true

		// This whole step is missing in the spec but without
		// it t0131 can't pass. So. YOLO.
		if slices.Contains(termDef.Container, KeywordIndex) {
			idxVal, idxOK := valueObj[KeywordIndex]
			if idxOK {
				var idx Null[string]
				if err := json.Unmarshal(idxVal, &idx); err != nil {
					return err
				}
				if idx.Set && !idx.Null {
					termDef.Index = idx.Value
				}
			}
		}

		// 13.7
		activeContext.defs[term] = termDef
		b := true
		defined[term] = &b
		return nil
	} else if idOK && term != idStr.Value {
		// 14.1) 14.2)
		if idErr != nil {
			return ErrInvalidIRIMapping
		}

		// 14.1)
		if !idStr.Null {
			// 14.2)
			if !isKeyword(idStr.Value) && looksLikeKeyword(idStr.Value) {
				// 14.2.2)
				p.logger.Warn("keyword lookalike value encountered",
					slog.String("value", idStr.Value))
				return nil
			}

			// 14.2.3)
			u, err := p.expandIRI(activeContext, idStr.Value, false, true, localContext, defined)
			if err != nil {
				return err
			}

			if !isKeyword(u) && !url.IsIRI(u) && u != BlankNode {
				return ErrInvalidIRIMapping
			}

			if u == KeywordContext {
				return ErrInvalidKeywordAlias
			}

			termDef.IRI = Null[string]{Set: true, Value: u}

			// 14.2.4)
			if strings.Contains(term, "/") || (!strings.HasPrefix(term, ":") && !strings.HasSuffix(term, ":") && strings.Contains(term, ":")) {
				b := true
				// 14.2.4.1)
				defined[term] = &b

				// 14.2.4.2)
				tu, err := p.expandIRI(activeContext, term, false, true, localContext, defined)
				if err != nil {
					return ErrInvalidIRIMapping
				}

				if tu != u {
					return ErrInvalidIRIMapping
				}
			} else {
				// 14.2.5)
				if simpleTerm && url.EndsInGenDelim(u) || u == BlankNode {
					if v, ok := p.remapPrefixIRIs[u]; ok {
						termDef.IRI = Null[string]{Set: true, Value: v}
					}
					termDef.Prefix = true
				}
			}
		}
	} else if strings.Contains(term[1:], ":") {
		// 15)
		prefix, suffix, _ := strings.Cut(term, ":")

		// 15.1)
		if !strings.HasPrefix(suffix, "//") {
			if _, ok := localContext[prefix]; ok {
				err := p.createTerm(activeContext, localContext, prefix, defined, newCreateTermOptions())
				if err != nil {
					return err
				}
			}
		}

		// 15.2)
		if def, ok := activeContext.defs[prefix]; ok {
			termDef.IRI = Null[string]{Set: true, Value: def.IRI.Value + suffix}
		} else {
			// 15.3)
			termDef.IRI = Null[string]{Set: true, Value: term}
		}
	} else if strings.Contains(term, "/") {
		// 16)
		// 16.2)
		u, err := p.expandIRI(activeContext, term, false, true, nil, nil)
		if err != nil {
			return ErrInvalidIRIMapping
		}
		if !url.IsIRI(u) {
			return ErrInvalidIRIMapping
		}
		termDef.IRI = Null[string]{Set: true, Value: u}
	} else if term == KeywordType {
		// 17)
		termDef.IRI = Null[string]{Set: true, Value: KeywordType}
	} else if activeContext.vocabMapping != "" {
		// 18)
		termDef.IRI = Null[string]{Set: true, Value: activeContext.vocabMapping + term}
	} else {
		return ErrInvalidIRIMapping
	}

	// 19)
	if cnt, ok := valueObj[KeywordContainer]; ok {
		if json.IsNull(cnt) {
			return ErrInvalidContainerMapping
		}

		// 19.2)
		// do this check early since we're going to rewrap
		// into an array
		if p.modeLD10 && !json.IsString(cnt) {
			return ErrInvalidContainerMapping
		}

		cnt = json.MakeArray(cnt)

		// 19.1)
		var values []string
		if err := json.Unmarshal(cnt, &values); err != nil {
			return ErrInvalidContainerMapping
		}

		for _, vl := range values {
			switch vl {
			case KeywordGraph, KeywordID, KeywordIndex,
				KeywordLanguage, KeywordList, KeywordSet,
				KeywordType:
			default:
				return ErrInvalidContainerMapping
			}
		}

		if slices.Contains(values, KeywordGraph) && (slices.Contains(values, KeywordID) || slices.Contains(values, KeywordIndex)) {
			kws := map[string]struct{}{}
			for _, vl := range values {
				kws[vl] = struct{}{}
			}
			delete(kws, KeywordGraph)
			delete(kws, KeywordIndex)
			delete(kws, KeywordID)
			if _, ok := kws[KeywordSet]; ok && len(kws) != 1 {
				return ErrInvalidIRIMapping
			}
		} else if slices.Contains(values, KeywordSet) {
			for _, vl := range values {
				switch vl {
				case KeywordGraph, KeywordID, KeywordIndex,
					KeywordLanguage, KeywordType,
					KeywordSet:
				default:
					return ErrInvalidContainerMapping
				}
			}
		}

		// 19.2)
		if p.modeLD10 {
			if len(values) > 1 {
				return ErrInvalidContainerMapping
			}
			switch values[0] {
			case KeywordID, KeywordGraph, KeywordType:
				return ErrInvalidContainerMapping
			}
		}

		// 19.3)
		termDef.Container = values

		// 19.4)
		if slices.Contains(values, KeywordType) {
			// 19.4.1)
			if termDef.Type == "" {
				termDef.Type = KeywordID
			}
			// 19.4.2)
			switch termDef.Type {
			case KeywordID, KeywordVocab, "":
			default:
				return ErrInvalidTypeMapping
			}
		}
	}

	// 20)
	if idx, ok := valueObj[KeywordIndex]; ok {
		// 20.1)
		if p.modeLD10 {
			return ErrInvalidTermDefinition
		}
		if !slices.Contains(termDef.Container, KeywordIndex) {
			return ErrInvalidTermDefinition
		}

		// 20.2)
		var s string
		if err := json.Unmarshal(idx, &s); err != nil {
			return ErrInvalidTermDefinition
		}
		u, err := p.expandIRI(activeContext, s, false, true, localContext, defined)
		if err != nil {
			return ErrInvalidTermDefinition
		}
		if !url.IsIRI(u) {
			return ErrInvalidTermDefinition
		}

		// 20.3)
		termDef.Index = s
	}

	// 21)
	if ctx, ok := valueObj[KeywordContext]; ok {
		// 21.1)
		if p.modeLD10 {
			return ErrInvalidTermDefinition
		}

		// 21.3)
		resolvOpts := newCtxProcessingOpts()
		resolvOpts.override = true
		resolvOpts.remotes = slices.Clone(opts.remotes)
		resolvOpts.validate = false
		_, err := p.context(
			activeContext,
			ctx,
			opts.baseURL,
			resolvOpts,
		)

		if err != nil {
			return ErrInvalidScopedContext
		}

		// 21.4
		termDef.Context = ctx
		termDef.BaseIRI = Null[string]{Set: true, Value: opts.baseURL}
	}

	_, hasType := valueObj[KeywordType]

	// 22)
	if lang, ok := valueObj[KeywordLanguage]; ok && !hasType {
		var lm Null[string]
		// 22.1)
		if err := json.Unmarshal(lang, &lm); err != nil {
			return ErrInvalidLanguageMapping
		}

		if !lm.Null {
			lm.Value = strings.ToLower(lm.Value)
		}

		// 22.2)
		termDef.Language = lm
	}

	// 23)
	if dir, ok := valueObj[KeywordDirection]; ok && !hasType {
		var d Null[string]
		// 23.1)
		if err := json.Unmarshal(dir, &d); err != nil {
			return ErrInvalidBaseDirection
		}

		if !d.Null {
			switch d.Value {
			case DirectionLTR, DirectionRTL:
			default:
				return ErrInvalidBaseDirection
			}
		}

		// 23.2)
		termDef.Direction = d
	}

	// 24)
	if nest, ok := valueObj[KeywordNest]; ok {
		// 24.1)
		if p.modeLD10 {
			return ErrInvalidTermDefinition
		}

		// 24.2)
		var n Null[string]
		if err := json.Unmarshal(nest, &n); err != nil {
			return ErrInvalidNestValue
		}
		if n.Null || (isKeyword(n.Value) && n.Value != KeywordNest) {
			return ErrInvalidNestValue
		}
		termDef.Nest = n
	}

	// 25)
	if prefix, ok := valueObj[KeywordPrefix]; ok {
		// 25.1)
		if p.modeLD10 {
			return ErrInvalidTermDefinition
		}
		if strings.Contains(term, ":") || strings.Contains(term, "/") {
			return ErrInvalidTermDefinition
		}

		// 25.2)
		var p Null[bool]
		if err := json.Unmarshal(prefix, &p); err != nil {
			return ErrInvalidPrefixValue
		}

		if p.Null {
			return ErrInvalidPrefixValue
		}

		termDef.Prefix = p.Value

		// 25.3)
		if termDef.Prefix && isKeyword(termDef.IRI.Value) {
			return ErrInvalidTermDefinition
		}
	}

	// 26)
	valKeys := map[string]struct{}{}
	for k := range valueObj {
		valKeys[k] = struct{}{}
	}

	for _, kw := range []string{KeywordID, KeywordReverse, KeywordContainer,
		KeywordContext, KeywordDirection, KeywordIndex, KeywordLanguage,
		KeywordNest, KeywordPrefix, KeywordProtected, KeywordType} {
		delete(valKeys, kw)
	}

	if len(valKeys) > 0 {
		return ErrInvalidTermDefinition
	}

	// 27)
	if oldDefOK && oldDef.Protected && !opts.override {
		// 27.1)
		if !oldDef.equalWithoutProtected(&termDef) {
			return ErrProtectedTermRedefinition
		}
		// 27.2)
		termDef = oldDef
	}

	// 28)
	activeContext.defs[term] = termDef
	b := true
	defined[term] = &b
	return nil
}

func selectTerm(
	activeContext *Context,
	keyIriVar string,
	containers []string,
	typeLanguage string,
	preferredValues []string,
) string {
	// 1)
	if activeContext.inverse == nil {
		activeContext.inverse = workIt(activeContext)
	}

	// 2)
	inverse := activeContext.inverse

	// 3)
	containerMap := inverse[keyIriVar]

	for _, container := range containers {
		// 4.1)
		// 4.2)
		typeLanguageMap, ok := containerMap[container]
		if !ok {
			continue
		}

		// 4.3)
		var valMap map[string]string
		switch typeLanguage {
		case KeywordLanguage:
			valMap = typeLanguageMap.Language
		case KeywordType:
			valMap = typeLanguageMap.Type
		case KeywordAny:
			valMap = typeLanguageMap.Any
		}

		// 4.4)
		for _, pval := range preferredValues {
			if v, ok := valMap[pval]; !ok {
				// 4.4.1)
				continue
			} else {
				// 4.4.2)
				return v
			}
		}
	}

	// 5)
	return ""
}
