package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/goload"
)

// branchDecls ref'teki Go dosyalarını ayrıştırıp, verilen hunk'ların yeni taraftaki satırlarına
// düşen işlev, metot ve tip bildirimlerinin adlarını verir (dosya -> adlar). Grafikte olmayan,
// yalnızca dalda bulunan kodu adlandırmak içindir (yeni işlevler, yeni test dosyaları).
func branchDecls(root, ref string, hs []hunk) map[string][]string {
	out := map[string][]string{}
	byFile := map[string][]hunk{}
	for _, h := range hs {
		if strings.HasSuffix(h.file, ".go") && h.newCount > 0 {
			byFile[h.file] = append(byFile[h.file], h)
		}
	}
	for file, list := range byFile {
		src, err := gitOut(root, "show", ref+":"+file)
		if err != nil {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, file, src, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		seen := map[string]bool{}
		for _, d := range f.Decls {
			name, kind := declName(d)
			if name == "" || seen[name] {
				continue
			}
			lo, hi := fset.Position(d.Pos()).Line, fset.Position(d.End()).Line
			for _, h := range list {
				if h.newStart <= hi && h.newStart+h.newCount-1 >= lo {
					seen[name] = true
					out[file] = append(out[file], fmt.Sprintf("%s (%s)", name, kind))
					break
				}
			}
		}
		sort.Strings(out[file])
	}
	return out
}

// declName bir bildirimin okunur adını ve türünü verir; ilgilenilmeyen bildirimde "".
func declName(d ast.Decl) (name, kind string) {
	switch d := d.(type) {
	case *ast.FuncDecl:
		if d.Recv != nil && len(d.Recv.List) == 1 {
			return goload.RecvExprName(d.Recv.List[0].Type) + "." + d.Name.Name, "method"
		}
		return d.Name.Name, "func"
	case *ast.GenDecl:
		if d.Tok == token.TYPE && len(d.Specs) == 1 {
			return d.Specs[0].(*ast.TypeSpec).Name.Name, "type"
		}
	}
	return "", ""
}
