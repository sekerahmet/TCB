package main

import (
	"go/ast"
	"strings"
	"unicode/utf8"
)

// Belge yorumları: her Go düğümü (işlev, metot, tür, alan, sabit, değişken, arayüz yöntemi) bildiriminin üstündeki doc
// yorumunu, yoksa satır sonu yorumunu taşır. Çoğu soru ("bu değişken ne için, kim okur") imzadan değil yorumdan
// cevap bulur; dosyayı açmak gerekmesin. Graf şişmesin diye yalnızca ilk paragrafın ilk docMaxLines satırı, en çok
// docMaxRunes karakter saklanır. TR tarafında aynı bilgi codeindex'in Doc alanından gelir.

const (
	docMaxLines = 3
	docMaxRunes = 300
)

// docText yorum gruplarından ilk boş olmayanın ilk paragrafını verir (`//go:` yönergeleri Text'te yoktur).
func docText(groups ...*ast.CommentGroup) string {
	for _, g := range groups {
		if g == nil {
			continue
		}
		var lines []string
		for _, l := range strings.Split(strings.TrimSpace(g.Text()), "\n") {
			if strings.TrimSpace(l) == "" {
				break // paragraf bitti
			}
			lines = append(lines, strings.TrimSpace(l))
			if len(lines) == docMaxLines {
				break
			}
		}
		if len(lines) == 0 {
			continue
		}
		s := strings.Join(lines, " ")
		if utf8.RuneCountInString(s) > docMaxRunes {
			s = string([]rune(s)[:docMaxRunes]) + "…"
		}
		return s
	}
	return ""
}

// singleSpecDoc tek belirtimli bildirimde (`const X = 1`, `type T int`) yorum bildirime bağlıdır, belirtime değil.
func singleSpecDoc(d *ast.GenDecl, spec *ast.CommentGroup) *ast.CommentGroup {
	if spec == nil && len(d.Specs) == 1 {
		return d.Doc
	}
	return spec
}

// interfaceMethodDocs bir arayüz tür bildiriminin yöntemlerinin belge yorumlarıdır (yöntem adı → metin).
func interfaceMethodDocs(ts *ast.TypeSpec) map[string]string {
	out := map[string]string{}
	it, ok := ts.Type.(*ast.InterfaceType)
	if !ok || it.Methods == nil {
		return out
	}
	for _, f := range it.Methods.List {
		for _, n := range f.Names {
			out[n.Name] = docText(f.Doc, f.Comment)
		}
	}
	return out
}

// docLine `//` işaretsiz çok satırlı bir yorumun (codeindex Doc) ilk paragrafını docText kurallarıyla kısaltır.
func docLine(doc string) string {
	cg := &ast.CommentGroup{}
	for _, l := range strings.Split(doc, "\n") {
		cg.List = append(cg.List, &ast.Comment{Text: "// " + strings.TrimSpace(l)})
	}
	return docText(cg)
}
