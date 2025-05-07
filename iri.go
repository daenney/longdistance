package longdistance

import (
	"log/slog"
	"strings"

	"sourcery.dny.nu/longdistance/internal/json"
	"sourcery.dny.nu/longdistance/internal/url"
)

func (p *Processor) expandIRI(
	activeContext *Context,
	value string,
	relative bool,
	vocab bool,
	localContext map[string]json.RawMessage,
	defined map[string]*bool,
) (string, error) {
	// 1)
	if isKeyword(value) {
		return value, nil
	}

	// 2)
	if looksLikeKeyword(value) {
		p.logger.Warn("keyword lookalike value encountered",
			slog.String("value", value))
		// we can't generate a warning, so return nil
		// any empty values will be dropped
		return "", nil
	}

	hasLocal := len(localContext) > 0

	// 3)
	if hasLocal {
		if _, ok := localContext[value]; ok {
			if v := defined[value]; v == nil || !*v {
				if err := p.createTerm(
					activeContext,
					localContext,
					value,
					defined,
					newCreateTermOptions(),
				); err != nil {
					return "", err
				}
			}
		}
	}

	// 4)
	if activeContext != nil {
		if t, ok := activeContext.defs[value]; ok {
			if isKeyword(t.IRI) {
				return t.IRI, nil
			}
		}
	}

	// 5)
	if vocab {
		if activeContext != nil {
			if t, ok := activeContext.defs[value]; ok {
				return t.IRI, nil
			}
		}
	}

	// 6)
	if strings.Index(value, ":") >= 1 {
		// 6.1)
		prefix, suffix, found := strings.Cut(value, ":")
		if found {
			// 6.2)
			if prefix == "_" || strings.HasPrefix(suffix, "//") {
				return value, nil
			}

			// 6.3)
			if hasLocal {
				if _, ok := localContext[prefix]; ok {
					if v := defined[prefix]; v == nil || !*v {
						if err := p.createTerm(
							activeContext,
							localContext,
							prefix,
							defined,
							newCreateTermOptions(),
						); err != nil {
							return "", err
						}
					}
				}
			}

			// 6.4)
			if activeContext != nil {
				if t, ok := activeContext.defs[prefix]; ok && t.IRI != "" && t.Prefix {
					return t.IRI + suffix, nil
				}
			}

			// 6.5)
			if url.IsIRI(value) {
				return value, nil
			}
		}
	}

	// 7)
	if vocab {
		if activeContext.vocabMapping != "" {
			return activeContext.vocabMapping + value, nil
		}
	}

	// 8)
	if relative {
		if activeContext.currentBaseIRI == "" {
			return value, nil
		}
		u, err := url.Resolve(activeContext.currentBaseIRI, value)
		if err != nil {
			return "", err
		}
		return u, nil
	}

	return value, nil
}
