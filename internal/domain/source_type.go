package domain

import "fmt"

// SourceType identifies the origin of a transaction.
type SourceType string

const (
	SourceGame    SourceType = "game"
	SourceServer  SourceType = "server"
	SourcePayment SourceType = "payment"
)

var validSourceTypes = map[SourceType]struct{}{
	SourceGame:    {},
	SourceServer:  {},
	SourcePayment: {},
}

// IsValid reports whether the source type is part of the business vocabulary.
func (s SourceType) IsValid() bool {
	_, ok := validSourceTypes[s]
	return ok
}

// ParseSourceType parses an external string into a validated SourceType.
func ParseSourceType(value string) (SourceType, error) {
	s := SourceType(value)
	if !s.IsValid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidSourceType, value)
	}
	return s, nil
}
