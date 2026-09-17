package longdistance

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json/jsontext"
	"log/slog"
	"slices"
	"strings"

	"sourcery.dny.nu/longdistance/internal/iri"
	"sourcery.dny.nu/longdistance/internal/twiddle"
)

// termState tracks the definition state of a term during context processing.
type termState uint8

const (
	termUndefined termState = iota // Term not yet processed
	termDefining                   // Term is being defined (for cycle detection)
	termDefined                    // Term definition is complete
)

// termFlag records which entries a term definition carried.
type termFlag uint16

const (
	flagSimple      termFlag = 1 << iota // Definition was a plain string
	flagNull                             // Definition was explicitly null
	flagUnknownKeys                      // Definition carried unknown entries

	flagID
	flagNullID // @id was explicitly null
	flagContainer
	flagNullContainer // @container was explicitly null
	flagLanguage
	flagNullLanguage // @language was explicitly null
	flagDirection
	flagNullDirection // @direction was explicitly null

	flagPrefix    // @prefix set, read Term.Prefix for the value
	flagProtected // @protected set, read Term.Protected for the value

	flagType    // @type set, not null or empty
	flagReverse // @reverse set, not null or empty
)

// Term represents a term definition in a JSON-LD context after it's been fully processed.
type Term struct {
	IRI       string
	Prefix    bool
	Protected bool
	Reverse   bool

	state termState
	flags twiddle.Field[termFlag]

	BaseIRI   string
	Context   jsontext.Value
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

type createTermOptions struct {
	baseURL   string
	remotes   []string
	protected bool
	override  bool
}

func (p *Processor) createTerm(
	ctx context.Context,
	activeCtx *Context,
	localCtx map[string]*Term,
	term string,
	termDef *Term,
	opts createTermOptions,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// 3) its the already passed in term

	// 1)
	if termDef.state != termUndefined {
		if termDef.state == termDefined {
			return nil
		}
		return ErrCyclicIRIMapping
	}

	// 2)
	if term == "" {
		return ErrInvalidTermDefinition
	} else {
		termDef.state = termDefining
	}

	// 6) do this here because 4) also uses this lookup
	oldDef, oldDefOK := activeCtx.defs[term]

	// 4)
	if term == KeywordType {
		if p.modeLD10 {
			return ErrKeywordRedefinition
		}

		// Check if @type is protected before validating the new definition
		// If protected and not overriding, return protected term redefinition first
		if oldDefOK && oldDef.Protected && !opts.override {
			return ErrProtectedTermRedefinition
		}

		// For @type, only @container and @protected are allowed
		if termDef.flags.Any(flagID, flagNullID, flagType, flagReverse) ||
			termDef.Index != "" || termDef.Context != nil ||
			termDef.flags.Any(flagLanguage, flagNullLanguage) ||
			termDef.flags.Any(flagDirection, flagNullDirection) ||
			termDef.Nest != "" || termDef.flags.Any(flagPrefix) ||
			termDef.flags.Any(flagUnknownKeys) {
			return ErrKeywordRedefinition
		}

		// @container must be @set if provided, and is required for non-simple definitions
		if termDef.flags.Any(flagContainer) {
			if len(termDef.Container) != 1 {
				return ErrKeywordRedefinition
			}
			if termDef.Container[0] != KeywordSet {
				return ErrKeywordRedefinition
			}
		} else if termDef.flags.None(flagSimple, flagNull) {
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
	if oldDefOK {
		delete(activeCtx.defs, term)
		delete(activeCtx.prefixes, term)
		delete(activeCtx.protected, term)
	}

	// 10) 11) @protected is already on the Term, only needs validating
	// or passing the default if not present
	if termDef.flags.Any(flagProtected) {
		if p.modeLD10 {
			return ErrInvalidTermDefinition
		}
	} else {
		termDef.Protected = opts.protected
	}

	// at this point protected is finalised, so add the
	// term to the protected set on activeContext
	if termDef.Protected {
		activeCtx.protected[term] = struct{}{}
	}

	// 12)
	if termDef.flags.Any(flagType) {
		// 12.2)
		u, err := p.expandIRI(ctx, activeCtx, termDef.Type, false, true, localCtx)
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
	if termDef.flags.Any(flagReverse) {
		// 13.1)
		if termDef.flags.Any(flagID, flagNullID) || termDef.Nest != "" {
			return ErrInvalidReverseProperty
		}

		// 13.3)
		if looksLikeKeyword(termDef.IRI) {
			p.logger.Warn("keyword lookalike value encountered",
				slog.String("value", termDef.IRI))
			return nil
		}

		// 13.4)
		u, err := p.expandIRI(ctx, activeCtx, termDef.IRI, false, true, localCtx)
		if err != nil {
			return ErrInvalidIRIMapping
		}

		if !iri.IsAbsolute(u) && !isBlankNode(u) {
			return ErrInvalidIRIMapping
		}

		termDef.IRI = u

		// 13.5)
		if termDef.flags.Any(flagContainer, flagNullContainer) {
			if termDef.flags.Any(flagContainer) {
				if termDef.Container[0] != KeywordSet &&
					termDef.Container[0] != KeywordIndex {
					return ErrInvalidReverseProperty
				}
			} else {
				termDef.Container = nil
			}
		}

		// 13.6)
		termDef.Reverse = true

		// This whole step is missing in the spec but without
		// it t0131 can't pass. So. YOLO.
		if !slices.Contains(termDef.Container, KeywordIndex) {
			termDef.Index = ""
		}

		// 13.7
		termDef.state = termDefined
		activeCtx.define(term, termDef)
		return nil
	} else if termDef.flags.Any(flagID) && term != termDef.IRI {
		// 14.2)
		if !isKeyword(termDef.IRI) && looksLikeKeyword(termDef.IRI) {
			// 14.2.2)
			p.logger.Warn("keyword lookalike value encountered",
				slog.String("value", termDef.IRI))
			return nil
		}

		// 14.2.3)
		u, err := p.expandIRI(ctx, activeCtx, termDef.IRI, false, true, localCtx)
		if err != nil {
			return err
		}

		if !isKeyword(u) && !iri.IsAbsolute(u) && !isBlankNode(u) {
			return ErrInvalidIRIMapping
		}

		if u == KeywordContext {
			return ErrInvalidKeywordAlias
		}

		termDef.IRI = u

		// 14.2.4)
		if strings.Contains(term, "/") || (!strings.HasPrefix(term, ":") && !strings.HasSuffix(term, ":") && strings.Contains(term, ":")) {
			// 14.2.4.1)
			termDef.state = termDefined

			// 14.2.4.2)
			tu, err := p.expandIRI(ctx, activeCtx, term, false, true, localCtx)
			if err != nil {
				return ErrInvalidIRIMapping
			}

			if tu != u {
				return ErrInvalidIRIMapping
			}
		} else {
			// 14.2.5)
			if termDef.flags.Any(flagSimple) &&
				(iri.EndsInGenDelim(u) || isBlankNode(u)) {
				if v, ok := p.remapPrefixIRIs[u]; ok {
					termDef.IRI = v
				}
				termDef.Prefix = true
			}
		}
	} else if termDef.flags.Any(flagNullID) {
		// 14.1) @id was explicitly null
	} else if strings.Contains(term[1:], ":") {
		// 15)
		prefix, suffix, _ := strings.Cut(term, ":")

		// 15.1)
		if !strings.HasPrefix(suffix, "//") {
			if t, ok := localCtx[prefix]; ok {
				if err := p.createTerm(ctx, activeCtx, localCtx, prefix, t, createTermOptions{}); err != nil {
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
		u, err := p.expandIRI(ctx, activeCtx, term, false, true, nil)
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
	if termDef.flags.Any(flagContainer, flagNullContainer) {
		if termDef.flags.Any(flagNullContainer) {
			return ErrInvalidContainerMapping
		}

		// 19.1)
		values := termDef.Container
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
	if termDef.Index != "" {
		// 20.1)
		if p.modeLD10 {
			return ErrInvalidTermDefinition
		}
		if !slices.Contains(termDef.Container, KeywordIndex) {
			return ErrInvalidTermDefinition
		}

		// 20.2)
		u, err := p.expandIRI(ctx, activeCtx, termDef.Index, false, true, localCtx)
		if err != nil {
			return ErrInvalidTermDefinition
		}
		if !iri.IsAbsolute(u) {
			return ErrInvalidTermDefinition
		}

		// 20.3)
	}

	// 21)
	if termDef.Context != nil {
		// 21.1)
		if p.modeLD10 {
			return ErrInvalidTermDefinition
		}

		// 21.3)
		resolvOpts := newCtxProcessingOpts()
		resolvOpts.override = true
		resolvOpts.remotes = opts.remotes
		resolvOpts.validate = false
		ctxDec := jsontext.NewDecoder(bytes.NewBuffer(termDef.Context))
		_, err := p.context(
			ctx,
			activeCtx,
			ctxDec,
			opts.baseURL,
			resolvOpts,
		)

		if err != nil {
			return ErrInvalidScopedContext
		}

		// 21.4)
		termDef.BaseIRI = opts.baseURL
	}

	// 22) this is slightly inverted from the spec, as we need to drop the decoded value
	if termDef.flags.None(flagLanguage, flagNullLanguage) || termDef.flags.Any(flagType) {
		termDef.Language = ""
	} else if termDef.flags.Any(flagNullLanguage) {
		termDef.Language = KeywordNull
	} else {
		termDef.Language = strings.ToLower(termDef.Language)
	}

	// 23) same here, drop the decoded value
	if termDef.flags.None(flagDirection, flagNullDirection) || termDef.flags.Any(flagType) {
		termDef.Direction = ""
	} else if termDef.flags.Any(flagNullDirection) {
		termDef.Direction = KeywordNull
	} else {
		switch termDef.Direction {
		case DirectionLTR, DirectionRTL:
		default:
			return ErrInvalidBaseDirection
		}
	}

	// 24)
	if termDef.Nest != "" {
		// 24.1)
		if p.modeLD10 {
			return ErrInvalidTermDefinition
		}

		if isKeyword(termDef.Nest) && termDef.Nest != KeywordNest {
			return ErrInvalidNestValue
		}
	}

	// 25)
	if termDef.flags.Any(flagPrefix) {
		// 25.1)
		if p.modeLD10 {
			return ErrInvalidTermDefinition
		}

		if strings.Contains(term, ":") || strings.Contains(term, "/") {
			return ErrInvalidTermDefinition
		}

		// 25.3)
		if termDef.Prefix && isKeyword(termDef.IRI) {
			return ErrInvalidTermDefinition
		}
	}

	// 26)
	if termDef.flags.Any(flagUnknownKeys) {
		return ErrInvalidTermDefinition
	}

	// the definition is complete
	termDef.state = termDefined

	// 27)
	def := termDef
	if oldDefOK && oldDef.Protected && !opts.override {
		// 27.1)
		if !oldDef.equalWithoutProtected(termDef) {
			return ErrProtectedTermRedefinition
		}

		// 27.2)
		def = oldDef
	}

	// 28)
	activeCtx.define(term, def)
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
	containerMap, _ := inverse.get(keyIriVar)

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

func flags[T twiddle.Size](isNil bool, set, null T) T {
	if isNil {
		return null
	}
	return set
}
