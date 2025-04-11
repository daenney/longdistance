package url

import (
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
)

func Parse(raw string) (*url.URL, error) {
	return url.Parse(raw)
}

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

	basePath := baseURL.Path
	if !strings.HasSuffix(basePath, "/") {
		basePath = filepath.Dir(basePath) + "/"
	}

	if baseURL.Path == absURL.Path {
		if absURL.Fragment != "" || absURL.RawQuery != "" {
			relURL := &url.URL{
				RawQuery: absURL.RawQuery,
				Fragment: absURL.Fragment,
			}
			return relURL.String(), nil
		}

		last := filepath.Base(absURL.Path)
		if last != "/" {
			return last, nil
		}
		return "./", nil
	}

	// it's a bit whack to use filepath functions here but
	// net/url lacks them and it works well enough
	relPath, err := filepath.Rel(basePath, absURL.Path)
	if err != nil {
		return "", fmt.Errorf("failed to compute relative path: %w", err)
	}
	relPath = filepath.ToSlash(relPath)

	// Include query and fragment if present
	relURL := &url.URL{
		Path:     relPath,
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
	delims := []string{":", "/", "?", "#", "[", "]", "@"}
	last := s[len(s)-1:]
	return slices.Contains(delims, last)
}

func IsRelative(s string) bool {
	_, err := url.Parse(s)
	return err == nil
}

func IsIRI(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}

	ns := u.String()
	if strings.HasSuffix(s, "#") {
		// preserve the empty fragment
		ns = ns + "#"
	}

	return u.IsAbs() && s == ns
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
