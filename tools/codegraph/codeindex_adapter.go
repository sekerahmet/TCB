package main

import (
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

// TR kaynak düğümleri: internal/devtools/codeindex paketinin dizininden alınır.
const (
	kindTRFunc   = "tr_func"
	kindTRMethod = "tr_method"
	kindTRStruct = "tr_struct"
	kindTRField  = "tr_field"
	kindTRConst  = "tr_const"
	kindTRVar    = "tr_var"
	kindTRTest   = "tr_test"
	kindTRParam  = "tr_param"
)

// trRoots .tr dosyalarının aranacağı klasörlerdir (modül köküne göre).
var trRoots = []string{"lib", "tools", "examples"}

func trFileID(rel string) string { return "tr:file:" + rel }

// extractTR lib/, tools/ ve examples/ altındaki .tr dosyalarını codeindex ile dizinler ve dizini
// grafiğe çevirir: işlev, yapı, metot, alan, sabit/değişken ve test düğümleri; çağrı, kullanım,
// ad kodu, aktarma ve test kenarları. Ayrıştırılamayan dosya uyarı olur. Kimlikler (tr:func:...)
// ajanların alışkanlığı bozulmasın diye korunur.
func extractTR(root string, b *builder) {
	idx, err := codeindex.Build(root, codeindex.Options{Dirs: trRoots, SkipDirs: skipDirs})
	if err != nil {
		b.warn("tr sources skipped: " + err.Error())
		return
	}
	for _, s := range idx.Skipped {
		if isErrorExample(s.File) {
			continue // bilerek hatalı örnek: uyarı değil
		}
		b.warn("tr syntax error, definitions skipped: " + s.File + ": " + s.Reason)
	}
	ids := map[string]string{}
	for _, n := range idx.Nodes {
		if n.Kind == codeindex.KindOption {
			continue // seçenek düğümleri yalnızca tr dizin içindir
		}
		ids[n.ID] = graphID(n)
		if n.Kind != codeindex.KindBuiltin { // ad kodu düğümlerini codes.go kurar
			b.addNode(graphNode(n, ids[n.ID]))
		}
	}
	for _, e := range idx.Edges {
		if ids[e.From] == "" || ids[e.To] == "" {
			continue
		}
		kind := trEdgeKind(e, ids)
		line := 0
		if e.Kind != codeindex.EdgeImports { // aktarma kenarının satırı grafikte hep 0'dı
			line = e.Pos.Line
		}
		for i := 0; i < max(e.Count, 1); i++ {
			b.addEdge(ids[e.From], ids[e.To], kind, string(e.Confidence), e.Pos.File, line)
		}
	}
	namedArgumentEdges(idx, ids, b)
}

// graphID bir codeindex düğümünün grafikteki kimliğidir.
func graphID(n codeindex.Node) string {
	switch n.Kind {
	case codeindex.KindBuiltin:
		return "name:" + n.Name
	case codeindex.KindModule:
		return trFileID(n.Pos.File)
	case codeindex.KindTest:
		return "tr:test:" + n.Pos.File + ":" + strconv.Itoa(n.Pos.Line)
	}
	prefix := map[codeindex.Kind]string{
		codeindex.KindFunction: "func", codeindex.KindStruct: "struct", codeindex.KindMethod: "method",
		codeindex.KindField: "field", codeindex.KindConstant: "const", codeindex.KindVariable: "var",
		codeindex.KindParameter: "param",
	}[n.Kind]
	return "tr:" + prefix + ":" + n.Pos.File + ":" + n.Name
}

func graphNode(n codeindex.Node, id string) Node {
	if n.Kind == codeindex.KindModule {
		detail := "tr"
		if n.Coded {
			detail = "tr, coded"
		}
		return Node{ID: id, Kind: kindFile, Name: n.Name, File: n.Pos.File, Line: 1, Detail: detail, Aliases: n.Aliases}
	}
	kind := map[codeindex.Kind]string{
		codeindex.KindFunction: kindTRFunc, codeindex.KindStruct: kindTRStruct, codeindex.KindMethod: kindTRMethod,
		codeindex.KindField: kindTRField, codeindex.KindConstant: kindTRConst, codeindex.KindVariable: kindTRVar,
		codeindex.KindTest: kindTRTest, codeindex.KindParameter: kindTRParam,
	}[n.Kind]
	return Node{ID: id, Kind: kind, Name: n.Qualified, File: n.Pos.File, Line: n.Pos.Line, End: n.Pos.EndLine, Detail: n.Signature}
}

// trEdgeKind codeindex kenar türünü grafik kenar türüne çevirir: yerleşik adlara `uses`
// grafikte `uses_name`, `defines` `contains` olur.
func trEdgeKind(e codeindex.Edge, ids map[string]string) string {
	switch {
	case e.Kind == codeindex.EdgeDefines:
		return edgeContains
	case e.Kind == codeindex.EdgeUses && len(ids[e.To]) > 5 && ids[e.To][:5] == "name:":
		return edgeUsesName
	}
	return string(e.Kind)
}

// namedArgumentEdges çağrı yerindeki `ad: değer` adlı argümanlarını çağrılan parametreye (kurucusuz
// yapı çağrısında alana) bağlar: kenar, çağrının bulunduğu bildirimden parametreye, `uses` türünde;
// böylece `who-uses` parametrenin adlı argüman olarak geçtiği yerleri verir.
func namedArgumentEdges(idx *codeindex.Index, ids map[string]string, b *builder) {
	for _, o := range idx.Occurrences {
		if o.Role != codeindex.RoleArgument || ids[o.Node] == "" || ids[o.In] == "" {
			continue
		}
		b.addEdge(ids[o.In], ids[o.Node], edgeUses, string(o.Confidence), o.Pos.File, o.Pos.Line)
	}
}

// isErrorExample bilerek sözdizimi hatalı yazılmış hata örneklerinin klasörlerindeki dosya mı
// (examples/tr/hatalar, examples/en/errors): oradaki ayrıştırma hataları beklenendir.
func isErrorExample(file string) bool {
	return strings.HasPrefix(file, "examples/") && (strings.Contains(file, "/hatalar/") || strings.Contains(file, "/errors/"))
}
