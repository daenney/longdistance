package longdistance

import (
	"bytes"
	"cmp"
	"log/slog"
	"slices"
	"strings"
	"unique"

	"sourcery.dny.nu/longdistance/internal/iri"
	"sourcery.dny.nu/longdistance/internal/json"
)

// termState tracks the definition state of a term during context processing.
type termState uint8

const (
	termUndefined termState = iota // Term not yet processed
	termDefining                   // Term is being defined (for cycle detection)
	termDefined                    // Term definition is complete
)

// Term represents a term definition in a JSON-LD context.
type Term struct {
	IRI       string
	Prefix    bool
	Protected bool
	Reverse   bool

	BaseIRI   string
	Context   json.RawMessage
	Container []string
	Direction string
	Index     string
	Language  string
	Nest      string
	Type      string
}

func (t *Term) equalWithoutProtected(ot *Term) bool {
	if t == nil && ot == nil {
		return true
	}
	if t == nil || ot == nil {
		return false
	}
	if t.IRI != ot.IRI {
		return false
	}
	if t.Prefix != ot.Prefix {
		return false
	}
	if t.Reverse != ot.Reverse {
		return false
	}
	if t.BaseIRI != ot.BaseIRI {
		return false
	}
	if !bytes.Equal(t.Context, ot.Context) {
		return false
	}
	if !slices.Equal(t.Container, ot.Container) {
		return false
	}
	if t.Direction != ot.Direction {
		return false
	}
	if t.Index != ot.Index {
		return false
	}
	if t.Language != ot.Language {
		return false
	}
	if t.Nest != ot.Nest {
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
	return t.IRI == "" && !t.Prefix && !t.Protected &&
		!t.Reverse && t.BaseIRI == "" && t.Context == nil &&
		t.Container == nil && t.Direction == "" &&
		t.Index == "" && t.Language == "" && t.Nest == "" &&
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

type array[T any] []T

func (a *array[T]) UnmarshalJSON(data []byte) error {
	if json.IsNull(data) {
		return nil
	}

	if json.IsEmptyArray(data) {
		return nil
	}

	data = json.MakeArray(data)

	var zero []T
	if err := json.Unmarshal(data, &zero); err != nil {
		return err
	}

	*a = zero
	return nil
}

type term struct {
	Null           bool
	Simple         bool
	ID             null[string]
	Type           string
	Reverse        string
	Container      null[array[string]]
	Index          string
	Context        json.RawMessage
	Language       null[string]
	Direction      null[string]
	Nest           string
	Prefix         null[bool]
	Protected      null[bool]
	HasUnknownKeys bool
}

func (p *Processor) createTerm(
	activeCtx *Context,
	localCtx map[string]term,
	term string,
	defined map[string]termState,
	opts createTermOptions,
) error {
	// 1)
	if state := defined[term]; state != termUndefined {
		if state == termDefined {
			return nil
		}
		return ErrCyclicIRIMapping
	}

	// 2)
	if term == "" {
		return ErrInvalidTermDefinition
	} else {
		defined[term] = termDefining
	}

	// 3)
	input := localCtx[term]

	// 4)
	if term == KeywordType {
		if p.modeLD10 {
			return ErrKeywordRedefinition
		}

		// Check if @type is protected before validating the new definition
		// If protected and not overriding, return protected term redefinition first
		if oldDef, oldOK := activeCtx.defs[term]; oldOK && oldDef.Protected && !opts.override {
			return ErrProtectedTermRedefinition
		}

		// For @type, only @container and @protected are allowed
		if input.ID.Set || input.Type != "" || input.Reverse != "" ||
			input.Index != "" || input.Context != nil || input.Language.Set ||
			input.Direction.Set || input.Nest != "" || (input.Prefix.Set && input.Prefix.Valid) ||
			input.HasUnknownKeys {
			return ErrKeywordRedefinition
		}

		// @container must be @set if provided, and is required for non-simple definitions
		if input.Container.Set && input.Container.Valid {
			if len(input.Container.Value) != 1 {
				return ErrKeywordRedefinition
			}
			if input.Container.Value[0] != KeywordSet {
				return ErrKeywordRedefinition
			}
		} else if !input.Simple && !input.Null {
			return ErrKeywordRedefinition
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
	oldDef, oldDefOK := activeCtx.defs[term]
	delete(activeCtx.defs, term)
	if !oldDefOK {
		// check for aliasses
		for _, def := range activeCtx.defs {
			if def.IRI != "" && def.IRI == term {
				oldDef = def
				oldDefOK = true
				delete(activeCtx.defs, term)
				break
			}
		}
	}

	// 10)
	termDef := Term{
		Protected: opts.protected,
	}

	// 11)
	if input.Protected.Set && input.Protected.Valid {
		if p.modeLD10 {
			return ErrInvalidTermDefinition
		}
		termDef.Protected = input.Protected.Value
	}

	// at this point protected is finalised, so add the
	// term to the protected set on activeContext
	if termDef.Protected {
		activeCtx.protected[term] = struct{}{}
	}

	// 12)
	if input.Type != "" {
		// 12.2)
		u, err := p.expandIRI(activeCtx, input.Type, false, true, localCtx, defined)
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
			if !iri.IsAbsolute(u) {
				return ErrInvalidTypeMapping
			}
		}

		// 12.5)
		termDef.Type = u
	}

	// 13)
	if input.Reverse != "" {
		// 13.1)
		if input.ID.Set || input.Nest != "" {
			return ErrInvalidReverseProperty
		}

		// 13.3)
		if looksLikeKeyword(input.Reverse) {
			p.logger.Warn("keyword lookalike value encountered",
				slog.String("value", input.Reverse))
			return nil
		}

		// 13.4)
		u, err := p.expandIRI(activeCtx, input.Reverse, false, true, localCtx, defined)
		if err != nil {
			return ErrInvalidIRIMapping
		}

		if !iri.IsAbsolute(u) && u != BlankNode {
			return ErrInvalidIRIMapping
		}

		termDef.IRI = u

		// 13.5)
		if input.Container.Set {
			if input.Container.Valid {
				if input.Container.Value[0] != KeywordSet &&
					input.Container.Value[0] != KeywordIndex {
					return ErrInvalidReverseProperty
				}
				termDef.Container = input.Container.Value
			} else {
				termDef.Container = nil
			}
		}

		// 13.6)
		termDef.Reverse = true

		// This whole step is missing in the spec but without
		// it t0131 can't pass. So. YOLO.
		if slices.Contains(termDef.Container, KeywordIndex) && input.Index != "" {
			termDef.Index = input.Index
		}

		// 13.7
		activeCtx.defs[term] = termDef
		if termDef.Prefix {
			activeCtx.prefixes[term] = struct{}{}
		}
		defined[term] = termDefined
		return nil
	} else if input.ID.Set && input.ID.Valid && term != input.ID.Value {
		// 14.2)
		if !isKeyword(input.ID.Value) && looksLikeKeyword(input.ID.Value) {
			// 14.2.2)
			p.logger.Warn("keyword lookalike value encountered",
				slog.String("value", input.ID.Value))
			return nil
		}

		// 14.2.3)
		u, err := p.expandIRI(activeCtx, input.ID.Value, false, true, localCtx, defined)
		if err != nil {
			return err
		}

		if !isKeyword(u) && !iri.IsAbsolute(u) && u != BlankNode {
			return ErrInvalidIRIMapping
		}

		if u == KeywordContext {
			return ErrInvalidKeywordAlias
		}

		termDef.IRI = u

		// 14.2.4)
		if strings.Contains(term, "/") || (!strings.HasPrefix(term, ":") && !strings.HasSuffix(term, ":") && strings.Contains(term, ":")) {
			// 14.2.4.1)
			defined[term] = termDefined

			// 14.2.4.2)
			tu, err := p.expandIRI(activeCtx, term, false, true, localCtx, defined)
			if err != nil {
				return ErrInvalidIRIMapping
			}

			if tu != u {
				return ErrInvalidIRIMapping
			}
		} else {
			// 14.2.5)
			if input.Simple && iri.EndsInGenDelim(u) || u == BlankNode {
				if v, ok := p.remapPrefixIRIs[u]; ok {
					termDef.IRI = v
				}
				termDef.Prefix = true
			}
		}
	} else if input.ID.Set && !input.ID.Valid {
		// 14.1) @id was explicitly null
	} else if strings.Contains(term[1:], ":") {
		// 15)
		prefix, suffix, _ := strings.Cut(term, ":")

		// 15.1)
		if !strings.HasPrefix(suffix, "//") {
			if _, ok := localCtx[prefix]; ok {
				if err := p.createTerm(activeCtx, localCtx, prefix, defined, newCreateTermOptions()); err != nil {
					return err
				}
			}
		}

		// 15.2)
		if def, ok := activeCtx.defs[prefix]; ok {
			termDef.IRI = def.IRI + suffix
		} else {
			// 15.3)
			termDef.IRI = term
		}
	} else if strings.Contains(term, "/") {
		// 16)
		// 16.2)
		u, err := p.expandIRI(activeCtx, term, false, true, nil, nil)
		if err != nil {
			return ErrInvalidIRIMapping
		}
		if !iri.IsAbsolute(u) {
			return ErrInvalidIRIMapping
		}
		termDef.IRI = u
	} else if term == KeywordType {
		// 17)
		termDef.IRI = KeywordType
	} else if activeCtx.vocabMapping != "" {
		// 18)
		termDef.IRI = activeCtx.vocabMapping + term
	} else {
		return ErrInvalidIRIMapping
	}

	// 19)
	if input.Container.Set {
		if !input.Container.Valid {
			return ErrInvalidContainerMapping
		}

		// 19.1)
		values := input.Container.Value
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
			termDef.Type = cmp.Or(
				termDef.Type,
				KeywordID,
			)

			// 19.4.2)
			switch termDef.Type {
			case KeywordID, KeywordVocab, "":
			default:
				return ErrInvalidTypeMapping
			}
		}
	}

	// 20)
	if input.Index != "" {
		// 20.1)
		if p.modeLD10 {
			return ErrInvalidTermDefinition
		}
		if !slices.Contains(termDef.Container, KeywordIndex) {
			return ErrInvalidTermDefinition
		}

		// 20.2)
		u, err := p.expandIRI(activeCtx, input.Index, false, true, localCtx, defined)
		if err != nil {
			return ErrInvalidTermDefinition
		}
		if !iri.IsAbsolute(u) {
			return ErrInvalidTermDefinition
		}

		// 20.3)
		termDef.Index = input.Index
	}

	// 21)
	if input.Context != nil {
		// 21.1)
		if p.modeLD10 {
			return ErrInvalidTermDefinition
		}

		// 21.3)
		resolvOpts := newCtxProcessingOpts()
		resolvOpts.override = true
		resolvOpts.remotes = slices.Clone(opts.remotes)
		resolvOpts.validate = false
		ctxDec := json.NewDecoder(bytes.NewReader(input.Context))
		_, err := p.context(
			activeCtx,
			ctxDec,
			opts.baseURL,
			resolvOpts,
		)

		if err != nil {
			return ErrInvalidScopedContext
		}

		// 21.4
		termDef.Context = input.Context
		termDef.BaseIRI = opts.baseURL
	}

	// 22)
	if input.Language.Set && input.Type == "" {
		if !input.Language.Valid {
			termDef.Language = KeywordNull
		} else {
			termDef.Language = strings.ToLower(input.Language.Value)
		}
	}

	// 23)
	if input.Direction.Set && input.Type == "" {
		if !input.Direction.Valid {
			termDef.Direction = KeywordNull
		} else {
			switch input.Direction.Value {
			case DirectionLTR, DirectionRTL:
			default:
				return ErrInvalidBaseDirection
			}
			termDef.Direction = input.Direction.Value
		}
	}

	// 24)
	if input.Nest != "" {
		// 24.1)
		if p.modeLD10 {
			return ErrInvalidTermDefinition
		}

		if isKeyword(input.Nest) && input.Nest != KeywordNest {
			return ErrInvalidNestValue
		}
		termDef.Nest = input.Nest
	}

	// 25)
	if input.Prefix.Set && input.Prefix.Valid {
		// 25.1)
		if p.modeLD10 {
			return ErrInvalidTermDefinition
		}

		if strings.Contains(term, ":") || strings.Contains(term, "/") {
			return ErrInvalidTermDefinition
		}

		// 25.3)
		if input.Prefix.Value && isKeyword(termDef.IRI) {
			return ErrInvalidTermDefinition
		}

		termDef.Prefix = input.Prefix.Value
	}

	// 26)
	if input.HasUnknownKeys {
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
	activeCtx.defs[term] = termDef
	if termDef.Prefix {
		activeCtx.prefixes[term] = struct{}{}
	}
	defined[term] = termDefined
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
	activeContext.initInverse()

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
		var valMap map[unique.Handle[string]]string
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
			if v, ok := valMap[unique.Make(pval)]; !ok {
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
