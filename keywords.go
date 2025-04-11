package longdistance

// JSON-LD keywords.
const (
	KeywordAny       = "@any"
	KeywordBase      = "@base"
	KeywordContainer = "@container"
	KeywordContext   = "@context"
	KeywordDefault   = "@default"
	KeywordDirection = "@direction"
	KeywordGraph     = "@graph"
	KeywordID        = "@id"
	KeywordImport    = "@import"
	KeywordIncluded  = "@included"
	KeywordIndex     = "@index"
	KeywordJSON      = "@json"
	KeywordLanguage  = "@language"
	KeywordList      = "@list"
	KeywordNest      = "@nest"
	KeywordNone      = "@none"
	KeywordNull      = "@null"
	KeywordPrefix    = "@prefix"
	KeywordPreserve  = "@preserve"
	KeywordPropagate = "@propagate"
	KeywordProtected = "@protected"
	KeywordReverse   = "@reverse"
	KeywordSet       = "@set"
	KeywordType      = "@type"
	KeywordValue     = "@value"
	KeywordVersion   = "@version"
	KeywordVocab     = "@vocab"
)

// isKeyword returns if the string matches a known JSON-LD keyword.
func isKeyword(s string) bool {
	switch s {
	case KeywordBase,
		KeywordContainer,
		KeywordContext,
		KeywordDefault,
		KeywordDirection,
		KeywordGraph,
		KeywordID,
		KeywordImport,
		KeywordIncluded,
		KeywordIndex,
		KeywordJSON,
		KeywordLanguage,
		KeywordList,
		KeywordNest,
		KeywordNone,
		KeywordPrefix,
		KeywordPreserve,
		KeywordPropagate,
		KeywordProtected,
		KeywordReverse,
		KeywordSet,
		KeywordType,
		KeywordValue,
		KeywordVersion,
		KeywordVocab:
		return true
	default:
		return false
	}
}

// looksLikeKeyword determines if a string has the general shape of a JSON-LD
// keyword.
//
// It returns true for strings of the form: "@[alpha]".
//
// This means that a string like @blabla1 will return false, but it's still
// strongly recommended against using those for keys just to avoid confusion.
func looksLikeKeyword(s string) bool {
	if s == "" {
		return false
	}

	if s == "@" {
		return false
	}

	if s[0] != '@' {
		return false
	}

	for _, char := range s[1:] {
		if (char < 'a' || char > 'z') &&
			(char < 'A' || char > 'Z') {
			return false
		}
	}

	return true
}
