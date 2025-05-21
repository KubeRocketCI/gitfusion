package pointer

// ValueOrEmpty returns the value pointed to by ptr, or the zero value if ptr is nil.
func ValueOrEmpty[T any](ptr *T) T {
	if ptr == nil {
		var zero T
		return zero
	}

	return *ptr
}
