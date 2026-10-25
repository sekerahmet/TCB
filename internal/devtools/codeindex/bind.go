package codeindex

import (
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Bu dosya üye erişimlerini, parametreleri, adlı argümanları ve komut_satırı anahtarlarını bağlar:
// yeniden adlandırma etkisinin ve "kullananlar"ın ihtiyacı olan, ada göre çözümlenemeyen yerler.

// ownerName sahip kimliğindeki "#"den sonraki addır ("Yapı.metot", "işlev").
func ownerName(id string) string { return id[strings.Index(id, "#")+1:] }

// bindOwner sahibin parametrelerini kapsama tanıtır ve türü çıkarılabilen tek atamalı yerelleri bulur (infer.go).
func (x *indexer) bindOwner(f *file, o owner, sc *scope, root any) {
	if fl, ok := root.(*ast.FunctionLiteral); ok && (o.kind == KindFunction || o.kind == KindMethod) {
		for _, p := range fl.Parameters {
			sc.params[p.Name] = NodeID(KindParameter, f.rel, ownerName(o.id)+"."+p.Name)
		}
	}
	ast.Walk(root, func(n any) bool {
		v, ok := n.(*ast.VarStatement)
		if !ok || v.Value == nil {
			return true
		}
		name := v.Name.Value
		if sc.bindCount[name] != 1 || sc.assigned[name] || sc.params[name] != "" {
			return true
		}
		if t, basis := x.exprType(f, sc, v.Value); t != "" {
			sc.localType[name], sc.localBasis[name] = t, basis
		}
		return true
	})
}

// calleeStruct çağrılan ifade bir yapıysa (bu dosyada, takma adsız aktarılan modülde ya da `a.Yapı`)
// yapının kimliğini verir.
func (x *indexer) calleeStruct(f *file, sc *scope, fn ast.Expression) (string, string) {
	switch fn := fn.(type) {
	case *ast.Identifier:
		if sc.locals[fn.Value] {
			return "", ""
		}
		if id := f.visible[fn.Value]; id != "" && x.nodes[id] != nil && x.nodes[id].Kind == KindStruct {
			return id, fn.Value
		}
	case *ast.MemberExpression:
		if obj, ok := fn.Object.(*ast.Identifier); ok && !sc.locals[obj.Value] {
			if target := f.aliases[obj.Value]; target != nil {
				if id := target.defs[fn.Property]; id != "" && x.nodes[id] != nil && x.nodes[id].Kind == KindStruct {
					return id, fn.Property
				}
			}
		}
	}
	return "", ""
}

// receiver `x.üye` ifadesindeki x'in yapısını bilinen yoldan verir: `kendi` (metodun yapısı, kesin) ya da
// yapıcıdan tek atamayla kurulan yerel (çıkarım). Bilinmiyorsa "".
func (x *indexer) receiver(f *file, o owner, sc *scope, obj ast.Expression) (structID string, conf Confidence, basis Basis) {
	id, ok := obj.(*ast.Identifier)
	if !ok {
		return "", "", ""
	}
	if id.Value == x.lang.Name(codes.KwSelf) && o.kind == KindMethod {
		name := ownerName(o.id)
		return NodeID(KindStruct, f.rel, name[:strings.LastIndex(name, ".")]), Extracted, BasisSelf
	}
	if t := sc.localType[id.Value]; t != "" && sc.locals[id.Value] {
		return t, Inferred, sc.localBasis[id.Value]
	}
	if t := x.globalType(f, sc, id.Value); t != "" {
		return t, Inferred, BasisInferredType
	}
	return "", "", ""
}

// structMember yapının alanının ya da metodunun düğümüdür ("" yoksa).
func (x *indexer) structMember(structID, prop string) (string, Kind) {
	sn := x.nodes[structID]
	if sn == nil || sn.Pos == nil {
		return "", ""
	}
	if id := NodeID(KindField, sn.Pos.File, sn.Name+"."+prop); x.nodes[id] != nil {
		return id, KindField
	}
	if id := NodeID(KindMethod, sn.Pos.File, sn.Name+"."+prop); x.nodes[id] != nil {
		return id, KindMethod
	}
	return "", ""
}

// bindMember `kendi.x` ve çıkarımlı yerel `d.x` erişimini alana ya da metoda bağlar; bağlandıysa true.
func (x *indexer) bindMember(f *file, o owner, sc *scope, m *ast.MemberExpression, pos Pos) bool {
	structID, conf, basis := x.receiver(f, o, sc, m.Object)
	if structID == "" {
		return false
	}
	id, kind := x.structMember(structID, m.Property)
	switch kind {
	case KindField:
		role := RoleRead
		if sc.writeMember[m] {
			role = RoleWrite
		}
		x.linkBasis(o, id, EdgeUses, conf, role, pos, basis)
	case KindMethod:
		role := RoleRead
		if sc.calledMember[m] {
			role = RoleCall
		}
		x.linkBasis(o, id, EdgeCalls, conf, role, pos, basis)
	default:
		return false
	}
	return true
}

// nameOnlyMembers alıcısı bilinmeyen `a.b` erişimi için adı tutan alanları (ve, aday sayısı kenar
// sınırını aşan metotları) yalnızca başvuru olarak ekler: kenar yoktur, "kullananlar" göstermez,
// yeniden adlandırma etkisi "olası" diye listeler.
func (x *indexer) nameOnlyMembers(f *file, o owner, sc *scope, m *ast.MemberExpression, pos Pos, methodsCapped bool) {
	seen := map[string]bool{}
	var ids []string
	for _, g := range x.visibleFiles(f) {
		for _, id := range g.fields[m.Property] {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
		if methodsCapped {
			for _, id := range g.methods[m.Property] {
				if !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		role := RoleRead
		switch {
		case x.nodes[id] != nil && x.nodes[id].Kind == KindMethod && sc.calledMember[m]:
			role = RoleCall
		case sc.writeMember[m]:
			role = RoleWrite
		}
		x.occurrenceBasis(id, role, Inferred, pos, o.id, BasisNameOnly)
	}
}

// identParam sahibin parametresine başvuruyu parametre düğümüne bağlar; başka bir yerde yeniden
// bağlanan ad (gölgeleme) tahmindir. Kenar çekilmez.
func (x *indexer) identParam(o owner, sc *scope, id *ast.Identifier, pos Pos) bool {
	pid := sc.params[id.Value]
	if pid == "" || sc.bindings[id] || sc.handled[id] {
		return false
	}
	conf := Extracted
	if sc.bindCount[id.Value] > 0 {
		conf = Inferred
	}
	role := RoleRead
	switch {
	case sc.called[id]:
		role = RoleCall
	case sc.writes[id]:
		role = RoleWrite
	}
	x.occurrenceBasis(pid, role, conf, pos, o.id, "")
	return true
}

// namedArgs çağrının adlı argümanlarını çağrılan işlevin, metodun ya da yapının parametresine (kurucusu
// yoksa alanına) bağlar; bağlanamayan, yerleşik adlı argüman olabilir (çağıran bakar).
func (x *indexer) namedArgTarget(f *file, o owner, sc *scope, c *ast.CallExpression, arg string) (string, Confidence, Basis) {
	def, conf, basis := x.callee(f, o, sc, c)
	return x.calleeParam(def, arg), conf, basis
}

// callee çağrının çağırdığı TR bildirimidir: işlev, takma adlı modülün işlevi, alıcısı bilinen metot ya da yapı
// (bilinmezse ""); güven ve dayanak alıcının çözümündendir.
func (x *indexer) callee(f *file, o owner, sc *scope, c *ast.CallExpression) (string, Confidence, Basis) {
	switch fn := c.Function.(type) {
	case *ast.Identifier:
		if sc.locals[fn.Value] {
			return "", "", ""
		}
		return f.visible[fn.Value], Extracted, ""
	case *ast.MemberExpression:
		if obj, ok := fn.Object.(*ast.Identifier); ok && !sc.locals[obj.Value] {
			if target := f.aliases[obj.Value]; target != nil {
				return target.defs[fn.Property], Extracted, ""
			}
		}
		structID, conf, basis := x.receiver(f, o, sc, fn.Object)
		if structID != "" {
			if mid, kind := x.structMember(structID, fn.Property); kind == KindMethod {
				return mid, conf, basis
			}
		}
	}
	return "", "", ""
}

// calleeParam bir tanımın (işlev, metot ya da yapı) arg adlı parametresinin düğümüdür; kurucusu olmayan yapıda aynı
// adlı alan.
func (x *indexer) calleeParam(def, arg string) string {
	n := x.nodes[def]
	if n == nil || n.Pos == nil {
		return ""
	}
	switch n.Kind {
	case KindFunction, KindMethod:
		return x.paramOf(def, arg)
	case KindStruct:
		if ctor, kind := x.structMember(def, x.lang.Name(codes.CtxInit)); kind == KindMethod {
			return x.paramOf(ctor, arg)
		}
		if fid, kind := x.structMember(def, arg); kind == KindField {
			return fid
		}
	}
	return ""
}

// calleeParamAt bir tanımın (işlev, metot ya da kurucusuyla yapı) i. konumlu parametresinin düğümüdür ("" yoksa ya
// da sıra bilinmiyorsa).
func (x *indexer) calleeParamAt(def string, i int) string {
	n := x.nodes[def]
	if n == nil {
		return ""
	}
	if n.Kind == KindStruct {
		ctor, kind := x.structMember(def, x.lang.Name(codes.CtxInit))
		if kind != KindMethod {
			return "" // kurucusuz yapıda konumlu argümanın alanı bilinmez
		}
		def = ctor
	}
	if list := x.paramsInOrder()[def]; i < len(list) {
		return list[i]
	}
	return ""
}

// paramsInOrder işlev ve metot kimliğinden parametre düğümlerine, başlıktaki sırayla. Düğümlerden kurulur (artımlı
// yenilemede önbellekten gelen dosyaların parametreleri de böyle görünür); bir parametresinin konumu bulunamamış
// (sütunsuz) bildirimin sırası bilinmez, listesi yoktur.
func (x *indexer) paramsInOrder() map[string][]string {
	if x.paramOrder != nil {
		return x.paramOrder
	}
	x.paramOrder = map[string][]string{}
	unknown := map[string]bool{}
	for id, n := range x.nodes {
		if n.Kind != KindParameter || n.Pos == nil {
			continue
		}
		owner := n.Name[:strings.LastIndex(n.Name, ".")]
		oid := NodeID(KindMethod, n.Pos.File, owner)
		if x.nodes[oid] == nil {
			oid = NodeID(KindFunction, n.Pos.File, owner)
		}
		if n.Pos.Col == 0 {
			unknown[oid] = true
		}
		x.paramOrder[oid] = append(x.paramOrder[oid], id)
	}
	for oid, list := range x.paramOrder {
		if unknown[oid] {
			delete(x.paramOrder, oid)
			continue
		}
		sort.Slice(list, func(a, b int) bool { return posLess(x.nodes[list[a]].Pos, x.nodes[list[b]].Pos) })
	}
	return x.paramOrder
}

// paramOf bir işlev ya da metot düğümünün arg adlı parametre düğümüdür ("" yoksa).
func (x *indexer) paramOf(fnID, arg string) string {
	n := x.nodes[fnID]
	if n == nil || n.Pos == nil {
		return ""
	}
	id := NodeID(KindParameter, n.Pos.File, n.Name+"."+arg)
	if x.nodes[id] == nil {
		return ""
	}
	return id
}

// optionName seçeneğin komut satırındaki adıdır. Kod taşıyan (kodlu dosyada bir üye kodu olan) çıplak anahtar çalışma
// anında derlenen dosyanın dilinde yazılır (K3, compiler.dictKey `s.Token.Code`: İngilizce betikte `{TR_mem_step: 100}` →
// `--step`); dizin dosyayı Türkçe okuduğu için anahtar `adım` görünür, ad dosyanın dilinden alınır. Kodsuz anahtar
// olduğu gibi kalır: Türkçe gösterimde dilin bir sözcüğüne denk gelen kodsuz ad `$adım` diye kaçışla yazılır (Decode,
// 2.11), kaçışsız bir üye adı bu yüzden yalnızca koddan gelebilir; İngilizce betikte düz yazılmış `adım` anahtarı
// `--adım`dır (bağımsız denetim grup 7). Türkçe dosya olduğu gibi kalır. Konum Türkçe gösterimdeki anahtarın yeridir.
// İki dildeki eş betiklerin seçenekleri ayrı dosyalarda, ayrı düğümlerdir.
func (x *indexer) optionName(f *file, key *ast.StringLiteral) string {
	if f.res.lang == "" || f.res.lang == codes.SourceLanguage || escapedAt(f, key.Token.Line, key.Token.Column) {
		return key.Value
	}
	code, ok := x.lang.Find(codes.NamespaceMember, key.Value)
	if !ok {
		return key.Value
	}
	if x.fileLangs == nil {
		x.fileLangs = map[string]*codes.Language{}
	}
	l, seen := x.fileLangs[f.res.lang]
	if !seen {
		l, _ = codes.LoadLanguage(f.res.lang)
		x.fileLangs[f.res.lang] = l
	}
	if l == nil || l.Name(code) == "" {
		return key.Value
	}
	return l.Name(code)
}

// escapedAt gösterim satırında sütundaki ad `$` kaçışıyla mı yazılmış.
func escapedAt(f *file, line, col int) bool {
	if line < 1 || line > len(f.lines) || col < 1 {
		return false
	}
	r := []rune(f.lines[line-1])
	return col <= len(r) && r[col-1] == '$'
}

// optionKeys komut_satırı({ad: ...}) çağrısının sözlük anahtarlarını (ve açıklamalar: {ad: ...}
// anahtarlarını) seçenek düğümlerine bağlar: anahtar komut satırında `--ad` olur.
func (x *indexer) optionKeys(f *file, o owner, sc *scope, c *ast.CallExpression) {
	fn, ok := c.Function.(*ast.Identifier)
	if !ok || sc.locals[fn.Value] || f.visible[fn.Value] != "" {
		return
	}
	if k, ok := x.lang.Find(codes.NamespaceGlobal, fn.Value); !ok || k != codes.FnCommandLine {
		return
	}
	var dicts []*ast.DictLiteral
	if len(c.Arguments) > 0 {
		if d, ok := c.Arguments[0].(*ast.DictLiteral); ok {
			dicts = append(dicts, d)
		}
	}
	for _, a := range c.NamedArgs {
		if k, ok := x.lang.Find(codes.NamespaceParam, a.Name); ok && k == codes.ArgDescriptions {
			if d, ok := a.Value.(*ast.DictLiteral); ok {
				dicts = append(dicts, d)
			}
		}
	}
	for _, d := range dicts {
		for _, p := range d.Pairs {
			key, ok := p.Key.(*ast.StringLiteral)
			if !ok || !f.res.dictKey(key.Token.Line, key.Token.Column) {
				continue
			}
			name := x.optionName(f, key)
			id := NodeID(KindOption, f.rel, name)
			pos := tokenPos(f, key.Token.Line, key.Token.Column, key.Value)
			if x.nodes[id] == nil {
				x.nodes[id] = &Node{ID: id, Kind: KindOption, Name: name, Pos: &pos, Qualified: f.base + "." + name}
			}
			x.occurrenceBasis(id, RoleOptionKey, Extracted, pos, o.id, "")
		}
	}
}
