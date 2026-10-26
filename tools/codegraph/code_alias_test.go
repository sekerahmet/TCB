package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"
	"testing"
)

// Kural: bir ad kodu paket içinde takma bir sabitle yazılabilir (`memberSkip = codes.MemSkip`, dataset/state.go); takma
// sabitin kullanımı kodun kullanımıdır (`uses_name`, codes paketine argümansa `reads_name`). Önceden yalnız takma sabitin
// bildirimi koda bağlıydı: `case memberSkip: ... o.skipTR` uygulamayı göstermiyor, `ö.atla`yı test dizesinde sınayan
// TestReaderSkipSameAsNext skipTR'nin testi görünmüyordu (eksikler listesi, kritik, ana oturum 2026-10-26). Takma sabitler
// ve kullanımları codegraph'tan bağımsız, modülün tipleri ve sözdizimiyle bulunur; her kullanımın dosyasından o koda bir
// kenar olmalı.
func TestANameCodeAliasIsAUseOfTheCode(t *testing.T) {
	root, m, g := repoGraph(t)
	keys := codeConstKeys(root)
	codesPkg := m.Path + "/" + codesDir
	aliases := map[string]string{} // paket yolu.ad -> kodun anahtarı
	for _, u := range m.Units {
		for _, f := range u.Files {
			for _, d := range f.Decls {
				gd, ok := d.(*ast.GenDecl)
				if !ok || gd.Tok != token.CONST {
					continue
				}
				for _, s := range gd.Specs {
					vs := s.(*ast.ValueSpec)
					for i, name := range vs.Names {
						if i >= len(vs.Values) {
							continue
						}
						sel, ok := vs.Values[i].(*ast.SelectorExpr)
						if !ok {
							continue
						}
						if c, ok := u.Info.Uses[sel.Sel].(*types.Const); ok && c.Pkg().Path() == codesPkg && keys[c.Name()] != "" {
							aliases[strings.TrimSuffix(u.PkgPath, "_test")+"."+name.Name] = keys[c.Name()]
						}
					}
				}
			}
		}
	}
	want := map[string]bool{} // "dosya name:kod"
	for _, u := range m.Units {
		for id, obj := range u.Info.Uses {
			if c, ok := obj.(*types.Const); ok && c.Pkg() != nil {
				if key := aliases[c.Pkg().Path()+"."+c.Name()]; key != "" {
					want[m.Rel(m.Fset.Position(id.Pos()).Filename)+" name:"+key] = true
				}
			}
		}
	}
	if len(aliases) < 20 || len(want) < 20 {
		t.Fatalf("%d aliases, %d files using them; the scan is broken", len(aliases), len(want))
	}
	file := map[string]string{}
	for _, n := range g.Nodes {
		file[n.ID] = n.File
	}
	have := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind == edgeUsesName || e.Kind == edgeReadsName {
			have[file[e.From]+" "+e.To] = true
		}
	}
	var missing []string
	for k := range want {
		if !have[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d files use a name code through an alias without an edge to the code, e.g. %v", len(missing), missing[:min(len(missing), 10)])
	}
	ix := newIndex(g)
	skip := ix.byID["go:func:"+m.Path+"/internal/stdlib/dataset.Reader.skipTR"]
	if skip == nil {
		t.Fatal("no dataset.Reader.skipTR node")
	}
	var out strings.Builder
	ix.printTestsFor(&out, skip, 0)
	if !strings.Contains(out.String(), "dataset_test.TestReaderSkipSameAsNext") {
		t.Errorf("tests-for dataset.Reader.skipTR lacks the test of ö.atla:\n%s", out.String())
	}
}
