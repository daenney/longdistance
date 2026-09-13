package longdistance

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"io"
	"iter"
	"maps"
	"slices"
	"strings"

	"sourcery.dny.nu/longdistance/internal/iri"
	"sourcery.dny.nu/longdistance/internal/jsonutil"
	"sourcery.dny.nu/longdistance/internal/twiddle"
)

// RemoteContextLimit is the recursion limit for resolving remote contexts.
const RemoteContextLimit = 10

// Context represents a processed JSON-LD context.
type Context struct {
	defs            map[string]*Term
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
	index   map[string][]string
}

func (l *lazyInverse) buildIndex() {
	// 3.1)
	idx := make(map[string][]string, len(l.context.defs))
	for term, def := range l.context.defs {
		idx[def.IRI] = append(idx[def.IRI], term)
	}

	for _, terms := range idx {
		slices.SortFunc(terms, sortedLeast)
	}

	l.index = idx
}

func (l *lazyInverse) get(iri string) (map[string]mapping, bool) {
	if val, ok := l.defs[iri]; ok {
		return val, ok
	}

	// we don't have a mapping yet, build it.
	l.workIt(iri)

	val, ok := l.defs[iri]
	return val, ok
}

// newContext initialises a new context with the specified documentURL set as
// the current and original base IRI.
func newContext(documentURL string) *Context {
	return &Context{
		defs:            make(map[string]*Term),
		prefixes:        make(map[string]struct{}, 8),
		protected:       make(map[string]struct{}),
		currentBaseIRI:  documentURL,
		originalBaseIRI: documentURL,
	}
}

// Terms returns an iterator over context term definitions.
func (c *Context) Terms() iter.Seq2[string, *Term] {
	return func(yield func(string, *Term) bool) {
		for k, v := range c.defs {
			if !yield(k, v) {
				return
			}
		}
	}
}

func (c *Context) initInverse() {
	if c.inverse == nil {
		c.inverse = &lazyInverse{
			defs:    make(map[string]map[string]mapping, len(c.defs)),
			context: c,
		}

		c.inverse.buildIndex()
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

var emptyTerm Term

func (c *Context) lookup(s string) (*Term, bool) {
	val, ok := c.defs[s]
	return cmp.Or(val, &emptyTerm), ok
}

// Context takes in [io.Reader] and parses it into a [Context].
func (p *Processor) Context(
	ctx context.Context,
	rawCtx io.Reader, baseURL string) (*Context, error) {
	dec := jsontext.NewDecoder(rawCtx)

	res, err := p.context(ctx, nil, dec, baseURL, newCtxProcessingOpts())
	if err != nil {
		return nil, err
	}

	if _, derr := dec.ReadToken(); !errors.Is(derr, io.EOF) {
		return nil, errors.Join(derr, fmt.Errorf("trailing garbage in JSON"))
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
	ctx context.Context,
	activeCtx *Context,
	rawCtx *jsontext.Decoder,
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

	inArray := false

	if rawCtx.PeekKind() == jsontext.KindBeginArray {
		if _, err := rawCtx.ReadToken(); err != nil {
			return nil, errors.Join(err, ErrInvalidLocalContext)
		}

		inArray = true
	}

	first := true

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if inArray && rawCtx.PeekKind() == jsontext.KindEndArray {
			break
		}

		switch rawCtx.PeekKind() {
		case jsontext.KindBeginObject:
			if _, err := rawCtx.ReadToken(); err != nil {
				return nil, errors.Join(err, ErrInvalidLocalContext)
			}

			ctxObj, err := p.decodeCtxObj(ctx, rawCtx)
			if err != nil {
				return nil, err
			}

			// 2) Check @propagate on first context
			if first && ctxObj.flags.Any(ctxPropagate) {
				opts.propagate = ctxObj.Propagate
			}

			// 3)
			if !opts.propagate && result.previousCtx == nil {
				result.previousCtx = activeCtx
			}

			// 5.5)
			if ctxObj.flags.Any(ctxVersion) {
				if err := p.handleVersion(ctxObj.Version); err != nil {
					return nil, err
				}
			}

			// 5.6)
			if ctxObj.flags.Any(ctxImport) && ctxObj.Import != "" {
				imported, err := p.handleImport(ctx, baseURL, ctxObj.Import, ctxObj.Terms)
				if err != nil {
					return nil, err
				}
				ctxObj.Terms = imported
			}

			// 5.7)
			if ctxObj.flags.Any(ctxBase, ctxNullBase) && len(opts.remotes) == 0 {
				if err := p.handleBase(result, ctxObj.Base, ctxObj.flags.Any(ctxNullBase)); err != nil {
					return nil, err
				}
			}

			// 5.8)
			if ctxObj.flags.Any(ctxVocab, ctxNullVocab) {
				if err := p.handleVocab(ctx, result, ctxObj.Vocab, ctxObj.flags.Any(ctxNullVocab)); err != nil {
					return nil, err
				}
			}

			// 5.9)
			if ctxObj.flags.Any(ctxLang, ctxNullLang) {
				if err := p.handleLanguage(result, ctxObj.Lang, ctxObj.flags.Any(ctxNullLang)); err != nil {
					return nil, err
				}
			}

			// 5.10)
			if ctxObj.flags.Any(ctxDir, ctxNullDir) {
				if err := p.handleDirection(result, ctxObj.Dir, ctxObj.flags.Any(ctxNullDir)); err != nil {
					return nil, err
				}
			}

			// 5.11)
			if ctxObj.flags.Any(ctxPropagate, ctxNullPropagate) {
				if err := p.handlePropagate(ctxObj.flags.Any(ctxNullPropagate)); err != nil {
					return nil, err
				}
			}

			protected := false
			if ctxObj.flags.Any(ctxProtected, ctxNullProtected) {
				if ctxObj.flags.Any(ctxNullProtected) {
					return nil, ErrInvalidProtectedValue
				}
				protected = ctxObj.Protected
			}

			// 5.12)
			if n := len(ctxObj.Terms); n > 8 && len(result.defs) == 0 {
				result.defs = make(map[string]*Term, n)
			}

			// 5.13)
			for k := range ctxObj.Terms {
				newOpts := newCreateTermOptions()
				newOpts.baseURL = baseURL
				newOpts.protected = protected
				newOpts.override = opts.override
				newOpts.remotes = opts.remotes
				if err := p.createTerm(
					ctx,
					result,
					ctxObj.Terms,
					k,
					newOpts,
				); err != nil {
					return nil, err
				}
			}

		case jsontext.KindNull:
			if err := rawCtx.SkipValue(); err != nil {
				return nil, errors.Join(err, ErrInvalidLocalContext)
			}

			// 5.1)
			if !opts.override && len(result.protected) != 0 {
				return nil, ErrInvalidContextNullificaton
			}

			previous := result
			result = newContext(result.originalBaseIRI)
			if !opts.propagate {
				result.previousCtx = previous
			}

		case jsontext.KindString:
			tok, err := rawCtx.ReadToken()
			if err != nil {
				return nil, errors.Join(err, ErrInvalidLocalContext)
			}

			t := tok.String()

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
			// Clip is load-bearing, it ensures cap=len so the append will allocate.
			// This avoids cloning opts.remotes everywhere else
			opts.remotes = append(slices.Clip(opts.remotes), iri)

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
				doc, err := p.retrieveRemoteContext(ctx, iri)
				if err != nil {
					return nil, err
				}

				// 5.2.6)
				newOpts := newCtxProcessingOpts()
				newOpts.remotes = opts.remotes
				newOpts.validate = opts.validate
				// https://github.com/w3c/json-ld-api/issues/708
				newOpts.override = opts.override
				remoteDec := jsontext.NewDecoder(bytes.NewBuffer(doc.Context))
				res, err := p.context(
					ctx,
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
			// 5.1) Only map, string or null are valid.
			_, err := rawCtx.ReadToken()
			return nil, errors.Join(err, ErrInvalidLocalContext)
		}

		first = false

		if !inArray {
			break
		}
	}

	if inArray {
		if _, err := rawCtx.ReadToken(); err != nil {
			return nil, errors.Join(err, ErrInvalidLocalContext)
		}
	}

	if first {
		return nil, nil
	}

	if f := p.validateContextFunc; f != nil && !f(result) {
		return nil, ErrInvalid
	}

	return result, nil
}

// ctxFlag records which entries a context object carried
type ctxFlag uint16

const (
	ctxVersion ctxFlag = 1 << iota
	ctxImport

	ctxBase
	ctxNullBase // @base was explicitly null
	ctxVocab
	ctxNullVocab // @vocab was explicitly null
	ctxLang
	ctxNullLang // @language was explicitly null
	ctxDir
	ctxNullDir // @direction was explicitly null

	ctxPropagate     // @propagate set, read contextObj.Propagate for the value
	ctxNullPropagate // @propagate was explicitly null
	ctxProtected     // @protected set, read contextObj.Protected for the value
	ctxNullProtected // @protected was explicitly null
)

// contextObj is a decoded context, before term processing takes place. This
// lets us process the context once, avoiding lookups into the JSON during term
// creation because we need to support forward resolution of terms.
type contextObj struct {
	Version   float64
	Import    string
	Base      string
	Vocab     string
	Lang      string
	Dir       string
	Propagate bool
	Protected bool

	flags twiddle.Field[ctxFlag]
	Terms map[string]*term
}

func (p *Processor) decodeCtxObj(ctx context.Context, dec *jsontext.Decoder) (*contextObj, error) {
	obj := &contextObj{
		Terms: make(map[string]*term),
	}

	for dec.PeekKind() != jsontext.KindEndObject {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		tok, err := dec.ReadToken()
		if err != nil {
			return nil, errors.Join(err, ErrInvalidLocalContext)
		}

		if tok.Kind() != jsontext.KindString {
			return nil, ErrInvalidLocalContext
		}

		key := tok.String()

		switch key {
		case KeywordVersion:
			if p.modeLD10 {
				return nil, ErrProcessingMode
			}

			val, _, err := jsonutil.DecodeFloat64(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidVersionValue)
			}

			obj.Version = val
			obj.flags.Set(ctxVersion)
		case KeywordImport:
			val, isNil, err := jsonutil.DecodeString(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidImportValue)
			}

			obj.Import = val
			if !isNil {
				obj.flags.Set(ctxImport)
			}
		case KeywordBase:
			val, isNil, err := jsonutil.DecodeString(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidBaseIRI)
			}

			obj.Base = val
			obj.flags.Set(flags(isNil, ctxBase, ctxNullBase))
		case KeywordVocab:
			val, isNil, err := jsonutil.DecodeString(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidVocabMapping)
			}

			obj.Vocab = val
			obj.flags.Set(flags(isNil, ctxVocab, ctxNullVocab))
		case KeywordLanguage:
			val, isNil, err := jsonutil.DecodeString(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidDefaultLanguage)
			}

			obj.Lang = val
			obj.flags.Set(flags(isNil, ctxLang, ctxNullLang))
		case KeywordDirection:
			if p.modeLD10 {
				return nil, ErrInvalidContextEntry
			}

			val, isNil, err := jsonutil.DecodeString(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidBaseDirection)
			}

			obj.Dir = val
			obj.flags.Set(flags(isNil, ctxDir, ctxNullDir))
		case KeywordPropagate:
			if p.modeLD10 {
				return nil, ErrInvalidContextEntry
			}

			val, isNil, err := jsonutil.DecodeBool(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidPropagateValue)
			}

			obj.Propagate = val
			obj.flags.Set(flags(isNil, ctxPropagate, ctxNullPropagate))
		case KeywordProtected:
			val, isNil, err := jsonutil.DecodeBool(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidProtectedValue)
			}

			obj.Protected = val
			obj.flags.Set(flags(isNil, ctxProtected, ctxNullProtected))
		default:
			input, err := p.decodeTerm(dec)
			if err != nil {
				return nil, err
			}
			obj.Terms[key] = input
		}
	}

	tok, err := dec.ReadToken()
	if err != nil {
		return nil, errors.Join(err, ErrInvalidLocalContext)
	}

	if tok.Kind() != jsontext.KindEndObject {
		return nil, ErrInvalidLocalContext
	}

	return obj, nil
}

func (p *Processor) decodeTerm(dec *jsontext.Decoder) (*term, error) {
	switch dec.PeekKind() {
	case jsontext.KindNull:
		if err := dec.SkipValue(); err != nil {
			return nil, err
		}

		t := &term{}
		t.flags.Set(flagNull, flagNullID)
		return t, nil
	case jsontext.KindString:
		tok, err := dec.ReadToken()
		if err != nil {
			return nil, err
		}

		t := &term{ID: tok.String()}
		t.flags.Set(flagSimple, flagID)
		return t, nil
	case jsontext.KindBeginObject:
		if _, err := dec.ReadToken(); err != nil {
			return nil, err
		}

		return p.decodeTermObj(dec)
	default:
		_, err := dec.ReadToken()
		return nil, errors.Join(err, ErrInvalidTermDefinition)
	}
}

func (p *Processor) decodeTermObj(dec *jsontext.Decoder) (*term, error) {
	var input term

	for dec.PeekKind() != jsontext.KindEndObject {
		tok, err := dec.ReadToken()
		if err != nil {
			return nil, err
		}

		if tok.Kind() != jsontext.KindString {
			return nil, ErrInvalidTermDefinition
		}

		key := tok.String()

		switch key {
		case KeywordID:
			val, isNil, err := jsonutil.DecodeString(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidIRIMapping)
			}

			input.ID = val
			input.flags.Set(flags(isNil, flagID, flagNullID))
		case KeywordType:
			val, _, err := jsonutil.DecodeString(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidTypeMapping)
			}

			input.Type = val
		case KeywordReverse:
			val, _, err := jsonutil.DecodeString(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidIRIMapping)
			}

			input.Reverse = val
		case KeywordContainer:
			if p.modeLD10 {
				// In LD 1.0 it must be a string and only a string
				val, _, err := jsonutil.DecodeString(dec)
				if err != nil {
					return nil, errors.Join(err, ErrInvalidContainerMapping)
				}

				input.Container = []string{val}
				input.flags.Set(flagContainer)

				continue
			}

			res, isNil, err := jsonutil.DecodeStringSlice(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidContainerMapping)
			}

			input.Container = res
			input.flags.Set(flags(isNil, flagContainer, flagNullContainer))
		case KeywordIndex:
			val, _, err := jsonutil.DecodeString(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidTermDefinition)
			}

			input.Index = val
		case KeywordContext:
			val, err := dec.ReadValue()
			if err != nil {
				return nil, errors.Join(err, ErrInvalidScopedContext)
			}

			input.Context = val.Clone()
		case KeywordLanguage:
			val, isNil, err := jsonutil.DecodeString(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidLanguageMapping)
			}

			input.Language = val
			input.flags.Set(flags(isNil, flagLanguage, flagNullLanguage))
		case KeywordDirection:
			val, isNil, err := jsonutil.DecodeString(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidBaseDirection)
			}

			input.Direction = val
			input.flags.Set(flags(isNil, flagDirection, flagNullDirection))
		case KeywordNest:
			val, _, err := jsonutil.DecodeString(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidNestValue)
			}

			input.Nest = val
		case KeywordPrefix:
			val, isNil, err := jsonutil.DecodeBool(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidPrefixValue)
			}

			if !isNil {
				input.Prefix = val
				input.flags.Set(flagPrefix)
			}
		case KeywordProtected:
			val, isNil, err := jsonutil.DecodeBool(dec)
			if err != nil {
				return nil, errors.Join(err, ErrInvalidProtectedValue)
			}

			if !isNil {
				input.Protected = val
				input.flags.Set(flagProtected)
			}
		default:
			if err := dec.SkipValue(); err != nil {
				return nil, err
			}
			input.flags.Set(flagUnknownKeys)
		}
	}

	tok, err := dec.ReadToken()
	if err != nil {
		return nil, err
	}

	if tok.Kind() != jsontext.KindEndObject {
		return nil, ErrInvalidTermDefinition
	}

	return &input, nil
}

func (p *Processor) handlePropagate(isNull bool) error {
	if isNull {
		return ErrInvalidPropagateValue
	}

	return nil
}

func (p *Processor) handleDirection(result *Context, dir string, isNull bool) error {
	if isNull {
		result.defaultDirection = ""
		return nil
	}

	switch dir {
	case DirectionLTR, DirectionRTL:
	default:
		return ErrInvalidBaseDirection
	}

	result.defaultDirection = dir
	return nil
}

func (p *Processor) handleLanguage(result *Context, lang string, isNull bool) error {
	if isNull {
		result.defaultLang = ""
		return nil
	}

	result.defaultLang = strings.ToLower(lang)
	return nil
}

func (p *Processor) handleVocab(ctx context.Context, result *Context, vocab string, isNull bool) error {
	// 5.8.2)
	if isNull {
		result.vocabMapping = ""
		return nil
	}

	// 5.8.3)
	if !(iri.IsAbsolute(vocab) || iri.IsRelative(vocab) || vocab == BlankNode) {
		return ErrInvalidVocabMapping
	}

	u, err := p.expandIRI(ctx, result, vocab, true, true, nil)
	if err != nil {
		return err
	}

	result.vocabMapping = u
	return nil
}

func (p *Processor) handleBase(result *Context, base string, isNull bool) error {
	// 5.7.2)
	if isNull {
		result.currentBaseIRI = ""
		return nil
	}

	// 5.7.3)
	if iri.IsAbsolute(base) {
		result.currentBaseIRI = base
		return nil
	}

	// 5.7.4)
	if iri.IsRelative(base) {
		u, err := iri.Resolve(result.currentBaseIRI, base)
		if err != nil {
			return ErrInvalidBaseIRI
		}
		result.currentBaseIRI = u
		return nil
	}

	// 5.7.5)
	return ErrInvalidBaseIRI
}

func (p *Processor) handleImport(
	ctx context.Context,
	baseURL string,
	uri string,
	terms map[string]*term,
) (map[string]*term, error) {
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
	res, err := p.retrieveRemoteContext(ctx, iri)
	if err != nil {
		return nil, err
	}

	if !jsonutil.IsMap(res.Context) {
		return nil, ErrInvalidRemoteContext
	}

	dec := jsontext.NewDecoder(bytes.NewBuffer(res.Context))
	tok, err := dec.ReadToken()
	if err != nil {
		return nil, ErrInvalidRemoteContext
	}
	if tok.Kind() != jsontext.KindBeginObject {
		return nil, ErrInvalidRemoteContext
	}

	importedTerms := make(map[string]*term)

	for dec.PeekKind() != jsontext.KindEndObject {
		tok, err := dec.ReadToken()
		if err != nil {
			return nil, ErrInvalidRemoteContext
		}

		if tok.Kind() != jsontext.KindString {
			return nil, ErrInvalidRemoteContext
		}

		key := tok.String()

		// 5.6.7) Check for nested @import
		if key == KeywordImport {
			return nil, ErrInvalidContextEntry
		}

		switch key {
		case KeywordVersion, KeywordBase, KeywordVocab,
			KeywordLanguage, KeywordDirection, KeywordPropagate, KeywordProtected:
			if err := dec.SkipValue(); err != nil {
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

	if _, err := dec.ReadToken(); err != nil {
		return nil, ErrInvalidRemoteContext
	}

	for k, v := range terms {
		importedTerms[k] = v
	}

	return importedTerms, nil
}

func (p *Processor) handleVersion(ver float64) error {
	if ver != 1.1 {
		return ErrInvalidVersionValue
	}

	return nil
}

func (p *Processor) retrieveRemoteContext(
	ctx context.Context,
	iri string,
) (Document, error) {
	// 5.2.4) 5.2.5) the document loader is expected to do the caching
	if p.loader == nil {
		return Document{}, fmt.Errorf("no loader %w", ErrLoadingRemoteContext)
	}
	doc, err := p.loader(ctx, iri)
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

	for _, key := range lctx.index[iri] {
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
