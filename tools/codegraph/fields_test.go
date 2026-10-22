package main

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

const (
	counterN    = "go:field:example.com/mini/shape.Counter.n"
	counterList = "go:field:example.com/mini/shape.Counter.list"
	otherN      = "go:field:example.com/mini/shape.Other.n"
	bumpID      = "go:func:example.com/mini/shape.Bump"
)

// siteAt bir kenarda dosya:satırdaki erişim yerini arar.
func siteAt(g *Graph, from, to, kind string, line int) *Site {
	e := edge(g, from, to, kind)
	if e == nil {
		return nil
	}
	for i := range e.Sites {
		if e.Sites[i].Line == line {
			return &e.Sites[i]
		}
	}
	return nil
}

func TestFieldNodesAndAccessModes(t *testing.T) {
	g := fixture(t)
	for _, id := range []string{counterN, counterList, otherN} {
		if !hasNode(g, id) {
			t.Fatalf("missing field node %s", id)
		}
	}
	mustEdge(t, g, "go:type:example.com/mini/shape.Counter", counterN, edgeContains, extracted)
	// Counter.n: 83 c.n++, 84 c.n = 5, 86 &c.n, 89 x := c.n, 92 c.n += x (kapanış), 94 return c.n (kapanış), 99 Counter{n: 1}
	want := map[int]struct{ kind, mode string }{
		83: {edgeReadsField, modeReadWrite}, 84: {edgeWritesField, modeWrite}, 86: {edgeWritesField, modeAddr},
		89: {edgeReadsField, modeRead}, 92: {edgeReadsField, modeReadWrite}, 94: {edgeReadsField, modeRead},
	}
	for line, w := range want {
		s := siteAt(g, bumpID, counterN, w.kind, line)
		if s == nil || s.Mode != w.mode {
			t.Errorf("line %d: want %s %s, got %+v", line, w.kind, w.mode, s)
		}
	}
	if s := siteAt(g, bumpID, counterN, edgeWritesField, 92); s == nil || s.Mode != modeReadWrite {
		t.Errorf("c.n += x also writes: %+v", s)
	}
	if s := siteAt(g, "go:func:example.com/mini/shape.Make", counterN, edgeWritesField, 99); s == nil || s.Mode != modeLiteral {
		t.Errorf("keyed literal: %+v", s)
	}
	if s := siteAt(g, bumpID, counterList, edgeWritesField, 85); s == nil || s.Mode != modeElemWrite {
		t.Errorf("c.list[0] = 1 is an element write: %+v", s)
	}
}

func TestFieldsOfOtherTypesAreNotMixed(t *testing.T) {
	g := fixture(t)
	if siteAt(g, bumpID, counterN, edgeWritesField, 88) != nil {
		t.Error("o.n = 9 belongs to Other.n, not Counter.n")
	}
	if s := siteAt(g, bumpID, otherN, edgeWritesField, 88); s == nil || s.Mode != modeWrite {
		t.Errorf("Other.n write: %+v", s)
	}
}

func TestClosureAccessesNameTheClosure(t *testing.T) {
	g := fixture(t)
	s := siteAt(g, bumpID, counterN, edgeReadsField, 94)
	if s == nil || s.Closure == "" {
		t.Fatalf("an access inside a func literal records the closure: %+v", s)
	}
	for _, n := range g.Nodes {
		if n.ID == s.Closure {
			if n.Kind != kindClosure || n.Name != "shape.Bump > func literal@L91" {
				t.Errorf("closure node: %+v", n)
			}
			return
		}
	}
	t.Errorf("closure node %s missing", s.Closure)
}

func TestFieldsCommands(t *testing.T) {
	code, out, _ := cli(t, "who-uses", "Counter.n")
	if code != exitOK {
		t.Fatalf("exit %d\n%s", code, out)
	}
	testkit.MustContain(t, out, "shape.Counter.n [field] shape/shape.go:", "7 accesses in 2 functions: 2 read, 1 write, 2 read+write, 1 addr, 1 literal", "shape/shape.go:83  read+write  shape.Bump",
		"shape/shape.go:94  read        shape.Bump > func literal@L91", "shape/shape.go:99  literal     shape.Make")
	if strings.Contains(out, ":88 ") {
		t.Errorf("Other.n must not appear:\n%s", out)
	}
	_, out, _ = cli(t, "fields", "Counter.n", "--mode", "write")
	testkit.MustContain(t, out, "write accesses (", "shape/shape.go:84  write")
	if strings.Contains(out, ":89 ") {
		t.Errorf("--mode write hides plain reads:\n%s", out)
	}
	_, out, _ = cli(t, "fields", "shape.Counter")
	testkit.MustContain(t, out, "n [int]:", "list [[]int]: 0 reads, 1 writes", "next [*shape.Counter]: 0 reads, 0 writes")
}

func TestCaptures(t *testing.T) {
	code, out, _ := cli(t, "captures", "shape.Bump")
	if code != exitOK {
		t.Fatalf("exit %d\n%s", code, out)
	}
	testkit.MustContain(t, out, "shape.Bump > func literal@L91", "c *shape.Counter", "x int", "sum int (assigned)")
	if strings.Contains(out, "o *shape.Other") {
		t.Errorf("o is not used in the closure:\n%s", out)
	}
	_, out, _ = cli(t, "captures", "shape.Bump", "--type", "Counter")
	testkit.MustContain(t, out, "c *shape.Counter")
	if strings.Contains(out, "x int") {
		t.Errorf("--type filters the captured variables:\n%s", out)
	}
	_, out, _ = cli(t, "captures", "shape.Describe")
	testkit.MustContain(t, out, "no func literals")
}
