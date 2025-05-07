package longdistance

import (
	"context"

	"sourcery.dny.nu/longdistance/internal/json"
)

// RemoteContextLoaderFunc is called to retrieve a remote context.
//
// It returns a Document, and an error in case retrieval failed.
//
// When building your own loader, please remember that:
//   - [Document.URL] is the URL the context was retrieved from after having
//     followed any redirects.
//   - [Document.Context] is the value of the [KeywordContext] in the returned
//     document, or the empty JSON map if the context was absent.
//   - Request a context with [ApplicationLDJSON] and profile [ProfileContext].
//     You can use [mime.FormatMediaType] to build the value for the Accept
//     header.
//   - Have proper timeouts, retry handling and request deduplication.
//   - Make sure to cache the resulting [Document] to avoid unnecessary future
//     requests. Contexts should not change for the lifetime of the application.
type RemoteContextLoaderFunc func(context.Context, string) (Document, error)

// Document holds a retrieved context.
//
//   - URL holds the final URL a context was retrieved from, after following
//     redirects.
//   - Context holds the value of the @context element, or the empty map.
type Document struct {
	URL     string
	Context json.RawMessage
}
