package longdistance

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"maps"
	"slices"

	"sourcery.dny.nu/longdistance/internal/jsonutil"
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

// PropertySet returns a [Set] with an entry for each property that is set on
// the [Node].
func (n *Node) PropertySet() map[string]struct{} {
	if n == nil {
		return nil
	}

	res := make(map[string]struct{}, len(n.Properties)+2)

	add := func(keyword string, set bool) {
		if set {
			res[keyword] = struct{}{}
		}
	}

	add(KeywordDirection, n.Direction != "")
	add(KeywordGraph, n.Graph != nil)
	add(KeywordID, n.ID != "")
	add(KeywordIncluded, n.Included != nil)
	add(KeywordIndex, n.Index != "")
	add(KeywordLanguage, n.Language != "")
	add(KeywordList, n.List != nil)
	add(KeywordReverse, n.Reverse != nil)
	add(KeywordSet, n.Set != nil)
	add(KeywordType, n.Type != nil)
	add(KeywordValue, n.Value != nil)

	for p := range n.Properties {
		res[p] = struct{}{}
	}

	return res
}

func (n *Node) propsWithout(props ...string) map[string]struct{} {
	if n == nil {
		return nil
	}

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

	if n.ID == "" {
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

	if n.List == nil {
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

	if n.Value == nil {
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

	if n.Graph == nil {
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

	if n.Graph == nil {
		return false
	}

	return len(n.propsWithout(KeywordIndex, KeywordGraph)) == 0
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

// MarshalJSONTo encodes to Expanded Document Form.
func (n *Node) MarshalJSONTo(enc *jsontext.Encoder) error {
	if err := enc.WriteToken(jsontext.BeginObject); err != nil {
		return err
	}

	if n == nil {
		return enc.WriteToken(jsontext.EndObject)
	}

	write := func(keyword string, value any) error {
		if err := enc.WriteToken(jsontext.String(keyword)); err != nil {
			return err
		}
		return json.MarshalEncode(enc, value)
	}

	if n.Direction != "" {
		if err := write(KeywordDirection, n.Direction); err != nil {
			return err
		}
	}

	if n.Graph != nil {
		if err := write(KeywordGraph, n.Graph); err != nil {
			return err
		}
	}

	if n.ID != "" {
		if err := write(KeywordID, n.ID); err != nil {
			return err
		}
	}

	if n.Included != nil {
		if err := write(KeywordIncluded, n.Included); err != nil {
			return err
		}
	}

	if n.Index != "" {
		if err := write(KeywordIndex, n.Index); err != nil {
			return err
		}
	}

	if n.Language != "" {
		if err := write(KeywordLanguage, n.Language); err != nil {
			return err
		}
	}

	if n.List != nil {
		if err := write(KeywordList, n.List); err != nil {
			return err
		}
	}

	if n.Reverse != nil {
		if err := enc.WriteToken(jsontext.String(KeywordReverse)); err != nil {
			return err
		}

		if err := enc.WriteToken(jsontext.BeginObject); err != nil {
			return err
		}

		for _, k := range slices.Sorted(maps.Keys(n.Reverse)) {
			if err := write(k, n.Reverse[k]); err != nil {
				return err
			}
		}

		if err := enc.WriteToken(jsontext.EndObject); err != nil {
			return err
		}
	}

	if n.Type != nil {
		var data any
		if n.Value != nil && len(n.Type) == 1 {
			data = n.Type[0]
		} else {
			data = n.Type
		}
		if err := write(KeywordType, data); err != nil {
			return err
		}
	}

	if n.Value != nil {
		if err := enc.WriteToken(jsontext.String(KeywordValue)); err != nil {
			return err
		}
		if err := enc.WriteValue(n.Value); err != nil {
			return err
		}
	}

	for _, k := range slices.Sorted(maps.Keys(n.Properties)) {
		if err := write(k, n.Properties[k]); err != nil {
			return err
		}
	}

	return enc.WriteToken(jsontext.EndObject)
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

type compactNode struct {
	Attr       string
	Value      jsontext.Value
	Properties []compactNode
	Members    []compactNode
	Context    jsontext.Value
	IsID       bool
	IsType     bool
}

func (c *compactNode) get(key string) (*compactNode, bool) {
	for i := range c.Properties {
		if c.Properties[i].Attr == key {
			return &c.Properties[i], true
		}
	}
	return nil, false
}

// child returns the child for attr, creating it if absent.
func (c *compactNode) child(attr string) *compactNode {
	if e, ok := c.get(attr); ok {
		return e
	}

	c.Properties = append(c.Properties, compactNode{
		Attr:       attr,
		Properties: []compactNode{},
	})

	return &c.Properties[len(c.Properties)-1]
}

func (c *compactNode) del(key string) {
	for i := range c.Properties {
		if c.Properties[i].Attr == key {
			c.Properties = slices.Delete(c.Properties, i, i+1)
			return
		}
	}
}

// addValue adds item for key to object.
//
// If key already exists it becomes an array. Otherwise arrayification is
// dictated by asArray.
func (c *compactNode) addValue(key string, item compactNode, asArray bool) {
	if e, ok := c.get(key); ok {
		if e.Members == nil {
			*e = compactNode{Attr: e.Attr, Members: []compactNode{*e}}
		}

		e.Members = append(e.Members, item)
		return
	}

	if asArray && item.Members == nil {
		c.Properties = append(c.Properties, compactNode{
			Attr:    key,
			Members: []compactNode{item},
		})
		return
	}

	item.Attr = key
	c.Properties = append(c.Properties, item)
}

// MarshalJSONTo marshals the compact node.
//
// In the case of an object, key order is fixed:
//   - @context, if present, comes first.
//   - @type (or its alias) follows.
//   - @id (or its alias) comes next.
//   - Remaining keys are sorted in lexicographically least order.
//
// The order of @context, @type and @id ensures that the resulting output can be processed
// in a streaming manner by a JSON-LD processor.
func (c compactNode) MarshalJSONTo(enc *jsontext.Encoder) error {
	switch {
	case c.Members != nil:
		return json.MarshalEncode(enc, c.Members)
	case c.Properties != nil:
		if err := enc.WriteToken(jsontext.BeginObject); err != nil {
			return err
		}

		if c.Context != nil {
			if err := enc.WriteToken(jsontext.String(KeywordContext)); err != nil {
				return err
			}

			if err := enc.WriteValue(c.Context); err != nil {
				return err
			}
		}

		rank := func(p compactNode) int {
			switch {
			case p.IsID:
				return 1
			case p.IsType:
				return 0
			default:
				return 2
			}
		}

		slices.SortFunc(c.Properties, func(a, b compactNode) int {
			if r := rank(a) - rank(b); r != 0 {
				return r
			}
			return sortedLeast(a.Attr, b.Attr)
		})

		for _, val := range c.Properties {
			if err := enc.WriteToken(jsontext.String(val.Attr)); err != nil {
				return err
			}

			if err := json.MarshalEncode(enc, val); err != nil {
				return err
			}
		}

		return enc.WriteToken(jsontext.EndObject)
	default:
		return enc.WriteValue(c.Value)
	}
}

func (c *compactNode) asString() (string, bool) {
	if !jsonutil.IsString(c.Value) {
		return "", false
	}

	if jsonutil.IsEmptyString(c.Value) {
		return "", true
	}

	res, err := jsontext.AppendUnquote(nil, c.Value)
	if err != nil {
		return "", false
	}

	return string(res), true
}
