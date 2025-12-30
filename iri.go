package longdistance

import (
	"log/slog"
	"strings"

	"sourcery.dny.nu/longdistance/internal/iri"
)

func (p *Processor) expandIRI(
	activeCtx *Context,
	value string,
	relative bool,
	vocab bool,
	localCtx map[string]term,
	defined map[string]termState,
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

	hasLocal := len(localCtx) > 0

	// 3)
	if hasLocal {
		if _, ok := localCtx[value]; ok {
			if state := defined[value]; state != termDefined {
				if err := p.createTerm(
					activeCtx,
					localCtx,
					value,
					defined,
					newCreateTermOptions(),
				); err != nil {
					return "", err
				}
			}
		}
	}

	// 4) 5)
	if activeCtx != nil {
		if t, ok := activeCtx.defs[value]; ok {
			if isKeyword(t.IRI) || vocab {
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
				if _, ok := localCtx[prefix]; ok {
					if state := defined[prefix]; state != termDefined {
						if err := p.createTerm(
							activeCtx,
							localCtx,
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
			if activeCtx != nil {
				if t, ok := activeCtx.defs[prefix]; ok && t.IRI != "" && t.Prefix {
					return t.IRI + suffix, nil
				}
			}

			// 6.5)
			if iri.IsAbsolute(value) {
				return value, nil
			}
		}
	}

	// 7)
	if vocab {
		if activeCtx.vocabMapping != "" {
			return activeCtx.vocabMapping + value, nil
		}
	}

	// 8)
	if relative {
		if activeCtx.currentBaseIRI == "" {
			return value, nil
		}
		u, err := iri.Resolve(activeCtx.currentBaseIRI, value)
		if err != nil {
			return "", err
		}
		return u, nil
	}

	return value, nil
}
