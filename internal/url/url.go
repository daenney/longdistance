package url

import (
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"
)

var Parse = url.Parse

func Relative(base string, iri string) (string, error) {
	baseURL, err := Parse(base)
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %w", err)
	}

	absURL, err := Parse(iri)
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

	relpaths := make([]string, 0, len(baseParts)-prefix)
	for range baseParts[prefix+1:] {
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
	delims := []string{":", "/", "?", "#", "[", "]", "@"}
	last := s[len(s)-1:]
	return slices.Contains(delims, last)
}

func IsRelative(s string) bool {
	_, err := Parse(s)
	return err == nil
}

func IsIRI(s string) bool {
	u, err := Parse(s)
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
	r, err := Parse(val)
	if err != nil {
		return "", err
	}

	u, err := Parse(base)
	if err != nil {
		return "", err
	}

	return u.ResolveReference(r).String(), nil
}
