package twiddle

type Size interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

// Field is a bitset over an unsigned integer.
type Field[T Size] struct {
	bits T
}

// Set ORs the flags of a and more into Field.
func (f *Field[T]) Set(a T, more ...T) {
	for _, m := range more {
		a |= m
	}
	f.bits |= a
}

// Bits returns the bits in Field.
func (f Field[T]) Bits() T {
	return f.bits
}

// Any reports if Field has at least one of the requested flags set.
func (f Field[T]) Any(a T, more ...T) bool {
	for _, m := range more {
		a |= m
	}
	return f.bits&a != 0
}

// All reports if Field has all of the requested flags set.
func (f Field[T]) All(a T, more ...T) bool {
	for _, m := range more {
		a |= m
	}
	return f.bits&a == a
}

// None reports if f has none of the requested flags set.
func (f Field[T]) None(a T, more ...T) bool {
	for _, m := range more {
		a |= m
	}
	return f.bits&a == 0
}
