package twiddle_test

import (
	"testing"

	"sourcery.dny.nu/longdistance/internal/twiddle"
)

func TestTwiddleSet(t *testing.T) {
	type Flag uint8
	const (
		FlagA Flag = 1 << iota
		FlagB
	)

	tests := []struct {
		name   string
		flags  []Flag
		result Flag
	}{
		{name: "single field", flags: []Flag{FlagA, 0}, result: FlagA},
		{name: "multiple flags", flags: []Flag{FlagA, FlagB}, result: FlagA | FlagB},
		{name: "repleated flags", flags: []Flag{FlagA, FlagB, FlagA}, result: FlagA | FlagB},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var field twiddle.Field[Flag]
			field.Set(tc.flags[0], tc.flags[1:]...)

			if field.Bits() != tc.result {
				t.Fatalf("expected: %d, got: %d", tc.result, field.Bits())
			}
		})
	}
}

func TestTwiddleAny(t *testing.T) {
	type Flag uint8
	const (
		FlagA Flag = 1 << iota
		FlagB
		FlagC
	)

	tests := []struct {
		name    string
		flags   []Flag
		looking []Flag
		found   bool
	}{
		{name: "present flag", flags: []Flag{FlagA, 0}, looking: []Flag{FlagA, 0}, found: true},
		{name: "absent flag", flags: []Flag{FlagA, 0}, looking: []Flag{FlagB, 0}, found: false},
		{name: "some flags present", flags: []Flag{FlagA, FlagB}, looking: []Flag{FlagB, FlagC}, found: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var field twiddle.Field[Flag]
			field.Set(tc.flags[0], tc.flags[1:]...)

			if res := field.Any(tc.looking[0], tc.looking[1:]...); res != tc.found {
				t.Fatalf("expected: %v, got: %v", tc.found, res)
			}
		})
	}
}

func TestTwiddleAll(t *testing.T) {
	type Flag uint8
	const (
		FlagA Flag = 1 << iota
		FlagB
		FlagC
	)

	tests := []struct {
		name    string
		flags   []Flag
		looking []Flag
		found   bool
	}{
		{name: "present flag", flags: []Flag{FlagA, 0}, looking: []Flag{FlagA, 0}, found: true},
		{name: "absent flag", flags: []Flag{FlagA, 0}, looking: []Flag{FlagB, 0}, found: false},
		{name: "some flags present", flags: []Flag{FlagA, FlagB}, looking: []Flag{FlagB, FlagC}, found: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var field twiddle.Field[Flag]
			field.Set(tc.flags[0], tc.flags[1:]...)

			if res := field.All(tc.looking[0], tc.looking[1:]...); res != tc.found {
				t.Fatalf("expected: %v, got: %v", tc.found, res)
			}
		})
	}
}

func TestTwiddleNone(t *testing.T) {
	type Flag uint8
	const (
		FlagA Flag = 1 << iota
		FlagB
		FlagC
	)

	tests := []struct {
		name    string
		flags   []Flag
		looking []Flag
		found   bool
	}{
		{name: "present flag", flags: []Flag{FlagA, 0}, looking: []Flag{FlagA, 0}, found: false},
		{name: "absent flag", flags: []Flag{FlagA, 0}, looking: []Flag{FlagB, 0}, found: true},
		{name: "some flags present", flags: []Flag{FlagA, FlagB}, looking: []Flag{FlagB, FlagC}, found: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var field twiddle.Field[Flag]
			field.Set(tc.flags[0], tc.flags[1:]...)

			if res := field.None(tc.looking[0], tc.looking[1:]...); res != tc.found {
				t.Fatalf("expected: %v, got: %v", tc.found, res)
			}
		})
	}
}
