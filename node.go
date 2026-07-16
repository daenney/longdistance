package longdistance

import (
	"cmp"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"iter"
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
	Direction string         // @direction / KeywordDirection
	Graph     []Node         // @graph / KeywordGraph
	ID        string         // @id / KeywordID
	Included  []Node         // @included / KeywordIncluded
	Index     string         // @index / KeywordIndex
	Language  string         // @language / KeywordLanguage
	List      []Node         // @list / KeywordList
	Reverse   Properties     // @reverse / KeywordReverse
	Set       []Node         // @set / KeywordSet
	Type      []string       // @type / KeywordType
	Value     jsontext.Value // @value / KeywordValue

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
		Value      jsontext.Value
		Properties Properties
	}
}

// PropertyKeys returns the key of every proprety set on [Node].
func (n *Node) PropertyKeys() iter.Seq[string] {
	return func(yield func(string) bool) {
		if n == nil {
			return
		}

		y := func(keyword string, ok bool) {
			if ok && !yield(keyword) {
				return
			}
		}

		y(KeywordDirection, n.Direction != "")
		y(KeywordGraph, n.Graph != nil)
		y(KeywordID, n.ID != "")
		y(KeywordIncluded, n.Included != nil)
		y(KeywordIndex, n.Index != "")
		y(KeywordLanguage, n.Language != "")
		y(KeywordList, n.List != nil)
		y(KeywordReverse, n.Reverse != nil)
		y(KeywordSet, n.Set != nil)
		y(KeywordType, n.Type != nil)
		y(KeywordValue, n.Value != nil)

		for p := range n.Properties {
			if !yield(p) {
				return
			}
		}
	}
}

// propsWithout returns how many properties are set on the node, excluding
// the given ones.
func (n *Node) propsWithout(props ...string) int {
	if n == nil {
		return 0
	}

	count := n.Len()
	for _, prop := range props {
		if n.Has(prop) {
			count--
		}
	}
	return count
}

func (n *Node) isNode() bool {
	if n == nil {
		return false
	}

	return n.List == nil && n.Value == nil && n.Set == nil
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
		_, ok := n.Properties[prop]
		return ok
	}
}

// IsZero returns if this is the zero value of a [Node].
func (n *Node) IsZero() bool {
	if n == nil {
		return true
	}

	return n.Direction == "" &&
		n.Graph == nil &&
		n.ID == "" &&
		n.Included == nil &&
		n.Index == "" &&
		n.Language == "" &&
		n.List == nil &&
		n.Reverse == nil &&
		n.Set == nil &&
		n.Type == nil &&
		n.Value == nil &&
		len(n.Properties) == 0
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

	if n.ID == "" {
		return false
	}

	return n.propsWithout(KeywordID, KeywordIndex) != 0
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

	if n.ID == "" {
		return false
	}

	return n.propsWithout(KeywordID, KeywordType) == 0
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

	if n.List == nil {
		return false
	}

	return n.propsWithout(KeywordList, KeywordIndex) == 0
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

	if n.Value == nil {
		return false
	}

	return n.propsWithout(
		KeywordValue,
		KeywordDirection,
		KeywordIndex,
		KeywordLanguage,
		KeywordType,
	) == 0
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

	if n.Graph == nil {
		return false
	}

	return n.propsWithout(KeywordID, KeywordIndex, KeywordGraph) == 0
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

	if n.Graph == nil {
		return false
	}

	return n.propsWithout(KeywordIndex, KeywordGraph) == 0
}

func (n *Node) Len() int {
	if n == nil {
		return 0
	}

	count := len(n.Properties)

	incr := func(set bool) {
		if set {
			count++
		}
	}

	incr(n.Direction != "")
	incr(n.Graph != nil)
	incr(n.ID != "")
	incr(n.Included != nil)
	incr(n.Index != "")
	incr(n.Language != "")
	incr(n.List != nil)
	incr(n.Reverse != nil)
	incr(n.Set != nil)
	incr(n.Type != nil)
	incr(n.Value != nil)

	return count
}

func (n *Node) MarshalJSONTo(enc *jsontext.Encoder) error {
	if err := enc.WriteToken(jsontext.BeginObject); err != nil {
		return err
	}

	if n == nil {
		return enc.WriteToken(jsontext.EndObject)
	}

	if err := encStringKV(enc, KeywordID, n.ID); err != nil {
		return err
	}

	if err := encStringKV(enc, KeywordIndex, n.Index); err != nil {
		return err
	}

	if n.Type != nil {
		if err := enc.WriteToken(jsontext.String(KeywordType)); err != nil {
			return err
		}

		if n.Value != nil && len(n.Type) == 1 {
			if err := enc.WriteToken(jsontext.String(n.Type[0])); err != nil {
				return err
			}
		} else {
			if err := enc.WriteToken(jsontext.BeginArray); err != nil {
				return err
			}
			for _, t := range n.Type {
				if err := enc.WriteToken(jsontext.String(t)); err != nil {
					return err
				}
			}
			if err := enc.WriteToken(jsontext.EndArray); err != nil {
				return err
			}
		}
	}

	if n.Value != nil {
		if err := cmp.Or(
			enc.WriteToken(jsontext.String(KeywordValue)),
			enc.WriteValue(n.Value)); err != nil {
			return err
		}
	}

	if err := encStringKV(enc, KeywordLanguage, n.Language); err != nil {
		return err
	}

	if err := encStringKV(enc, KeywordDirection, n.Direction); err != nil {
		return err
	}

	if err := encNodes(enc, KeywordList, n.List); err != nil {
		return err
	}

	if err := encNodes(enc, KeywordGraph, n.Graph); err != nil {
		return err
	}

	if err := encNodes(enc, KeywordIncluded, n.Included); err != nil {
		return err
	}

	if n.Reverse != nil {
		if err := cmp.Or(
			enc.WriteToken(jsontext.String(KeywordReverse)),
			json.MarshalEncode(enc, n.Reverse),
		); err != nil {
			return err
		}
	}

	for k, v := range n.Properties {
		if err := cmp.Or(
			enc.WriteToken(jsontext.String(k)),
			json.MarshalEncode(enc, v),
		); err != nil {
			return err
		}
	}

	return enc.WriteToken(jsontext.EndObject)
}

func encStringKV(enc *jsontext.Encoder, k, v string) error {
	if v != "" {
		err := cmp.Or(
			enc.WriteToken(jsontext.String(k)),
			enc.WriteToken(jsontext.String(v)),
		)

		if err != nil {
			return err
		}
	}

	return nil
}

func encNodes(enc *jsontext.Encoder, k string, nodes []Node) error {
	if nodes != nil {
		if err := cmp.Or(
			enc.WriteToken(jsontext.String(k)),
			enc.WriteToken(jsontext.BeginArray)); err != nil {
			return err
		}

		for _, t := range nodes {
			if err := t.MarshalJSONTo(enc); err != nil {
				return err
			}
		}

		if err := enc.WriteToken(jsontext.EndArray); err != nil {
			return err
		}
	}

	return nil
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
		v, ok := n.Properties[property]
		if !ok {
			return nil
		}
		return v
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
