package longdistance

import (
	"encoding/json"
	"log/slog"
)

// ProcessorOption can be used to customise the behaviour of a [Processor].
type ProcessorOption func(*Processor)

// Processor represents a JSON-LD processor.
//
// Your application should only ever need one of them. Do not create a new one
// for each request you're handling.
//
// Create one with [NewProcessor] and pass any [ProcessorOption] to configure
// the processor.
type Processor struct {
	modeLD10                  bool
	ordered                   bool
	baseIRI                   string
	compactArrays             bool
	compactToRelative         bool
	loader                    RemoteContextLoaderFunc
	logger                    *slog.Logger
	expandContext             json.RawMessage
	excludeIRIsFromCompaction []string
	remapPrefixIRIs           map[string]string
	validateContextFunc       ValidateContextFunc
	processedContext          map[string]*Context
}

// NewProcessor creates a new JSON-LD processor.
//
// By default:
//   - Processing mode is JSON-LD 1.1. This can handle both JSON-LD 1.0 and
//     JSON-LD 1.1 documents. To switch to JSON-LD 1.0 only, configure it with
//     [With10Processing].
//   - No loader is configured. Without one, remote contexts as well as @import
//     contexts cannot be processed. Set it with [WithRemoteContextLoader].
//   - Arrays are compacted. Change it with [WithCompactArrays].
//   - IRIs can compact to relative IRIs. Change it with
//     [WithCompactToRelative].
//   - Logger is [slog.DiscardHandler]. Set it with [WithLogger]. The logger is
//     only used to emit warnings.
func NewProcessor(options ...ProcessorOption) *Processor {
	p := &Processor{
		compactArrays:     true,
		compactToRelative: true,
		logger:            slog.New(slog.DiscardHandler),
	}

	for _, opt := range options {
		opt(p)
	}

	if p.expandContext != nil {
		p.processedContext = nil
	}

	return p
}

// With10Processing sets the processing mode to json-ld-1.0.
func With10Processing(b bool) ProcessorOption {
	return func(p *Processor) {
		p.modeLD10 = b
	}
}

// WithRemoteContextLoader sets the context loader function.
func WithRemoteContextLoader(l RemoteContextLoaderFunc) ProcessorOption {
	return func(p *Processor) {
		p.loader = l
	}
}

// WithLogger sets the logger that'll be used to emit warnings during
// processing.
//
// Without a logger no warnings will be emitted when keyword lookalikes are
// encountered that are ignored.
func WithLogger(l *slog.Logger) ProcessorOption {
	return func(p *Processor) {
		p.logger = l
	}
}

// WithOrdered ensures that object elements and language maps are processed in
// lexicographical order.
//
// This is typically not needed, but helps to stabilise the test suite.
func WithOrdered(b bool) ProcessorOption {
	return func(p *Processor) {
		p.ordered = b
	}
}

// WithBaseIRI sets an explicit base IRI to use.
func WithBaseIRI(iri string) ProcessorOption {
	return func(p *Processor) {
		p.baseIRI = iri
	}
}

// WithCompactArrays sets whether single-valued arrays should
// be reduced to their value where possible.
func WithCompactArrays(b bool) ProcessorOption {
	return func(p *Processor) {
		p.compactArrays = b
	}
}

// WithCompactToRelative sets whether IRIs can be transformed into
// relative IRIs during IRI compaction.
func WithCompactToRelative(b bool) ProcessorOption {
	return func(p *Processor) {
		p.compactToRelative = b
	}
}

// WithExpandContext provides an additional out-of-band context
// that's used during expansion.
func WithExpandContext(ctx json.RawMessage) ProcessorOption {
	return func(p *Processor) {
		p.expandContext = ctx
	}
}

// WithExcludeIRIsFromCompaction disables IRI compaction for the specified IRIs.
func WithExcludeIRIsFromCompaction(iri ...string) ProcessorOption {
	return func(p *Processor) {
		p.excludeIRIsFromCompaction = iri
	}
}

// WithRemapPrefixIRIs can remap a prefix IRI during context processing.
//
// Prefixes are only remapped for an exact match.
//
// This is useful to remap the incorrect schema.org# to schema.org/.
func WithRemapPrefixIRIs(old, new string) ProcessorOption {
	return func(p *Processor) {
		if p.remapPrefixIRIs == nil {
			p.remapPrefixIRIs = make(map[string]string, 2)
		}
		p.remapPrefixIRIs[old] = new
	}
}

type ValidateContextFunc func(*Context) bool

// WithValidateContext sets the function that will be used to validate the
// context after it's been processed.
//
// This can be used in situations where both JSON-LD aware and JSON-LD unaware
// processors will process the same message. It can be used to protect term
// definitions from an unprotected normative context to avoid semantic confusion
// for JSON-LD unaware processors.
func WithValidateContext(f ValidateContextFunc) ProcessorOption {
	return func(p *Processor) {
		p.validateContextFunc = f
	}
}

// WithProcessedContext stores the processed context for an IRI.
//
// It's used to initiate the context if and only if:
//   - No terms have been defined on the context yet.
//   - The first, or only, entry in the document's @context is a remote context.
//
// This can be used to amortise the cost of the initial context processing when
// handling documents that all share a well-known remote context. Any additional
// contexts will be processed normally.
//
// This has no benefit if [WithExpandContext] is used, as in that case terms are
// already defined on the context before any remote contexts are retrieved.
func WithProcessedContext(iri string, ctx *Context) ProcessorOption {
	return func(p *Processor) {
		if p.processedContext == nil {
			p.processedContext = make(map[string]*Context, 2)
		}
		p.processedContext[iri] = ctx
	}
}
