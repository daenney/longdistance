package longdistance_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	ld "sourcery.dny.nu/longdistance"
)

// TestExpand runs the W3C expansion tests.
//
// Despite the fact that this is the compaction test suite, many of the
// inputs are in compacted or partially expanded form. So all inputs
// have to be expanded first, before we attempt compaction.
func TestExpand(t *testing.T) {
	// some tests are marked with ordered: true in order to stabilise
	// order so we can diff with simple JSON comparison
	tests := []struct {
		id, name                                    string
		ld10, ld11                                  bool
		input, context, output, base, expandContext string
		err                                         string
	}{
		{id: "t0001", name: "drop free-floating nodes", ld10: true, ld11: true, input: "w3c/expand/0001-in.jsonld", output: "w3c/expand/0001-out.jsonld"},
		{id: "t0002", name: "basic", ld10: true, ld11: true, input: "w3c/expand/0002-in.jsonld", output: "w3c/expand/0002-out.jsonld"},
		{id: "t0003", name: "drop null and unmapped properties", ld10: true, ld11: true, input: "w3c/expand/0003-in.jsonld", output: "w3c/expand/0003-out.jsonld"},
		{id: "t0004", name: "optimize @set, keep empty arrays", ld10: true, ld11: true, input: "w3c/expand/0004-in.jsonld", output: "w3c/expand/0004-out.jsonld"},
		{id: "t0005", name: "do not expand aliased @id/@type", ld10: true, ld11: true, input: "w3c/expand/0005-in.jsonld", output: "w3c/expand/0005-out.jsonld"},
		{id: "t0006", name: "alias keywords", ld10: true, ld11: true, input: "w3c/expand/0006-in.jsonld", output: "w3c/expand/0006-out.jsonld"},
		{id: "t0007", name: "date type-coercion", ld10: true, ld11: true, input: "w3c/expand/0007-in.jsonld", output: "w3c/expand/0007-out.jsonld"},
		{id: "t0008", name: "@value with @language", ld10: true, ld11: true, input: "w3c/expand/0008-in.jsonld", output: "w3c/expand/0008-out.jsonld"},
		{id: "t0009", name: "@graph with terms", ld10: true, ld11: true, input: "w3c/expand/0009-in.jsonld", output: "w3c/expand/0009-out.jsonld"},
		{id: "t0010", name: "native types", ld10: true, ld11: true, input: "w3c/expand/0010-in.jsonld", output: "w3c/expand/0010-out.jsonld"},
		{id: "t0011", name: "coerced @id", ld10: true, ld11: true, input: "w3c/expand/0011-in.jsonld", output: "w3c/expand/0011-out.jsonld"},
		{id: "t0012", name: "@graph with embed", ld10: true, ld11: true, input: "w3c/expand/0012-in.jsonld", output: "w3c/expand/0012-out.jsonld"},
		{id: "t0013", name: "expand already expanded", ld10: true, ld11: true, input: "w3c/expand/0013-in.jsonld", output: "w3c/expand/0013-out.jsonld"},
		{id: "t0014", name: "@set of @value objects with keyword aliases", ld10: true, ld11: true, input: "w3c/expand/0014-in.jsonld", output: "w3c/expand/0014-out.jsonld"},
		{id: "t0015", name: "collapse set of sets, keep empty lists", ld10: true, ld11: true, input: "w3c/expand/0015-in.jsonld", output: "w3c/expand/0015-out.jsonld"},
		{id: "t0016", name: "context reset", ld10: true, ld11: true, input: "w3c/expand/0016-in.jsonld", output: "w3c/expand/0016-out.jsonld"},
		{id: "t0017", name: "@graph and @id aliased", ld10: true, ld11: true, input: "w3c/expand/0017-in.jsonld", output: "w3c/expand/0017-out.jsonld"},
		{id: "t0018", name: "override default @language", ld10: true, ld11: true, input: "w3c/expand/0018-in.jsonld", output: "w3c/expand/0018-out.jsonld"},
		{id: "t0019", name: "remove @value = null", ld10: true, ld11: true, input: "w3c/expand/0019-in.jsonld", output: "w3c/expand/0019-out.jsonld"},
		{id: "t0020", name: "do not remove @graph if not at top-level", ld10: true, ld11: true, input: "w3c/expand/0020-in.jsonld", output: "w3c/expand/0020-out.jsonld"},
		{id: "t0021", name: "do not remove @graph at top-level if not only property", ld10: true, ld11: true, input: "w3c/expand/0021-in.jsonld", output: "w3c/expand/0021-out.jsonld"},
		{id: "t0022", name: "expand value with default language", ld10: true, ld11: true, input: "w3c/expand/0022-in.jsonld", output: "w3c/expand/0022-out.jsonld"},
		{id: "t0023", name: "Expanding list/set with coercion", ld10: true, ld11: true, input: "w3c/expand/0023-in.jsonld", output: "w3c/expand/0023-out.jsonld"},
		{id: "t0024", name: "Multiple contexts", ld10: true, ld11: true, input: "w3c/expand/0024-in.jsonld", output: "w3c/expand/0024-out.jsonld"},
		{id: "t0025", name: "Problematic IRI expansion tests", ld10: true, ld11: true, input: "w3c/expand/0025-in.jsonld", output: "w3c/expand/0025-out.jsonld"},
		{id: "t0026", name: "Term definition with @id: @type", ld10: false, ld11: false, input: "w3c/expand/0026-in.jsonld", output: "w3c/expand/0026-out.jsonld"},
		{id: "t0027", name: "Duplicate values in @list and @set", ld10: true, ld11: true, input: "w3c/expand/0027-in.jsonld", output: "w3c/expand/0027-out.jsonld"},
		{id: "t0028", name: "Use @vocab in properties and @type but not in @id", ld10: true, ld11: true, input: "w3c/expand/0028-in.jsonld", output: "w3c/expand/0028-out.jsonld"},
		{id: "t0029", name: "Relative IRIs", ld10: true, ld11: true, input: "w3c/expand/0029-in.jsonld", output: "w3c/expand/0029-out.jsonld"},
		{id: "t0030", name: "Language maps", ld10: true, ld11: true, input: "w3c/expand/0030-in.jsonld", output: "w3c/expand/0030-out.jsonld"},
		{id: "t0031", name: "type-coercion of native types", ld10: true, ld11: true, input: "w3c/expand/0031-in.jsonld", output: "w3c/expand/0031-out.jsonld"},
		{id: "t0032", name: "Null term and @vocab", ld10: true, ld11: true, input: "w3c/expand/0032-in.jsonld", output: "w3c/expand/0032-out.jsonld"},
		{id: "t0033", name: "Using @vocab with with type-coercion", ld10: true, ld11: true, input: "w3c/expand/0033-in.jsonld", output: "w3c/expand/0033-out.jsonld"},
		{id: "t0034", name: "Multiple properties expanding to the same IRI", ld10: true, ld11: true, input: "w3c/expand/0034-in.jsonld", output: "w3c/expand/0034-out.jsonld"},
		{id: "t0035", name: "Language maps with @vocab, default language, and colliding property", ld10: true, ld11: true, input: "w3c/expand/0035-in.jsonld", output: "w3c/expand/0035-out.jsonld"},
		{id: "t0036", name: "Expanding @index", ld10: true, ld11: true, input: "w3c/expand/0036-in.jsonld", output: "w3c/expand/0036-out.jsonld"},
		{id: "t0037", name: "Expanding @reverse", ld10: true, ld11: true, input: "w3c/expand/0037-in.jsonld", output: "w3c/expand/0037-out.jsonld"},
		{id: "t0038", name: "Expanding blank node labels", ld10: false, ld11: false, input: "w3c/expand/0038-in.jsonld", output: "w3c/expand/0038-out.jsonld"},
		{id: "t0039", name: "Using terms in a reverse-maps", ld10: true, ld11: true, input: "w3c/expand/0039-in.jsonld", output: "w3c/expand/0039-out.jsonld"},
		{id: "t0040", name: "language and index expansion on non-objects", ld10: true, ld11: true, input: "w3c/expand/0040-in.jsonld", output: "w3c/expand/0040-out.jsonld"},
		{id: "t0041", name: "@language: null", ld10: true, ld11: true, input: "w3c/expand/0041-in.jsonld", output: "w3c/expand/0041-out.jsonld"},
		{id: "t0042", name: "Reverse properties", ld10: true, ld11: true, input: "w3c/expand/0042-in.jsonld", output: "w3c/expand/0042-out.jsonld"},
		{id: "t0043", name: "Using reverse properties inside a @reverse-container", ld10: true, ld11: true, input: "w3c/expand/0043-in.jsonld", output: "w3c/expand/0043-out.jsonld"},
		{id: "t0044", name: "Index maps with language mappings", ld10: true, ld11: true, input: "w3c/expand/0044-in.jsonld", output: "w3c/expand/0044-out.jsonld"},
		{id: "t0045", name: "Top-level value objects", ld10: true, ld11: true, input: "w3c/expand/0045-in.jsonld", output: "w3c/expand/0045-out.jsonld"},
		{id: "t0046", name: "Free-floating nodes", ld10: true, ld11: true, input: "w3c/expand/0046-in.jsonld", output: "w3c/expand/0046-out.jsonld"},
		{id: "t0047", name: "Free-floating values in sets and free-floating lists", ld10: true, ld11: true, input: "w3c/expand/0047-in.jsonld", output: "w3c/expand/0047-out.jsonld"},
		{id: "t0048", name: "Terms are ignored in @id", ld10: true, ld11: true, input: "w3c/expand/0048-in.jsonld", output: "w3c/expand/0048-out.jsonld"},
		{id: "t0049", name: "String values of reverse properties", ld10: true, ld11: true, input: "w3c/expand/0049-in.jsonld", output: "w3c/expand/0049-out.jsonld"},
		{id: "t0050", name: "Term definitions with prefix separate from prefix definitions", ld10: true, ld11: true, input: "w3c/expand/0050-in.jsonld", output: "w3c/expand/0050-out.jsonld"},
		{id: "t0051", name: "Expansion of keyword aliases in term definitions", ld10: true, ld11: true, input: "w3c/expand/0051-in.jsonld", output: "w3c/expand/0051-out.jsonld"},
		{id: "t0052", name: "@vocab-relative IRIs in term definitions", ld10: true, ld11: true, input: "w3c/expand/0052-in.jsonld", output: "w3c/expand/0052-out.jsonld"},
		{id: "t0053", name: "Expand absolute IRI with @type: @vocab", ld10: true, ld11: true, input: "w3c/expand/0053-in.jsonld", output: "w3c/expand/0053-out.jsonld"},
		{id: "t0054", name: "Expand term with @type: @vocab", ld10: true, ld11: true, input: "w3c/expand/0054-in.jsonld", output: "w3c/expand/0054-out.jsonld"},
		{id: "t0055", name: "Expand @vocab-relative term with @type: @vocab", ld10: true, ld11: true, input: "w3c/expand/0055-in.jsonld", output: "w3c/expand/0055-out.jsonld"},
		{id: "t0056", name: "Use terms with @type: @vocab but not with @type: @id", ld10: true, ld11: true, input: "w3c/expand/0056-in.jsonld", output: "w3c/expand/0056-out.jsonld"},
		{id: "t0057", name: "Expand relative IRI with @type: @vocab", ld10: true, ld11: true, input: "w3c/expand/0057-in.jsonld", output: "w3c/expand/0057-out.jsonld"},
		{id: "t0058", name: "Expand compact IRI with @type: @vocab", ld10: true, ld11: true, input: "w3c/expand/0058-in.jsonld", output: "w3c/expand/0058-out.jsonld"},
		{id: "t0059", name: "Reset @vocab by setting it to null", ld10: true, ld11: true, input: "w3c/expand/0059-in.jsonld", output: "w3c/expand/0059-out.jsonld"},
		{id: "t0060", name: "Overwrite document base with @base and reset it again", ld10: true, ld11: true, input: "w3c/expand/0060-in.jsonld", output: "w3c/expand/0060-out.jsonld"},
		{id: "t0061", name: "Coercing native types to arbitrary datatypes", ld10: true, ld11: true, input: "w3c/expand/0061-in.jsonld", output: "w3c/expand/0061-out.jsonld"},
		{id: "t0062", name: "Various relative IRIs with with @base", ld10: true, ld11: true, input: "w3c/expand/0062-in.jsonld", output: "w3c/expand/0062-out.jsonld"},
		{id: "t0063", name: "Reverse property and index container", ld10: true, ld11: true, input: "w3c/expand/0063-in.jsonld", output: "w3c/expand/0063-out.jsonld"},
		{id: "t0064", name: "bnode values of reverse properties", ld10: true, ld11: true, input: "w3c/expand/0064-in.jsonld", output: "w3c/expand/0064-out.jsonld"},
		{id: "t0065", name: "Drop unmapped keys in reverse map", ld10: true, ld11: true, input: "w3c/expand/0065-in.jsonld", output: "w3c/expand/0065-out.jsonld"},
		{id: "t0066", name: "Reverse-map keys with @vocab", ld10: true, ld11: true, input: "w3c/expand/0066-in.jsonld", output: "w3c/expand/0066-out.jsonld"},
		{id: "t0067", name: "prefix://suffix not a compact IRI", ld10: true, ld11: true, input: "w3c/expand/0067-in.jsonld", output: "w3c/expand/0067-out.jsonld"},
		{id: "t0068", name: "_:suffix values are not a compact IRI", ld10: true, ld11: true, input: "w3c/expand/0068-in.jsonld", output: "w3c/expand/0068-out.jsonld"},
		{id: "t0069", name: "Compact IRI as term with type mapping", ld10: true, ld11: true, input: "w3c/expand/0069-in.jsonld", output: "w3c/expand/0069-out.jsonld"},
		{id: "t0070", name: "Compact IRI as term defined using equivalent compact IRI", ld10: true, ld11: true, input: "w3c/expand/0070-in.jsonld", output: "w3c/expand/0070-out.jsonld"},
		{id: "t0071", name: "Redefine terms looking like compact IRIs", ld10: false, ld11: false, input: "w3c/expand/0071-in.jsonld", output: "w3c/expand/0071-out.jsonld"},
		{id: "t0072", name: "Redefine term using @vocab, not itself", ld10: true, ld11: true, input: "w3c/expand/0072-in.jsonld", output: "w3c/expand/0072-out.jsonld"},
		{id: "t0073", name: "@context not first property", ld10: true, ld11: true, input: "w3c/expand/0073-in.jsonld", output: "w3c/expand/0073-out.jsonld"},
		{id: "t0074", name: "@id not first property", ld10: true, ld11: true, input: "w3c/expand/0074-in.jsonld", output: "w3c/expand/0074-out.jsonld"},
		{id: "t0075", name: "@vocab as blank node identifier", ld10: true, ld11: true, input: "w3c/expand/0075-in.jsonld", output: "w3c/expand/0075-out.jsonld"},
		{id: "t0076", name: "base option overrides document location", ld10: true, ld11: true, base: "http://example/base/", input: "w3c/expand/0076-in.jsonld", output: "w3c/expand/0076-out.jsonld"},
		{id: "t0077", name: "expandContext option", ld10: true, ld11: true, input: "w3c/expand/0077-in.jsonld", output: "w3c/expand/0077-out.jsonld", context: "w3c/expand/0077-context.jsonld", expandContext: "w3c/expand/0077-context.jsonld"},
		{id: "t0078", name: "multiple reverse properties", ld10: true, ld11: true, input: "w3c/expand/0078-in.jsonld", output: "w3c/expand/0078-out.jsonld"},
		{id: "t0079", name: "expand @graph container", ld10: false, ld11: true, input: "w3c/expand/0079-in.jsonld", output: "w3c/expand/0079-out.jsonld"},
		{id: "t0080", name: "expand [@graph, @set] container", ld10: false, ld11: true, input: "w3c/expand/0080-in.jsonld", output: "w3c/expand/0080-out.jsonld"},
		{id: "t0081", name: "Creates an @graph container if value is a graph", ld10: false, ld11: true, input: "w3c/expand/0081-in.jsonld", output: "w3c/expand/0081-out.jsonld"},
		{id: "t0082", name: "expand [@graph, @index] container", ld10: false, ld11: true, input: "w3c/expand/0082-in.jsonld", output: "w3c/expand/0082-out.jsonld"},
		{id: "t0083", name: "expand [@graph, @index, @set] container", ld10: false, ld11: true, input: "w3c/expand/0083-in.jsonld", output: "w3c/expand/0083-out.jsonld"},
		{id: "t0084", name: "Do not expand [@graph, @index] container if value is a graph", ld10: false, ld11: true, input: "w3c/expand/0084-in.jsonld", output: "w3c/expand/0084-out.jsonld"},
		{id: "t0085", name: "expand [@graph, @id] container", ld10: false, ld11: true, input: "w3c/expand/0085-in.jsonld", output: "w3c/expand/0085-out.jsonld"},
		{id: "t0086", name: "expand [@graph, @id, @set] container", ld10: false, ld11: true, input: "w3c/expand/0086-in.jsonld", output: "w3c/expand/0086-out.jsonld"},
		{id: "t0087", name: "Do not expand [@graph, @id] container if value is a graph", ld10: false, ld11: true, input: "w3c/expand/0087-in.jsonld", output: "w3c/expand/0087-out.jsonld"},
		{id: "t0088", name: "Do not expand native values to IRIs", ld10: true, ld11: true, input: "w3c/expand/0088-in.jsonld", output: "w3c/expand/0088-out.jsonld"},
		{id: "t0089", name: "empty @base applied to the base option", ld10: true, ld11: true, base: "http://example/base/", input: "w3c/expand/0089-in.jsonld", output: "w3c/expand/0089-out.jsonld"},
		{id: "t0090", name: "relative @base overrides base option and document location", ld10: true, ld11: true, base: "http://example/base/", input: "w3c/expand/0090-in.jsonld", output: "w3c/expand/0090-out.jsonld"},
		{id: "t0091", name: "relative and absolute @base overrides base option and document location", ld10: true, ld11: true, base: "http://example/base/", input: "w3c/expand/0091-in.jsonld", output: "w3c/expand/0091-out.jsonld"},
		{id: "t0092", name: "Various relative IRIs as properties with with @vocab: ''", ld10: false, ld11: true, input: "w3c/expand/0092-in.jsonld", output: "w3c/expand/0092-out.jsonld"},
		{id: "t0093", name: "expand @graph container (multiple objects)", ld10: false, ld11: true, input: "w3c/expand/0093-in.jsonld", output: "w3c/expand/0093-out.jsonld"},
		{id: "t0094", name: "expand [@graph, @set] container (multiple objects)", ld10: false, ld11: true, input: "w3c/expand/0094-in.jsonld", output: "w3c/expand/0094-out.jsonld"},
		{id: "t0095", name: "Creates an @graph container if value is a graph (multiple objects)", ld10: false, ld11: true, input: "w3c/expand/0095-in.jsonld", output: "w3c/expand/0095-out.jsonld"},
		{id: "t0096", name: "expand [@graph, @index] container (multiple indexed objects)", ld10: false, ld11: true, input: "w3c/expand/0096-in.jsonld", output: "w3c/expand/0096-out.jsonld"},
		{id: "t0097", name: "expand [@graph, @index, @set] container (multiple objects)", ld10: false, ld11: true, input: "w3c/expand/0097-in.jsonld", output: "w3c/expand/0097-out.jsonld"},
		{id: "t0098", name: "Do not expand [@graph, @index] container if value is a graph (multiple objects)", ld10: false, ld11: true, input: "w3c/expand/0098-in.jsonld", output: "w3c/expand/0098-out.jsonld"},
		{id: "t0099", name: "expand [@graph, @id] container (multiple objects)", ld10: false, ld11: true, input: "w3c/expand/0099-in.jsonld", output: "w3c/expand/0099-out.jsonld"},
		{id: "t0100", name: "expand [@graph, @id, @set] container (multiple objects)", ld10: false, ld11: true, input: "w3c/expand/0100-in.jsonld", output: "w3c/expand/0100-out.jsonld"},
		{id: "t0101", name: "Do not expand [@graph, @id] container if value is a graph (multiple objects)", ld10: false, ld11: true, input: "w3c/expand/0101-in.jsonld", output: "w3c/expand/0101-out.jsonld"},
		{id: "t0102", name: "Expand @graph container if value is a graph (multiple objects)", ld10: false, ld11: true, input: "w3c/expand/0102-in.jsonld", output: "w3c/expand/0102-out.jsonld"},
		{id: "t0103", name: "Expand @graph container if value is a graph (multiple graphs)", ld10: false, ld11: true, input: "w3c/expand/0103-in.jsonld", output: "w3c/expand/0103-out.jsonld"},
		{id: "t0104", name: "Creates an @graph container if value is a graph (mixed graph and object)", ld10: false, ld11: true, input: "w3c/expand/0104-in.jsonld", output: "w3c/expand/0104-out.jsonld"},
		{id: "t0105", name: "Do not expand [@graph, @index] container if value is a graph (mixed graph and object)", ld10: false, ld11: true, input: "w3c/expand/0105-in.jsonld", output: "w3c/expand/0105-out.jsonld"},
		{id: "t0106", name: "Do not expand [@graph, @id] container if value is a graph (mixed graph and object)", ld10: false, ld11: true, input: "w3c/expand/0106-in.jsonld", output: "w3c/expand/0106-out.jsonld"},
		{id: "t0107", name: "expand [@graph, @index] container (indexes with multiple objects)", ld10: false, ld11: true, input: "w3c/expand/0107-in.jsonld", output: "w3c/expand/0107-out.jsonld"},
		{id: "t0108", name: "expand [@graph, @id] container (multiple ids and objects)", ld10: false, ld11: true, input: "w3c/expand/0108-in.jsonld", output: "w3c/expand/0108-out.jsonld"},
		{id: "t0109", name: "IRI expansion of fragments including ':'", ld10: true, ld11: true, input: "w3c/expand/0109-in.jsonld", output: "w3c/expand/0109-out.jsonld"},
		{id: "t0110", name: "Various relative IRIs as properties with with relative @vocab", ld10: false, ld11: true, input: "w3c/expand/0110-in.jsonld", output: "w3c/expand/0110-out.jsonld"},
		{id: "t0111", name: "Various relative IRIs as properties with with relative @vocab itself relative to an existing vocabulary base", ld10: false, ld11: true, input: "w3c/expand/0111-in.jsonld", output: "w3c/expand/0111-out.jsonld"},
		{id: "t0112", name: "Various relative IRIs as properties with with relative @vocab relative to another relative vocabulary base", ld10: false, ld11: true, input: "w3c/expand/0112-in.jsonld", output: "w3c/expand/0112-out.jsonld"},
		{id: "t0113", name: "context with JavaScript Object property names", ld10: true, ld11: true, input: "w3c/expand/0113-in.jsonld", output: "w3c/expand/0113-out.jsonld"},
		{id: "t0114", name: "Expansion allows multiple properties expanding to @type", ld10: false, ld11: true, input: "w3c/expand/0114-in.jsonld", output: "w3c/expand/0114-out.jsonld"},
		{id: "t0115", name: "Verifies that relative IRIs as properties with @vocab: '' in 1.0 generate an error", ld10: false, ld11: false, input: "w3c/expand/0115-in.jsonld", err: "invalid vocab mapping"},
		{id: "t0116", name: "Verifies that relative IRIs as properties with relative @vocab in 1.0 generate an error", ld10: false, ld11: false, input: "w3c/expand/0116-in.jsonld", err: "invalid vocab mapping"},
		{id: "t0117", name: "A term starting with a colon can expand to a different IRI", ld10: false, ld11: true, input: "w3c/expand/0117-in.jsonld", output: "w3c/expand/0117-out.jsonld"},
		{id: "t0118", name: "Expanding a value staring with a colon does not treat that value as an IRI", ld10: false, ld11: true, input: "w3c/expand/0118-in.jsonld", output: "w3c/expand/0118-out.jsonld"},
		{id: "t0119", name: "Ignore some terms with @, allow others.", ld10: false, ld11: true, input: "w3c/expand/0119-in.jsonld", output: "w3c/expand/0119-out.jsonld"},
		{id: "t0120", name: "Ignore some values of @id with @, allow others.", ld10: false, ld11: true, input: "w3c/expand/0120-in.jsonld", output: "w3c/expand/0120-out.jsonld"},
		{id: "t0121", name: "Ignore some values of @reverse with @, allow others.", ld10: false, ld11: true, input: "w3c/expand/0121-in.jsonld", output: "w3c/expand/0121-out.jsonld"},
		{id: "t0122", name: "Ignore some IRIs when that start with @ when expanding.", ld10: false, ld11: true, input: "w3c/expand/0122-in.jsonld", output: "w3c/expand/0122-out.jsonld", err: "invalid @id value"}, // We override this test to include an error, since the document results in invalid JSON-LD and we made the decision to error out on that instead. The test is non-normative anyway.
		{id: "t0123", name: "Value objects including invalid literal datatype IRIs are rejected", ld10: false, ld11: true, input: "w3c/expand/0123-in.jsonld", err: "invalid typed value"},
		{id: "t0124", name: "compact IRI as @vocab", ld10: false, ld11: true, input: "w3c/expand/0124-in.jsonld", output: "w3c/expand/0124-out.jsonld"},
		{id: "t0125", name: "term as @vocab", ld10: false, ld11: true, input: "w3c/expand/0125-in.jsonld", output: "w3c/expand/0125-out.jsonld"},
		{id: "t0126", name: "A scoped context may include itself recursively (direct)", ld10: false, ld11: true, input: "w3c/expand/0126-in.jsonld", output: "w3c/expand/0126-out.jsonld"},
		{id: "t0127", name: "A scoped context may include itself recursively (indirect)", ld10: false, ld11: true, input: "w3c/expand/0127-in.jsonld", output: "w3c/expand/0127-out.jsonld"},
		{id: "t0128", name: "Two scoped context may include a shared context", ld10: false, ld11: true, input: "w3c/expand/0128-in.jsonld", output: "w3c/expand/0128-out.jsonld"},
		{id: "t0129", name: "Base without trailing slash, without path", ld10: true, ld11: true, input: "w3c/expand/0129-in.jsonld", output: "w3c/expand/0129-out.jsonld"},
		{id: "t0130", name: "Base without trailing slash, with path", ld10: true, ld11: true, input: "w3c/expand/0130-in.jsonld", output: "w3c/expand/0130-out.jsonld"},
		{id: "t0131", name: "Reverse term with property based indexed container", ld10: false, ld11: true, input: "w3c/expand/0131-in.jsonld", output: "w3c/expand/0131-out.jsonld"},
		{id: "tc001", name: "adding new term", ld10: false, ld11: true, input: "w3c/expand/c001-in.jsonld", output: "w3c/expand/c001-out.jsonld"},
		{id: "tc002", name: "overriding a term", ld10: false, ld11: true, input: "w3c/expand/c002-in.jsonld", output: "w3c/expand/c002-out.jsonld"},
		{id: "tc003", name: "property and value with different terms mapping to the same expanded property", ld10: false, ld11: true, input: "w3c/expand/c003-in.jsonld", output: "w3c/expand/c003-out.jsonld"},
		{id: "tc004", name: "deep @context affects nested nodes", ld10: false, ld11: true, input: "w3c/expand/c004-in.jsonld", output: "w3c/expand/c004-out.jsonld"},
		{id: "tc005", name: "scoped context layers on intemediate contexts", ld10: false, ld11: true, input: "w3c/expand/c005-in.jsonld", output: "w3c/expand/c005-out.jsonld"},
		{id: "tc006", name: "adding new term", ld10: false, ld11: true, input: "w3c/expand/c006-in.jsonld", output: "w3c/expand/c006-out.jsonld"},
		{id: "tc007", name: "overriding a term", ld10: false, ld11: true, input: "w3c/expand/c007-in.jsonld", output: "w3c/expand/c007-out.jsonld"},
		{id: "tc008", name: "alias of @type", ld10: false, ld11: true, input: "w3c/expand/c008-in.jsonld", output: "w3c/expand/c008-out.jsonld"},
		{id: "tc009", name: "deep @type-scoped @context does NOT affect nested nodes", ld10: false, ld11: true, input: "w3c/expand/c009-in.jsonld", output: "w3c/expand/c009-out.jsonld"},
		{id: "tc010", name: "scoped context layers on intemediate contexts", ld10: false, ld11: true, input: "w3c/expand/c010-in.jsonld", output: "w3c/expand/c010-out.jsonld"},
		{id: "tc011", name: "orders @type terms when applying scoped contexts", ld10: false, ld11: true, input: "w3c/expand/c011-in.jsonld", output: "w3c/expand/c011-out.jsonld"},
		{id: "tc012", name: "deep property-term scoped @context in @type-scoped @context affects nested nodes", ld10: false, ld11: true, input: "w3c/expand/c012-in.jsonld", output: "w3c/expand/c012-out.jsonld"},
		{id: "tc013", name: "type maps use scoped context from type index and not scoped context from containing", ld10: false, ld11: true, input: "w3c/expand/c013-in.jsonld", output: "w3c/expand/c013-out.jsonld"},
		{id: "tc014", name: "type-scoped context nullification", ld10: false, ld11: true, input: "w3c/expand/c014-in.jsonld", output: "w3c/expand/c014-out.jsonld"},
		{id: "tc015", name: "type-scoped base", ld10: false, ld11: true, input: "w3c/expand/c015-in.jsonld", output: "w3c/expand/c015-out.jsonld"},
		{id: "tc016", name: "type-scoped vocab", ld10: false, ld11: true, input: "w3c/expand/c016-in.jsonld", output: "w3c/expand/c016-out.jsonld"},
		{id: "tc017", name: "multiple type-scoped contexts are properly reverted", ld10: false, ld11: true, input: "w3c/expand/c017-in.jsonld", output: "w3c/expand/c017-out.jsonld"},
		{id: "tc018", name: "multiple type-scoped types resolved against previous context", ld10: false, ld11: true, input: "w3c/expand/c018-in.jsonld", output: "w3c/expand/c018-out.jsonld"},
		{id: "tc019", name: "type-scoped context with multiple property scoped terms", ld10: false, ld11: true, input: "w3c/expand/c019-in.jsonld", output: "w3c/expand/c019-out.jsonld"},
		{id: "tc020", name: "type-scoped value", ld10: false, ld11: true, input: "w3c/expand/c020-in.jsonld", output: "w3c/expand/c020-out.jsonld"},
		{id: "tc021", name: "type-scoped value mix", ld10: false, ld11: true, input: "w3c/expand/c021-in.jsonld", output: "w3c/expand/c021-out.jsonld"},
		{id: "tc022", name: "type-scoped property-scoped contexts including @type:@vocab", ld10: false, ld11: true, input: "w3c/expand/c022-in.jsonld", output: "w3c/expand/c022-out.jsonld"},
		{id: "tc023", name: "composed type-scoped property-scoped contexts including @type:@vocab", ld10: false, ld11: true, input: "w3c/expand/c023-in.jsonld", output: "w3c/expand/c023-out.jsonld"},
		{id: "tc024", name: "type-scoped + property-scoped + values evaluates against previous context", ld10: false, ld11: true, input: "w3c/expand/c024-in.jsonld", output: "w3c/expand/c024-out.jsonld"},
		{id: "tc025", name: "type-scoped + graph container", ld10: false, ld11: true, input: "w3c/expand/c025-in.jsonld", output: "w3c/expand/c025-out.jsonld"},
		{id: "tc026", name: "@propagate: true on type-scoped context", ld10: false, ld11: true, input: "w3c/expand/c026-in.jsonld", output: "w3c/expand/c026-out.jsonld"},
		{id: "tc027", name: "@propagate: false on property-scoped context", ld10: false, ld11: true, input: "w3c/expand/c027-in.jsonld", output: "w3c/expand/c027-out.jsonld"},
		{id: "tc028", name: "@propagate: false on embedded context", ld10: false, ld11: true, input: "w3c/expand/c028-in.jsonld", output: "w3c/expand/c028-out.jsonld"},
		{id: "tc029", name: "@propagate is invalid in 1.0", ld10: true, ld11: false, input: "w3c/expand/c029-in.jsonld", err: "invalid context entry"},
		{id: "tc030", name: "@propagate must be boolean valued", ld10: false, ld11: true, input: "w3c/expand/c030-in.jsonld", err: "invalid @propagate value"},
		{id: "tc031", name: "@context resolutions respects relative URLs.", ld10: false, ld11: true, input: "w3c/expand/c031-in.jsonld", output: "w3c/expand/c031-out.jsonld"},
		{id: "tc032", name: "Unused embedded context with error.", ld10: false, ld11: true, input: "w3c/expand/c032-in.jsonld", err: "invalid scoped context"},
		{id: "tc033", name: "Unused context with an embedded context error.", ld10: false, ld11: true, input: "w3c/expand/c033-in.jsonld", err: "invalid scoped context"},
		{id: "tc034", name: "Remote scoped context.", ld10: false, ld11: true, input: "w3c/expand/c034-in.jsonld", output: "w3c/expand/c034-out.jsonld"},
		{id: "tc035", name: "Term scoping with embedded contexts.", ld10: false, ld11: true, input: "w3c/expand/c035-in.jsonld", output: "w3c/expand/c035-out.jsonld"},
		{id: "tc036", name: "Expansion with empty property-scoped context.", ld10: false, ld11: true, input: "w3c/expand/c036-in.jsonld", output: "w3c/expand/c036-out.jsonld"},
		{id: "tc037", name: "property-scoped contexts which are alias of @nest", ld10: false, ld11: true, input: "w3c/expand/c037-in.jsonld", output: "w3c/expand/c037-out.jsonld"},
		{id: "tc038", name: "Bibframe example (poor-mans inferrence)", ld10: false, ld11: true, input: "w3c/expand/c038-in.jsonld", output: "w3c/expand/c038-out.jsonld"},
		{id: "tdi01", name: "Expand string using default and term directions", ld10: false, ld11: true, input: "w3c/expand/di01-in.jsonld", output: "w3c/expand/di01-out.jsonld"},
		{id: "tdi02", name: "Expand string using default and term directions and languages", ld10: false, ld11: true, input: "w3c/expand/di02-in.jsonld", output: "w3c/expand/di02-out.jsonld"},
		{id: "tdi03", name: "expand list values with @direction", ld10: false, ld11: true, input: "w3c/expand/di03-in.jsonld", output: "w3c/expand/di03-out.jsonld"},
		{id: "tdi04", name: "simple language map with term direction", ld10: false, ld11: true, input: "w3c/expand/di04-in.jsonld", output: "w3c/expand/di04-out.jsonld"},
		{id: "tdi05", name: "simple language mapwith overriding term direction", ld10: false, ld11: true, input: "w3c/expand/di05-in.jsonld", output: "w3c/expand/di05-out.jsonld"},
		{id: "tdi06", name: "simple language mapwith overriding null direction", ld10: false, ld11: true, input: "w3c/expand/di06-in.jsonld", output: "w3c/expand/di06-out.jsonld"},
		{id: "tdi07", name: "simple language map with mismatching term direction", ld10: false, ld11: true, input: "w3c/expand/di07-in.jsonld", output: "w3c/expand/di07-out.jsonld"},
		{id: "tdi08", name: "@direction must be one of ltr or rtl", ld10: false, ld11: true, input: "w3c/expand/di08-in.jsonld", err: "invalid base direction"},
		{id: "tdi09", name: "@direction is incompatible with @type", ld10: false, ld11: true, input: "w3c/expand/di09-in.jsonld", err: "invalid value object"},
		{id: "tec01", name: "Invalid keyword in term definition", ld10: false, ld11: true, input: "w3c/expand/ec01-in.jsonld", err: "invalid term definition"},
		{id: "tec02", name: "Term definition on @type with empty map", ld10: false, ld11: true, input: "w3c/expand/ec02-in.jsonld", err: "keyword redefinition"},
		{id: "tem01", name: "Invalid container mapping", ld10: false, ld11: true, input: "w3c/expand/em01-in.jsonld", err: "invalid container mapping"},
		{id: "ten01", name: "@nest MUST NOT have a string value", ld10: false, ld11: true, input: "w3c/expand/en01-in.jsonld", err: "invalid @nest value"},
		{id: "ten02", name: "@nest MUST NOT have a boolen value", ld10: false, ld11: true, input: "w3c/expand/en02-in.jsonld", err: "invalid @nest value"},
		{id: "ten03", name: "@nest MUST NOT have a numeric value", ld10: false, ld11: true, input: "w3c/expand/en03-in.jsonld", err: "invalid @nest value"},
		{id: "ten04", name: "@nest MUST NOT have a value object value", ld10: false, ld11: true, input: "w3c/expand/en04-in.jsonld", err: "invalid @nest value"},
		{id: "ten05", name: "does not allow a keyword other than @nest for the value of @nest", ld10: false, ld11: true, input: "w3c/expand/en05-in.jsonld", err: "invalid @nest value"},
		{id: "ten06", name: "does not allow @nest with @reverse", ld10: false, ld11: true, input: "w3c/expand/en06-in.jsonld", err: "invalid reverse property"},
		{id: "tep02", name: "processingMode json-ld-1.0 conflicts with @version: 1.1", ld10: true, ld11: false, input: "w3c/expand/ep02-in.jsonld", err: "processing mode conflict"},
		{id: "tep03", name: "@version must be 1.1", ld10: false, ld11: true, input: "w3c/expand/ep03-in.jsonld", err: "invalid @version value"},
		{id: "ter01", name: "Keywords cannot be aliased to other keywords", ld10: true, ld11: true, input: "w3c/expand/er01-in.jsonld", err: "keyword redefinition"},
		{id: "ter02", name: "A context may not include itself recursively (direct)", ld10: true, ld11: false, input: "w3c/expand/er02-in.jsonld", err: "recursive context inclusion"},
		{id: "ter03", name: "A context may not include itself recursively (indirect)", ld10: true, ld11: false, input: "w3c/expand/er03-in.jsonld", err: "recursive context inclusion"},
		{id: "ter04", name: "Error dereferencing a remote context", ld10: true, ld11: true, input: "w3c/expand/er04-in.jsonld", err: "loading remote context failed"},
		{id: "ter05", name: "Invalid remote context", ld10: false, ld11: true, input: "w3c/expand/er05-in.jsonld", err: "invalid remote context"},
		{id: "ter06", name: "Invalid local context", ld10: true, ld11: true, input: "w3c/expand/er06-in.jsonld", err: "invalid local context"},
		{id: "ter07", name: "Invalid base IRI", ld10: true, ld11: true, input: "w3c/expand/er07-in.jsonld", err: "invalid base IRI"},
		{id: "ter08", name: "Invalid vocab mapping", ld10: true, ld11: true, input: "w3c/expand/er08-in.jsonld", err: "invalid vocab mapping"},
		{id: "ter09", name: "Invalid default language", ld10: true, ld11: true, input: "w3c/expand/er09-in.jsonld", err: "invalid default language"},
		{id: "ter10", name: "Cyclic IRI mapping", ld10: true, ld11: true, input: "w3c/expand/er10-in.jsonld", err: "cyclic IRI mapping"},
		{id: "ter11", name: "Invalid term definition", ld10: true, ld11: true, input: "w3c/expand/er11-in.jsonld", err: "invalid term definition"},
		{id: "ter12", name: "Invalid type mapping (not a string)", ld10: true, ld11: true, input: "w3c/expand/er12-in.jsonld", err: "invalid type mapping"},
		{id: "ter13", name: "Invalid type mapping (not absolute IRI)", ld10: true, ld11: true, input: "w3c/expand/er13-in.jsonld", err: "invalid type mapping"},
		{id: "ter14", name: "Invalid reverse property (contains @id)", ld10: true, ld11: true, input: "w3c/expand/er14-in.jsonld", err: "invalid reverse property"},
		{id: "ter15", name: "Invalid IRI mapping (@reverse not a string)", ld10: true, ld11: true, input: "w3c/expand/er15-in.jsonld", err: "invalid IRI mapping"},
		{id: "ter17", name: "Invalid reverse property (invalid @container)", ld10: true, ld11: true, input: "w3c/expand/er17-in.jsonld", err: "invalid reverse property"},
		{id: "ter18", name: "Invalid IRI mapping (@id not a string)", ld10: true, ld11: true, input: "w3c/expand/er18-in.jsonld", err: "invalid IRI mapping"},
		{id: "ter19", name: "Invalid keyword alias (@context)", ld10: true, ld11: true, input: "w3c/expand/er19-in.jsonld", err: "invalid keyword alias"},
		{id: "ter20", name: "Invalid IRI mapping (no vocab mapping)", ld10: true, ld11: true, input: "w3c/expand/er20-in.jsonld", err: "invalid IRI mapping"},
		{id: "ter21", name: "Invalid container mapping", ld10: true, ld11: false, input: "w3c/expand/er21-in.jsonld", err: "invalid container mapping"},
		{id: "ter22", name: "Invalid language mapping", ld10: true, ld11: true, input: "w3c/expand/er22-in.jsonld", err: "invalid language mapping"},
		{id: "ter23", name: "Invalid IRI mapping (relative IRI in @type)", ld10: true, ld11: true, input: "w3c/expand/er23-in.jsonld", err: "invalid type mapping"},
		{id: "ter24", name: "List of lists (from array)", ld10: false, ld11: false, input: "w3c/expand/er24-in.jsonld", err: "list of lists"},
		{id: "ter25", name: "Invalid reverse property map", ld10: true, ld11: true, input: "w3c/expand/er25-in.jsonld", err: "invalid reverse property map"},
		{id: "ter26", name: "Colliding keywords", ld10: true, ld11: true, input: "w3c/expand/er26-in.jsonld", err: "colliding keywords"},
		{id: "ter27", name: "Invalid @id value", ld10: true, ld11: true, input: "w3c/expand/er27-in.jsonld", err: "invalid @id value"},
		{id: "ter28", name: "Invalid type value", ld10: true, ld11: true, input: "w3c/expand/er28-in.jsonld", err: "invalid type value"},
		{id: "ter29", name: "Invalid value object value", ld10: true, ld11: true, input: "w3c/expand/er29-in.jsonld", err: "invalid value object value"},
		{id: "ter30", name: "Invalid language-tagged string", ld10: true, ld11: true, input: "w3c/expand/er30-in.jsonld", err: "invalid language-tagged string"},
		{id: "ter31", name: "Invalid @index value", ld10: true, ld11: true, input: "w3c/expand/er31-in.jsonld", err: "invalid @index value"},
		{id: "ter32", name: "List of lists (from array)", ld10: false, ld11: false, input: "w3c/expand/er32-in.jsonld", err: "list of lists"},
		{id: "ter33", name: "Invalid @reverse value", ld10: true, ld11: true, input: "w3c/expand/er33-in.jsonld", err: "invalid @reverse value"},
		{id: "ter34", name: "Invalid reverse property value (in @reverse)", ld10: true, ld11: true, input: "w3c/expand/er34-in.jsonld", err: "invalid reverse property value"},
		{id: "ter35", name: "Invalid language map value", ld10: true, ld11: true, input: "w3c/expand/er35-in.jsonld", err: "invalid language map value"},
		{id: "ter36", name: "Invalid reverse property value (through coercion)", ld10: true, ld11: true, input: "w3c/expand/er36-in.jsonld", err: "invalid reverse property value"},
		{id: "ter37", name: "Invalid value object (unexpected keyword)", ld10: true, ld11: true, input: "w3c/expand/er37-in.jsonld", err: "invalid value object"},
		{id: "ter38", name: "Invalid value object (@type and @language)", ld10: true, ld11: true, input: "w3c/expand/er38-in.jsonld", err: "invalid value object"},
		{id: "ter39", name: "Invalid language-tagged value", ld10: true, ld11: true, input: "w3c/expand/er39-in.jsonld", err: "invalid language-tagged value"},
		{id: "ter40", name: "Invalid typed value", ld10: true, ld11: true, input: "w3c/expand/er40-in.jsonld", err: "invalid typed value"},
		{id: "ter41", name: "Invalid set or list object", ld10: true, ld11: true, input: "w3c/expand/er41-in.jsonld", err: "invalid set or list object"},
		{id: "ter42", name: "Keywords may not be redefined in 1.0", ld10: true, ld11: false, input: "w3c/expand/er42-in.jsonld", err: "keyword redefinition"},
		{id: "ter43", name: "Term definition with @id: @type", ld10: false, ld11: true, input: "w3c/expand/er43-in.jsonld", err: "invalid IRI mapping"},
		{id: "ter44", name: "Redefine terms looking like compact IRIs", ld10: false, ld11: true, input: "w3c/expand/er44-in.jsonld", err: "invalid IRI mapping"},
		{id: "ter48", name: "Invalid term as relative IRI", ld10: false, ld11: true, input: "w3c/expand/er48-in.jsonld", err: "invalid IRI mapping"},
		{id: "ter49", name: "A relative IRI cannot be used as a prefix", ld10: false, ld11: true, input: "w3c/expand/er49-in.jsonld", err: "invalid term definition"},
		{id: "ter50", name: "Invalid reverse id", ld10: true, ld11: true, input: "w3c/expand/er50-in.jsonld", err: "invalid IRI mapping"},
		{id: "ter51", name: "Invalid value object value using a value alias", ld10: true, ld11: true, input: "w3c/expand/er51-in.jsonld", err: "invalid value object value"},
		{id: "ter52", name: "Definition for the empty term", ld10: true, ld11: true, input: "w3c/expand/er52-in.jsonld", err: "invalid term definition"},
		{id: "ter53", name: "Invalid prefix value", ld10: false, ld11: true, input: "w3c/expand/er53-in.jsonld", err: "invalid @prefix value"},
		{id: "ter54", name: "Invalid value object, multiple values for @type.", ld10: true, ld11: true, input: "w3c/expand/er54-in.jsonld", err: "invalid typed value"},
		{id: "ter55", name: "Invalid term definition, multiple values for @type.", ld10: true, ld11: true, input: "w3c/expand/er55-in.jsonld", err: "invalid type mapping"},
		{id: "ter56", name: "Invalid redefinition of @context keyword.", ld10: true, ld11: true, input: "w3c/expand/er56-in.jsonld", err: "keyword redefinition"},
		{id: "tes01", name: "Using an array value for @context is illegal in JSON-LD 1.0", ld10: true, ld11: false, input: "w3c/expand/es01-in.jsonld", err: "invalid container mapping"},
		{id: "tes02", name: "Mapping @container: [@list, @set] is invalid", ld10: false, ld11: true, input: "w3c/expand/es02-in.jsonld", err: "invalid container mapping"},
		{id: "tin01", name: "Basic Included array", ld10: false, ld11: true, input: "w3c/expand/in01-in.jsonld", output: "w3c/expand/in01-out.jsonld"},
		{id: "tin02", name: "Basic Included object", ld10: false, ld11: true, input: "w3c/expand/in02-in.jsonld", output: "w3c/expand/in02-out.jsonld"},
		{id: "tin03", name: "Multiple properties mapping to @included are folded together", ld10: false, ld11: true, input: "w3c/expand/in03-in.jsonld", output: "w3c/expand/in03-out.jsonld"},
		{id: "tin04", name: "Included containing @included", ld10: false, ld11: true, input: "w3c/expand/in04-in.jsonld", output: "w3c/expand/in04-out.jsonld"},
		{id: "tin05", name: "Property value with @included", ld10: false, ld11: true, input: "w3c/expand/in05-in.jsonld", output: "w3c/expand/in05-out.jsonld"},
		{id: "tin06", name: "json.api example", ld10: false, ld11: true, input: "w3c/expand/in06-in.jsonld", output: "w3c/expand/in06-out.jsonld"},
		{id: "tin07", name: "Error if @included value is a string", ld10: false, ld11: true, input: "w3c/expand/in07-in.jsonld", err: "invalid @included value"},
		{id: "tin08", name: "Error if @included value is a value object", ld10: false, ld11: true, input: "w3c/expand/in08-in.jsonld", err: "invalid @included value"},
		{id: "tin09", name: "Error if @included value is a list object", ld10: false, ld11: true, input: "w3c/expand/in09-in.jsonld", err: "invalid @included value"},
		{id: "tjs01", name: "Expand JSON literal (boolean true)", ld10: false, ld11: true, input: "w3c/expand/js01-in.jsonld", output: "w3c/expand/js01-out.jsonld"},
		{id: "tjs02", name: "Expand JSON literal (boolean false)", ld10: false, ld11: true, input: "w3c/expand/js02-in.jsonld", output: "w3c/expand/js02-out.jsonld"},
		{id: "tjs03", name: "Expand JSON literal (double)", ld10: false, ld11: true, input: "w3c/expand/js03-in.jsonld", output: "w3c/expand/js03-out.jsonld"},
		{id: "tjs04", name: "Expand JSON literal (double-zero)", ld10: false, ld11: true, input: "w3c/expand/js04-in.jsonld", output: "w3c/expand/js04-out.jsonld"},
		{id: "tjs05", name: "Expand JSON literal (integer)", ld10: false, ld11: true, input: "w3c/expand/js05-in.jsonld", output: "w3c/expand/js05-out.jsonld"},
		{id: "tjs06", name: "Expand JSON literal (object)", ld10: false, ld11: true, input: "w3c/expand/js06-in.jsonld", output: "w3c/expand/js06-out.jsonld"},
		{id: "tjs07", name: "Expand JSON literal (array)", ld10: false, ld11: true, input: "w3c/expand/js07-in.jsonld", output: "w3c/expand/js07-out.jsonld"},
		{id: "tjs08", name: "Expand JSON literal with array canonicalization", ld10: false, ld11: true, input: "w3c/expand/js08-in.jsonld", output: "w3c/expand/js08-out.jsonld"},
		{id: "tjs09", name: "Transform JSON literal with string canonicalization", ld10: false, ld11: true, input: "w3c/expand/js09-in.jsonld", output: "w3c/expand/js09-out.jsonld"},
		{id: "tjs10", name: "Expand JSON literal with structural canonicalization", ld10: false, ld11: true, input: "w3c/expand/js10-in.jsonld", output: "w3c/expand/js10-out.jsonld"},
		{id: "tjs11", name: "Expand JSON literal with unicode canonicalization", ld10: false, ld11: true, input: "w3c/expand/js11-in.jsonld", output: "w3c/expand/js11-out.jsonld"},
		{id: "tjs12", name: "Expand JSON literal with value canonicalization", ld10: false, ld11: true, input: "w3c/expand/js12-in.jsonld", output: "w3c/expand/js12-out.jsonld"},
		{id: "tjs13", name: "Expand JSON literal with wierd canonicalization", ld10: false, ld11: true, input: "w3c/expand/js13-in.jsonld", output: "w3c/expand/js13-out.jsonld"},
		{id: "tjs14", name: "Expand JSON literal without expanding contents", ld10: false, ld11: true, input: "w3c/expand/js14-in.jsonld", output: "w3c/expand/js14-out.jsonld"},
		{id: "tjs15", name: "Expand JSON literal aleady in expanded form", ld10: false, ld11: true, input: "w3c/expand/js15-in.jsonld", output: "w3c/expand/js15-out.jsonld"},
		{id: "tjs16", name: "Expand JSON literal aleady in expanded form with aliased keys", ld10: false, ld11: true, input: "w3c/expand/js16-in.jsonld", output: "w3c/expand/js16-out.jsonld"},
		{id: "tjs17", name: "Expand JSON literal (string)", ld10: false, ld11: true, input: "w3c/expand/js17-in.jsonld", output: "w3c/expand/js17-out.jsonld"},
		{id: "tjs18", name: "Expand JSON literal (null)", ld10: false, ld11: true, input: "w3c/expand/js18-in.jsonld", output: "w3c/expand/js18-out.jsonld"},
		{id: "tjs19", name: "Expand JSON literal with aliased @type", ld10: false, ld11: true, input: "w3c/expand/js19-in.jsonld", output: "w3c/expand/js19-out.jsonld"},
		{id: "tjs20", name: "Expand JSON literal with aliased @value", ld10: false, ld11: true, input: "w3c/expand/js20-in.jsonld", output: "w3c/expand/js20-out.jsonld"},
		{id: "tjs21", name: "Expand JSON literal with @context", ld10: false, ld11: true, input: "w3c/expand/js21-in.jsonld", output: "w3c/expand/js21-out.jsonld"},
		{id: "tjs22", name: "Expand JSON literal (null) aleady in expanded form.", ld10: false, ld11: true, input: "w3c/expand/js22-in.jsonld", output: "w3c/expand/js22-out.jsonld"},
		{id: "tjs23", name: "Expand JSON literal (empty array).", ld10: false, ld11: true, input: "w3c/expand/js23-in.jsonld", output: "w3c/expand/js23-out.jsonld"},
		{id: "tl001", name: "Language map with null value", ld10: false, ld11: true, input: "w3c/expand/l001-in.jsonld", output: "w3c/expand/l001-out.jsonld"},
		{id: "tli01", name: "@list containing @list", ld10: false, ld11: true, input: "w3c/expand/li01-in.jsonld", output: "w3c/expand/li01-out.jsonld"},
		{id: "tli02", name: "@list containing empty @list", ld10: false, ld11: true, input: "w3c/expand/li02-in.jsonld", output: "w3c/expand/li02-out.jsonld"},
		{id: "tli03", name: "@list containing @list (with coercion)", ld10: false, ld11: true, input: "w3c/expand/li03-in.jsonld", output: "w3c/expand/li03-out.jsonld"},
		{id: "tli04", name: "@list containing empty @list (with coercion)", ld10: false, ld11: true, input: "w3c/expand/li04-in.jsonld", output: "w3c/expand/li04-out.jsonld"},
		{id: "tli05", name: "coerced @list containing an array", ld10: false, ld11: true, input: "w3c/expand/li05-in.jsonld", output: "w3c/expand/li05-out.jsonld"},
		{id: "tli06", name: "coerced @list containing an empty array", ld10: false, ld11: true, input: "w3c/expand/li06-in.jsonld", output: "w3c/expand/li06-out.jsonld"},
		{id: "tli07", name: "coerced @list containing deep arrays", ld10: false, ld11: true, input: "w3c/expand/li07-in.jsonld", output: "w3c/expand/li07-out.jsonld"},
		{id: "tli08", name: "coerced @list containing deep empty arrays", ld10: false, ld11: true, input: "w3c/expand/li08-in.jsonld", output: "w3c/expand/li08-out.jsonld"},
		{id: "tli09", name: "coerced @list containing multiple lists", ld10: false, ld11: true, input: "w3c/expand/li09-in.jsonld", output: "w3c/expand/li09-out.jsonld"},
		{id: "tli10", name: "coerced @list containing mixed list values", ld10: false, ld11: true, input: "w3c/expand/li10-in.jsonld", output: "w3c/expand/li10-out.jsonld"},
		{id: "tm001", name: "Adds @id to object not having an @id", ld10: false, ld11: true, input: "w3c/expand/m001-in.jsonld", output: "w3c/expand/m001-out.jsonld"},
		{id: "tm002", name: "Retains @id in object already having an @id", ld10: false, ld11: true, input: "w3c/expand/m002-in.jsonld", output: "w3c/expand/m002-out.jsonld"},
		{id: "tm003", name: "Adds @type to object not having an @type", ld10: false, ld11: true, input: "w3c/expand/m003-in.jsonld", output: "w3c/expand/m003-out.jsonld"},
		{id: "tm004", name: "Prepends @type in object already having an @type", ld10: false, ld11: true, input: "w3c/expand/m004-in.jsonld", output: "w3c/expand/m004-out.jsonld"},
		{id: "tm005", name: "Adds expanded @id to object", ld10: false, ld11: true, base: "http://example.org/", input: "w3c/expand/m005-in.jsonld", output: "w3c/expand/m005-out.jsonld"},
		{id: "tm006", name: "Adds vocabulary expanded @type to object", ld10: false, ld11: true, input: "w3c/expand/m006-in.jsonld", output: "w3c/expand/m006-out.jsonld"},
		{id: "tm007", name: "Adds document expanded @type to object", ld10: false, ld11: true, input: "w3c/expand/m007-in.jsonld", output: "w3c/expand/m007-out.jsonld"},
		{id: "tm008", name: "When type is in a type map", ld10: false, ld11: true, input: "w3c/expand/m008-in.jsonld", output: "w3c/expand/m008-out.jsonld"},
		{id: "tm009", name: "language map with @none", ld10: false, ld11: true, input: "w3c/expand/m009-in.jsonld", output: "w3c/expand/m009-out.jsonld"},
		{id: "tm010", name: "language map with alias of @none", ld10: false, ld11: true, input: "w3c/expand/m010-in.jsonld", output: "w3c/expand/m010-out.jsonld"},
		{id: "tm011", name: "id map with @none", ld10: false, ld11: true, input: "w3c/expand/m011-in.jsonld", output: "w3c/expand/m011-out.jsonld"},
		{id: "tm012", name: "type map with alias of @none", ld10: false, ld11: true, input: "w3c/expand/m012-in.jsonld", output: "w3c/expand/m012-out.jsonld"},
		{id: "tm013", name: "graph index map with @none", ld10: false, ld11: true, input: "w3c/expand/m013-in.jsonld", output: "w3c/expand/m013-out.jsonld"},
		{id: "tm014", name: "graph index map with alias @none", ld10: false, ld11: true, input: "w3c/expand/m014-in.jsonld", output: "w3c/expand/m014-out.jsonld"},
		{id: "tm015", name: "graph id index map with aliased @none", ld10: false, ld11: true, input: "w3c/expand/m015-in.jsonld", output: "w3c/expand/m015-out.jsonld"},
		{id: "tm016", name: "graph id index map with aliased @none", ld10: false, ld11: true, input: "w3c/expand/m016-in.jsonld", output: "w3c/expand/m016-out.jsonld"},
		{id: "tm017", name: "string value of type map expands to node reference", ld10: false, ld11: true, input: "w3c/expand/m017-in.jsonld", output: "w3c/expand/m017-out.jsonld"},
		{id: "tm018", name: "string value of type map expands to node reference with @type: @id", ld10: false, ld11: true, input: "w3c/expand/m018-in.jsonld", output: "w3c/expand/m018-out.jsonld"},
		{id: "tm019", name: "string value of type map expands to node reference with @type: @vocab", ld10: false, ld11: true, input: "w3c/expand/m019-in.jsonld", output: "w3c/expand/m019-out.jsonld"},
		{id: "tm020", name: "string value of type map must not be a literal", ld10: false, ld11: true, input: "w3c/expand/m020-in.jsonld", err: "invalid type mapping"},
		{id: "tn001", name: "Expands input using @nest", ld10: false, ld11: true, input: "w3c/expand/n001-in.jsonld", output: "w3c/expand/n001-out.jsonld"},
		{id: "tn002", name: "Expands input using aliased @nest", ld10: false, ld11: true, input: "w3c/expand/n002-in.jsonld", output: "w3c/expand/n002-out.jsonld"},
		{id: "tn003", name: "Appends nested values when property at base and nested", ld10: false, ld11: true, input: "w3c/expand/n003-in.jsonld", output: "w3c/expand/n003-out.jsonld"},
		{id: "tn004", name: "Appends nested values from all @nest aliases", ld10: false, ld11: true, input: "w3c/expand/n004-in.jsonld", output: "w3c/expand/n004-out.jsonld"},
		{id: "tn005", name: "Nested nested containers", ld10: false, ld11: true, input: "w3c/expand/n005-in.jsonld", output: "w3c/expand/n005-out.jsonld"},
		{id: "tn006", name: "Arrays of nested values", ld10: false, ld11: true, input: "w3c/expand/n006-in.jsonld", output: "w3c/expand/n006-out.jsonld"},
		{id: "tn007", name: "A nest of arrays", ld10: false, ld11: true, input: "w3c/expand/n007-in.jsonld", output: "w3c/expand/n007-out.jsonld"},
		{id: "tn008", name: "Multiple keys may mapping to @type when nesting", ld10: false, ld11: true, input: "w3c/expand/n008-in.jsonld", output: "w3c/expand/n008-out.jsonld"},
		{id: "tp001", name: "@version may be specified after first context", ld10: false, ld11: true, input: "w3c/expand/p001-in.jsonld", output: "w3c/expand/p001-out.jsonld"},
		{id: "tp002", name: "@version setting [1.0, 1.1, 1.0]", ld10: false, ld11: true, input: "w3c/expand/p002-in.jsonld", output: "w3c/expand/p002-out.jsonld"},
		{id: "tp003", name: "@version setting [1.1, 1.0]", ld10: false, ld11: true, input: "w3c/expand/p003-in.jsonld", output: "w3c/expand/p003-out.jsonld"},
		{id: "tp004", name: "@version setting [1.1, 1.0, 1.1]", ld10: false, ld11: true, input: "w3c/expand/p004-in.jsonld", output: "w3c/expand/p004-out.jsonld"},
		{id: "tpi01", name: "error if @version is json-ld-1.0 for property-valued index", ld10: true, ld11: false, input: "w3c/expand/pi01-in.jsonld", err: "invalid term definition"},
		{id: "tpi02", name: "error if @container does not include @index for property-valued index", ld10: false, ld11: true, input: "w3c/expand/pi02-in.jsonld", err: "invalid term definition"},
		{id: "tpi03", name: "error if @index is a keyword for property-valued index", ld10: false, ld11: true, input: "w3c/expand/pi03-in.jsonld", err: "invalid term definition"},
		{id: "tpi04", name: "error if @index is not a string for property-valued index", ld10: false, ld11: true, input: "w3c/expand/pi04-in.jsonld", err: "invalid term definition"},
		{id: "tpi05", name: "error if attempting to add property to value object for property-valued index", ld10: false, ld11: true, input: "w3c/expand/pi05-in.jsonld", err: "invalid value object"},
		{id: "tpi06", name: "property-valued index expands to property value, instead of @index (value)", ld10: false, ld11: true, input: "w3c/expand/pi06-in.jsonld", output: "w3c/expand/pi06-out.jsonld"},
		{id: "tpi07", name: "property-valued index appends to property value, instead of @index (value)", ld10: false, ld11: true, input: "w3c/expand/pi07-in.jsonld", output: "w3c/expand/pi07-out.jsonld"},
		{id: "tpi08", name: "property-valued index expands to property value, instead of @index (node)", ld10: false, ld11: true, input: "w3c/expand/pi08-in.jsonld", output: "w3c/expand/pi08-out.jsonld"},
		{id: "tpi09", name: "property-valued index appends to property value, instead of @index (node)", ld10: false, ld11: true, input: "w3c/expand/pi09-in.jsonld", output: "w3c/expand/pi09-out.jsonld"},
		{id: "tpi10", name: "property-valued index does not output property for @none", ld10: false, ld11: true, input: "w3c/expand/pi10-in.jsonld", output: "w3c/expand/pi10-out.jsonld"},
		{id: "tpi11", name: "property-valued index adds property to graph object", ld10: false, ld11: true, input: "w3c/expand/pi11-in.jsonld", output: "w3c/expand/pi11-out.jsonld"},
		{id: "tpr01", name: "Protect a term", ld10: false, ld11: true, input: "w3c/expand/pr01-in.jsonld", err: "protected term redefinition"},
		{id: "tpr02", name: "Set a term to not be protected", ld10: false, ld11: true, input: "w3c/expand/pr02-in.jsonld", output: "w3c/expand/pr02-out.jsonld"},
		{id: "tpr03", name: "Protect all terms in context", ld10: false, ld11: true, input: "w3c/expand/pr03-in.jsonld", err: "protected term redefinition"},
		{id: "tpr04", name: "Do not protect term with @protected: false", ld10: false, ld11: true, input: "w3c/expand/pr04-in.jsonld", err: "protected term redefinition"},
		{id: "tpr05", name: "Clear active context with protected terms from an embedded context", ld10: false, ld11: true, input: "w3c/expand/pr05-in.jsonld", err: "invalid context nullification"},
		{id: "tpr06", name: "Clear active context of protected terms from a term.", ld10: false, ld11: true, input: "w3c/expand/pr06-in.jsonld", output: "w3c/expand/pr06-out.jsonld"},
		{id: "tpr08", name: "Term with protected scoped context.", ld10: false, ld11: true, input: "w3c/expand/pr08-in.jsonld", err: "protected term redefinition"},
		{id: "tpr09", name: "Attempt to redefine term in other protected context.", ld10: false, ld11: true, input: "w3c/expand/pr09-in.jsonld", err: "protected term redefinition"},
		{id: "tpr10", name: "Simple protected and unprotected terms.", ld10: false, ld11: true, input: "w3c/expand/pr10-in.jsonld", output: "w3c/expand/pr10-out.jsonld"},
		{id: "tpr11", name: "Fail to override protected term.", ld10: false, ld11: true, input: "w3c/expand/pr11-in.jsonld", err: "protected term redefinition"},
		{id: "tpr12", name: "Scoped context fail to override protected term.", ld10: false, ld11: true, input: "w3c/expand/pr12-in.jsonld", err: "protected term redefinition"},
		{id: "tpr13", name: "Override unprotected term.", ld10: false, ld11: true, input: "w3c/expand/pr13-in.jsonld", output: "w3c/expand/pr13-out.jsonld"},
		{id: "tpr14", name: "Clear protection with null context.", ld10: false, ld11: true, input: "w3c/expand/pr14-in.jsonld", output: "w3c/expand/pr14-out.jsonld"},
		{id: "tpr15", name: "Clear protection with array with null context", ld10: false, ld11: true, input: "w3c/expand/pr15-in.jsonld", output: "w3c/expand/pr15-out.jsonld"},
		{id: "tpr16", name: "Override protected terms after null.", ld10: false, ld11: true, input: "w3c/expand/pr16-in.jsonld", output: "w3c/expand/pr16-out.jsonld"},
		{id: "tpr17", name: "Fail to override protected terms with type.", ld10: false, ld11: true, input: "w3c/expand/pr17-in.jsonld", err: "invalid context nullification"},
		{id: "tpr18", name: "Fail to override protected terms with type+null+ctx.", ld10: false, ld11: true, input: "w3c/expand/pr18-in.jsonld", err: "invalid context nullification"},
		{id: "tpr19", name: "Mix of protected and unprotected terms.", ld10: false, ld11: true, input: "w3c/expand/pr19-in.jsonld", output: "w3c/expand/pr19-out.jsonld"},
		{id: "tpr20", name: "Fail with mix of protected and unprotected terms with type+null+ctx.", ld10: false, ld11: true, input: "w3c/expand/pr20-in.jsonld", err: "invalid context nullification"},
		{id: "tpr21", name: "Fail with mix of protected and unprotected terms with type+null.", ld10: false, ld11: true, input: "w3c/expand/pr21-in.jsonld", err: "invalid context nullification"},
		{id: "tpr22", name: "Check legal overriding of type-scoped protected term from nested node.", ld10: false, ld11: true, input: "w3c/expand/pr22-in.jsonld", output: "w3c/expand/pr22-out.jsonld"},
		{id: "tpr23", name: "Allows redefinition of protected alias term with same definition.", ld10: false, ld11: true, input: "w3c/expand/pr23-in.jsonld", output: "w3c/expand/pr23-out.jsonld"},
		{id: "tpr24", name: "Allows redefinition of protected prefix term with same definition.", ld10: false, ld11: true, input: "w3c/expand/pr24-in.jsonld", output: "w3c/expand/pr24-out.jsonld"},
		{id: "tpr25", name: "Allows redefinition of terms with scoped contexts using same definitions.", ld10: false, ld11: true, input: "w3c/expand/pr25-in.jsonld", output: "w3c/expand/pr25-out.jsonld"},
		{id: "tpr26", name: "Fails on redefinition of terms with scoped contexts using different definitions.", ld10: false, ld11: true, input: "w3c/expand/pr26-in.jsonld", err: "protected term redefinition"},
		{id: "tpr27", name: "Allows redefinition of protected alias term with same definition modulo protected flag.", ld10: false, ld11: true, input: "w3c/expand/pr27-in.jsonld", output: "w3c/expand/pr27-out.jsonld"},
		{id: "tpr28", name: "Fails if trying to redefine a protected null term.", ld10: false, ld11: true, input: "w3c/expand/pr28-in.jsonld", err: "protected term redefinition"},
		{id: "tpr29", name: "Does not expand a Compact IRI using a non-prefix term.", ld10: false, ld11: true, input: "w3c/expand/pr29-in.jsonld", output: "w3c/expand/pr29-out.jsonld"},
		{id: "tpr30", name: "Keywords may be protected.", ld10: false, ld11: true, input: "w3c/expand/pr30-in.jsonld", output: "w3c/expand/pr30-out.jsonld"},
		{id: "tpr31", name: "Protected keyword aliases cannot be overridden.", ld10: false, ld11: true, input: "w3c/expand/pr31-in.jsonld", err: "protected term redefinition"},
		{id: "tpr32", name: "Protected @type cannot be overridden.", ld10: false, ld11: true, input: "w3c/expand/pr32-in.jsonld", err: "protected term redefinition"},
		{id: "tpr33", name: "Fails if trying to declare a keyword alias as prefix.", ld10: false, ld11: true, input: "w3c/expand/pr33-in.jsonld", err: "invalid term definition"},
		{id: "tpr34", name: "Ignores a non-keyword term starting with '@'", ld10: false, ld11: true, input: "w3c/expand/pr34-in.jsonld", output: "w3c/expand/pr34-out.jsonld"},
		{id: "tpr35", name: "Ignores a non-keyword term starting with '@' (with @vocab)", ld10: false, ld11: true, input: "w3c/expand/pr35-in.jsonld", output: "w3c/expand/pr35-out.jsonld"},
		{id: "tpr36", name: "Ignores a term mapping to a value in the form of a keyword.", ld10: false, ld11: true, input: "w3c/expand/pr36-in.jsonld", output: "w3c/expand/pr36-out.jsonld"},
		{id: "tpr37", name: "Ignores a term mapping to a value in the form of a keyword (with @vocab).", ld10: false, ld11: true, input: "w3c/expand/pr37-in.jsonld", output: "w3c/expand/pr37-out.jsonld"},
		{id: "tpr38", name: "Ignores a term mapping to a value in the form of a keyword (@reverse).", ld10: false, ld11: true, input: "w3c/expand/pr38-in.jsonld", output: "w3c/expand/pr38-out.jsonld"},
		{id: "tpr39", name: "Ignores a term mapping to a value in the form of a keyword (@reverse with @vocab).", ld10: false, ld11: true, input: "w3c/expand/pr39-in.jsonld", output: "w3c/expand/pr39-out.jsonld"},
		{id: "tpr40", name: "Protected terms and property-scoped contexts", ld10: false, ld11: true, input: "w3c/expand/pr40-in.jsonld", output: "w3c/expand/pr40-out.jsonld"},
		{id: "tpr41", name: "Allows protected redefinition of equivalent id terms", ld10: false, ld11: true, input: "w3c/expand/pr41-in.jsonld", output: "w3c/expand/pr41-out.jsonld"},
		{id: "tpr42", name: "Fail if protected flag not retained during redefinition", ld10: false, ld11: true, input: "w3c/expand/pr42-in.jsonld", err: "protected term redefinition"},
		{id: "tpr43", name: "Clear protection in @graph @container with null context.", ld10: false, ld11: true, input: "w3c/expand/pr43-in.jsonld", output: "w3c/expand/pr43-out.jsonld"},
		{id: "tso01", name: "@import is invalid in 1.0.", ld10: true, ld11: false, input: "w3c/expand/so01-in.jsonld", err: "invalid context entry"},
		{id: "tso02", name: "@import must be a string", ld10: false, ld11: true, input: "w3c/expand/so02-in.jsonld", err: "invalid @import value"},
		{id: "tso03", name: "@import overflow", ld10: false, ld11: true, input: "w3c/expand/so03-in.jsonld", err: "invalid context entry"},
		{id: "tso05", name: "@propagate: true on type-scoped context with @import", ld10: false, ld11: true, input: "w3c/expand/so05-in.jsonld", output: "w3c/expand/so05-out.jsonld"},
		{id: "tso06", name: "@propagate: false on property-scoped context with @import", ld10: false, ld11: true, input: "w3c/expand/so06-in.jsonld", output: "w3c/expand/so06-out.jsonld"},
		{id: "tso07", name: "Protect all terms in sourced context", ld10: false, ld11: true, input: "w3c/expand/so07-in.jsonld", err: "protected term redefinition"},
		{id: "tso08", name: "Override term defined in sourced context", ld10: false, ld11: true, input: "w3c/expand/so08-in.jsonld", output: "w3c/expand/so08-out.jsonld"},
		{id: "tso09", name: "Override @vocab defined in sourced context", ld10: false, ld11: true, input: "w3c/expand/so09-in.jsonld", output: "w3c/expand/so09-out.jsonld"},
		{id: "tso10", name: "Protect terms in sourced context", ld10: false, ld11: true, input: "w3c/expand/so10-in.jsonld", err: "protected term redefinition"},
		{id: "tso11", name: "Override protected terms in sourced context", ld10: false, ld11: true, input: "w3c/expand/so11-in.jsonld", output: "w3c/expand/so11-out.jsonld"},
		{id: "tso12", name: "@import may not be used in an imported context.", ld10: false, ld11: true, input: "w3c/expand/so12-in.jsonld", err: "invalid context entry"},
		{id: "tso13", name: "@import can only reference a single context", ld10: false, ld11: true, input: "w3c/expand/so13-in.jsonld", err: "invalid remote context"},
		{id: "ttn01", name: "@type: @none is illegal in 1.0.", ld10: true, ld11: false, input: "w3c/expand/tn01-in.jsonld", err: "invalid type mapping"},
		{id: "ttn02", name: "@type: @none expands strings as value objects", ld10: false, ld11: true, input: "w3c/expand/tn02-in.jsonld", output: "w3c/expand/tn02-out.jsonld"},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s-%s", tc.id, tc.name), func(t *testing.T) {
			t.Parallel()

			input := LoadData(t, tc.input)
			var want json.RawMessage
			if tc.output == "" {
				want = json.RawMessage(`null`)
			} else {
				want = LoadData(t, tc.output)
			}

			docIRI := fmt.Sprintf("https://w3c.github.io/json-ld-api/tests/expand/%s-in.jsonld", tc.id[1:])

			t.Run("json-ld-1.0", func(t *testing.T) {
				t.Parallel()
				tc := tc
				if !tc.ld10 {
					t.Skip()
				}

				var expContext json.RawMessage
				if tc.expandContext != "" {
					expContext = LoadData(t, tc.expandContext)
				}

				p := ld.NewProcessor(
					ld.WithBaseIRI(tc.base),
					ld.WithRemoteContextLoader(FileLoader(t)),
					ld.With10Processing(true),
					ld.WithExpandContext(expContext),
					ld.WithLogger(slog.New(slog.DiscardHandler)),
				)

				expanded, err := p.Expand(bytes.NewReader(input), docIRI)

				if tc.err != "" && err == nil {
					t.Fatalf("expected error: %s, got nil", tc.err)
				}

				if tc.err == "" && err != nil {
					t.Fatalf("expected no error, got: %s", err)
				}

				if tc.err != "" && err != nil {
					if !strings.Contains(err.Error(), tc.err) {
						t.Fatalf("expected error: %s, got: %s", tc.err, err)
					}
				} else {
					got, err := json.Marshal(expanded)
					if err != nil {
						t.Fatalf("failed to marshal to expanded JSON: %s", err)
					}
					if diff := cmp.Diff(want, json.RawMessage(got), JSONDiff()); diff != "" {
						if *dump {
							data, _ := json.MarshalIndent(expanded, "", "    ")
							t.Logf("expanded from: %s", string(data))
						}
						t.Errorf("expansion mismatch (-want +got):\n%s", diff)
					}
				}
			})

			t.Run("json-ld-1.1", func(t *testing.T) {
				t.Parallel()
				tc := tc
				if !tc.ld11 {
					t.Skip()
				}

				var expContext json.RawMessage
				if tc.expandContext != "" {
					expContext = LoadData(t, tc.expandContext)
				}

				p := ld.NewProcessor(
					ld.WithBaseIRI(tc.base),
					ld.WithRemoteContextLoader(FileLoader(t)),
					ld.With10Processing(false),
					ld.WithExpandContext(expContext),
					ld.WithLogger(slog.New(slog.DiscardHandler)),
				)

				expanded, err := p.Expand(bytes.NewReader(input), docIRI)

				if tc.err != "" && err == nil {
					t.Fatalf("expected error: %s, got nil", tc.err)
				}

				if tc.err == "" && err != nil {
					t.Fatalf("expected no error, got: %s", err)
				}

				if tc.err != "" && err != nil {
					if !strings.Contains(err.Error(), tc.err) {
						t.Fatalf("expected error: %s, got: %s", tc.err, err)
					}
				} else {
					got, err := json.Marshal(expanded)
					if err != nil {
						t.Fatalf("failed to marshal to expanded JSON: %s", err)
					}
					if diff := cmp.Diff(want, json.RawMessage(got), JSONDiff()); diff != "" {
						if *dump {
							data, _ := json.MarshalIndent(expanded, "", "    ")
							t.Logf("expanded from: %s", string(data))
						}
						t.Errorf("expansion mismatch (-want +got):\n%s", diff)
					}
				}
			})
		})
	}
}
