package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// FuzzEmbeddedPackage: 'tr build' ile üretilmiş bir uygulama başlarken kendi ikilisinin
// sonundaki paketi çözer, modülleri geçici klasöre yazar ve 'aktar' yollarını yönlendirir.
// Paketi değiştirebilen biri ikiliyi de değiştirebileceği için bu bir güven sınırı değildir;
// yine de bozuk (yarım indirilmiş, kırpılmış) bir uygulama çökmemeli, açık bir hata vermeli
// ve modül dosyaları geçici klasörün dışına yazılmamalıdır. Derin arama:
//
//	go test ./cmd/tr -run '^$' -fuzz FuzzEmbeddedPackage -fuzztime 5m
func FuzzEmbeddedPackage(f *testing.F) {
	for _, pk := range []*pkg{
		{Main: packageFile{Name: "ana.tr", Code: "TR_print(1)\n"}},
		{
			Main:    packageFile{Name: "ana.tr", Code: "TR_import \"m\"\nTR_import \"alt/ç\" TR_as c\n", Imports: []int{0, 1}},
			Modules: []packageFile{{Name: "m.tr", Code: "TR_var a = 1\n"}, {Name: "alt/ç.tr", Code: "TR_import \"m\"\n", Imports: []int{0}}},
		},
		{Main: packageFile{Code: "\ufeffaktar \"..\"\n", Imports: []int{0}}, Modules: []packageFile{{Name: "..", Code: ""}}},
	} {
		coded, err := encodePackage(pk)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(append([]byte("YORUMLAYICI"), coded...))
	}

	f.Fuzz(func(t *testing.T, exe []byte) {
		pk, templateSize, found, err := extractPackage(exe)
		if templateSize < 0 || templateSize > len(exe) {
			t.Fatalf("şablon boyu %d, ikili %d bayt", templateSize, len(exe))
		}
		if !found || err != nil {
			return
		}
		if len(pk.Modules) > 16 {
			return
		}
		dir := t.TempDir()
		paths, err := writeEmbeddedModules(pk, dir)
		if err != nil {
			return
		}
		for _, y := range paths {
			if rel, err := filepath.Rel(dir, y); err != nil || rel == "." || strings.HasPrefix(rel, "..") {
				t.Fatalf("modül geçici klasörün dışına yazıldı: %q (klasör %q)", y, dir)
			}
		}
		_, _ = redirectImports(pk.Main, paths)
	})
}
