// Package shape is a fixture.
package shape

import (
	"example.com/mini/internal/lang/codes"
	"example.com/mini/internal/lang/messages"
)

// Shape has an area.
type Shape interface {
	Area() float64
}

// Square is a Shape.
type Square struct{ Side float64 }

// Area implements Shape.
func (s Square) Area() float64 { return s.Side * s.Side }

// Describe reports a shape; it fails for an empty one.
func Describe(s Shape) error {
	if s.Area() == 0 {
		return messages.New(messages.H0001)
	}
	_ = codes.A001
	return nil
}

// Twice calls Describe twice.
func Twice(s Shape) error {
	if err := Describe(s); err != nil {
		return err
	}
	return Describe(s)
}

// Report is called by a test and calls Twice: Describe is tested only indirectly.
func Report(s Shape) error { return Twice(s) }

// Orphan has no test.
func Orphan() {}

// Sides is a package-level constant.
const Sides = 4

// Default is a package-level variable.
var Default = Square{1}

// Corners uses both.
func Corners() int {
	_ = Default
	return Sides
}

// Both calls two functions of messages.
func Both() string {
	_ = messages.New(messages.H0001)
	return messages.Text(messages.H0001)
}

// table registers implementations by name code.
var table = []struct {
	code codes.Code
	fn   func()
}{
	{codes.A001, runIf},
}

func runIf() {}

// Counter has fields that are read and written in several ways.
type Counter struct {
	n    int
	list []int
	next *Counter
}

// Other has a field named like Counter's.
type Other struct{ n int }

// Bump touches Counter.n in every way and returns a closure that captures variables.
func Bump(c *Counter, o *Other) func() int {
	c.n++
	c.n = 5
	c.list[0] = 1
	p := &c.n
	_ = p
	o.n = 9
	x := c.n
	sum := 0
	return func() int {
		c.n += x
		sum++
		return c.n
	}
}

// Make builds a Counter with a keyed literal.
func Make() Counter { return Counter{n: 1} }
