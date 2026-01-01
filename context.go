package longdistance

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"maps"
	"slices"
	"strings"

	"sourcery.dny.nu/longdistance/internal/iri"
	"sourcery.dny.nu/longdistance/internal/json"
)

// RemoteContextLimit is the recursion limit for resolving remote contexts.
const RemoteContextLimit = 10

// Context represents a processed JSON-LD context.
type Context struct {
	defs            map[string]Term
	prefixes        map[string]struct{}
	protected       map[string]struct{}
	currentBaseIRI  string
	originalBaseIRI string

	vocabMapping     string
	defaultLang      string
	defaultDirection string
	previousCtx      *Context
	inverse          *lazyInverse
}

type lazyInverse struct {
	context *Context
	defs    map[string]map[string]mapping
	built   map[string]struct{}
}

func (l *lazyInverse) get(iri string) (map[string]mapping, bool) {
	if _, ok := l.built[iri]; ok {
		return l.defs[iri], ok
	}

	// we don't have a mapping yet, build it.
	l.workIt(iri)
	l.built[iri] = struct{}{}
	return l.defs[iri], true
}

// newContext initialises a new context with the specified documentURL set as
// the current and original base IRI.
func newContext(documentURL string) *Context {
	return &Context{
		defs:            make(map[string]Term),
		prefixes:        make(map[string]struct{}, 8),
		protected:       make(map[string]struct{}),
		currentBaseIRI:  documentURL,
		originalBaseIRI: documentURL,
	}
}

// Terms returns an iterator over context term definitions.
func (c *Context) Terms() iter.Seq2[string, Term] {
	return func(yield func(string, Term) bool) {
		for k, v := range c.defs {
			if !yield(k, v) {
				return
			}
		}
	}
}

// TermMap returns a map of term to term definitions.
//
// This is a copy, modifying it will not modify the context.
func (c *Context) TermMap() map[string]Term {
	return maps.Clone(c.defs)
}

func (c *Context) initInverse() {
	if c.inverse == nil {
		c.inverse = &lazyInverse{
			defs:    make(map[string]map[string]mapping, len(c.defs)/3),
			built:   make(map[string]struct{}, len(c.defs)/3),
			context: c,
		}
	}
}

func (c *Context) clone() *Context {
	return &Context{
		defs:             maps.Clone(c.defs),
		prefixes:         maps.Clone(c.prefixes),
		protected:        maps.Clone(c.protected),
		currentBaseIRI:   c.currentBaseIRI,
		originalBaseIRI:  c.originalBaseIRI,
		vocabMapping:     c.vocabMapping,
		defaultLang:      c.defaultLang,
		defaultDirection: c.defaultDirection,
		previousCtx:      c.previousCtx,
		inverse:          nil,
	}
}

// isBlank returns if the context is in a state where we can swap it out with
// the context from [WithProcessedContext].
func (c *Context) isBlank() bool {
	if c == nil {
		return true
	}

	return len(c.defs) == 0 &&
		len(c.protected) == 0 &&
		c.previousCtx == nil &&
		c.vocabMapping == "" &&
		c.defaultDirection == "" &&
		c.defaultLang == "" &&
		c.inverse == nil
}

// Context takes in [io.Reader] and parses it into a [Context].
func (p *Processor) Context(ctx io.Reader, baseURL string) (*Context, error) {
	dec := json.NewDecoder(ctx)

	res, err := p.context(nil, dec, baseURL, newCtxProcessingOpts())
	if _, derr := dec.Token(); derr != io.EOF {
		err = errors.Join(derr, fmt.Errorf("trailing garbage in JSON"))
	}

	return res, err
}

type ctxProcessingOpts struct {
	remotes   []string
	override  bool
	propagate bool
	validate  bool
}

func newCtxProcessingOpts() ctxProcessingOpts {
	return ctxProcessingOpts{
		propagate: true,
		validate:  true,
	}
}

func (p *Processor) context(
	activeCtx *Context,
	ctx *json.Decoder,
	baseURL string,
	opts ctxProcessingOpts,
) (*Context, error) {
	if activeCtx == nil {
		activeCtx = newContext(baseURL)
	}

	activeCtx.currentBaseIRI = cmp.Or(
		p.baseIRI,
		activeCtx.currentBaseIRI,
	)

	// 1)
	var result *Context
	if activeCtx.isBlank() {
		result = activeCtx
	} else {
		result = activeCtx.clone()
	}

	tok, err := ctx.Token()
	if err != nil {
		return nil, errors.Join(err, ErrInvalidLocalContext)
	}

	finalFunc := func() error { return nil }

	if delim, ok := tok.(json.Delim); ok && delim == '[' {
		finalFunc = func() error {
			_, err = ctx.Token()
			if err != nil {
				return errors.Join(err, ErrInvalidLocalContext)
			}

			return nil
		}

		if !ctx.More() {
			return nil, nil
		}

		tok, err = ctx.Token()
		if err != nil {
			return nil, errors.Join(err, ErrInvalidLocalContext)
		}
	}

	first := true

	for {
		switch t := tok.(type) {
		case json.Delim:
			// 5.1) Nested arrays are invalid
			if t != '{' {
				return nil, ErrInvalidLocalContext
			}

			ctxObj, err := p.decodeCtxObj(ctx)
			if err != nil {
				return nil, err
			}

			// 2) Check @propagate on first context
			if first && ctxObj.Propagate.Set && ctxObj.Propagate.Valid {
				opts.propagate = ctxObj.Propagate.Value
			}

			// 3)
			if !opts.propagate && result.previousCtx == nil {
				result.previousCtx = activeCtx
			}

			// 5.5)
			if ctxObj.Version.Set {
				if err := p.handleVersion(ctxObj.Version); err != nil {
					return nil, err
				}
			}

			// 5.6)
			if ctxObj.Import.Set && ctxObj.Import.Valid && ctxObj.Import.Value != "" {
				imported, err := p.handleImport(baseURL, ctxObj.Import.Value, ctxObj.Terms)
				if err != nil {
					return nil, err
				}
				ctxObj.Terms = imported
			}

			// 5.7)
			if ctxObj.Base.Set && len(opts.remotes) == 0 {
				if err := p.handleBase(result, ctxObj.Base); err != nil {
					return nil, err
				}
			}

			// 5.8)
			if ctxObj.Vocab.Set {
				if err := p.handleVocab(result, ctxObj.Vocab); err != nil {
					return nil, err
				}
			}

			// 5.9)
			if ctxObj.Lang.Set {
				if err := p.handleLanguage(result, ctxObj.Lang); err != nil {
					return nil, err
				}
			}

			// 5.10)
			if ctxObj.Dir.Set {
				if err := p.handleDirection(result, ctxObj.Dir); err != nil {
					return nil, err
				}
			}

			// 5.11)
			if ctxObj.Propagate.Set {
				if err := p.handlePropagate(ctxObj.Propagate); err != nil {
					return nil, err
				}
			}

			protected := false
			if ctxObj.Protected.Set {
				if !ctxObj.Protected.Valid {
					return nil, ErrInvalidProtectedValue
				}
				protected = ctxObj.Protected.Value
			}

			// 5.12)
			defined := map[string]termState{}

			// 5.13)
			for k := range ctxObj.Terms {
				newOpts := newCreateTermOptions()
				newOpts.baseURL = baseURL
				newOpts.protected = protected
				newOpts.override = opts.override
				newOpts.remotes = slices.Clone(opts.remotes)
				if err := p.createTerm(
					result,
					ctxObj.Terms,
					k,
					defined,
					newOpts,
				); err != nil {
					return nil, err
				}
			}

		case nil:
			// 5.1)
			if !opts.override && len(result.protected) != 0 {
				return nil, ErrInvalidContextNullificaton
			}

			previous := result
			result = newContext(result.originalBaseIRI)
			if !opts.propagate {
				result.previousCtx = previous
			}

		case string:
			// 5.2)
			if !iri.IsAbsolute(baseURL) && !iri.IsAbsolute(t) {
				return nil, ErrLoadingDocument
			}

			iri, err := iri.Resolve(baseURL, t)
			if err != nil {
				return nil, ErrLoadingDocument
			}

			// 5.2.2)
			if !opts.validate && slices.Contains(opts.remotes, iri) {
				return nil, nil
			}

			// 5.2.3)
			if len(opts.remotes) > RemoteContextLimit {
				if p.modeLD10 {
					return nil, ErrRecursiveContextInclusion
				}
				return nil, ErrContextOverflow
			}
			opts.remotes = append(opts.remotes, iri)

			cached := false
			if result.isBlank() {
				if pctx, ok := p.processedContext[iri]; ok {
					curIRI := result.currentBaseIRI
					origIRI := result.originalBaseIRI

					result = pctx.clone()
					result.currentBaseIRI = curIRI
					result.originalBaseIRI = origIRI

					cached = true
				}
			}

			if !cached {
				// 5.2.4) 5.2.5)
				doc, err := p.retrieveRemoteContext(iri)
				if err != nil {
					return nil, err
				}

				// 5.2.6)
				newOpts := newCtxProcessingOpts()
				newOpts.remotes = slices.Clone(opts.remotes)
				newOpts.validate = opts.validate
				remoteDec := json.NewDecoder(bytes.NewReader(doc.Context))
				res, err := p.context(
					result,
					remoteDec,
					doc.URL,
					newOpts,
				)
				if err != nil {
					return nil, err
				}

				result = res
			}
		default:
			return nil, ErrInvalidLocalContext
		}

		first = false

		if !ctx.More() {
			break
		}

		tok, err = ctx.Token()
		if err != nil {
			return nil, errors.Join(err, ErrInvalidLocalContext)
		}
	}

	if err := finalFunc(); err != nil {
		return nil, err
	}

	if first {
		return nil, nil
	}

	if f := p.validateContextFunc; f != nil && !f(result) {
		return nil, ErrInvalid
	}

	return result, nil
}

type null[T any] struct {
	Set   bool
	Valid bool
	Value T
}

func (n *null[T]) UnmarshalJSON(data []byte) error {
	n.Set = true
	if json.IsNull(data) {
		return nil
	}

	var zero T
	if err := json.Unmarshal(data, &zero); err != nil {
		return err
	}

	n.Valid = true
	n.Value = zero
	return nil
}

// contextObj is a decoded context, before term processing takes place. This
// lets us process the context once, avoiding lookups into the JSON during term
// creation because we need to support forward resolution of terms.
type contextObj struct {
	Version   null[float64]
	Import    null[string]
	Base      null[string]
	Vocab     null[string]
	Lang      null[string]
	Dir       null[string]
	Propagate null[bool]
	Protected null[bool]
	Terms     map[string]term
}

func (p *Processor) decodeCtxObj(dec *json.Decoder) (*contextObj, error) {
	obj := &contextObj{
		Terms: make(map[string]term),
	}

	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, errors.Join(err, ErrInvalidLocalContext)
		}

		key, ok := tok.(string)
		if !ok {
			return nil, ErrInvalidLocalContext
		}

		switch key {
		case KeywordVersion:
			if p.modeLD10 {
				return nil, ErrProcessingMode
			}

			if err := dec.Decode(&obj.Version); err != nil {
				return nil, errors.Join(err, ErrInvalidVersionValue)
			}
		case KeywordImport:
			if err := dec.Decode(&obj.Import); err != nil {
				return nil, errors.Join(err, ErrInvalidImportValue)
			}
		case KeywordBase:
			if err := dec.Decode(&obj.Base); err != nil {
				return nil, errors.Join(err, ErrInvalidBaseIRI)
			}
		case KeywordVocab:
			if err := dec.Decode(&obj.Vocab); err != nil {
				return nil, errors.Join(err, ErrInvalidVocabMapping)
			}
		case KeywordLanguage:
			if err := dec.Decode(&obj.Lang); err != nil {
				return nil, errors.Join(err, ErrInvalidDefaultLanguage)
			}
		case KeywordDirection:
			if p.modeLD10 {
				return nil, ErrInvalidContextEntry
			}

			if err := dec.Decode(&obj.Dir); err != nil {
				return nil, errors.Join(err, ErrInvalidBaseDirection)
			}
		case KeywordPropagate:
			if p.modeLD10 {
				return nil, ErrInvalidContextEntry
			}

			if err := dec.Decode(&obj.Propagate); err != nil {
				return nil, errors.Join(err, ErrInvalidPropagateValue)
			}
		case KeywordProtected:
			if err := dec.Decode(&obj.Protected); err != nil {
				return nil, errors.Join(err, ErrInvalidProtectedValue)
			}
		default:
			input, err := p.decodeTerm(dec)
			if err != nil {
				return nil, err
			}
			obj.Terms[key] = input
		}
	}

	tok, err := dec.Token()
	if err != nil {
		return nil, errors.Join(err, ErrInvalidLocalContext)
	}

	if delim, ok := tok.(json.Delim); !ok || delim != '}' {
		return nil, ErrInvalidLocalContext
	}

	return obj, nil
}

func (p *Processor) decodeTerm(dec *json.Decoder) (term, error) {
	tok, err := dec.Token()
	if err != nil {
		return term{}, err
	}

	switch t := tok.(type) {
	case nil:
		return term{Null: true, ID: null[string]{Set: true}}, nil
	case string:
		return term{Simple: true, ID: null[string]{Set: true, Valid: true, Value: t}}, nil
	case json.Delim:
		if t != '{' {
			return term{}, ErrInvalidTermDefinition
		}
		return p.decodeTermObj(dec)
	default:
		return term{}, ErrInvalidTermDefinition
	}
}

func (p *Processor) decodeTermObj(dec *json.Decoder) (term, error) {
	var input term

	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return input, err
		}

		key, ok := tok.(string)
		if !ok {
			return input, ErrInvalidTermDefinition
		}

		switch key {
		case KeywordID:
			if err := dec.Decode(&input.ID); err != nil {
				return input, ErrInvalidIRIMapping
			}
		case KeywordType:
			if err := dec.Decode(&input.Type); err != nil {
				return input, ErrInvalidTypeMapping
			}
		case KeywordReverse:
			if err := dec.Decode(&input.Reverse); err != nil {
				return input, ErrInvalidIRIMapping
			}
		case KeywordContainer:
			if p.modeLD10 {
				// In LD 1.0 it must be a string and only a string
				var s string
				if err := dec.Decode(&s); err != nil {
					return input, ErrInvalidContainerMapping
				}

				input.Container = null[array[string]]{
					Set:   true,
					Valid: true,
					Value: []string{s},
				}

				continue
			}

			if err := dec.Decode(&input.Container); err != nil {
				return input, ErrInvalidContainerMapping
			}
		case KeywordIndex:
			if err := dec.Decode(&input.Index); err != nil {
				return input, ErrInvalidTermDefinition
			}
		case KeywordContext:
			if err := dec.Decode(&input.Context); err != nil {
				return input, ErrInvalidScopedContext
			}
		case KeywordLanguage:
			if err := dec.Decode(&input.Language); err != nil {
				return input, ErrInvalidLanguageMapping
			}
		case KeywordDirection:
			if err := dec.Decode(&input.Direction); err != nil {
				return input, ErrInvalidBaseDirection
			}
		case KeywordNest:
			if err := dec.Decode(&input.Nest); err != nil {
				return input, ErrInvalidNestValue
			}
		case KeywordPrefix:
			if err := dec.Decode(&input.Prefix); err != nil {
				return input, ErrInvalidPrefixValue
			}
		case KeywordProtected:
			if err := dec.Decode(&input.Protected); err != nil {
				return input, ErrInvalidProtectedValue
			}
		default:
			if _, err := dec.Token(); err != nil {
				return input, err
			}
			input.HasUnknownKeys = true
		}
	}

	tok, err := dec.Token()
	if err != nil {
		return input, err
	}

	if delim, ok := tok.(json.Delim); !ok || delim != '}' {
		return input, ErrInvalidTermDefinition
	}

	return input, nil
}

func (p *Processor) handlePropagate(prop null[bool]) error {
	if !prop.Valid {
		return ErrInvalidPropagateValue
	}

	return nil
}

func (p *Processor) handleDirection(result *Context, dir null[string]) error {
	if !dir.Valid {
		result.defaultDirection = ""
		return nil
	}

	switch dir.Value {
	case DirectionLTR, DirectionRTL:
	default:
		return ErrInvalidBaseDirection
	}

	result.defaultDirection = dir.Value
	return nil
}

func (p *Processor) handleLanguage(result *Context, lang null[string]) error {
	if !lang.Valid {
		result.defaultLang = ""
		return nil
	}

	result.defaultLang = strings.ToLower(lang.Value)
	return nil
}

func (p *Processor) handleVocab(result *Context, vocab null[string]) error {
	// 5.8.2)
	if !vocab.Valid {
		result.vocabMapping = ""
		return nil
	}

	// 5.8.3)
	if !(iri.IsAbsolute(vocab.Value) || iri.IsRelative(vocab.Value) || vocab.Value == BlankNode) {
		return ErrInvalidVocabMapping
	}

	u, err := p.expandIRI(result, vocab.Value, true, true, nil, nil)
	if err != nil {
		return err
	}

	result.vocabMapping = u
	return nil
}

func (p *Processor) handleBase(result *Context, base null[string]) error {
	// 5.7.2)
	if !base.Valid {
		result.currentBaseIRI = ""
		return nil
	}

	// 5.7.3)
	if iri.IsAbsolute(base.Value) {
		result.currentBaseIRI = base.Value
		return nil
	}

	// 5.7.4)
	if iri.IsRelative(base.Value) {
		u, err := iri.Resolve(result.currentBaseIRI, base.Value)
		if err != nil {
			return ErrInvalidBaseIRI
		}
		result.currentBaseIRI = u
		return nil
	}

	// 5.7.5)
	return ErrInvalidBaseIRI
}

func (p *Processor) handleImport(baseURL string, uri string, terms map[string]term) (map[string]term, error) {
	// 5.6.1)
	if p.modeLD10 {
		return nil, ErrInvalidContextEntry
	}

	// 5.6.3)
	iri, err := iri.Resolve(baseURL, uri)
	if err != nil {
		return nil, ErrInvalidRemoteContext
	}

	// 5.6.4) 5.6.5)
	res, err := p.retrieveRemoteContext(iri)
	if err != nil {
		return nil, err
	}

	if !json.IsMap(res.Context) {
		return nil, ErrInvalidRemoteContext
	}

	dec := json.NewDecoder(bytes.NewReader(res.Context))
	tok, err := dec.Token()
	if err != nil {
		return nil, ErrInvalidRemoteContext
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, ErrInvalidRemoteContext
	}

	importedTerms := make(map[string]term)

	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, ErrInvalidRemoteContext
		}

		key, ok := tok.(string)
		if !ok {
			return nil, ErrInvalidRemoteContext
		}

		// 5.6.7) Check for nested @import
		if key == KeywordImport {
			return nil, ErrInvalidContextEntry
		}

		switch key {
		case KeywordVersion, KeywordBase, KeywordVocab,
			KeywordLanguage, KeywordDirection, KeywordPropagate, KeywordProtected:
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			continue
		}

		input, err := p.decodeTerm(dec)
		if err != nil {
			return nil, err
		}
		importedTerms[key] = input
	}

	if _, err := dec.Token(); err != nil {
		return nil, ErrInvalidRemoteContext
	}

	for k, v := range terms {
		importedTerms[k] = v
	}

	return importedTerms, nil
}

func (p *Processor) handleVersion(ver null[float64]) error {
	if ver.Value != 1.1 {
		return ErrInvalidVersionValue
	}

	return nil
}

func (p *Processor) retrieveRemoteContext(
	iri string,
) (Document, error) {
	// 5.2.4) 5.2.5) the document loader is expected to do the caching
	if p.loader == nil {
		return Document{}, fmt.Errorf("no loader %w", ErrLoadingRemoteContext)
	}
	doc, err := p.loader(context.TODO(), iri)
	if err != nil {
		return Document{}, err
	}

	return doc, nil
}

type mapping struct {
	Language map[string]string
	Type     map[string]string
	Any      map[string]string
}

// workIt flips a context and reverses it
//
// ​ti esrever dna ti pilf ,nwod gniht ym tuP
func (lctx *lazyInverse) workIt(iri string) {
	// 2)
	defaultLang := cmp.Or(
		strings.ToLower(lctx.context.defaultLang),
		KeywordNone,
	)

	terms := make([]string, 0, 4)
	for key, def := range lctx.context.defs {
		// 3.1)
		if !def.IsZero() && def.IRI == iri {
			terms = append(terms, key)
		}
	}

	slices.SortFunc(terms, sortedLeast)

	for _, key := range terms {
		// 3)
		def := lctx.context.defs[key]

		// 3.2)
		container := KeywordNone
		if def.Container != nil {
			dc := slices.Clone(def.Container)
			slices.Sort(dc)
			container = strings.Join(dc, "")
		}

		// 3.3) 3.4) 3.5)
		containerMap, ok := lctx.defs[iri]
		if !ok {
			containerMap = map[string]mapping{}
			lctx.defs[iri] = containerMap
		}

		// 3.6)
		if _, ok := containerMap[container]; !ok {
			containerMap[container] = mapping{
				Language: make(map[string]string),
				Type:     make(map[string]string),
				Any: map[string]string{
					KeywordAny: key,
				},
			}
		}

		// 3.7)
		typeLanguage := containerMap[container]

		// 3.8)
		typeMap := typeLanguage.Type

		// 3.9)
		langMap := typeLanguage.Language

		if def.Reverse {
			// 3.10)
			if _, ok := typeMap[KeywordReverse]; !ok {
				typeMap[KeywordReverse] = key
			}
		} else if def.Type == KeywordNone {
			// 3.11)
			if _, ok := langMap[KeywordAny]; !ok {
				// 3.11.1)
				langMap[KeywordAny] = key
			}
			if _, ok := typeMap[KeywordAny]; !ok {
				// 3.11.2)
				typeMap[KeywordAny] = key
			}
		} else if def.Type != "" {
			// 3.12)
			if _, ok := typeMap[def.Type]; !ok {
				// 3.12.1
				typeMap[def.Type] = key
			}
		} else if def.Language != "" && def.Direction != "" {
			// 3.13)
			// 3.13.1) + 3.13.5)
			langDir := KeywordNone
			if def.Language != KeywordNull && def.Direction != KeywordNull {
				// 3.13.2)
				langDir = strings.ToLower(def.Language) + "_" + def.Direction
			} else if def.Language != KeywordNull {
				// 3.13.3)
				langDir = strings.ToLower(def.Language)
			} else if def.Direction != KeywordNull {
				// 3.13.4)
				langDir = "_" + def.Direction
			}

			// 3.13.6)
			if _, ok := langMap[langDir]; !ok {
				langMap[langDir] = key
			}
		} else if def.Language != "" {
			// 3.14)
			lang := KeywordNull
			if def.Language != KeywordNull {
				lang = strings.ToLower(def.Language)
			}

			if _, ok := langMap[lang]; !ok {
				langMap[lang] = key
			}
		} else if def.Direction != "" {
			// 3.15)
			dir := KeywordNone
			if def.Direction != KeywordNull {
				dir = "_" + def.Direction
			}

			if _, ok := langMap[dir]; !ok {
				langMap[dir] = key
			}
		} else if defDir := lctx.context.defaultDirection; defDir != "" {
			// 3.16)
			langDir := strings.ToLower(defaultLang) + "_" + defDir
			if _, ok := langMap[langDir]; !ok {
				langMap[langDir] = key
			}
			if _, ok := langMap[KeywordNone]; !ok {
				langMap[KeywordNone] = key
			}
			if _, ok := typeMap[KeywordNone]; !ok {
				typeMap[KeywordNone] = key
			}
		} else {
			// 3.17)

			// 3.17.1)
			if _, ok := langMap[defaultLang]; !ok {
				langMap[defaultLang] = key
			}

			// 3.17.2)
			if _, ok := langMap[KeywordNone]; !ok {
				langMap[KeywordNone] = key
			}

			// 3.17.3)
			if _, ok := typeMap[KeywordNone]; !ok {
				typeMap[KeywordNone] = key
			}
		}
	}
}
