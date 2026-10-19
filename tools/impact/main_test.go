package main

import (
	"strings"
	"sync"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// The module is loaded once for all tests (a few seconds).
var (
	loadOnce sync.Once
	loaded   *module
	loadErr  error
)

func repo(t *testing.T) *module {
	t.Helper()
	loadOnce.Do(func() { loaded, loadErr = load(reporoot.Kok()) })
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	return loaded
}

func query(t *testing.T, spec string) string {
	t.Helper()
	var b strings.Builder
	if err := run(&b, repo(t), spec); err != nil {
		t.Fatalf("%s: %v", spec, err)
	}
	return b.String()
}

func mustContain(t *testing.T, out string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(out, p) {
			t.Errorf("output lacks %q:\n%s", p, out)
		}
	}
}

// A name's uses are found across packages, including test files.
func TestNameUses(t *testing.T) {
	out := query(t, "codes.Name")
	mustContain(t, out, "internal/lang/codes/language.go", "internal/lang/hint\n", "internal/lang/hint/hint.go:", "_test.go:")
}

// The hooks the interpreter finds by type assertion: the implementers must be listed, because
// a renamed method drops a hook without a compile error.
func TestInterfaceImplementers(t *testing.T) {
	mustContain(t, query(t, "interp.MemberAssignable"), "1 implementers", "ml.Iyilestirici")
	mustContain(t, query(t, "interp.Iterable"), "ml.Deger")
	mustContain(t, query(t, "interp.MemberAssignable.SetMember"), "ml.Iyilestirici")
}

// Methods are written Type.Member; a package can be named by its path or its last element.
func TestMembersAndPackages(t *testing.T) {
	mustContain(t, query(t, "interp.VM.Interrupt"), "internal/lang/interp/interrupt.go:")
	out := query(t, "internal/lang/parser")
	mustContain(t, out, "internal/lang/source: ", "Parser.RecordRoles (1)")
	if out != query(t, "parser") {
		t.Error("the package's path and its last element gave different answers")
	}
}

func TestUnknownNames(t *testing.T) {
	for _, spec := range []string{"codes.NoSuchName", "nosuchpkg.Name", "interp.VM.NoSuchMethod"} {
		if err := run(new(strings.Builder), repo(t), spec); err == nil {
			t.Errorf("%s: expected an error", spec)
		}
	}
}
