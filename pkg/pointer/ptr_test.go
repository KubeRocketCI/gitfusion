package pointer

import (
	"reflect"
	"testing"
)

func TestValueOrEmpty(t *testing.T) {
	type testCase[T any] struct {
		name     string
		input    *T
		expected T
	}

	intVal := 42
	strVal := "hello"
	boolVal := true

	type custom struct{ X int }

	customVal := custom{X: 7}

	testsInt := []testCase[int]{
		{
			name:     "nil int pointer returns zero",
			input:    nil,
			expected: 0,
		},
		{
			name:     "non-nil int pointer returns value",
			input:    &intVal,
			expected: 42,
		},
	}

	testsString := []testCase[string]{
		{
			name:     "nil string pointer returns empty string",
			input:    nil,
			expected: "",
		},
		{
			name:     "non-nil string pointer returns value",
			input:    &strVal,
			expected: "hello",
		},
	}

	testsBool := []testCase[bool]{
		{
			name:     "nil bool pointer returns false",
			input:    nil,
			expected: false,
		},
		{
			name:     "non-nil bool pointer returns value",
			input:    &boolVal,
			expected: true,
		},
	}

	testsCustom := []testCase[custom]{
		{
			name:     "nil custom pointer returns zero value",
			input:    nil,
			expected: custom{},
		},
		{
			name:     "non-nil custom pointer returns value",
			input:    &customVal,
			expected: custom{X: 7},
		},
	}

	for _, tc := range testsInt {
		t.Run("int/"+tc.name, func(t *testing.T) {
			got := ValueOrEmpty(tc.input)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("ValueOrEmpty() = %v, want %v", got, tc.expected)
			}
		})
	}

	for _, tc := range testsString {
		t.Run("string/"+tc.name, func(t *testing.T) {
			got := ValueOrEmpty(tc.input)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("ValueOrEmpty() = %v, want %v", got, tc.expected)
			}
		})
	}

	for _, tc := range testsBool {
		t.Run("bool/"+tc.name, func(t *testing.T) {
			got := ValueOrEmpty(tc.input)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("ValueOrEmpty() = %v, want %v", got, tc.expected)
			}
		})
	}

	for _, tc := range testsCustom {
		t.Run("custom/"+tc.name, func(t *testing.T) {
			got := ValueOrEmpty(tc.input)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("ValueOrEmpty() = %v, want %v", got, tc.expected)
			}
		})
	}
}
