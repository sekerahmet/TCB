package main

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"io/fs"
	"regexp"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

// Go test dosyaları TR kodunu dize olarak çalıştırır (`run(t, "kn.güncelle(1)")`). Davranış testlerinin çoğu böyledir ve
// bu testler dosya yolu taşımadığı için grafikte hiçbir şeyi sınamıyor görünürdü. Çözüm yardımcıyı adla sabitlemez:
// dize, TR ayrıştırıcısına varan bir yoldaki işlevin dize parametresine giden her şeydir. Yol çağrı grafiğinden
// bulunur: önce dilin giriş noktaları (trSinks) "TR metni alan parametre" sayılır; bir işlev kendi dize parametresini
// böyle bir parametreye iletiyorsa o parametre de TR metni alır (sabit noktaya kadar). Sonra TR metni alan bir
// parametreye test dosyasında verilen her dize (sabit, Sprintf kalıbı, sabit adı, başka işlevin döndürdüğü metin,
// yerel değişken) bir TR programı sayılır, codeindex.Snippet onun kullandığı ad kodlarını verir ve çağrıyı içeren
// işlevi çağıran testlerden o ad koduna `tests` kenarı çekilir (dosya yolu sabitiyle aynı zincir kuralı).
// Dosyaya yazılıp `tr` ikilisine verilen TR metni de aynı yoldan bağlanır (tr_file_writes.go).

// trSinks TR metnini okuyan dil giriş noktalarıdır (paket yolu modül köküne göre → işlev adları); dize tipli
// parametreleri TR metni sayılır.
var trSinks = map[string][]string{
	"internal/lang/lexer":  {"New", "NewBody", "NewBodyIn", "NewBodyView"},
	"internal/lang/source": {"Prepare", "Split", "Decode", "Encode", "EncodeIn"},
}

// paramKey bir işlevin dize parametresidir.
type paramKey struct {
	fn  string
	idx int
}

// textSrc bir ifadenin TR metni olabilecek kaynağıdır: sabit (lit), Sprintf kalıbı (format) ya da başka bir işlevin
// döndürdüğü metin (call).
type textSrc struct {
	lit    string
	format bool
	call   string
	parts  []textSrc // art arda eklenen parçalar (`a + f(x) + b`); bilinmeyen parça yer tutucu olur
}

// maxJoined bir birleştirmenin (`a + b`) verdiği en çok metin sayısıdır: iki yanın metinleri çarpılır.
const maxJoined = 64

// placeholder metni bilinmeyen bir ifadenin yerine konur: genellikle TR dizesinin içindeki bir yol ya da sayıdır.
var placeholder = textSrc{lit: "x"}

// snippetFlow bir çağrı bağımsız değişkenidir: callee'nin parametresine giden ifadenin metin kaynakları ya da
// çağıran işlevin kendi parametresi (from).
type snippetFlow struct {
	callee paramKey
	from   paramKey
	texts  []textSrc
	owner  string
	file   string
	line   int
	site   int  // çağrının sırası: aynı çağrının argümanları (dosyanın içeriği ve yolu) birlikte okunur
	trFile bool // yolu .tr ile biten dosyaya yazılan içerik (linkSnippets)
}

// snippetState çıkarıcının TR metni takibinin durumudur.
type snippetState struct {
	flows       []snippetFlow
	sinks       map[paramKey]bool
	varTexts    map[string][]textSrc // değişken (bildirim konumu) -> atanan metinler
	returnTexts map[string][]textSrc // işlev kimliği -> döndürdüğü metinler
	curParams   map[*types.Var]paramKey
	// Tablo testleri (`for _, c := range []struct{ code string }{{"ortanca(1)"}} { run(c.code) }`): tables yerel
	// değişkene atanan tablo, fieldTexts döngü değişkeninin alanının (bildirim konumu + "." + alan) tablodaki metinleri.
	tables     map[string]*ast.CompositeLit
	fieldTexts map[string][]textSrc
	sites      int // görülen çağrı sayısı (snippetFlow.site)
}

func newSnippetState() *snippetState {
	return &snippetState{sinks: map[paramKey]bool{}, varTexts: map[string][]textSrc{}, returnTexts: map[string][]textSrc{},
		curParams: map[*types.Var]paramKey{}, tables: map[string]*ast.CompositeLit{}, fieldTexts: map[string][]textSrc{}}
}

// enterFunc bir işlev bildirimine girerken parametrelerini kaydeder; dil giriş noktasıysa dize parametrelerini
// TR metni alan parametre sayar.
func (x *goExtractor) enterFunc(obj *types.Func, id, pkgRel string, hasRecv bool) {
	st := x.idx.snip
	st.curParams = map[*types.Var]paramKey{}
	sig, ok := obj.Type().(*types.Signature)
	if !ok {
		return
	}
	seed := !hasRecv && slices.Contains(trSinks[pkgRel], obj.Name())
	for i := 0; i < sig.Params().Len(); i++ {
		p := sig.Params().At(i)
		if !isStringType(p.Type()) {
			continue
		}
		st.curParams[p] = paramKey{id, i}
		if seed {
			st.sinks[paramKey{id, i}] = true
		}
	}
}

// isStringType tip string (ya da ...string'in öğesi) mi.
func isStringType(t types.Type) bool {
	if s, ok := t.(*types.Slice); ok {
		t = s.Elem()
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Kind() == types.String
}

// textSources bir ifadenin TR metni olabilecek kaynaklarıdır (bilinmeyen ifade için boş).
func (x *goExtractor) textSources(fc *fileCtx, e ast.Expr) []textSrc {
	switch v := e.(type) {
	case *ast.ParenExpr:
		return x.textSources(fc, v.X)
	case *ast.BasicLit:
		if s, ok := stringLit(v); ok {
			return []textSrc{{lit: s}}
		}
	case *ast.Ident:
		switch o := fc.u.Info.Uses[v].(type) {
		case *types.Const:
			if o.Val().Kind() == constant.String {
				return []textSrc{{lit: constant.StringVal(o.Val())}}
			}
		case *types.Var:
			return x.idx.snip.varTexts[x.posKey(o.Pos())]
		}
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			l, r := x.textSources(fc, v.X), x.textSources(fc, v.Y)
			if len(l) == 0 && len(r) == 0 {
				return nil
			}
			return joinTexts([][]textSrc{l, r}, "")
		}
	case *ast.CallExpr:
		if len(v.Args) == 1 && isConversion(fc, v) {
			return x.textSources(fc, v.Args[0])
		}
		return x.callTexts(fc, v)
	case *ast.SelectorExpr:
		if id, ok := v.X.(*ast.Ident); ok {
			if o, ok := fc.u.Info.Uses[id].(*types.Var); ok {
				return x.idx.snip.fieldTexts[x.posKey(o.Pos())+"."+v.Sel.Name]
			}
		}
	}
	return nil
}

// joinTexts parçaların metinlerini sep ile birleştirir: her olası metin (tablonun her satırı: `c.code + "\n"`), en çok
// maxJoined; metni bilinmeyen parça yer tutucu olur.
func joinTexts(lists [][]textSrc, sep string) []textSrc {
	out := []textSrc{{}}
	for i, l := range lists {
		if len(l) == 0 {
			l = []textSrc{placeholder}
		}
		var next []textSrc
		for _, a := range out {
			for _, b := range l {
				if len(next) < maxJoined {
					parts := append([]textSrc(nil), a.parts...)
					if i > 0 && sep != "" {
						parts = append(parts, textSrc{lit: sep})
					}
					next = append(next, textSrc{parts: append(parts, b)})
				}
			}
		}
		out = next
	}
	return out
}

// isConversion çağrı bir tip dönüşümü mü (`[]byte(metin)`, `string(b)`).
func isConversion(fc *fileCtx, c *ast.CallExpr) bool {
	if _, ok := c.Fun.(*ast.ArrayType); ok {
		return true
	}
	id, ok := c.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	_, ok = fc.u.Info.Uses[id].(*types.TypeName)
	return ok
}

// structFields tablo öğesinin yapı tipinin alan adlarıdır, sırasıyla: yazılı yapı tipinden (`[]struct{ code string }`)
// ya da adlı tipin tanımından. İfade tipleri (Info.Types) yüklenmez, tip yazıdan okunur.
func (x *goExtractor) structFields(fc *fileCtx, t ast.Expr) []string {
	var names []string
	if st, ok := t.(*ast.StructType); ok {
		for _, f := range st.Fields.List {
			for _, n := range f.Names {
				names = append(names, n.Name)
			}
		}
		return names
	}
	if id := nameIdent(t); id != nil {
		if tn, ok := fc.u.Info.Uses[id].(*types.TypeName); ok {
			if st, ok := tn.Type().Underlying().(*types.Struct); ok {
				for i := 0; i < st.NumFields(); i++ {
					names = append(names, st.Field(i).Name())
				}
			}
		}
	}
	return names
}

// noteRangeTable tablo üzerinde dönen döngünün değişkeninin alanlarına tablodaki metinleri bağlar: tablo döngüde
// yazılır ya da yerel bir değişkene atanmıştır (`cases := []struct{...}{...}`); öğenin alanı adla ya da sırayla
// verilir. Testlerin çoğu TR kodunu böyle tablodan koşar (stats_test.go: `New().Run(mustParseProgram(t, c.code))`).
func (x *goExtractor) noteRangeTable(fc *fileCtx, rs *ast.RangeStmt) {
	lit, _ := rs.X.(*ast.CompositeLit)
	if id, ok := rs.X.(*ast.Ident); ok && fc.u.Info.Uses[id] != nil {
		lit = x.idx.snip.tables[x.posKey(fc.u.Info.Uses[id].Pos())]
	}
	if lit == nil {
		return
	}
	if _, ok := lit.Type.(*ast.MapType); ok {
		x.noteRangeMap(fc, rs, lit)
		return
	}
	val, ok := rs.Value.(*ast.Ident)
	if !ok || fc.u.Info.Defs[val] == nil {
		return
	}
	arr, ok := lit.Type.(*ast.ArrayType)
	if !ok {
		return
	}
	fields := x.structFields(fc, arr.Elt)
	key := x.posKey(fc.u.Info.Defs[val].Pos()) + "."
	for _, e := range lit.Elts {
		elem, ok := e.(*ast.CompositeLit)
		if !ok {
			continue
		}
		for i, f := range elem.Elts {
			name, value := "", f
			if kv, ok := f.(*ast.KeyValueExpr); ok {
				if k, ok := kv.Key.(*ast.Ident); ok {
					name, value = k.Name, kv.Value
				}
			} else if i < len(fields) {
				name = fields[i]
			}
			if texts := x.textSources(fc, value); name != "" && len(texts) > 0 {
				x.idx.snip.fieldTexts[key+name] = append(x.idx.snip.fieldTexts[key+name], texts...)
			}
		}
	}
}

// noteRangeMap sözlük üzerinde dönen döngünün anahtar ve değer değişkenlerine sözlükteki metinleri bağlar: testler
// dosyaları `for ad, metin := range map[string]string{"lib/katman.tr": "..."}` ile yazar.
func (x *goExtractor) noteRangeMap(fc *fileCtx, rs *ast.RangeStmt, lit *ast.CompositeLit) {
	for i, v := range []ast.Expr{rs.Key, rs.Value} {
		id, ok := v.(*ast.Ident)
		if !ok || fc.u.Info.Defs[id] == nil {
			continue
		}
		key := x.posKey(fc.u.Info.Defs[id].Pos())
		for _, e := range lit.Elts {
			if kv, ok := e.(*ast.KeyValueExpr); ok {
				x.idx.snip.varTexts[key] = append(x.idx.snip.varTexts[key], x.textSources(fc, []ast.Expr{kv.Key, kv.Value}[i])...)
			}
		}
	}
}

// callTexts çağrının metni: fmt.Sprintf'in kalıbı ya da dize döndüren bir modül işlevinin döndürdüğü metinler.
func (x *goExtractor) callTexts(fc *fileCtx, c *ast.CallExpr) []textSrc {
	id := nameIdent(c.Fun)
	if id == nil {
		return nil
	}
	fn, ok := fc.u.Info.Uses[id].(*types.Func)
	if !ok || fn.Pkg() == nil {
		return nil
	}
	if fn.Pkg().Path() == "fmt" && fn.Name() == "Sprintf" && len(c.Args) > 0 {
		var out []textSrc
		for _, t := range x.textSources(fc, c.Args[0]) {
			t.format = true
			out = append(out, t)
		}
		return out
	}
	if texts, ok := x.pathTexts(fc, fn, c); ok {
		return texts
	}
	if ref := funcRef(fn.Origin()); ref != "" {
		if sig := fn.Type().(*types.Signature); sig.Results().Len() == 1 && isStringType(sig.Results().At(0).Type()) {
			return []textSrc{{call: ref}}
		}
	}
	return nil
}

// noteSnippets scan'in düğüm başına çağırdığı kancadır: atamalardan değişken metinlerini, return'lerden işlev
// metinlerini, çağrılardan metin akışını toplar.
func (x *goExtractor) noteSnippets(fc *fileCtx, owner string, n ast.Node) {
	st := x.idx.snip
	switch s := n.(type) {
	case *ast.AssignStmt:
		if len(s.Lhs) == len(s.Rhs) {
			for i, l := range s.Lhs {
				x.noteVarText(fc, l, s.Rhs[i])
			}
		}
	case *ast.ValueSpec:
		for i, name := range s.Names {
			if i < len(s.Values) {
				x.noteVarText(fc, name, s.Values[i])
			}
		}
	case *ast.RangeStmt:
		if fc.isTest {
			x.noteRangeTable(fc, s)
		}
	case *ast.ReturnStmt:
		if len(s.Results) == 1 {
			st.returnTexts[owner] = append(st.returnTexts[owner], x.textSources(fc, s.Results[0])...)
		}
	case *ast.CallExpr:
		x.noteFlows(fc, owner, s)
	}
}

// noteVarText `x := metin` / `x = metin` atamasını değişkenin metni olarak kaydeder.
func (x *goExtractor) noteVarText(fc *fileCtx, lhs, rhs ast.Expr) {
	id, ok := lhs.(*ast.Ident)
	if !ok || id.Name == "_" {
		return
	}
	obj := fc.u.Info.Defs[id]
	if obj == nil {
		obj = fc.u.Info.Uses[id]
	}
	if lit, ok := rhs.(*ast.CompositeLit); ok && obj != nil && fc.isTest {
		x.idx.snip.tables[x.posKey(obj.Pos())] = lit
	}
	if v, ok := obj.(*types.Var); ok && isStringType(v.Type()) {
		if texts := x.textSources(fc, rhs); len(texts) > 0 {
			key := x.posKey(v.Pos())
			x.idx.snip.varTexts[key] = append(x.idx.snip.varTexts[key], texts...)
		}
	}
}

// noteFlows çağrının dize parametresine giden bağımsız değişkenleri akış olarak kaydeder: argüman çağıran işlevin
// kendi parametresiyse parametre akışı, aksi halde (yalnızca test dosyalarında) metin kaynakları.
func (x *goExtractor) noteFlows(fc *fileCtx, owner string, c *ast.CallExpr) {
	id := nameIdent(c.Fun)
	if id == nil {
		return
	}
	fn, ok := fc.u.Info.Uses[id].(*types.Func)
	if !ok {
		return
	}
	ref := funcRef(fn.Origin())
	sig, isSig := fn.Type().(*types.Signature)
	if ref == "" || !isSig || sig.Params().Len() == 0 {
		return
	}
	st := x.idx.snip
	st.sites++
	for k, arg := range c.Args {
		idx := min(k, sig.Params().Len()-1)
		if !isTextType(sig.Params().At(idx).Type()) {
			continue
		}
		fl := snippetFlow{callee: paramKey{ref, idx}, owner: owner, file: fc.rel, line: x.line(arg.Pos()), site: st.sites}
		if pid, ok := paramArg(fc, arg).(*ast.Ident); ok {
			if v, ok := fc.u.Info.Uses[pid].(*types.Var); ok {
				if from, isParam := st.curParams[v]; isParam {
					fl.from = from
					st.flows = append(st.flows, fl)
					continue
				}
			}
		}
		if fc.isTest {
			if fl.texts = x.textSources(fc, arg); len(fl.texts) > 0 {
				st.flows = append(st.flows, fl)
			}
		}
	}
}

var (
	verbQ    = regexp.MustCompile(`%[-+# 0-9.]*q`)
	verbAny  = regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z]`)
	maxDepth = 3
)

// expandTexts metin kaynaklarını somut metinlere açar: Sprintf kalıbındaki fiiller yer tutucuya (%q → "x", diğerleri
// 0) çevrilir, başka işlevin döndürdüğü metin en çok maxDepth adım açılır.
func (st *snippetState) expandTexts(texts []textSrc, depth int) []string {
	var out []string
	for _, t := range texts {
		switch {
		case len(t.parts) > 0:
			var sb strings.Builder
			for _, part := range t.parts {
				if got := st.expandTexts([]textSrc{part}, depth); len(got) > 0 {
					sb.WriteString(got[0])
				} else {
					sb.WriteString(placeholder.lit)
				}
			}
			out = append(out, sb.String())
		case t.call != "":
			if depth < maxDepth {
				out = append(out, st.expandTexts(st.returnTexts[t.call], depth+1)...)
			}
		case t.format:
			s := strings.ReplaceAll(t.lit, "%%", "\x00")
			s = verbQ.ReplaceAllString(s, `"x"`)
			s = verbAny.ReplaceAllString(s, "0")
			out = append(out, strings.ReplaceAll(s, "\x00", "%"))
		default:
			out = append(out, t.lit)
		}
	}
	return out
}

// linkSnippets TR metni alan parametreleri sabit noktaya kadar genişletir, metinlerden ad kodlarını çıkarır ve metni
// çalıştıran işlevi çağıran testlerden ad koduna `tests` kenarı çeker. library depodaki kitaplıktır (lib/): metnin
// aktardığı modülün adları kitaplık kodlarıyla gelir.
func linkSnippets(b *builder, idx *goIndex, reach *testReach, library fs.FS) {
	st := idx.snip
	st.linkFileWrites()
	for changed := true; changed; {
		changed = false
		for _, f := range st.flows {
			if f.from.fn != "" && st.sinks[f.callee] && !st.sinks[f.from] {
				st.sinks[f.from] = true
				changed = true
			}
		}
	}
	cache := map[string][]codeindex.SnippetUse{}
	for _, f := range st.flows {
		if f.from.fn != "" || !st.sinks[f.callee] && !f.trFile {
			continue
		}
		tests := reach.testsWithin(f.owner, testMaxSteps/2) // "doğrudan" test bağı: zincir en çok yarı sınır (S3)
		if len(tests) == 0 {
			continue
		}
		for _, text := range st.expandTexts(f.texts, 0) {
			uses, seen := cache[text]
			if !seen {
				uses, _ = codeindex.Snippet(text, library)
				cache[text] = uses
			}
			for _, u := range uses {
				for _, t := range tests {
					b.addEdge(t, "name:"+u.Code, edgeTests, inferred, f.file, f.line)
				}
			}
		}
	}
}
