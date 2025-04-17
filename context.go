package longdistance

import (
	"context"
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"

	"code.dny.dev/longdistance/internal/json"
	"code.dny.dev/longdistance/internal/url"
)

// RemoteContextLimit is the recursion limit for resolving remote contexts.
const RemoteContextLimit = 10

// Context represents a processed JSON-LD context.
type Context struct {
	defs            map[string]Term
	protected       map[string]struct{}
	currentBaseIRI  string
	originalBaseIRI string

	vocabMapping     string
	defaultLang      string
	defaultDirection string
	previousContext  *Context
	inverse          inverseContext
}

// newContext initialises a new context with the specified documentURL set as
// the current and original base IRI.
func newContext(documentURL string) *Context {
	return &Context{
		defs:             make(map[string]Term),
		protected:        make(map[string]struct{}),
		defaultLang:      "",
		defaultDirection: "",
		previousContext:  nil,
		inverse:          nil,
		currentBaseIRI:   documentURL,
		originalBaseIRI:  documentURL,
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

func (c *Context) initInverse() {
	if c.inverse == nil {
		c.inverse = workIt(c)
	}
}

func (c *Context) clone() *Context {
	return &Context{
		defs:             maps.Clone(c.defs),
		protected:        maps.Clone(c.protected),
		currentBaseIRI:   c.currentBaseIRI,
		originalBaseIRI:  c.originalBaseIRI,
		vocabMapping:     c.vocabMapping,
		defaultLang:      c.defaultLang,
		defaultDirection: c.defaultDirection,
		previousContext:  c.previousContext,
		inverse:          nil,
	}
}

// Context takes in JSON and parses it into a [Context].
func (p *Processor) Context(localContext json.RawMessage, baseURL string) (*Context, error) {
	if len(localContext) == 0 {
		return nil, nil
	}

	if json.IsNull(localContext) {
		return nil, nil
	}

	return p.context(nil, localContext, baseURL, newCtxProcessingOpts())
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
	activeContext *Context,
	localContext json.RawMessage,
	baseURL string,
	opts ctxProcessingOpts,
) (*Context, error) {
	if activeContext == nil {
		activeContext = newContext(baseURL)
	}

	if p.baseIRI != "" {
		activeContext.currentBaseIRI = p.baseIRI
	}

	// 1)
	result := activeContext.clone()

	// 2)
	if json.IsMap(localContext) {
		var propcheck struct {
			Propagate *bool `json:"@propagate,omitempty"`
		}
		if err := json.Unmarshal(localContext, &propcheck); err != nil {
			return nil, ErrInvalidPropagateValue
		}
		if propcheck.Propagate != nil {
			opts.propagate = *propcheck.Propagate
		}
	}

	// 3)
	if !opts.propagate {
		if result.previousContext == nil {
			result.previousContext = activeContext.clone()
		}
	}

	// 4)
	localContext = json.MakeArray(localContext)

	var contexts []json.RawMessage
	if err := json.Unmarshal(localContext, &contexts); err != nil {
		return nil, fmt.Errorf("invalid context document")
	}

	if len(contexts) == 0 {
		return nil, nil
	}

	// 5)
	for _, context := range contexts {
		// 5.1)
		switch context[0] {
		case '[':
			return nil, ErrInvalidLocalContext
		case '{':
			// goes on after the switch
		default:
			var s Null[string]
			if err := json.Unmarshal(context, &s); err != nil {
				return nil, ErrInvalidLocalContext
			}

			// 5.1)
			if s.Null {
				// 5.1.1)
				if !opts.override && len(result.protected) != 0 {
					return nil, ErrInvalidContextNullificaton
				}

				// 5.1.2)
				previous := result.clone()
				result = newContext(activeContext.originalBaseIRI)
				if !opts.propagate {
					result.previousContext = previous
				}

				// 5.1.3)
				continue
			}

			// 5.2)
			// 5.2.1)
			if !url.IsIRI(baseURL) && !url.IsIRI(s.Value) {
				return nil, ErrLoadingDocument
			}

			iri, err := url.Resolve(baseURL, s.Value)
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

			// 5.2.4) 5.2.5)
			doc, err := p.retrieveRemoteContext(iri)
			if err != nil {
				return nil, err
			}

			// 5.2.6)
			newOpts := newCtxProcessingOpts()
			newOpts.remotes = slices.Clone(opts.remotes)
			newOpts.validate = opts.validate
			res, err := p.context(
				result,
				doc.Context,
				doc.URL,
				newOpts,
			)
			if err != nil {
				return nil, err
			}
			result = res
			continue
		}

		// 5.3)
		var ctxObj map[string]json.RawMessage
		if err := json.Unmarshal(context, &ctxObj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal context: %s %w", err, ErrInvalidLocalContext)
		}

		// 5.5)
		if version, ok := ctxObj[KeywordVersion]; ok {
			if err := p.handleVersion(version); err != nil {
				return nil, err
			}
		}

		// 5.6)
		if imp, ok := ctxObj[KeywordImport]; ok {
			res, err := p.handleImport(baseURL, imp, ctxObj)
			if err != nil {
				return nil, err
			}
			ctxObj = res
		}

		// 5.7)
		if base, ok := ctxObj[KeywordBase]; ok && len(opts.remotes) == 0 {
			if err := p.handleBase(result, base); err != nil {
				return nil, err
			}
		}

		// 5.8)
		if vocab, ok := ctxObj[KeywordVocab]; ok {
			if err := p.handleVocab(result, vocab); err != nil {
				return nil, err
			}
		}

		// 5.9)
		if lang, ok := ctxObj[KeywordLanguage]; ok {
			if err := p.handleLanguage(result, lang); err != nil {
				return nil, err
			}
		}

		// 5.10)
		if dir, ok := ctxObj[KeywordDirection]; ok {
			if err := p.handleDirection(result, dir); err != nil {
				return nil, err
			}
		}

		// 5.11)
		if prop, ok := ctxObj[KeywordPropagate]; ok {
			if err := p.handlePropagate(prop); err != nil {
				return nil, err
			}
		}

		protected := false
		if prot, ok := ctxObj[KeywordProtected]; ok {
			var b Null[bool]
			if err := json.Unmarshal(prot, &b); err != nil {
				return nil, ErrInvalidProtectedValue
			}
			if !b.Null {
				protected = b.Value
			}
		}

		// 5.12)
		defined := map[string]*bool{}

		// 5.13)
		for k := range ctxObj {
			switch k {
			case KeywordBase, KeywordDirection, KeywordImport,
				KeywordLanguage, KeywordPropagate, KeywordProtected,
				KeywordVersion, KeywordVocab:
			default:
				newOpts := newCreateTermOptions()
				newOpts.baseURL = baseURL
				newOpts.protected = protected
				newOpts.override = opts.override
				newOpts.remotes = slices.Clone(opts.remotes)
				if err := p.createTerm(
					result,
					ctxObj,
					k,
					defined,
					newOpts,
				); err != nil {
					return nil, err
				}
			}
		}
	}

	return result, nil
}

func (p *Processor) handlePropagate(prop json.RawMessage) error {
	if p.modeLD10 {
		return ErrInvalidContextEntry
	}
	var b Null[bool]
	if err := json.Unmarshal(prop, &b); err != nil {
		return ErrInvalidPropagateValue
	}

	if b.Null {
		return ErrInvalidPropagateValue
	}

	return nil
}

func (p *Processor) handleDirection(result *Context, dir json.RawMessage) error {
	if p.modeLD10 {
		return ErrInvalidContextEntry
	}

	var d Null[string]
	if err := json.Unmarshal(dir, &d); err != nil {
		return ErrInvalidBaseDirection
	}

	if d.Null {
		result.defaultDirection = ""
	} else {
		switch d.Value {
		case DirectionLTR, DirectionRTL:
		default:
			return ErrInvalidBaseDirection
		}
		result.defaultDirection = d.Value
	}

	return nil
}

func (p *Processor) handleLanguage(result *Context, lang json.RawMessage) error {
	var l Null[string]
	if err := json.Unmarshal(lang, &l); err != nil {
		return ErrInvalidDefaultLanguage
	}
	if !l.Null {
		result.defaultLang = strings.ToLower(l.Value)
	}
	if l.Null {
		result.defaultLang = ""
	}
	return nil
}

func (p *Processor) handleVocab(result *Context, vocab json.RawMessage) error {
	// 5.8.2)
	var s Null[string]
	if err := json.Unmarshal(vocab, &s); err != nil {
		return ErrInvalidVocabMapping
	}

	if s.Null {
		result.vocabMapping = ""
		return nil
	}

	// 5.8.3)
	if !(url.IsIRI(s.Value) || url.IsRelative(s.Value) || s.Value == BlankNode) {
		return ErrInvalidVocabMapping
	}

	u, err := p.expandIRI(result, s.Value, true, true, nil, nil)
	if err != nil {
		return err
	}

	result.vocabMapping = u
	return nil
}

func (p *Processor) handleBase(result *Context, base json.RawMessage) error {
	// 5.7.2)
	var iri Null[string]
	if err := json.Unmarshal(base, &iri); err != nil {
		return ErrInvalidBaseIRI
	}

	if iri.Null {
		result.currentBaseIRI = ""
		return nil
	}

	// 5.7.3)
	if url.IsIRI(iri.Value) {
		result.currentBaseIRI = iri.Value
		return nil
	}

	// 5.7.4)
	if url.IsRelative(iri.Value) {
		u, err := url.Resolve(result.currentBaseIRI, iri.Value)
		if err != nil {
			return ErrInvalidBaseIRI
		}
		result.currentBaseIRI = u
		return nil
	}

	// 5.7.5)
	return ErrInvalidBaseIRI
}

func (p *Processor) handleImport(baseURL string, data json.RawMessage, context map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	// 5.6.1)
	if p.modeLD10 {
		return nil, ErrInvalidContextEntry
	}

	// 5.6.2)
	var val string
	if err := json.Unmarshal(data, &val); err != nil {
		return nil, ErrInvalidImportValue
	}

	// 5.6.3)
	iri, err := url.Resolve(baseURL, val)
	if err != nil {
		return nil, ErrInvalidRemoteContext
	}

	// 5.6.4) 5.6.5)
	res, err := p.retrieveRemoteContext(iri)
	if err != nil {
		return nil, err
	}

	// 5.6.6)
	var ctxObj map[string]json.RawMessage
	if err := json.Unmarshal(res.Context, &ctxObj); err != nil {
		return nil, ErrInvalidRemoteContext
	}

	// 5.6.7)
	if _, ok := ctxObj[KeywordImport]; ok {
		return nil, ErrInvalidContextEntry
	}

	maps.Copy(ctxObj, context)
	return ctxObj, nil
}

func (p *Processor) handleVersion(data json.RawMessage) error {
	var ver float64
	if err := json.Unmarshal(data, &ver); err != nil {
		return ErrInvalidVersionValue
	}
	if ver != 1.1 {
		return ErrInvalidVersionValue
	}
	if p.modeLD10 {
		return ErrProcessingMode
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

type inverseContext map[string]map[string]mapping

type mapping struct {
	Language map[string]string
	Type     map[string]string
	Any      map[string]string
}

// workIt flips a context and reverses it
//
// ​ti esrever dna ti pilf ,nwod gniht ym tuP
func workIt(activeContext *Context) inverseContext {
	// 1)
	result := inverseContext{}

	// 2)
	defaultLang := KeywordNone
	if activeContext.defaultLang != "" {
		defaultLang = strings.ToLower(activeContext.defaultLang)
	}

	// 3)
	terms := slices.Collect(maps.Keys(activeContext.defs))
	slices.SortFunc(terms, sortedLeast)

	for _, key := range terms {
		def := activeContext.defs[key]
		// 3.1)
		if def.IsZero() {
			continue
		}

		// 3.2)
		container := KeywordNone
		if def.Container != nil {
			dc := slices.Clone(def.Container)
			slices.Sort(dc)
			container = strings.Join(dc, "")
		}

		// 3.3)
		vvar := def.IRI

		// 3.4)
		if _, ok := result[vvar]; !ok {
			result[vvar] = map[string]mapping{}
		}

		// 3.5)
		containerMap := result[vvar]

		// 3.6)
		if _, ok := containerMap[container]; !ok {
			containerMap[container] = mapping{
				Language: map[string]string{},
				Type:     map[string]string{},
				Any: map[string]string{
					KeywordNone: key,
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
		} else if def.Type != "" {
			if def.Type == KeywordNone {
				// 3.11)
				if _, ok := langMap[KeywordAny]; !ok {
					// 3.11.1)
					langMap[KeywordAny] = key
				}
				if _, ok := typeMap[KeywordAny]; !ok {
					// 3.11.2)
					typeMap[KeywordAny] = key
				}
			} else {
				// 3.12)
				if _, ok := typeMap[def.Type]; !ok {
					// 3.12.1
					typeMap[def.Type] = key
				}
			}
		} else if def.Language.Set || def.Direction.Set {
			if def.Language.Set && def.Direction.Set {
				// 3.13)
				// 3.13.1) + 3.13.5)
				langDir := KeywordNone
				if !def.Language.Null && !def.Direction.Null {
					// 3.13.2)
					langDir = strings.ToLower(def.Language.Value) + "_" + def.Direction.Value
				} else if !def.Language.Null {
					// 3.13.3)
					langDir = strings.ToLower(def.Language.Value)
				} else if !def.Direction.Null {
					// 3.13.4)
					langDir = "_" + def.Direction.Value
				}
				// 3.13.6)
				if _, ok := langMap[langDir]; !ok {
					langMap[langDir] = key
				}
			} else if def.Language.Set {
				// 3.14)
				lang := KeywordNull
				if !def.Language.Null {
					lang = strings.ToLower(def.Language.Value)
				}
				if _, ok := langMap[lang]; !ok {
					langMap[lang] = key
				}
			} else if def.Direction.Set {
				// 3.15)
				dir := KeywordNone
				if !def.Direction.Null {
					dir = "_" + def.Direction.Value
				}
				if _, ok := langMap[dir]; !ok {
					langMap[dir] = key
				}
			}
		} else if activeContext.defaultDirection != "" {
			// 3.16)
			langDir := strings.ToLower(defaultLang) + "_" + activeContext.defaultDirection
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

	return result
}
