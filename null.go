package longdistance

import (
	"code.dny.dev/longdistance/internal/json"
)

// null represents values so we can differentiate between the explicit JSON null
// versus it being set to some other value, or being absent.
type null struct {
	Null  bool
	Set   bool
	Value string
}

// Equal checks if two values are the same.
func (n *null) Equal(on *null) bool {
	if n == nil && on == nil {
		return true
	}
	if n == nil || on == nil {
		return false
	}

	return n.Null == on.Null && n.Set == on.Set && n.Value == on.Value
}

func (n *null) UnmarshalJSON(data []byte) error {
	n.Set = true
	if json.IsNull(data) {
		n.Null = true
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	n.Value = s
	return nil
}
