// Package messages is a fixture.
package messages

// Code is a message code.
type Code uint16

const (
	H0001 Code = 1
	H0002 Code = 2
)

// New builds a message.
func New(c Code) error { return nil }

// Text renders a message.
func Text(c Code) string { return "" }
