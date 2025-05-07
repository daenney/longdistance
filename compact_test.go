package longdistance_test

import (
	"fmt"
	"strings"
	"testing"

	ld "sourcery.dny.nu/longdistance"
	"sourcery.dny.nu/longdistance/internal/json"
	"github.com/google/go-cmp/cmp"
)

// TestCompact runs the W3C compaction tests.
//
// Despite the fact that this is the compaction test suite, many of the
// inputs are in compacted or partially expanded form. So all inputs
// have to be expanded first, before we attempt compaction.
func TestCompact(t *testing.T) {
	tests := []struct {
		id, name                     string
		ld10, ld11                   bool
		input, context, output, base string
		compactArrays                bool
		compactToRelative            bool
		err                          string
	}{
		{id: "t0001", name: "drop free-floating nodes", ld10: true, ld11: true, input: "testdata/w3c/compact/0001-in.jsonld", output: "testdata/w3c/compact/0001-out.jsonld", context: "testdata/w3c/compact/0001-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0002", name: "basic", ld10: true, ld11: true, input: "testdata/w3c/compact/0002-in.jsonld", output: "testdata/w3c/compact/0002-out.jsonld", context: "testdata/w3c/compact/0002-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0003", name: "drop null and unmapped properties", ld10: true, ld11: true, input: "testdata/w3c/compact/0003-in.jsonld", output: "testdata/w3c/compact/0003-out.jsonld", context: "testdata/w3c/compact/0003-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0004", name: "optimize @set, keep empty arrays", ld10: true, ld11: true, input: "testdata/w3c/compact/0004-in.jsonld", output: "testdata/w3c/compact/0004-out.jsonld", context: "testdata/w3c/compact/0004-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0005", name: "@type and prefix compaction", ld10: true, ld11: true, input: "testdata/w3c/compact/0005-in.jsonld", output: "testdata/w3c/compact/0005-out.jsonld", context: "testdata/w3c/compact/0005-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0006", name: "keep expanded object format if @type doesn't match", ld10: true, ld11: true, input: "testdata/w3c/compact/0006-in.jsonld", output: "testdata/w3c/compact/0006-out.jsonld", context: "testdata/w3c/compact/0006-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0007", name: "add context", ld10: true, ld11: true, input: "testdata/w3c/compact/0007-in.jsonld", output: "testdata/w3c/compact/0007-out.jsonld", context: "testdata/w3c/compact/0007-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0008", name: "alias keywords", ld10: true, ld11: true, input: "testdata/w3c/compact/0008-in.jsonld", output: "testdata/w3c/compact/0008-out.jsonld", context: "testdata/w3c/compact/0008-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0009", name: "compact @id", ld10: true, ld11: true, input: "testdata/w3c/compact/0009-in.jsonld", output: "testdata/w3c/compact/0009-out.jsonld", context: "testdata/w3c/compact/0009-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0010", name: "array to @graph", ld10: true, ld11: true, input: "testdata/w3c/compact/0010-in.jsonld", output: "testdata/w3c/compact/0010-out.jsonld", context: "testdata/w3c/compact/0010-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0011", name: "compact date", ld10: true, ld11: true, input: "testdata/w3c/compact/0011-in.jsonld", output: "testdata/w3c/compact/0011-out.jsonld", context: "testdata/w3c/compact/0011-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0012", name: "native types", ld10: true, ld11: true, input: "testdata/w3c/compact/0012-in.jsonld", output: "testdata/w3c/compact/0012-out.jsonld", context: "testdata/w3c/compact/0012-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0013", name: "@value with @language", ld10: true, ld11: true, input: "testdata/w3c/compact/0013-in.jsonld", output: "testdata/w3c/compact/0013-out.jsonld", context: "testdata/w3c/compact/0013-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0014", name: "array to aliased @graph", ld10: true, ld11: true, input: "testdata/w3c/compact/0014-in.jsonld", output: "testdata/w3c/compact/0014-out.jsonld", context: "testdata/w3c/compact/0014-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0015", name: "best match compaction", ld10: true, ld11: true, input: "testdata/w3c/compact/0015-in.jsonld", output: "testdata/w3c/compact/0015-out.jsonld", context: "testdata/w3c/compact/0015-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0016", name: "recursive named graphs", ld10: true, ld11: true, input: "testdata/w3c/compact/0016-in.jsonld", output: "testdata/w3c/compact/0016-out.jsonld", context: "testdata/w3c/compact/0016-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0017", name: "A term mapping to null removes the mapping", ld10: true, ld11: true, input: "testdata/w3c/compact/0017-in.jsonld", output: "testdata/w3c/compact/0017-out.jsonld", context: "testdata/w3c/compact/0017-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0018", name: "best matching term for lists", ld10: true, ld11: true, input: "testdata/w3c/compact/0018-in.jsonld", output: "testdata/w3c/compact/0018-out.jsonld", context: "testdata/w3c/compact/0018-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0019", name: "Keep duplicate values in @list and @set", ld10: true, ld11: true, input: "testdata/w3c/compact/0019-in.jsonld", output: "testdata/w3c/compact/0019-out.jsonld", context: "testdata/w3c/compact/0019-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0020", name: "Compact @id that is a property IRI when @container is @list", ld10: true, ld11: true, input: "testdata/w3c/compact/0020-in.jsonld", output: "testdata/w3c/compact/0020-out.jsonld", context: "testdata/w3c/compact/0020-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0021", name: "Compact properties and types using @vocab", ld10: true, ld11: true, input: "testdata/w3c/compact/0021-in.jsonld", output: "testdata/w3c/compact/0021-out.jsonld", context: "testdata/w3c/compact/0021-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0022", name: "@list compaction of nested properties", ld10: true, ld11: true, input: "testdata/w3c/compact/0022-in.jsonld", output: "testdata/w3c/compact/0022-out.jsonld", context: "testdata/w3c/compact/0022-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0023", name: "prefer @vocab over compacted IRIs", ld10: true, ld11: true, input: "testdata/w3c/compact/0023-in.jsonld", output: "testdata/w3c/compact/0023-out.jsonld", context: "testdata/w3c/compact/0023-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0024", name: "most specific term matching in @list.", ld10: true, ld11: true, input: "testdata/w3c/compact/0024-in.jsonld", output: "testdata/w3c/compact/0024-out.jsonld", context: "testdata/w3c/compact/0024-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0025", name: "Language maps", ld10: true, ld11: true, input: "testdata/w3c/compact/0025-in.jsonld", output: "testdata/w3c/compact/0025-out.jsonld", context: "testdata/w3c/compact/0025-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0026", name: "Language map term selection with complications", ld10: true, ld11: true, input: "testdata/w3c/compact/0026-in.jsonld", output: "testdata/w3c/compact/0026-out.jsonld", context: "testdata/w3c/compact/0026-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0027", name: "@container: @set with multiple values", ld10: true, ld11: true, input: "testdata/w3c/compact/0027-in.jsonld", output: "testdata/w3c/compact/0027-out.jsonld", context: "testdata/w3c/compact/0027-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0028", name: "Alias keywords and use @vocab", ld10: true, ld11: true, input: "testdata/w3c/compact/0028-in.jsonld", output: "testdata/w3c/compact/0028-out.jsonld", context: "testdata/w3c/compact/0028-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0029", name: "Simple @index map", ld10: true, ld11: true, input: "testdata/w3c/compact/0029-in.jsonld", output: "testdata/w3c/compact/0029-out.jsonld", context: "testdata/w3c/compact/0029-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0030", name: "non-matching @container: @index", ld10: true, ld11: true, input: "testdata/w3c/compact/0030-in.jsonld", output: "testdata/w3c/compact/0030-out.jsonld", context: "testdata/w3c/compact/0030-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0031", name: "Compact @reverse", ld10: true, ld11: true, input: "testdata/w3c/compact/0031-in.jsonld", output: "testdata/w3c/compact/0031-out.jsonld", context: "testdata/w3c/compact/0031-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0032", name: "Compact keys in reverse-maps", ld10: true, ld11: true, input: "testdata/w3c/compact/0032-in.jsonld", output: "testdata/w3c/compact/0032-out.jsonld", context: "testdata/w3c/compact/0032-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0033", name: "Compact reverse-map to reverse property", ld10: true, ld11: true, input: "testdata/w3c/compact/0033-in.jsonld", output: "testdata/w3c/compact/0033-out.jsonld", context: "testdata/w3c/compact/0033-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0034", name: "Skip property with @reverse if no match", ld10: true, ld11: true, input: "testdata/w3c/compact/0034-in.jsonld", output: "testdata/w3c/compact/0034-out.jsonld", context: "testdata/w3c/compact/0034-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0035", name: "Compact @reverse node references using strings", ld10: true, ld11: true, input: "testdata/w3c/compact/0035-in.jsonld", output: "testdata/w3c/compact/0035-out.jsonld", context: "testdata/w3c/compact/0035-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0036", name: "Compact reverse properties using index containers", ld10: true, ld11: true, input: "testdata/w3c/compact/0036-in.jsonld", output: "testdata/w3c/compact/0036-out.jsonld", context: "testdata/w3c/compact/0036-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0037", name: "Compact keys in @reverse using @vocab", ld10: true, ld11: true, input: "testdata/w3c/compact/0037-in.jsonld", output: "testdata/w3c/compact/0037-out.jsonld", context: "testdata/w3c/compact/0037-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0038", name: "Index map round-tripping", ld10: false, ld11: false, input: "testdata/w3c/compact/0038-in.jsonld", output: "testdata/w3c/compact/0038-out.jsonld", context: "testdata/w3c/compact/0038-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "ta038", name: "Index map round-tripping", ld10: true, ld11: true, input: "testdata/w3c/compact/0038-in.jsonld", output: "testdata/w3c/compact/0038a-out.jsonld", context: "testdata/w3c/compact/0038-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0039", name: "@graph is array", ld10: true, ld11: true, input: "testdata/w3c/compact/0039-in.jsonld", output: "testdata/w3c/compact/0039-out.jsonld", context: "testdata/w3c/compact/0039-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0040", name: "@list is array", ld10: true, ld11: true, input: "testdata/w3c/compact/0040-in.jsonld", output: "testdata/w3c/compact/0040-out.jsonld", context: "testdata/w3c/compact/0040-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0041", name: "index rejects term having @list", ld10: true, ld11: true, input: "testdata/w3c/compact/0041-in.jsonld", output: "testdata/w3c/compact/0041-out.jsonld", context: "testdata/w3c/compact/0041-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0042", name: "@list keyword aliasing", ld10: true, ld11: true, input: "testdata/w3c/compact/0042-in.jsonld", output: "testdata/w3c/compact/0042-out.jsonld", context: "testdata/w3c/compact/0042-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0043", name: "select term over @vocab", ld10: true, ld11: true, input: "testdata/w3c/compact/0043-in.jsonld", output: "testdata/w3c/compact/0043-out.jsonld", context: "testdata/w3c/compact/0043-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0044", name: "@type: @vocab in reverse-map", ld10: true, ld11: true, input: "testdata/w3c/compact/0044-in.jsonld", output: "testdata/w3c/compact/0044-out.jsonld", context: "testdata/w3c/compact/0044-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0045", name: "@id value uses relative IRI, not term", ld10: true, ld11: true, input: "testdata/w3c/compact/0045-in.jsonld", output: "testdata/w3c/compact/0045-out.jsonld", context: "testdata/w3c/compact/0045-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0046", name: "multiple objects without @context use @graph", ld10: true, ld11: true, input: "testdata/w3c/compact/0046-in.jsonld", output: "testdata/w3c/compact/0046-out.jsonld", context: "testdata/w3c/compact/0046-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0047", name: "Round-trip relative URLs", ld10: true, ld11: true, input: "testdata/w3c/compact/0047-in.jsonld", output: "testdata/w3c/compact/0047-out.jsonld", context: "testdata/w3c/compact/0047-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0048", name: "term with @language: null", ld10: true, ld11: true, input: "testdata/w3c/compact/0048-in.jsonld", output: "testdata/w3c/compact/0048-out.jsonld", context: "testdata/w3c/compact/0048-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0049", name: "Round tripping of lists that contain just IRIs", ld10: true, ld11: true, input: "testdata/w3c/compact/0049-in.jsonld", output: "testdata/w3c/compact/0049-out.jsonld", context: "testdata/w3c/compact/0049-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0050", name: "Reverse properties require @type: @id to use string values", ld10: true, ld11: true, input: "testdata/w3c/compact/0050-in.jsonld", output: "testdata/w3c/compact/0050-out.jsonld", context: "testdata/w3c/compact/0050-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0051", name: "Round tripping @list with scalar", ld10: true, ld11: true, input: "testdata/w3c/compact/0051-in.jsonld", output: "testdata/w3c/compact/0051-out.jsonld", context: "testdata/w3c/compact/0051-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0052", name: "Round tripping @list with scalar and @graph alias", ld10: true, ld11: true, input: "testdata/w3c/compact/0052-in.jsonld", output: "testdata/w3c/compact/0052-out.jsonld", context: "testdata/w3c/compact/0052-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0053", name: "Use @type: @vocab if no @type: @id", ld10: true, ld11: true, input: "testdata/w3c/compact/0053-in.jsonld", output: "testdata/w3c/compact/0053-out.jsonld", context: "testdata/w3c/compact/0053-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0054", name: "Compact to @type: @vocab and compact @id to term", ld10: true, ld11: true, input: "testdata/w3c/compact/0054-in.jsonld", output: "testdata/w3c/compact/0054-out.jsonld", context: "testdata/w3c/compact/0054-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0055", name: "Round tripping @type: @vocab", ld10: true, ld11: true, input: "testdata/w3c/compact/0055-in.jsonld", output: "testdata/w3c/compact/0055-out.jsonld", context: "testdata/w3c/compact/0055-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0056", name: "Prefer @type: @vocab over @type: @id for terms", ld10: true, ld11: true, input: "testdata/w3c/compact/0056-in.jsonld", output: "testdata/w3c/compact/0056-out.jsonld", context: "testdata/w3c/compact/0056-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0057", name: "Complex round tripping @type: @vocab and @type: @id", ld10: true, ld11: true, input: "testdata/w3c/compact/0057-in.jsonld", output: "testdata/w3c/compact/0057-out.jsonld", context: "testdata/w3c/compact/0057-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0058", name: "Prefer @type: @id over @type: @vocab for non-terms", ld10: true, ld11: true, input: "testdata/w3c/compact/0058-in.jsonld", output: "testdata/w3c/compact/0058-out.jsonld", context: "testdata/w3c/compact/0058-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0059", name: "Term with @type: @vocab if no @type: @id", ld10: true, ld11: true, input: "testdata/w3c/compact/0059-in.jsonld", output: "testdata/w3c/compact/0059-out.jsonld", context: "testdata/w3c/compact/0059-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0060", name: "Term with @type: @id if no @type: @vocab and term value", ld10: true, ld11: true, input: "testdata/w3c/compact/0060-in.jsonld", output: "testdata/w3c/compact/0060-out.jsonld", context: "testdata/w3c/compact/0060-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0061", name: "@type: @vocab/@id with values matching either", ld10: true, ld11: true, input: "testdata/w3c/compact/0061-in.jsonld", output: "testdata/w3c/compact/0061-out.jsonld", context: "testdata/w3c/compact/0061-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0062", name: "@type: @vocab and relative IRIs", ld10: true, ld11: true, input: "testdata/w3c/compact/0062-in.jsonld", output: "testdata/w3c/compact/0062-out.jsonld", context: "testdata/w3c/compact/0062-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0063", name: "Compact IRI round-tripping with @type: @vocab", ld10: true, ld11: true, input: "testdata/w3c/compact/0063-in.jsonld", output: "testdata/w3c/compact/0063-out.jsonld", context: "testdata/w3c/compact/0063-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0064", name: "Compact language-tagged and indexed strings to index-map", ld10: true, ld11: true, input: "testdata/w3c/compact/0064-in.jsonld", output: "testdata/w3c/compact/0064-out.jsonld", context: "testdata/w3c/compact/0064-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0065", name: "Language-tagged and indexed strings with language-map", ld10: true, ld11: true, input: "testdata/w3c/compact/0065-in.jsonld", output: "testdata/w3c/compact/0065-out.jsonld", context: "testdata/w3c/compact/0065-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0066", name: "Relative IRIs", ld10: true, ld11: true, input: "testdata/w3c/compact/0066-in.jsonld", output: "testdata/w3c/compact/0066-out.jsonld", context: "testdata/w3c/compact/0066-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0067", name: "Reverse properties with blank nodes", ld10: true, ld11: true, input: "testdata/w3c/compact/0067-in.jsonld", output: "testdata/w3c/compact/0067-out.jsonld", context: "testdata/w3c/compact/0067-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0068", name: "Single value reverse properties", ld10: true, ld11: true, input: "testdata/w3c/compact/0068-in.jsonld", output: "testdata/w3c/compact/0068-out.jsonld", context: "testdata/w3c/compact/0068-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0069", name: "Single value reverse properties with @set", ld10: true, ld11: true, input: "testdata/w3c/compact/0069-in.jsonld", output: "testdata/w3c/compact/0069-out.jsonld", context: "testdata/w3c/compact/0069-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0070", name: "compactArrays option", ld10: true, ld11: true, input: "testdata/w3c/compact/0070-in.jsonld", output: "testdata/w3c/compact/0070-out.jsonld", context: "testdata/w3c/compact/0070-context.jsonld", compactArrays: false, compactToRelative: true},
		{id: "t0071", name: "Input has multiple @contexts, output has one", ld10: true, ld11: true, input: "testdata/w3c/compact/0071-in.jsonld", output: "testdata/w3c/compact/0071-out.jsonld", context: "testdata/w3c/compact/0071-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0072", name: "Default language and unmapped properties", ld10: true, ld11: true, input: "testdata/w3c/compact/0072-in.jsonld", output: "testdata/w3c/compact/0072-out.jsonld", context: "testdata/w3c/compact/0072-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0073", name: "Mapped @id and @type", ld10: true, ld11: true, input: "testdata/w3c/compact/0073-in.jsonld", output: "testdata/w3c/compact/0073-out.jsonld", context: "testdata/w3c/compact/0073-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0074", name: "Container as a list with type of @id", ld10: true, ld11: true, input: "testdata/w3c/compact/0074-in.jsonld", output: "testdata/w3c/compact/0074-out.jsonld", context: "testdata/w3c/compact/0074-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0075", name: "Compact using relative fragment identifier", ld10: true, ld11: true, base: "http://example.org/", input: "testdata/w3c/compact/0075-in.jsonld", output: "testdata/w3c/compact/0075-out.jsonld", context: "testdata/w3c/compact/0075-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0076", name: "Compacting IRI equivalent to base", ld10: true, ld11: true, input: "testdata/w3c/compact/0076-in.jsonld", output: "testdata/w3c/compact/0076-out.jsonld", context: "testdata/w3c/compact/0076-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0077", name: "Compact a @graph container", ld10: false, ld11: true, input: "testdata/w3c/compact/0077-in.jsonld", output: "testdata/w3c/compact/0077-out.jsonld", context: "testdata/w3c/compact/0077-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0078", name: "Compact a [@graph, @set] container", ld10: false, ld11: true, input: "testdata/w3c/compact/0078-in.jsonld", output: "testdata/w3c/compact/0078-out.jsonld", context: "testdata/w3c/compact/0078-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0079", name: "Compact a @graph container having @index", ld10: false, ld11: true, input: "testdata/w3c/compact/0079-in.jsonld", output: "testdata/w3c/compact/0079-out.jsonld", context: "testdata/w3c/compact/0079-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0080", name: "Do not compact a graph having @id with a term having an @graph container", ld10: false, ld11: true, input: "testdata/w3c/compact/0080-in.jsonld", output: "testdata/w3c/compact/0080-out.jsonld", context: "testdata/w3c/compact/0080-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0081", name: "Compact a [@graph, @index] container", ld10: false, ld11: true, input: "testdata/w3c/compact/0081-in.jsonld", output: "testdata/w3c/compact/0081-out.jsonld", context: "testdata/w3c/compact/0081-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0082", name: "Compact a [@graph, @index, @set] container", ld10: false, ld11: true, input: "testdata/w3c/compact/0082-in.jsonld", output: "testdata/w3c/compact/0082-out.jsonld", context: "testdata/w3c/compact/0082-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0083", name: "[@graph, @index] does not compact graph with @id", ld10: false, ld11: true, input: "testdata/w3c/compact/0083-in.jsonld", output: "testdata/w3c/compact/0083-out.jsonld", context: "testdata/w3c/compact/0083-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0084", name: "Compact a simple graph with a [@graph, @id] container", ld10: false, ld11: true, input: "testdata/w3c/compact/0084-in.jsonld", output: "testdata/w3c/compact/0084-out.jsonld", context: "testdata/w3c/compact/0084-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0085", name: "Compact a named graph with a [@graph, @id] container", ld10: false, ld11: true, input: "testdata/w3c/compact/0085-in.jsonld", output: "testdata/w3c/compact/0085-out.jsonld", context: "testdata/w3c/compact/0085-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0086", name: "Compact a simple graph with a [@graph, @id, @set] container", ld10: false, ld11: true, input: "testdata/w3c/compact/0086-in.jsonld", output: "testdata/w3c/compact/0086-out.jsonld", context: "testdata/w3c/compact/0086-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0087", name: "Compact a named graph with a [@graph, @id, @set] container", ld10: false, ld11: true, input: "testdata/w3c/compact/0087-in.jsonld", output: "testdata/w3c/compact/0087-out.jsonld", context: "testdata/w3c/compact/0087-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0088", name: "Compact a graph with @index using a [@graph, @id] container", ld10: false, ld11: true, input: "testdata/w3c/compact/0088-in.jsonld", output: "testdata/w3c/compact/0088-out.jsonld", context: "testdata/w3c/compact/0088-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0089", name: "Language map term selection with complications", ld10: true, ld11: true, input: "testdata/w3c/compact/0089-in.jsonld", output: "testdata/w3c/compact/0089-out.jsonld", context: "testdata/w3c/compact/0089-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0090", name: "Compact input with @graph container to output without @graph container", ld10: false, ld11: true, input: "testdata/w3c/compact/0090-in.jsonld", output: "testdata/w3c/compact/0090-out.jsonld", context: "testdata/w3c/compact/0090-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0091", name: "Compact input with @graph container to output without @graph container with compactArrays unset", ld10: false, ld11: true, input: "testdata/w3c/compact/0091-in.jsonld", output: "testdata/w3c/compact/0091-out.jsonld", context: "testdata/w3c/compact/0091-context.jsonld", compactArrays: false, compactToRelative: true},
		{id: "t0092", name: "Compact input with [@graph, @set] container to output without [@graph, @set] container", ld10: false, ld11: true, input: "testdata/w3c/compact/0092-in.jsonld", output: "testdata/w3c/compact/0092-out.jsonld", context: "testdata/w3c/compact/0092-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0093", name: "Compact input with [@graph, @set] container to output without [@graph, @set] container with compactArrays unset", ld10: false, ld11: true, input: "testdata/w3c/compact/0093-in.jsonld", output: "testdata/w3c/compact/0093-out.jsonld", context: "testdata/w3c/compact/0093-context.jsonld", compactArrays: false, compactToRelative: true},
		{id: "t0094", name: "Compact input with [@graph, @set] container to output without [@graph, @set] container", ld10: false, ld11: true, input: "testdata/w3c/compact/0094-in.jsonld", output: "testdata/w3c/compact/0094-out.jsonld", context: "testdata/w3c/compact/0094-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0095", name: "Relative propererty IRIs with @vocab: ''", ld10: true, ld11: true, input: "testdata/w3c/compact/0095-in.jsonld", output: "testdata/w3c/compact/0095-out.jsonld", context: "testdata/w3c/compact/0095-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0096", name: "Compact @graph container (multiple graphs)", ld10: false, ld11: true, input: "testdata/w3c/compact/0096-in.jsonld", output: "testdata/w3c/compact/0096-out.jsonld", context: "testdata/w3c/compact/0096-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0097", name: "Compact [@graph, @set] container (multiple graphs)", ld10: false, ld11: true, input: "testdata/w3c/compact/0097-in.jsonld", output: "testdata/w3c/compact/0097-out.jsonld", context: "testdata/w3c/compact/0097-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0098", name: "Compact [@graph, @index] container (multiple indexed objects)", ld10: false, ld11: true, input: "testdata/w3c/compact/0098-in.jsonld", output: "testdata/w3c/compact/0098-out.jsonld", context: "testdata/w3c/compact/0098-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0099", name: "Compact [@graph, @index, @set] container (multiple indexed objects)", ld10: false, ld11: true, input: "testdata/w3c/compact/0099-in.jsonld", output: "testdata/w3c/compact/0099-out.jsonld", context: "testdata/w3c/compact/0099-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0100", name: "Compact [@graph, @id] container (multiple indexed objects)", ld10: false, ld11: true, input: "testdata/w3c/compact/0100-in.jsonld", output: "testdata/w3c/compact/0100-out.jsonld", context: "testdata/w3c/compact/0100-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0101", name: "Compact [@graph, @id, @set] container (multiple indexed objects)", ld10: false, ld11: true, input: "testdata/w3c/compact/0101-in.jsonld", output: "testdata/w3c/compact/0101-out.jsonld", context: "testdata/w3c/compact/0101-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0102", name: "Compact [@graph, @index] container (multiple indexes and objects)", ld10: false, ld11: true, input: "testdata/w3c/compact/0102-in.jsonld", output: "testdata/w3c/compact/0102-out.jsonld", context: "testdata/w3c/compact/0102-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0103", name: "Compact [@graph, @id] container (multiple ids and objects)", ld10: false, ld11: true, input: "testdata/w3c/compact/0103-in.jsonld", output: "testdata/w3c/compact/0103-out.jsonld", context: "testdata/w3c/compact/0103-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0104", name: "Compact @type with @container: @set", ld10: false, ld11: true, input: "testdata/w3c/compact/0104-in.jsonld", output: "testdata/w3c/compact/0104-out.jsonld", context: "testdata/w3c/compact/0104-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0105", name: "Compact @type with @container: @set using an alias of @type", ld10: false, ld11: true, input: "testdata/w3c/compact/0105-in.jsonld", output: "testdata/w3c/compact/0105-out.jsonld", context: "testdata/w3c/compact/0105-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0106", name: "Do not compact @type with @container: @set to an array using an alias of @type", ld10: true, ld11: false, input: "testdata/w3c/compact/0106-in.jsonld", output: "testdata/w3c/compact/0106-out.jsonld", context: "testdata/w3c/compact/0106-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0107", name: "Relative propererty IRIs with @vocab: ''", ld10: true, ld11: true, input: "testdata/w3c/compact/0107-in.jsonld", output: "testdata/w3c/compact/0107-out.jsonld", context: "testdata/w3c/compact/0107-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0108", name: "context with JavaScript Object property names", ld10: true, ld11: true, input: "testdata/w3c/compact/0108-in.jsonld", output: "testdata/w3c/compact/0108-out.jsonld", context: "testdata/w3c/compact/0108-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0109", name: "Compact @graph container (multiple objects)", ld10: false, ld11: true, input: "testdata/w3c/compact/0109-in.jsonld", output: "testdata/w3c/compact/0109-out.jsonld", context: "testdata/w3c/compact/0109-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0110", name: "Compact [@graph, @set] container (multiple objects)", ld10: false, ld11: true, input: "testdata/w3c/compact/0110-in.jsonld", output: "testdata/w3c/compact/0110-out.jsonld", context: "testdata/w3c/compact/0110-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0111", name: "Keyword-like relative IRIs", ld10: false, ld11: true, input: "testdata/w3c/compact/0111-in.jsonld", output: "testdata/w3c/compact/0111-out.jsonld", context: "testdata/w3c/compact/0111-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0112", name: "Compact property index using Compact IRI index", ld10: false, ld11: true, input: "testdata/w3c/compact/0112-in.jsonld", output: "testdata/w3c/compact/0112-out.jsonld", context: "testdata/w3c/compact/0112-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0113", name: "Compact property index using Absolute IRI index", ld10: false, ld11: true, input: "testdata/w3c/compact/0113-in.jsonld", output: "testdata/w3c/compact/0113-out.jsonld", context: "testdata/w3c/compact/0113-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "t0114", name: "Reverse term with property based indexed container", ld10: false, ld11: true, base: "https://example.org/", input: "testdata/w3c/compact/0114-in.jsonld", output: "testdata/w3c/compact/0114-out.jsonld", context: "testdata/w3c/compact/0114-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc001", name: "adding new term", ld10: false, ld11: true, input: "testdata/w3c/compact/c001-in.jsonld", output: "testdata/w3c/compact/c001-out.jsonld", context: "testdata/w3c/compact/c001-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc002", name: "overriding a term", ld10: false, ld11: true, input: "testdata/w3c/compact/c002-in.jsonld", output: "testdata/w3c/compact/c002-out.jsonld", context: "testdata/w3c/compact/c002-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc003", name: "property and value with different terms mapping to the same expanded property", ld10: false, ld11: true, input: "testdata/w3c/compact/c003-in.jsonld", output: "testdata/w3c/compact/c003-out.jsonld", context: "testdata/w3c/compact/c003-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc004", name: "deep @context affects nested nodes", ld10: false, ld11: true, input: "testdata/w3c/compact/c004-in.jsonld", output: "testdata/w3c/compact/c004-out.jsonld", context: "testdata/w3c/compact/c004-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc005", name: "scoped context layers on intemediate contexts", ld10: false, ld11: true, input: "testdata/w3c/compact/c005-in.jsonld", output: "testdata/w3c/compact/c005-out.jsonld", context: "testdata/w3c/compact/c005-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc006", name: "adding new term", ld10: false, ld11: true, input: "testdata/w3c/compact/c006-in.jsonld", output: "testdata/w3c/compact/c006-out.jsonld", context: "testdata/w3c/compact/c006-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc007", name: "overriding a term", ld10: false, ld11: true, input: "testdata/w3c/compact/c007-in.jsonld", output: "testdata/w3c/compact/c007-out.jsonld", context: "testdata/w3c/compact/c007-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc008", name: "alias of @type", ld10: false, ld11: true, input: "testdata/w3c/compact/c008-in.jsonld", output: "testdata/w3c/compact/c008-out.jsonld", context: "testdata/w3c/compact/c008-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc009", name: "deep @type-scoped @context does NOT affect nested nodes", ld10: false, ld11: true, input: "testdata/w3c/compact/c009-in.jsonld", output: "testdata/w3c/compact/c009-out.jsonld", context: "testdata/w3c/compact/c009-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc010", name: "scoped context layers on intemediate contexts", ld10: false, ld11: true, input: "testdata/w3c/compact/c010-in.jsonld", output: "testdata/w3c/compact/c010-out.jsonld", context: "testdata/w3c/compact/c010-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc011", name: "applies context for all values", ld10: false, ld11: true, input: "testdata/w3c/compact/c011-in.jsonld", output: "testdata/w3c/compact/c011-out.jsonld", context: "testdata/w3c/compact/c011-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc012", name: "orders @type terms when applying scoped contexts", ld10: false, ld11: true, input: "testdata/w3c/compact/c012-in.jsonld", output: "testdata/w3c/compact/c012-out.jsonld", context: "testdata/w3c/compact/c012-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc013", name: "deep property-term scoped @context in @type-scoped @context affects nested nodes", ld10: false, ld11: true, input: "testdata/w3c/compact/c013-in.jsonld", output: "testdata/w3c/compact/c013-out.jsonld", context: "testdata/w3c/compact/c013-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc014", name: "type-scoped context nullification", ld10: false, ld11: true, input: "testdata/w3c/compact/c014-in.jsonld", output: "testdata/w3c/compact/c014-out.jsonld", context: "testdata/w3c/compact/c014-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc015", name: "type-scoped base", ld10: false, ld11: true, input: "testdata/w3c/compact/c015-in.jsonld", output: "testdata/w3c/compact/c015-out.jsonld", context: "testdata/w3c/compact/c015-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc016", name: "type-scoped vocab", ld10: false, ld11: true, input: "testdata/w3c/compact/c016-in.jsonld", output: "testdata/w3c/compact/c016-out.jsonld", context: "testdata/w3c/compact/c016-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc017", name: "multiple type-scoped contexts are properly reverted", ld10: false, ld11: true, input: "testdata/w3c/compact/c017-in.jsonld", output: "testdata/w3c/compact/c017-out.jsonld", context: "testdata/w3c/compact/c017-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc018", name: "multiple type-scoped types resolved against previous context", ld10: false, ld11: true, input: "testdata/w3c/compact/c018-in.jsonld", output: "testdata/w3c/compact/c018-out.jsonld", context: "testdata/w3c/compact/c018-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc019", name: "type-scoped context with multiple property scoped terms", ld10: false, ld11: true, input: "testdata/w3c/compact/c019-in.jsonld", output: "testdata/w3c/compact/c019-out.jsonld", context: "testdata/w3c/compact/c019-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc020", name: "type-scoped value", ld10: false, ld11: true, input: "testdata/w3c/compact/c020-in.jsonld", output: "testdata/w3c/compact/c020-out.jsonld", context: "testdata/w3c/compact/c020-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc021", name: "type-scoped value mix", ld10: false, ld11: true, input: "testdata/w3c/compact/c021-in.jsonld", output: "testdata/w3c/compact/c021-out.jsonld", context: "testdata/w3c/compact/c021-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc022", name: "type-scoped property-scoped contexts including @type:@vocab", ld10: false, ld11: true, input: "testdata/w3c/compact/c022-in.jsonld", output: "testdata/w3c/compact/c022-out.jsonld", context: "testdata/w3c/compact/c022-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc023", name: "composed type-scoped property-scoped contexts including @type:@vocab", ld10: false, ld11: true, input: "testdata/w3c/compact/c023-in.jsonld", output: "testdata/w3c/compact/c023-out.jsonld", context: "testdata/w3c/compact/c023-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc024", name: "type-scoped + property-scoped + values evaluates against previous context", ld10: false, ld11: true, input: "testdata/w3c/compact/c024-in.jsonld", output: "testdata/w3c/compact/c024-out.jsonld", context: "testdata/w3c/compact/c024-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc025", name: "type-scoped + graph container", ld10: false, ld11: true, input: "testdata/w3c/compact/c025-in.jsonld", output: "testdata/w3c/compact/c025-out.jsonld", context: "testdata/w3c/compact/c025-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc026", name: "@propagate: true on type-scoped context", ld10: false, ld11: true, input: "testdata/w3c/compact/c026-in.jsonld", output: "testdata/w3c/compact/c026-out.jsonld", context: "testdata/w3c/compact/c026-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc027", name: "@propagate: false on property-scoped context", ld10: false, ld11: true, input: "testdata/w3c/compact/c027-in.jsonld", output: "testdata/w3c/compact/c027-out.jsonld", context: "testdata/w3c/compact/c027-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tc028", name: "Empty-property scoped context does not affect term selection.", ld10: false, ld11: true, input: "testdata/w3c/compact/c028-in.jsonld", output: "testdata/w3c/compact/c028-out.jsonld", context: "testdata/w3c/compact/c028-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tdi01", name: "term direction null", ld10: false, ld11: true, input: "testdata/w3c/compact/di01-in.jsonld", output: "testdata/w3c/compact/di01-out.jsonld", context: "testdata/w3c/compact/di01-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tdi02", name: "use alias of @direction", ld10: false, ld11: true, input: "testdata/w3c/compact/di02-in.jsonld", output: "testdata/w3c/compact/di02-out.jsonld", context: "testdata/w3c/compact/di02-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tdi03", name: "term selection with lists and direction", ld10: false, ld11: true, input: "testdata/w3c/compact/di03-in.jsonld", output: "testdata/w3c/compact/di03-out.jsonld", context: "testdata/w3c/compact/di03-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tdi04", name: "simple language map with term direction", ld10: false, ld11: true, input: "testdata/w3c/compact/di04-in.jsonld", output: "testdata/w3c/compact/di04-out.jsonld", context: "testdata/w3c/compact/di04-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tdi05", name: "simple language map with overriding term direction", ld10: false, ld11: true, input: "testdata/w3c/compact/di05-in.jsonld", output: "testdata/w3c/compact/di05-out.jsonld", context: "testdata/w3c/compact/di05-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tdi06", name: "simple language map with overriding null direction", ld10: false, ld11: true, input: "testdata/w3c/compact/di06-in.jsonld", output: "testdata/w3c/compact/di06-out.jsonld", context: "testdata/w3c/compact/di06-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tdi07", name: "simple language map with mismatching term direction", ld10: false, ld11: true, input: "testdata/w3c/compact/di07-in.jsonld", output: "testdata/w3c/compact/di07-out.jsonld", context: "testdata/w3c/compact/di07-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "te001", name: "Compaction to list of lists", ld10: false, ld11: false, input: "testdata/w3c/compact/e001-in.jsonld", context: "testdata/w3c/compact/e001-context.jsonld", compactArrays: true, compactToRelative: true, err: "compaction to list of lists"},
		{id: "te002", name: "Absolute IRI confused with Compact IRI", ld10: false, ld11: true, input: "testdata/w3c/compact/e002-in.jsonld", context: "testdata/w3c/compact/e002-context.jsonld", compactArrays: true, compactToRelative: true, err: "IRI confused with prefix"},
		{id: "ten01", name: "Nest term not defined", ld10: false, ld11: true, input: "testdata/w3c/compact/en01-in.jsonld", context: "testdata/w3c/compact/en01-context.jsonld", compactArrays: true, compactToRelative: true, err: "invalid @nest value"},
		{id: "tep05", name: "processingMode json-ld-1.0 conflicts with @version: 1.1", ld10: true, ld11: false, input: "testdata/w3c/compact/ep05-in.jsonld", context: "testdata/w3c/compact/ep05-context.jsonld", compactArrays: true, compactToRelative: true, err: "processing mode conflict"},
		{id: "tep06", name: "@version must be 1.1", ld10: false, ld11: true, input: "testdata/w3c/compact/ep06-in.jsonld", context: "testdata/w3c/compact/ep06-context.jsonld", compactArrays: true, compactToRelative: true, err: "invalid @version value"},
		{id: "tep07", name: "@prefix is not allowed in 1.0", ld10: true, ld11: false, input: "testdata/w3c/compact/ep07-in.jsonld", context: "testdata/w3c/compact/ep07-context.jsonld", compactArrays: true, compactToRelative: true, err: "invalid term definition"},
		{id: "tep08", name: "@prefix must be a boolean", ld10: false, ld11: true, input: "testdata/w3c/compact/ep08-in.jsonld", context: "testdata/w3c/compact/ep08-context.jsonld", compactArrays: true, compactToRelative: true, err: "invalid @prefix value"},
		{id: "tep09", name: "@prefix not allowed on compact IRI term", ld10: false, ld11: true, input: "testdata/w3c/compact/ep09-in.jsonld", context: "testdata/w3c/compact/ep09-context.jsonld", compactArrays: true, compactToRelative: true, err: "invalid term definition"},
		{id: "tep10", name: "@nest is not allowed in 1.0", ld10: true, ld11: false, input: "testdata/w3c/compact/ep10-in.jsonld", context: "testdata/w3c/compact/ep10-context.jsonld", compactArrays: true, compactToRelative: true, err: "invalid term definition"},
		{id: "tep11", name: "@context is not allowed in 1.0", ld10: true, ld11: false, input: "testdata/w3c/compact/ep11-in.jsonld", context: "testdata/w3c/compact/ep11-context.jsonld", compactArrays: true, compactToRelative: true, err: "invalid term definition"},
		{id: "tep12", name: "@container may not be an array in 1.0", ld10: true, ld11: false, input: "testdata/w3c/compact/ep12-in.jsonld", context: "testdata/w3c/compact/ep12-context.jsonld", compactArrays: true, compactToRelative: true, err: "invalid container mapping"},
		{id: "tep13", name: "@container may not be @id in 1.0", ld10: true, ld11: false, input: "testdata/w3c/compact/ep13-in.jsonld", context: "testdata/w3c/compact/ep13-context.jsonld", compactArrays: true, compactToRelative: true, err: "invalid container mapping"},
		{id: "tep14", name: "@container may not be @type in 1.0", ld10: true, ld11: false, input: "testdata/w3c/compact/ep14-in.jsonld", context: "testdata/w3c/compact/ep14-context.jsonld", compactArrays: true, compactToRelative: true, err: "invalid container mapping"},
		{id: "tep15", name: "@container may not be @graph in 1.0", ld10: true, ld11: false, input: "testdata/w3c/compact/ep15-in.jsonld", context: "testdata/w3c/compact/ep15-context.jsonld", compactArrays: true, compactToRelative: true, err: "invalid container mapping"},
		{id: "tin01", name: "Basic Included array", ld10: false, ld11: true, input: "testdata/w3c/compact/in01-in.jsonld", output: "testdata/w3c/compact/in01-out.jsonld", context: "testdata/w3c/compact/in01-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tin02", name: "Basic Included object", ld10: false, ld11: true, input: "testdata/w3c/compact/in02-in.jsonld", output: "testdata/w3c/compact/in02-out.jsonld", context: "testdata/w3c/compact/in02-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tin03", name: "Multiple properties mapping to @included are folded together", ld10: false, ld11: true, input: "testdata/w3c/compact/in03-in.jsonld", output: "testdata/w3c/compact/in03-out.jsonld", context: "testdata/w3c/compact/in03-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tin04", name: "Included containing @included", ld10: false, ld11: true, input: "testdata/w3c/compact/in04-in.jsonld", output: "testdata/w3c/compact/in04-out.jsonld", context: "testdata/w3c/compact/in04-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tin05", name: "Property value with @included", ld10: false, ld11: true, input: "testdata/w3c/compact/in05-in.jsonld", output: "testdata/w3c/compact/in05-out.jsonld", context: "testdata/w3c/compact/in05-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tjs01", name: "Compact JSON literal (boolean true)", ld10: false, ld11: true, input: "testdata/w3c/compact/js01-in.jsonld", output: "testdata/w3c/compact/js01-out.jsonld", context: "testdata/w3c/compact/js01-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tjs02", name: "Compact JSON literal (boolean false)", ld10: false, ld11: true, input: "testdata/w3c/compact/js02-in.jsonld", output: "testdata/w3c/compact/js02-out.jsonld", context: "testdata/w3c/compact/js02-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tjs03", name: "Compact JSON literal (double)", ld10: false, ld11: true, input: "testdata/w3c/compact/js03-in.jsonld", output: "testdata/w3c/compact/js03-out.jsonld", context: "testdata/w3c/compact/js03-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tjs04", name: "Compact JSON literal (double-zero)", ld10: false, ld11: true, input: "testdata/w3c/compact/js04-in.jsonld", output: "testdata/w3c/compact/js04-out.jsonld", context: "testdata/w3c/compact/js04-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tjs05", name: "Compact JSON literal (integer)", ld10: false, ld11: true, input: "testdata/w3c/compact/js05-in.jsonld", output: "testdata/w3c/compact/js05-out.jsonld", context: "testdata/w3c/compact/js05-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tjs06", name: "Compact JSON literal (object)", ld10: false, ld11: true, input: "testdata/w3c/compact/js06-in.jsonld", output: "testdata/w3c/compact/js06-out.jsonld", context: "testdata/w3c/compact/js06-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tjs07", name: "Compact JSON literal (array)", ld10: false, ld11: true, input: "testdata/w3c/compact/js07-in.jsonld", output: "testdata/w3c/compact/js07-out.jsonld", context: "testdata/w3c/compact/js07-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tjs08", name: "Compact already expanded JSON literal", ld10: false, ld11: true, input: "testdata/w3c/compact/js08-in.jsonld", output: "testdata/w3c/compact/js08-out.jsonld", context: "testdata/w3c/compact/js08-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tjs09", name: "Compact already expanded JSON literal with aliased keys", ld10: false, ld11: true, input: "testdata/w3c/compact/js09-in.jsonld", output: "testdata/w3c/compact/js09-out.jsonld", context: "testdata/w3c/compact/js09-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tjs10", name: "Compact JSON literal (string)", ld10: false, ld11: true, input: "testdata/w3c/compact/js10-in.jsonld", output: "testdata/w3c/compact/js10-out.jsonld", context: "testdata/w3c/compact/js10-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tjs11", name: "Compact JSON literal (null)", ld10: false, ld11: true, input: "testdata/w3c/compact/js11-in.jsonld", output: "testdata/w3c/compact/js11-out.jsonld", context: "testdata/w3c/compact/js11-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tla01", name: "most specific term matching in @list.", ld10: true, ld11: true, input: "testdata/w3c/compact/la01-in.jsonld", output: "testdata/w3c/compact/la01-out.jsonld", context: "testdata/w3c/compact/la01-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tli01", name: "coerced @list containing an empty list", ld10: false, ld11: true, input: "testdata/w3c/compact/li01-in.jsonld", output: "testdata/w3c/compact/li01-out.jsonld", context: "testdata/w3c/compact/li01-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tli02", name: "coerced @list containing a list", ld10: false, ld11: true, input: "testdata/w3c/compact/li02-in.jsonld", output: "testdata/w3c/compact/li02-out.jsonld", context: "testdata/w3c/compact/li02-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tli03", name: "coerced @list containing an deep list", ld10: false, ld11: true, input: "testdata/w3c/compact/li03-in.jsonld", output: "testdata/w3c/compact/li03-out.jsonld", context: "testdata/w3c/compact/li03-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tli04", name: "coerced @list containing multiple lists", ld10: false, ld11: true, input: "testdata/w3c/compact/li04-in.jsonld", output: "testdata/w3c/compact/li04-out.jsonld", context: "testdata/w3c/compact/li04-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tli05", name: "coerced @list containing mixed list values", ld10: false, ld11: true, input: "testdata/w3c/compact/li05-in.jsonld", output: "testdata/w3c/compact/li05-out.jsonld", context: "testdata/w3c/compact/li05-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm001", name: "Indexes to object not having an @id", ld10: false, ld11: true, input: "testdata/w3c/compact/m001-in.jsonld", output: "testdata/w3c/compact/m001-out.jsonld", context: "testdata/w3c/compact/m001-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm002", name: "Indexes to object already having an @id", ld10: false, ld11: true, input: "testdata/w3c/compact/m002-in.jsonld", output: "testdata/w3c/compact/m002-out.jsonld", context: "testdata/w3c/compact/m002-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm003", name: "Indexes to object not having an @type", ld10: false, ld11: true, input: "testdata/w3c/compact/m003-in.jsonld", output: "testdata/w3c/compact/m003-out.jsonld", context: "testdata/w3c/compact/m003-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm004", name: "Indexes to object already having an @type", ld10: false, ld11: true, input: "testdata/w3c/compact/m004-in.jsonld", output: "testdata/w3c/compact/m004-out.jsonld", context: "testdata/w3c/compact/m004-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm005", name: "Indexes to object using compact IRI @id", ld10: false, ld11: true, input: "testdata/w3c/compact/m005-in.jsonld", output: "testdata/w3c/compact/m005-out.jsonld", context: "testdata/w3c/compact/m005-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm006", name: "Indexes using compacted @type", ld10: false, ld11: true, input: "testdata/w3c/compact/m006-in.jsonld", output: "testdata/w3c/compact/m006-out.jsonld", context: "testdata/w3c/compact/m006-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm007", name: "When type is in a type map", ld10: false, ld11: true, input: "testdata/w3c/compact/m007-in.jsonld", output: "testdata/w3c/compact/m007-out.jsonld", context: "testdata/w3c/compact/m007-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm008", name: "@index map with @none node definition", ld10: false, ld11: true, input: "testdata/w3c/compact/m008-in.jsonld", output: "testdata/w3c/compact/m008-out.jsonld", context: "testdata/w3c/compact/m008-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm009", name: "@index map with @none value", ld10: false, ld11: true, input: "testdata/w3c/compact/m009-in.jsonld", output: "testdata/w3c/compact/m009-out.jsonld", context: "testdata/w3c/compact/m009-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm010", name: "@index map with @none value using alias of @none", ld10: false, ld11: true, input: "testdata/w3c/compact/m010-in.jsonld", output: "testdata/w3c/compact/m010-out.jsonld", context: "testdata/w3c/compact/m010-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm011", name: "@language map with no @language", ld10: false, ld11: true, input: "testdata/w3c/compact/m011-in.jsonld", output: "testdata/w3c/compact/m011-out.jsonld", context: "testdata/w3c/compact/m011-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm012", name: "language map with no @language using alias of @none", ld10: false, ld11: true, input: "testdata/w3c/compact/m012-in.jsonld", output: "testdata/w3c/compact/m012-out.jsonld", context: "testdata/w3c/compact/m012-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm013", name: "id map using @none", ld10: false, ld11: true, input: "testdata/w3c/compact/m013-in.jsonld", output: "testdata/w3c/compact/m013-out.jsonld", context: "testdata/w3c/compact/m013-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm014", name: "id map using @none with alias", ld10: false, ld11: true, input: "testdata/w3c/compact/m014-in.jsonld", output: "testdata/w3c/compact/m014-out.jsonld", context: "testdata/w3c/compact/m014-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm015", name: "type map using @none with alias", ld10: false, ld11: true, input: "testdata/w3c/compact/m015-in.jsonld", output: "testdata/w3c/compact/m015-out.jsonld", context: "testdata/w3c/compact/m015-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm016", name: "type map using @none with alias", ld10: false, ld11: true, input: "testdata/w3c/compact/m016-in.jsonld", output: "testdata/w3c/compact/m016-out.jsonld", context: "testdata/w3c/compact/m016-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm017", name: "graph index map using @none", ld10: false, ld11: true, input: "testdata/w3c/compact/m017-in.jsonld", output: "testdata/w3c/compact/m017-out.jsonld", context: "testdata/w3c/compact/m017-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm018", name: "graph id map using @none", ld10: false, ld11: true, input: "testdata/w3c/compact/m018-in.jsonld", output: "testdata/w3c/compact/m018-out.jsonld", context: "testdata/w3c/compact/m018-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm019", name: "graph id map using alias of @none", ld10: false, ld11: true, input: "testdata/w3c/compact/m019-in.jsonld", output: "testdata/w3c/compact/m019-out.jsonld", context: "testdata/w3c/compact/m019-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm020", name: "node reference compacts to string value of type map", ld10: false, ld11: true, input: "testdata/w3c/compact/m020-in.jsonld", output: "testdata/w3c/compact/m020-out.jsonld", context: "testdata/w3c/compact/m020-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm021", name: "node reference compacts to string value of type map with @type: @id", ld10: false, ld11: true, input: "testdata/w3c/compact/m021-in.jsonld", output: "testdata/w3c/compact/m021-out.jsonld", context: "testdata/w3c/compact/m021-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm022", name: "node reference compacts to string value of type map with @type: @vocab", ld10: false, ld11: true, input: "testdata/w3c/compact/m022-in.jsonld", output: "testdata/w3c/compact/m022-out.jsonld", context: "testdata/w3c/compact/m022-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tm023", name: "compact IRI with container: @type", ld10: false, ld11: true, input: "testdata/w3c/compact/m023-in.jsonld", output: "testdata/w3c/compact/m023-out.jsonld", context: "testdata/w3c/compact/m023-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tn001", name: "Indexes to @nest for property with @nest", ld10: false, ld11: true, input: "testdata/w3c/compact/n001-in.jsonld", output: "testdata/w3c/compact/n001-out.jsonld", context: "testdata/w3c/compact/n001-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tn002", name: "Indexes to @nest for all properties with @nest", ld10: false, ld11: true, input: "testdata/w3c/compact/n002-in.jsonld", output: "testdata/w3c/compact/n002-out.jsonld", context: "testdata/w3c/compact/n002-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tn003", name: "Nests using alias of @nest", ld10: false, ld11: true, input: "testdata/w3c/compact/n003-in.jsonld", output: "testdata/w3c/compact/n003-out.jsonld", context: "testdata/w3c/compact/n003-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tn004", name: "Arrays of nested values", ld10: false, ld11: true, input: "testdata/w3c/compact/n004-in.jsonld", output: "testdata/w3c/compact/n004-out.jsonld", context: "testdata/w3c/compact/n004-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tn005", name: "Nested @container: @list", ld10: false, ld11: true, input: "testdata/w3c/compact/n005-in.jsonld", output: "testdata/w3c/compact/n005-out.jsonld", context: "testdata/w3c/compact/n005-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tn006", name: "Nested @container: @index", ld10: false, ld11: true, input: "testdata/w3c/compact/n006-in.jsonld", output: "testdata/w3c/compact/n006-out.jsonld", context: "testdata/w3c/compact/n006-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tn007", name: "Nested @container: @language", ld10: false, ld11: true, input: "testdata/w3c/compact/n007-in.jsonld", output: "testdata/w3c/compact/n007-out.jsonld", context: "testdata/w3c/compact/n007-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tn008", name: "Nested @container: @type", ld10: false, ld11: true, input: "testdata/w3c/compact/n008-in.jsonld", output: "testdata/w3c/compact/n008-out.jsonld", context: "testdata/w3c/compact/n008-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tn009", name: "Nested @container: @id", ld10: false, ld11: true, input: "testdata/w3c/compact/n009-in.jsonld", output: "testdata/w3c/compact/n009-out.jsonld", context: "testdata/w3c/compact/n009-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tn010", name: "Multiple nest aliases", ld10: false, ld11: true, input: "testdata/w3c/compact/n010-in.jsonld", output: "testdata/w3c/compact/n010-out.jsonld", context: "testdata/w3c/compact/n010-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tn011", name: "Nests using alias of @nest (defined with @id)", ld10: false, ld11: true, input: "testdata/w3c/compact/n011-in.jsonld", output: "testdata/w3c/compact/n011-out.jsonld", context: "testdata/w3c/compact/n011-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tp001", name: "Compact IRI will not use an expanded term definition in 1.0", ld10: true, ld11: false, input: "testdata/w3c/compact/p001-in.jsonld", output: "testdata/w3c/compact/p001-out.jsonld", context: "testdata/w3c/compact/p001-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tp002", name: "Compact IRI does not use expanded term definition in 1.1", ld10: false, ld11: true, input: "testdata/w3c/compact/p002-in.jsonld", output: "testdata/w3c/compact/p002-out.jsonld", context: "testdata/w3c/compact/p002-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tp003", name: "Compact IRI does not use simple term that does not end with a gen-delim", ld10: false, ld11: true, input: "testdata/w3c/compact/p003-in.jsonld", output: "testdata/w3c/compact/p003-out.jsonld", context: "testdata/w3c/compact/p003-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tp004", name: "Compact IRIs using simple terms ending with gen-delim", ld10: false, ld11: true, input: "testdata/w3c/compact/p004-in.jsonld", output: "testdata/w3c/compact/p004-out.jsonld", context: "testdata/w3c/compact/p004-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tp005", name: "Compact IRI uses term with definition including @prefix: true", ld10: false, ld11: true, input: "testdata/w3c/compact/p005-in.jsonld", output: "testdata/w3c/compact/p005-out.jsonld", context: "testdata/w3c/compact/p005-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tp006", name: "Compact IRI uses term with definition including @prefix: true", ld10: false, ld11: true, input: "testdata/w3c/compact/p006-in.jsonld", output: "testdata/w3c/compact/p006-out.jsonld", context: "testdata/w3c/compact/p006-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tp007", name: "Compact IRI not used as prefix", ld10: false, ld11: true, input: "testdata/w3c/compact/p007-in.jsonld", output: "testdata/w3c/compact/p007-out.jsonld", context: "testdata/w3c/compact/p007-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tp008", name: "Compact IRI does not use term with definition including @prefix: false", ld10: false, ld11: true, input: "testdata/w3c/compact/p008-in.jsonld", output: "testdata/w3c/compact/p008-out.jsonld", context: "testdata/w3c/compact/p008-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tpi01", name: "property-valued index indexes property value, instead of property (value)", ld10: false, ld11: true, input: "testdata/w3c/compact/pi01-in.jsonld", output: "testdata/w3c/compact/pi01-out.jsonld", context: "testdata/w3c/compact/pi01-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tpi02", name: "property-valued index indexes property value, instead of property (multiple values)", ld10: false, ld11: true, input: "testdata/w3c/compact/pi02-in.jsonld", output: "testdata/w3c/compact/pi02-out.jsonld", context: "testdata/w3c/compact/pi02-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tpi03", name: "property-valued index indexes property value, instead of property (node)", ld10: false, ld11: true, input: "testdata/w3c/compact/pi03-in.jsonld", output: "testdata/w3c/compact/pi03-out.jsonld", context: "testdata/w3c/compact/pi03-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tpi04", name: "property-valued index indexes property value, instead of property (multiple nodes)", ld10: false, ld11: true, input: "testdata/w3c/compact/pi04-in.jsonld", output: "testdata/w3c/compact/pi04-out.jsonld", context: "testdata/w3c/compact/pi04-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tpi05", name: "property-valued index indexes using @none if no property value exists", ld10: false, ld11: true, input: "testdata/w3c/compact/pi05-in.jsonld", output: "testdata/w3c/compact/pi05-out.jsonld", context: "testdata/w3c/compact/pi05-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tpi06", name: "property-valued index indexes using @none if no property value does not compact to string", ld10: false, ld11: true, input: "testdata/w3c/compact/pi06-in.jsonld", output: "testdata/w3c/compact/pi06-out.jsonld", context: "testdata/w3c/compact/pi06-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tpr01", name: "Check illegal clearing of context with protected terms", ld10: false, ld11: true, input: "testdata/w3c/compact/pr01-in.jsonld", context: "testdata/w3c/compact/pr01-context.jsonld", compactArrays: true, compactToRelative: true, err: "invalid context nullification"},
		{id: "tpr02", name: "Check illegal overriding of protected term", ld10: false, ld11: true, input: "testdata/w3c/compact/pr02-in.jsonld", context: "testdata/w3c/compact/pr02-context.jsonld", compactArrays: true, compactToRelative: true, err: "protected term redefinition"},
		{id: "tpr03", name: "Check illegal overriding of protected term from type-scoped context", ld10: false, ld11: true, input: "testdata/w3c/compact/pr03-in.jsonld", context: "testdata/w3c/compact/pr03-context.jsonld", compactArrays: true, compactToRelative: true, err: "protected term redefinition"},
		{id: "tpr04", name: "Check legal overriding of protected term from property-scoped context", ld10: false, ld11: true, input: "testdata/w3c/compact/pr04-in.jsonld", output: "testdata/w3c/compact/pr04-out.jsonld", context: "testdata/w3c/compact/pr04-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tpr05", name: "Check legal overriding of type-scoped protected term from nested node", ld10: false, ld11: true, input: "testdata/w3c/compact/pr05-in.jsonld", output: "testdata/w3c/compact/pr05-out.jsonld", context: "testdata/w3c/compact/pr05-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tr001", name: "Expands and compacts to document base by default", ld10: false, ld11: true, base: "http://example.org/", input: "testdata/w3c/compact/r001-in.jsonld", output: "testdata/w3c/compact/r001-out.jsonld", context: "testdata/w3c/compact/r001-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "tr002", name: "Expands and does not compact to document base with compactToRelative false", ld10: false, ld11: true, input: "testdata/w3c/compact/r002-in.jsonld", output: "testdata/w3c/compact/r002-out.jsonld", context: "testdata/w3c/compact/r002-context.jsonld", compactArrays: true, compactToRelative: false},
		{id: "ts001", name: "@context with single array values", ld10: false, ld11: true, input: "testdata/w3c/compact/s001-in.jsonld", output: "testdata/w3c/compact/s001-out.jsonld", context: "testdata/w3c/compact/s001-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "ts002", name: "@context with array including @set uses array values", ld10: false, ld11: true, input: "testdata/w3c/compact/s002-in.jsonld", output: "testdata/w3c/compact/s002-out.jsonld", context: "testdata/w3c/compact/s002-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "ttn01", name: "@type: @none does not compact values", ld10: false, ld11: true, input: "testdata/w3c/compact/tn01-in.jsonld", output: "testdata/w3c/compact/tn01-out.jsonld", context: "testdata/w3c/compact/tn01-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "ttn02", name: "@type: @none does not use arrays by default", ld10: false, ld11: true, input: "testdata/w3c/compact/tn02-in.jsonld", output: "testdata/w3c/compact/tn02-out.jsonld", context: "testdata/w3c/compact/tn02-context.jsonld", compactArrays: true, compactToRelative: true},
		{id: "ttn03", name: "@type: @none uses arrays with @container: @set", ld10: false, ld11: true, input: "testdata/w3c/compact/tn03-in.jsonld", output: "testdata/w3c/compact/tn03-out.jsonld", context: "testdata/w3c/compact/tn03-context.jsonld", compactArrays: true, compactToRelative: true},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s-%s", tc.id, tc.name), func(t *testing.T) {
			t.Parallel()

			src := LoadData(t, tc.input)
			ctxData := LoadData(t, tc.context)
			var ctx struct {
				Context json.RawMessage `json:"@context"`
			}
			if err := json.Unmarshal(ctxData, &ctx); err != nil {
				t.Fatal(err.Error())
			}

			var want json.RawMessage
			if tc.err == "" {
				want = LoadData(t, tc.output)
			}

			docIRI := fmt.Sprintf("https://w3c.github.io/json-ld-api/tests/compact/%s-in.jsonld", tc.id[1:])

			t.Run("json-ld-1.0", func(t *testing.T) {
				t.Parallel()
				tc := tc
				if !tc.ld10 {
					t.Skip("test not enabled for LD 1.0")
				}

				proc := ld.NewProcessor(
					ld.WithBaseIRI(tc.base),
					ld.With10Processing(true),
					ld.WithCompactArrays(tc.compactArrays),
					ld.WithCompactToRelative(tc.compactToRelative),
				)

				expanded, err := proc.Expand(src, docIRI)
				if err != nil {
					t.Fatalf("expected successful expand, got: %s", err)
				}

				got, err := proc.Compact(ctx.Context, expanded, docIRI)

				if err == nil && tc.err != "" {
					t.Fatalf("expected error: %s, got nil", tc.err)
				}

				if err != nil && tc.err == "" {
					t.Fatalf("expected no error, got: %s", err)
				}

				if tc.err != "" && err != nil {
					if !strings.Contains(err.Error(), tc.err) {
						t.Fatalf("expected error: %s, got: %s", tc.err, err)
					}
				} else {
					if diff := cmp.Diff(want, json.RawMessage(got), JSONDiff()); diff != "" {
						if *dump {
							data, _ := json.MarshalIndent(got, "", "    ")
							t.Logf("compacted from: %s", string(data))
						}
						t.Errorf("compaction mismatch (-want +got):\n%s", diff)
					}
				}
			})

			t.Run("json-ld-1.1", func(t *testing.T) {
				t.Parallel()
				tc := tc
				if !tc.ld11 {
					t.Skip("test not enabled for LD 1.1")
				}

				proc := ld.NewProcessor(
					ld.WithBaseIRI(tc.base),
					ld.With10Processing(false),
					ld.WithCompactArrays(tc.compactArrays),
					ld.WithCompactToRelative(tc.compactToRelative),
				)

				expanded, err := proc.Expand(src, docIRI)
				if err != nil {
					t.Fatalf("expected successful expand, got: %s", err)
				}

				got, err := proc.Compact(ctx.Context, expanded, docIRI)

				if err == nil && tc.err != "" {
					t.Fatalf("expected error: %s, got nil", tc.err)
				}

				if err != nil && tc.err == "" {
					t.Fatalf("expected no error, got: %s", err)
				}

				if tc.err != "" && err != nil {
					if !strings.Contains(err.Error(), tc.err) {
						t.Fatalf("expected error: %s, got: %s", tc.err, err)
					}
				} else {
					if diff := cmp.Diff(want, json.RawMessage(got), JSONDiff()); diff != "" {
						if *dump {
							data, _ := json.MarshalIndent(got, "", "    ")
							t.Logf("compacted from: %s", string(data))
						}
						t.Errorf("compaction mismatch (-want +got):\n%s", diff)
					}
				}
			})
		})
	}
}
