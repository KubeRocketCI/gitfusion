package pointer

// To returns a pointer to the given value.
func To[T any](v T) *T {
	return &v
}

// ValueOrEmpty returns the value pointed to by ptr, or the zero value if ptr is nil.
func ValueOrEmpty[T any](ptr *T) T {
	if ptr == nil {
		var zero T
		return zero
	}

	return *ptr
}
