package longdistance

const (
	// BlankNode is the blank node prefix.
	BlankNode = "_:"
)

// Values for @direction.
const (
	DirectionLTR = "ltr"
	DirectionRTL = "rtl"
)

// JSON-LD MIME types and profiles.
const (
	ApplicationLDJSON = "application/ld+json"
	ApplicationJSON   = "application/json"

	ProfileExpanded  = "http://www.w3.org/ns/json-ld#expanded"
	ProfileCompacted = "http://www.w3.org/ns/json-ld#compacted"
	ProfileContext   = "http://www.w3.org/ns/json-ld#context"
	ProfileFlattened = "http://www.w3.org/ns/json-ld#flattened"
	ProfileFrame     = "http://www.w3.org/ns/json-ld#frame"
	ProfileFramed    = "http://www.w3.org/ns/json-ld#framed"
)
