// impact answers "what does changing this touch?" from the code itself, so the answer is never
// out of date. It type-checks the whole module (with tests) using only the standard library.
//
// Usage (from the repository root):
//
//	go run ./tools/impact codes.Name             // every use of a name, by package and file
//	go run ./tools/impact interp.Iterable        // an interface: its uses and its implementers
//	go run ./tools/impact interp.VM.Interrupt    // a method or field: Type.Member
//	go run ./tools/impact internal/lang/parser   // a package: who uses it, and which names
//
// A package may be written as its import path, its path inside the module, or its last
// element when that is unique. Only files built for the current operating system are read;
// platform-specific files (_linux.go, build tags) are not covered.
//
// Layer: tools (tools); not shipped to users.
package main

import (
	"fmt"
	"go/types"
	"io"
	"os"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/impact <package | package.Name | package.Type.Member>")
		os.Exit(2)
	}
	dir, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	m, err := load(dir)
	if err != nil {
		fail(err)
	}
	if err := run(os.Stdout, m, os.Args[1]); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "impact:", err)
	os.Exit(1)
}

// run answers one query: a package when spec names one, otherwise a name.
func run(w io.Writer, m *module, spec string) error {
	if path, err := m.findPackage(spec); err == nil {
		printPackage(w, m, path)
		return nil
	}
	obj, err := m.findObject(spec)
	if err != nil {
		return err
	}
	printObject(w, m, spec, obj)
	return nil
}

func printObject(w io.Writer, m *module, spec string, obj types.Object) {
	p := m.Fset.Position(obj.Pos())
	fmt.Fprintf(w, "%s  %s:%d\n", spec, m.rel(p.Filename), p.Line)
	uses := m.usesOf(obj)
	pkgs := map[string]bool{}
	for _, u := range uses {
		pkgs[u.pkg] = true
	}
	fmt.Fprintf(w, "\n%d uses in %d packages\n", len(uses), len(pkgs))
	lastPkg, lastFile := "", ""
	var lines []string
	flush := func() {
		if lastFile != "" {
			fmt.Fprintf(w, "    %s: %s\n", lastFile, strings.Join(lines, ", "))
		}
		lines = nil
	}
	for _, u := range uses {
		if u.pkg != lastPkg {
			flush()
			fmt.Fprintf(w, "  %s\n", u.pkg)
			lastPkg, lastFile = u.pkg, ""
		}
		if u.file != lastFile {
			flush()
			lastFile = u.file
		}
		if l := fmt.Sprint(u.line); len(lines) == 0 || lines[len(lines)-1] != l {
			lines = append(lines, l)
		}
	}
	flush()
	if it := interfaceOf(obj); it != nil {
		impl := m.implementers(it)
		fmt.Fprintf(w, "\n%d implementers\n", len(impl))
		for _, s := range impl {
			fmt.Fprintf(w, "  %s\n", s)
		}
	}
}

func printPackage(w io.Writer, m *module, path string) {
	users := m.packageUsers(path)
	var names []string
	for who := range users {
		names = append(names, who)
	}
	sort.Strings(names)
	fmt.Fprintf(w, "%s\n\nused by %d packages\n", m.shortPath(path), len(names))
	for _, who := range names {
		used := users[who]
		var parts []string
		for n, c := range used {
			parts = append(parts, fmt.Sprintf("%s (%d)", n, c))
		}
		sort.Strings(parts)
		fmt.Fprintf(w, "  %s: %s\n", who, strings.Join(parts, ", "))
	}
}
