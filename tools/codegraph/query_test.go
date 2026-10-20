package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func mustContain(t *testing.T, out string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(out, p) {
			t.Errorf("output lacks %q:\n%s", p, out)
		}
	}
}

func TestQueryShowsNeighbours(t *testing.T) {
	code, out, _ := cli(t, "query", "shape.Describe")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, out, "shape.Describe [func] shape/shape.go:21", "-> calls", "shape.Shape.Area", "-> uses_message", "H0001",
		"<- calls", "shape.Twice")
}

func TestQueryAmbiguousListsCandidates(t *testing.T) {
	code, out, _ := cli(t, "query", "Code")
	if code != exitAmbiguous {
		t.Fatalf("exit %d, want %d", code, exitAmbiguous)
	}
	mustContain(t, out, "ambiguous", "messages.Code", "codes.Code")
}

func TestQueryQualifiedNameDisambiguates(t *testing.T) {
	code, out, _ := cli(t, "query", "internal/lang/codes.Code")
	if code != exitOK || !strings.Contains(out, "codes.Code [type]") {
		t.Errorf("exit %d:\n%s", code, out)
	}
}

func TestQueryNotFound(t *testing.T) {
	code, out, errOut := cli(t, "query", "Describ")
	if code != exitNotFound || out != "" {
		t.Errorf("exit %d, stdout %q", code, out)
	}
	mustContain(t, errOut, "not found", "similar:", "shape.Describe")
}

func TestFlagsMayFollowNames(t *testing.T) {
	code, out, _ := cli(t, "query", "shape.Describe", "--limit", "1")
	if code != exitOK || !strings.Contains(out, "shape.Describe") {
		t.Errorf("exit %d:\n%s", code, out)
	}
}

func TestWhoUsesMessage(t *testing.T) {
	code, out, _ := cli(t, "who-uses", "H0001")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, out, "first message", "<- uses_message", "shape.Describe [func] at shape/shape.go:")
	code, out, _ = cli(t, "who-uses", "H0002")
	if code != exitOK || !strings.Contains(out, "no uses") {
		t.Errorf("H0002 is unused: exit %d\n%s", code, out)
	}
}

func TestTestsFor(t *testing.T) {
	code, out, _ := cli(t, "tests-for", "shape.Describe")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, out, "shape.TestDescribe", "rules guarded by these tests", "Empty shapes fail")
	code, out, _ = cli(t, "tests-for", "shape.Square.Area")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, out, "shape.TestSquare_Area")
	_, out, _ = cli(t, "tests-for", "shape.Twice")
	mustContain(t, out, "no tests found")
}

func TestPath(t *testing.T) {
	code, out, _ := cli(t, "path", "shape.Twice", "H0001")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, out, "path (directed, 2 steps)", "-calls->", "-uses_message->", "H0001")
	code, _, errOut := cli(t, "path", "H0002", "A001")
	if code != exitNotFound || !strings.Contains(errOut, "no path") {
		t.Errorf("exit %d: %s", code, errOut)
	}
}

func TestUsageErrors(t *testing.T) {
	var so, se bytes.Buffer
	if code := run(nil, &so, &se); code != exitUsage {
		t.Errorf("no args: exit %d", code)
	}
	if code := run([]string{"bogus"}, &so, &se); code != exitUsage {
		t.Errorf("unknown command: exit %d", code)
	}
	if code, _, _ := cli(t, "path", "only-one"); code != exitUsage {
		t.Errorf("path with one name: exit %d", code)
	}
}

func TestMissingGraphFile(t *testing.T) {
	var so, se bytes.Buffer
	code := run([]string{"query", "--graph", filepath.Join(t.TempDir(), "none.json"), "x"}, &so, &se)
	if code != exitFail || !strings.Contains(se.String(), "run `go run ./tools/codegraph build`") {
		t.Errorf("exit %d: %s", code, se.String())
	}
}

func TestBuildCommandWritesGraph(t *testing.T) {
	out := filepath.Join(t.TempDir(), "sub", "graph.json")
	var so, se bytes.Buffer
	code := run([]string{"build", "--root", filepath.Join("testdata", "mod"), "--out", out}, &so, &se)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, se.String())
	}
	g, err := readGraph(out)
	if err != nil || len(g.Nodes) == 0 {
		t.Fatalf("graph not readable: %v", err)
	}
	mustContain(t, so.String(), "nodes", "EXTRACTED", "warnings:")
}
