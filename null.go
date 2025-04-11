package longdistance

import (
	"code.dny.dev/longdistance/internal/json"
)

// Scalar is the interface of Go types that match JSON scalars.
type Scalar interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64 | ~bool |
		~string
}

// Null represents values so we can differentiate between the explicit JSON Null
// versus it being set to some other value, or being absent.
type Null[T Scalar] struct {
	Null  bool
	Set   bool
	Value T
}

// Equal checks if two values are the same.
func (n *Null[T]) Equal(on *Null[T]) bool {
	if n == nil && on == nil {
		return true
	}
	if n == nil || on == nil {
		return false
	}

	return n.Null == on.Null && n.Set == on.Set && n.Value == on.Value
}

func (n *Null[T]) UnmarshalJSON(data []byte) error {
	n.Set = true
	if json.IsNull(data) {
		n.Null = true
		return nil
	}

	var s T
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	n.Value = s
	return nil
}
