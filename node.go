package longdistance

import (
	"sourcery.dny.nu/longdistance/internal/json"
)

// Properties is a key-to-array-of-[Node] map.
//
// It's used to hold any property that's not a JSON-LD keyword.
type Properties map[string][]Node

// Node represents a node in a JSON-LD graph.
//
// Every supported JSON-LD keyword has a field of its own. All remaining
// properties are tracked on the Properties field.
type Node struct {
	Direction string          // @direction / KeywordDirection
	Graph     []Node          // @graph / KeywordGraph
	ID        string          // @id / KeywordID
	Included  []Node          // @included / KeywordIncluded
	Index     string          // @index / KeywordIndex
	Language  string          // @language / KeywordLanguage
	List      []Node          // @list / KeywordList
	Reverse   Properties      // @reverse / KeywordReverse
	Set       []Node          // @set / KeywordSet
	Type      []string        // @type / KeywordType
	Value     json.RawMessage // @value / KeywordValue

	Properties Properties // everything else
}

// Internal is a generic type that matches the internals of [Node].
//
// This can be used to convert to a [Node] from any type outside this package
// that happens to be a [Node] underneath.
type Internal interface {
	~struct {
		Direction  string
		Graph      []Node
		ID         string
		Included   []Node
		Index      string
		Language   string
		List       []Node
		Reverse    Properties
		Set        []Node
		Type       []string
		Value      json.RawMessage
		Properties Properties
	}
}

// PropertySet returns a [Set] with an entry for each property that is set on
// the [Node].
func (n *Node) PropertySet() map[string]struct{} {
	if n == nil {
		return nil
	}

	res := make(map[string]struct{}, len(n.Properties)+2)
	if n.Has(KeywordDirection) {
		res[KeywordDirection] = struct{}{}
	}
	if n.Has(KeywordGraph) {
		res[KeywordGraph] = struct{}{}
	}
	if n.Has(KeywordID) {
		res[KeywordID] = struct{}{}
	}
	if n.Has(KeywordIncluded) {
		res[KeywordIncluded] = struct{}{}
	}
	if n.Has(KeywordIndex) {
		res[KeywordIndex] = struct{}{}
	}
	if n.Has(KeywordLanguage) {
		res[KeywordLanguage] = struct{}{}
	}
	if n.Has(KeywordList) {
		res[KeywordList] = struct{}{}
	}
	if n.Has(KeywordReverse) {
		res[KeywordReverse] = struct{}{}
	}
	if n.Has(KeywordSet) {
		res[KeywordSet] = struct{}{}
	}
	if n.Has(KeywordType) {
		res[KeywordType] = struct{}{}
	}
	if n.Has(KeywordValue) {
		res[KeywordValue] = struct{}{}
	}

	for p := range n.Properties {
		res[p] = struct{}{}
	}

	return res
}

func (n *Node) propsWithout(props ...string) map[string]struct{} {
	nprops := n.PropertySet()
	for _, prop := range props {
		delete(nprops, prop)
	}
	return nprops
}

func (n *Node) isNode() bool {
	if n == nil {
		return false
	}

	return !n.Has(KeywordList) && !n.Has(KeywordValue) && !n.Has(KeywordSet)
}

// Has returns if a node has the requested property.
//
// Properties must either be a JSON-LD keyword, or an expanded IRI.
func (n *Node) Has(prop string) bool {
	if n == nil {
		return false
	}

	switch prop {
	case KeywordID:
		return n.ID != ""
	case KeywordValue:
		return n.Value != nil
	case KeywordLanguage:
		return n.Language != ""
	case KeywordDirection:
		return n.Direction != ""
	case KeywordType:
		return n.Type != nil
	case KeywordList:
		return n.List != nil
	case KeywordSet:
		return n.Set != nil
	case KeywordGraph:
		return n.Graph != nil
	case KeywordIncluded:
		return n.Included != nil
	case KeywordIndex:
		return n.Index != ""
	case KeywordReverse:
		return n.Reverse != nil
	default:
		for key := range n.Properties {
			if prop == key {
				return true
			}
		}
		return false
	}
}

// IsZero returns if this is the zero value of a [Node].
func (n *Node) IsZero() bool {
	if n == nil {
		return true
	}

	return len(n.PropertySet()) == 0
}

// IsSubject checks if this node is a subject.
//
// This means:
//   - It has an @id.
//   - It may have an @type.
//   - It has at least one other property.
func (n *Node) IsSubject() bool {
	if n == nil {
		return false
	}

	if !n.Has(KeywordID) {
		return false
	}

	return len(n.propsWithout(KeywordID, KeywordIndex)) != 0
}

// IsSubjectReference checks if this node is a subject reference.
//
// This means:
//   - It has an @id.
//   - It may have an @type.
//   - It has no other properties.
func (n *Node) IsSubjectReference() bool {
	if n == nil {
		return false
	}

	if !n.Has(KeywordID) {
		return false
	}

	return len(n.propsWithout(KeywordID, KeywordType)) == 0
}

// IsList checks if this node is a list.
//
// This means:
//   - It has an @list.
//   - It has no other properties.
func (n *Node) IsList() bool {
	if n == nil {
		return false
	}

	if !n.Has(KeywordList) {
		return false
	}

	return len(n.propsWithout(KeywordList, KeywordIndex)) == 0
}

// IsValue checks if this is a value node.
//
// This means:
//   - It has an @value.
//   - It may have an @direction, @index, @langauge and @type.
//   - It has no other properties.
//
// Additionally, it's invalid to have @type together with @language or
// @direction.
func (n *Node) IsValue() bool {
	if n == nil {
		return false
	}

	if !n.Has(KeywordValue) {
		return false
	}

	return len(n.propsWithout(
		KeywordValue,
		KeywordDirection,
		KeywordIndex,
		KeywordLanguage,
		KeywordType,
	)) == 0
}

// IsGraph returns if the object is a graph.
//
// This requires:
//   - It must have an @graph.
//   - It may have @id and @index.
//   - It has no other properties.
func (n *Node) IsGraph() bool {
	if n == nil {
		return false
	}

	if !n.Has(KeywordGraph) {
		return false
	}

	return len(n.propsWithout(KeywordID, KeywordIndex, KeywordGraph)) == 0
}

// IsSimpleGraph returns if the object is a simple graph.
//
// This requires:
//   - It must have an @graph.
//   - It may have @index.
//   - It has no other properties.
func (n *Node) IsSimpleGraph() bool {
	if n == nil {
		return false
	}

	if !n.Has(KeywordGraph) {
		return false
	}

	return len(n.propsWithout(KeywordIndex, KeywordGraph)) == 0
}

// MarshalJSON encodes to Expanded Document Form.
func (n *Node) MarshalJSON() ([]byte, error) {
	result := map[string]any{}

	if n.Has(KeywordID) {
		result[KeywordID] = n.ID
	}

	if n.Has(KeywordIndex) {
		result[KeywordIndex] = n.Index
	}

	if n.Has(KeywordType) {
		var data any
		if n.Value != nil && len(n.Type) == 1 {
			data = n.Type[0]
		} else {
			data = n.Type
		}
		result[KeywordType] = data
	}

	if n.Has(KeywordValue) {
		result[KeywordValue] = n.Value
	}

	if n.Has(KeywordLanguage) {
		result[KeywordLanguage] = n.Language
	}

	if n.Has(KeywordDirection) {
		result[KeywordDirection] = n.Direction
	}

	if n.Has(KeywordList) {
		result[KeywordList] = n.List
	}

	if n.Has(KeywordGraph) {
		result[KeywordGraph] = n.Graph
	}

	if n.Has(KeywordIncluded) {
		result[KeywordIncluded] = n.Included
	}

	if n.Has(KeywordReverse) {
		result[KeywordReverse] = n.Reverse
	}

	for k, v := range n.Properties {
		result[k] = v
	}

	return json.Marshal(result)
}

// GetNodes returns the nodes stored in property.
func (n *Node) GetNodes(property string) []Node {
	switch property {
	case KeywordGraph:
		return n.Graph
	case KeywordIncluded:
		return n.Included
	case KeywordList:
		return n.List
	case KeywordSet:
		return n.Set
	default:
		if !n.Has(property) {
			return nil
		}
		return n.Properties[property]
	}
}

// AddNodes appends the nodes stored in property.
func (n *Node) AddNodes(property string, nodes ...Node) {
	n.Properties[property] = append(n.Properties[property], nodes...)
}

// SetNodes overrides the nodes stored in property.
func (n *Node) SetNodes(property string, nodes ...Node) {
	n.Properties[property] = nodes
}
