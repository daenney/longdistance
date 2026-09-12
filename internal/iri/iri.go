package iri

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

func Relative(base string, iri string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %w", err)
	}

	absURL, err := url.Parse(iri)
	if err != nil {
		return "", fmt.Errorf("failed to parse absolute URL: %w", err)
	}

	if baseURL.Scheme != absURL.Scheme || baseURL.Host != absURL.Host {
		return "", fmt.Errorf("cannot create relative URL when host or scheme differ")
	}

	basePath := baseURL.EscapedPath()
	absPath := absURL.EscapedPath()
	if basePath == absPath {
		if absURL.Fragment != "" || absURL.RawQuery != "" {
			return (&url.URL{
				RawQuery: absURL.RawQuery,
				Fragment: absURL.Fragment,
			}).String(), nil
		}
	}

	last := strings.LastIndex(basePath, "/")
	basePath = basePath[:last+1]
	baseParts := strings.Split(basePath, "/")
	absParts := strings.Split(absPath, "/")

	prefix := 0
	lap := len(absParts)
	count := min(len(baseParts), lap)
	for i, elem := range baseParts[:count] {
		if elem == absParts[i] {
			prefix++
		} else {
			break
		}
	}

	n := max(0, len(baseParts)-prefix-1)
	relpaths := make([]string, 0, n+len(absParts)-prefix)
	for range n {
		relpaths = append(relpaths, "..")
	}

	relpaths = append(relpaths, absParts[prefix:]...)
	final := path.Join(relpaths...)

	// Include query and fragment if present
	relURL := &url.URL{
		Path:     final,
		RawQuery: absURL.RawQuery,
		Fragment: absURL.Fragment,
	}

	res := relURL.String()
	if strings.HasSuffix(res, "..") {
		res = res + "/"
	}

	return res, nil
}

func EndsInGenDelim(s string) bool {
	if len(s) == 0 {
		return false
	}

	switch s[len(s)-1:] {
	case ":", "/", "?", "#", "[", "]", "@":
		return true
	default:
		return false
	}
}

func IsRelative(s string) bool {
	if absolute(s) {
		return false
	}

	u, err := url.Parse(s)
	return err == nil && !u.IsAbs()
}

func IsAbsolute(s string) bool {
	if absolute(s) {
		return true
	}

	u, err := url.Parse(s)
	return err == nil &&
		u.IsAbs() &&
		(u.RawPath == "" || u.RawPath == u.EscapedPath()) &&
		(u.RawFragment == "" || u.RawFragment == u.EscapedFragment())
}

func Resolve(base string, val string) (string, error) {
	r, err := url.Parse(val)
	if err != nil {
		return "", err
	}

	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	return u.ResolveReference(r).String(), nil
}

const (
	iriSchemeStart uint8 = 1 << iota // ALPHA
	iriScheme                        // ALPHA / DIGIT / "+" / "-" / "."
	iriHost                          // ALPHA / DIGIT "-" / "."
	iriTail                          // Characters that don't need URL encoding in path
)

var iriClass [256]uint8

func init() {
	for c := range 256 {
		b := byte(c)
		alpha := (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
		digit := b >= '0' && b <= '9'

		var m uint8
		if alpha {
			m |= iriSchemeStart | iriScheme | iriHost | iriTail
		}

		if digit {
			m |= iriScheme | iriHost | iriTail
		}

		switch b {
		case '-', '.':
			m |= iriScheme | iriHost | iriTail
		case '+':
			m |= iriScheme | iriTail
		case '_', '~', '/', ':', '?', '#', '@', '!', '$',
			'&', '\'', '(', ')', '*', ',', ';', '=':
			m |= iriTail
		}

		iriClass[b] = m
	}
}

// absolute speedruns detecting if an IRI is absolute
//
// The JSON-LD spec requires that IRIs are well-formed. When it comes to absolute IRIs
// the format we encounter 99.999% of the time is: scheme://host/path#fragment.
// This can be checked quickly without paying the cost of [url.Parse].
//
// It returns false if it cannot determine whether we're dealing with an absolute IRI. It
// may still be an absolute IRI, but it needs a full [url.Parse] to check.
func absolute(s string) bool {
	if len(s) == 0 || iriClass[s[0]]&iriSchemeStart == 0 {
		return false
	}

	i := 1

	// loop until we hit a character that's not allowed in scheme
	// this should be the ':'
	for i < len(s) && iriClass[s[i]]&iriScheme != 0 {
		i++
	}

	if i == len(s) || s[i] != ':' { // we don't have a scheme
		return false
	}

	i++ // move past the :

	// only handle scheme://, scheme:something falls back to url.Parse
	rest, found := strings.CutPrefix(s[i:], "//")
	if !found {
		return false
	}

	// now we need to deal with the authority section.
	// only handle the simple case:
	//   - hostname or IPv4 literal.
	//   - tail starts at the first /, ? or # and must not include characters
	//     that should be URL encoded.
	// everything else gets passed to url.Parse

	i = 0
	for i < len(rest) {
		c := rest[i]

		// we're done with the authority once we hit the tail
		// that should be the first path separater, query or fragment
		if c == '/' || c == '?' || c == '#' {
			break
		}

		// we found a character that's not allowed in the authority
		if iriClass[c]&iriHost == 0 {
			return false
		}

		i++
	}

	if i == 0 { // empty authority
		return false
	}

	fragment := false
	for i < len(rest) {
		c := rest[i]

		// we found a character that should be URL encoded
		if iriClass[c]&iriTail == 0 {
			return false
		}

		if c == '#' { // # is only valid the first time, to indicate the fragment
			if fragment {
				return false
			}

			fragment = true
		}

		i++
	}

	return true
}
