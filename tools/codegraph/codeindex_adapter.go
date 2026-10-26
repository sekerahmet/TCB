package main

import (
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
	"github.com/sekerahmet/TCB/internal/lang/codes"
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
	kindTROption = "tr_option" // komut_satırı sözlüğünün anahtarı: betiğin komut satırı seçeneği
)

// trRoots .tr dosyalarının aranacağı klasörlerdir (modül köküne göre).
var trRoots = []string{"lib", "tools", "examples", "tests"}

// trSkipped .tr dizini kurulamadığında (ör. gömülü dil listesi bozuk) grafiğe yazılan uyarının
// önekidir; sorgular onu görünce TR cevaplarının eksik olduğunu söyler (loadGraph).
const trSkipped = "tr sources skipped: "

func trFileID(rel string) string { return "tr:file:" + rel }

// extractTR trRoots altındaki .tr dosyalarını codeindex ile dizinler ve dizini
// grafiğe çevirir: işlev, yapı, metot, alan, sabit/değişken ve test düğümleri; çağrı, kullanım,
// ad kodu, aktarma ve test kenarları. Ayrıştırılamayan dosya uyarı olur. Kimlikler (tr:func:...)
// ajanların alışkanlığı bozulmasın diye korunur.
func extractTR(root string, b *builder, m *module) {
	idx, err := codeindex.Build(root, codeindex.Options{Dirs: trRoots, Tracked: true})
	if err != nil {
		b.warn(trSkipped + err.Error())
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
	libraryCodeEdges(idx, ids, b)
	if m != nil {
		linkConstructs(b, readDispatch(m), idx, ids, m.Path+"/internal/lang/interp")
	}
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
		codeindex.KindParameter: "param", codeindex.KindOption: "option",
	}[n.Kind]
	return "tr:" + prefix + ":" + n.Pos.File + ":" + n.Name
}

func graphNode(n codeindex.Node, id string) Node {
	if n.Kind == codeindex.KindModule {
		detail := "tr"
		if n.Coded {
			detail = "tr, coded"
		}
		return Node{ID: id, Kind: kindFile, Name: n.Name, File: n.Pos.File, Line: 1, Detail: detail, Aliases: n.Aliases, Doc: docLine(n.Doc)}
	}
	kind := map[codeindex.Kind]string{
		codeindex.KindFunction: kindTRFunc, codeindex.KindStruct: kindTRStruct, codeindex.KindMethod: kindTRMethod,
		codeindex.KindField: kindTRField, codeindex.KindConstant: kindTRConst, codeindex.KindVariable: kindTRVar,
		codeindex.KindTest: kindTRTest, codeindex.KindParameter: kindTRParam, codeindex.KindOption: kindTROption,
	}[n.Kind]
	node := Node{ID: id, Kind: kind, Name: n.Qualified, File: n.Pos.File, Line: n.Pos.Line, End: n.Pos.EndLine, Detail: n.Signature, Doc: docLine(n.Doc)}
	// Dilin öğesi olan bildirim (Node.Code) her yazım dilindeki adıyla bulunur, tr dizin gibi. Kodun kimliği grafikte
	// ad düğümünü gösterir (nameNodeNames); kitaplık kodunun bildirimine oradan uses_name kenarıyla gidilir.
	if k, err := codes.Parse(n.Code); err == nil {
		node.Aliases = codeindex.LanguageNames(k)
	}
	return node
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

// libraryCodeEdges kitaplığın bildirimlerini kodlarına bağlar (kitaplığın dili yoktur, dil tanımı 17.3): lib/
// altındaki yapı, fonksiyon, sabit, alan, metot ve parametre bildirimi kitaplık kodunun (dizinin Node.Code'u) ad
// düğümüne `uses_name` kenarı verir. Bildirimi (parametrede sahibi olan fonksiyonu ya da yapıyı) sınayan ya da okuyan test
// kodu da sınar (`tests`, çıkarım): çekirdekte yerleşiğin okunması onu sınamaktır, kitaplıkta bildirimin. Kitaplığın
// iç yardımcısı (ayar_değeri, NS_A) testine çağıranları üzerinden ulaşır (libraryTests).
func libraryCodeEdges(idx *codeindex.Index, ids map[string]string, b *builder) {
	parent := map[string]string{}          // bildirim → onu tanımlayan (metot → yapı, parametre → fonksiyon)
	users := map[string][]codeindex.Edge{} // bildirim → onu çağıran ya da okuyan kenarlar
	for _, e := range idx.Edges {
		switch e.Kind {
		case codeindex.EdgeDefines:
			parent[e.To] = e.From
		case codeindex.EdgeTests, codeindex.EdgeCalls, codeindex.EdgeUses:
			users[e.To] = append(users[e.To], e)
		}
	}
	for _, n := range idx.Nodes {
		if k, err := codes.Parse(n.Code); err != nil || !k.IsLibrary() {
			continue // çekirdekle ortak kodu (TR_arg_step) kitaplığın bildirimi sınamaz
		}
		name := "name:" + n.Code
		b.addEdge(ids[n.ID], name, edgeUsesName, extracted, n.Pos.File, n.Pos.Line)
		for _, e := range libraryTests([]string{n.ID, parent[n.ID], parent[parent[n.ID]]}, users) {
			b.addEdge(ids[e.From], name, edgeTests, inferred, e.Pos.File, e.Pos.Line)
		}
	}
}

// libraryTests bildirimlere çağıran ve okuyan zincirinde (en çok libraryTestDepth adım) ulaşan test bloklarının
// kenarlarıdır.
func libraryTests(start []string, users map[string][]codeindex.Edge) []codeindex.Edge {
	const libraryTestDepth = 6
	var out []codeindex.Edge
	seen := map[string]bool{}
	frontier := start
	for depth := 0; depth < libraryTestDepth && len(frontier) > 0; depth++ {
		var next []string
		for _, id := range frontier {
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			for _, e := range users[id] {
				if strings.HasPrefix(e.From, string(codeindex.KindTest)) {
					out = append(out, e)
				} else {
					next = append(next, e.From)
				}
			}
		}
		frontier = next
	}
	return out
}

// nameNodeNames bir ad kodu düğümünün sorguda aranan adlarıdır: kimliği ve dillerdeki adları. Kitaplık kodunun
// dillerdeki adlarını kitaplıktaki bildirimi taşır (libraryCodeEdges); kod yalnız kimliğiyle aranır, yoksa her
// kitaplık adı bildirimiyle birlikte iki aday olurdu.
func nameNodeNames(n *Node) []string {
	if k, err := codes.Parse(n.Name); err == nil && k.IsLibrary() {
		return []string{n.Name}
	}
	return append([]string{n.Name}, n.Aliases...)
}

// isErrorExample bilerek sözdizimi hatalı yazılmış hata örneklerinin klasörlerindeki dosya mı
// (examples/tr/hatalar): oradaki ayrıştırma hataları beklenendir.
func isErrorExample(file string) bool {
	return strings.HasPrefix(file, "examples/") && (strings.Contains(file, "/hatalar/") || strings.Contains(file, "/errors/"))
}
