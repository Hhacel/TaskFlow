package tfutil

// StringPtr returns a pointer to the given string value
func StringPtr(s string) *string {
	return &s
}

// ToPtr returns a pointer to the given value of any type
func ToPtr[T any](v T) *T {
	return &v
}
