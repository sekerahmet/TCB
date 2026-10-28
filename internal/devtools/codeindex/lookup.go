package codeindex

import (
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// positionQuery "dosya.tr:satır[:sütun]" biçimli sorgudur: imlecin altındaki ad.
var positionQuery = regexp.MustCompile(`^(.+\.tr):(\d+)(?::(\d+))?$`)

// Fold Türkçe büyük-küçük harf kuralıyla küçültür (İ→i, I→ı): düz ToLower "I"yı "i" yapardı. İki aracın (tr dizin ve
// codegraph) tek harf katlamasıdır.
func Fold(s string) string { return strings.ToLowerSpecial(unicode.TurkishCase, s) }

// hasUpper sorguda büyük harf var mı (NameKeys).
func hasUpper(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

// NameKey ad karşılaştırmasının bir geçişidir: Folded false ise ad aynen Key olmalı, true ise adın Türkçe katlanmışı
// (Fold) Key olmalı.
type NameKey struct {
	Key    string
	Folded bool
	Alt    string // harfsiz geçişte ikinci anahtar: sorgunun ASCII küçültmesi (`IKINCI` → `ikinci`), Türkçe katlamadan farklıysa
}

// NameKeys sorgunun harf kuralıdır; iki aracın (tr dizin ve codegraph) tek kuralı (bağımsız denetim grup 6, R1: biri
// her zaman katlıyor, öbürü büyük harfli sorguda tam eşleşme istiyordu). Sorguda büyük harf yoksa büyük-küçük harf
// Türkçe kuralıyla yok sayılır (`ışık` = `Işık`, `isik` ≠ `Işık`); varsa önce tam eşleşme aranır (ripgrep 14
// `--smart-case` kuralı; `go doc` da büyük harfi tam eşleştirir), hiçbir aday tam tutmazsa harf yok sayılarak bir kez
// daha (`GÖMMEKATMANI` → `GömmeKatmanı`: bütün harfleri büyük yazılmış sorgu bir harf kararı değildir).
//
// İki ek (bağımsız denetim grup 7): bütün harfleri büyük yazılmış sorgu İngilizce klavyeyle de yazılmış olabilir
// (`IKINCI`, `GIZLI_DURUM`: `I` Türkçede `ı` olur), harfsiz geçiş ASCII küçültmesiyle de denenir. Sorgu dilin bir
// sözcüğüyle birebir aynıysa (`Metin` tip adı, `Tensör`) harfsiz geçiş yoktur: dizinde düğümü olmasa da o sözcük
// kastedilmiştir, benzer harfli bir yerleşik (`metin()`) sessizce verilmez.
func NameKeys(q string) []NameKey {
	if !hasUpper(q) {
		return []NameKey{{Key: Fold(q), Folded: true}}
	}
	keys := []NameKey{{Key: q}}
	if languageWord(q) {
		return keys
	}
	folded := NameKey{Key: Fold(q), Folded: true}
	if ascii := strings.ToLower(q); ascii != folded.Key {
		folded.Alt = ascii // iki katlama aynı geçişte: biri bir düğümde durup öbürünün adayını gizlemesin
	}
	return append(keys, folded)
}

// programNamespaces bir TR programında yazılan dil adlarının ad alanlarıdır.
var programNamespaces = []codes.Namespace{codes.NamespaceGlobal, codes.NamespaceType, codes.NamespaceMember, codes.NamespaceParam}

// writingLanguages gömülü yazım dilleridir (bir kez yüklenir).
var writingLanguages = sync.OnceValue(func() []*codes.Language {
	var out []*codes.Language
	for _, code := range codes.Languages() {
		if l, err := codes.LoadLanguage(code); err == nil {
			out = append(out, l)
		}
	}
	return out
})

// languageWord q herhangi bir yazım dilinde bir program adının (anahtar sözcük, tip, yerleşik, üye, adlı argüman)
// yazımı mı.
func languageWord(q string) bool {
	for _, l := range writingLanguages() {
		for _, ns := range programNamespaces {
			if _, ok := l.Find(ns, q); ok {
				return true
			}
		}
	}
	return false
}

// Match name (katlanmışı folded) bu geçişte uyuyor mu.
func (k NameKey) Match(name, folded string) bool {
	if k.Folded {
		return folded == k.Key || k.Alt != "" && folded == k.Alt
	}
	return name == k.Key
}

// HasSuffix name (katlanmışı folded) bu geçişte suffix'le bitiyor mu (suffix sorgu tarafındadır, ör. "." + anahtar).
func (k NameKey) HasSuffix(name, folded, prefix string) bool {
	if k.Folded {
		return strings.HasSuffix(folded, prefix+k.Key) || k.Alt != "" && strings.HasSuffix(folded, prefix+k.Alt)
	}
	return strings.HasSuffix(name, prefix+k.Key)
}

// NameClass bir adayın çıplak ad çözümündeki sınıfıdır (PickByName).
type NameClass int

const (
	ClassModule      NameClass = iota // modül adı ya da yolu (`katman`, `tr/katman`)
	ClassDeclaration                  // bildirilmiş kısa ad: işlev, yapı, alan, metot, parametre, seçenek ...
	ClassBuiltin                      // yerleşik ad (her dilde) ya da ad kodu
	ClassTest                         // test bloğunun açıklaması
	ClassNone                         // uymuyor
)

// PickByName çıplak adın adaylarını seçer; iki aracın tek sınıf kuralı. class i. adayın bu sorgudaki sınıfıdır.
// Modül adı bildirimlerden önce gelir (`katman` modüldür, aynı adlı parametreler değil); yerleşik ad uyan ilk sınıfa
// katılır, biri sessizce seçilmez (`metin` hem yerleşik hem modül, `her_dakika` hem seçenek hem adlı argüman); test
// bloğunun açıklaması bir ad değil metindir, başka aday yoksa gelir (bağımsız denetim grup 6, R1: `ortalama` yerleşiği
// aynı açıklamalı bir testle belirsiz, `metin` yerleşiği modülün arkasında kayboluyordu). Sıralar artan döner.
func PickByName(n int, class func(i int) NameClass) []int {
	var by [ClassNone][]int
	for i := 0; i < n; i++ {
		if c := class(i); c < ClassNone {
			by[c] = append(by[c], i)
		}
	}
	var out []int
	switch {
	case len(by[ClassModule]) > 0:
		out = append(by[ClassModule], by[ClassBuiltin]...)
	case len(by[ClassDeclaration]) > 0:
		out = append(by[ClassDeclaration], by[ClassBuiltin]...)
	case len(by[ClassBuiltin]) > 0:
		return by[ClassBuiltin]
	default:
		return by[ClassTest]
	}
	sort.Ints(out)
	return out
}

// builtinNames kodların her gömülü dildeki adlarını, okuyanın dilindeki adı ve program metnindeki yazımını
// (SpelledNames: TR_print, x.TR_map) toplar: bir yerleşik, kimin hangi dilde yazdığına bakılmadan bulunur.
func builtinNames(idx *Index) map[string][]string {
	names := map[string][]string{}
	add := func(name, id string) {
		if name != "" {
			names[name] = append(names[name], id)
		}
	}
	var langs []*codes.Language
	for _, code := range codes.Languages() {
		if l, err := codes.LoadLanguage(code); err == nil {
			langs = append(langs, l)
		}
	}
	for _, n := range idx.Nodes {
		if n.Kind != KindBuiltin {
			continue
		}
		k, err := codes.Parse(n.Name)
		if err != nil {
			continue
		}
		add(n.Name, n.ID)
		add(codes.DisplayName(k), n.ID)
		for _, l := range langs {
			add(l.Name(k), n.ID)
		}
		for _, s := range SpelledNames(k) {
			add(s, n.ID)
		}
	}
	return names
}

// Resolve bir sorgu metnini düğümlere çözer; ilk eşleşme veren basamaktaki bütün düğümleri döner
// (birden çoksa belirsizdir). Basamaklar: "dosya.tr:satır:sütun" konumu; kimlik; çıplak ad (PickByName: modül yolu ya
// da adı `katman`, `tr/katman`; bildirilmiş kısa ad, dilin öğesi olan bildirimde (Node.Code) kodu ve her yazım
// dilindeki adı da; yerleşik adı, her dilde; test açıklaması); modülle nitelenmiş ad
// (`katman.Doğrusal.ileri`); nitelenmiş adın sonu (`Doğrusal.ileri`). Harf kuralı NameKeys'tir: her geçişte bütün
// basamaklar denenir. kind boş değilse yalnızca o tür.
func (r *Reader) Resolve(q, kind string) []*Node {
	if m := positionQuery.FindStringSubmatch(q); m != nil {
		line, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		return r.atQuery(m[1], line, col, kind)
	}
	if n := r.byID[q]; n != nil && (kind == "" || string(n.Kind) == kind) {
		return []*Node{n}
	}
	nodes := r.Index.Nodes
	at := func(list []int) []*Node {
		out := make([]*Node, len(list))
		for i, j := range list {
			out[i] = &nodes[j]
		}
		return out
	}
	for _, k := range NameKeys(q) {
		// q'ya uyan yerleşik adların düğümleri bir kez bulunur (her yerleşik düğümde bütün adları yeniden katlamak sorguyu
		// yavaşlatıyordu: 278 adlık denetimde 16 sn).
		builtin, coded := map[string]bool{}, map[string]bool{}
		for _, id := range idsFor(k, r.builtins, r.builtinsFolded) {
			builtin[id] = true
		}
		for _, id := range idsFor(k, r.codeNames, r.codeNamesFolded) {
			coded[id] = true
		}
		if found := PickByName(len(nodes), func(i int) NameClass {
			if kind != "" && string(nodes[i].Kind) != kind {
				return ClassNone
			}
			switch {
			case coded[nodes[i].ID]:
				return ClassDeclaration
			case nodes[i].Code != "":
				return ClassNone // kodlu bildirimin kısa adı kimliğidir; adları codeNames'tedir
			}
			return r.nameClass(i, k, builtin)
		}); len(found) > 0 {
			return at(found)
		}
		if IdentityQuery(q) {
			continue // çıplak kimlik kodun kendi düğümüdür (yerleşik ya da kitaplık bildirimi), nitelenmiş bir adın sonu değil
		}
		for _, match := range []func(i int) bool{
			func(i int) bool {
				return r.qualifiedMatch(i, func(name, folded string) bool { return k.Match(name, folded) })
			},
			func(i int) bool {
				return r.qualifiedMatch(i, func(name, folded string) bool { return k.HasSuffix(name, folded, ".") })
			},
		} {
			var found []int
			for i := range nodes {
				if (kind == "" || string(nodes[i].Kind) == kind) && nodes[i].Qualified != "" && match(i) {
					found = append(found, i)
				}
			}
			if len(found) > 0 {
				return at(found)
			}
		}
	}
	return nil
}

// foldedNames bir düğümün sorguda karşılaştırılan adlarının katlanmışlarıdır (NewReader bir kez hesaplar).
type foldedNames struct {
	short, qualified string
	aliases          []string
	qualifiedIn      [][2]string // dilin öğesi olan parçaları her yazım dilinde ve kodla yazılmış nitelenmiş ad: yazım, katlanmışı
}

// nameClass i. düğümün k geçişindeki çıplak ad sınıfıdır (PickByName); builtin k'ye uyan yerleşik düğümlerdir.
func (r *Reader) nameClass(i int, k NameKey, builtin map[string]bool) NameClass {
	n, f := &r.Index.Nodes[i], r.folded[i]
	switch n.Kind {
	case KindModule:
		if k.Match(n.Name, f.short) {
			return ClassModule
		}
		for j, a := range n.Aliases {
			if k.Match(a, f.aliases[j]) {
				return ClassModule
			}
		}
	case KindBuiltin:
		if builtin[n.ID] {
			return ClassBuiltin
		}
	case KindMessage:
		if !k.Folded && n.Name == k.Key {
			return ClassBuiltin // ileti kodu bir kimliktir: yalnız yazıldığı gibi (H6932; h6932 değil)
		}
	case KindTest:
		if k.Match(n.Name, f.short) {
			return ClassTest
		}
	default:
		if k.Match(shortName(n), f.short) {
			return ClassDeclaration
		}
	}
	return ClassNone
}

// idsFor k'ye uyan adların düğüm kimlikleridir: names ad -> kimlikler, folded aynısı Türkçe katlanmış (yerleşik adları,
// kodlu düğümlerin adları).
func idsFor(k NameKey, names, folded map[string][]string) []string {
	if !k.Folded {
		return names[k.Key]
	}
	if k.Alt == "" {
		return folded[k.Key]
	}
	return append(slices.Clone(folded[k.Key]), folded[k.Alt]...)
}

// qualifiedMatch i. düğümün nitelenmiş adı (kimlikle yazılmış Qualified ya da kodlu parçaları bir yazım dilinde) uyuyor mu.
func (r *Reader) qualifiedMatch(i int, match func(name, folded string) bool) bool {
	if match(r.Index.Nodes[i].Qualified, r.folded[i].qualified) {
		return true
	}
	for _, q := range r.folded[i].qualifiedIn {
		if match(q[0], q[1]) {
			return true
		}
	}
	return false
}

// atQuery konum sorgusunu düğüme çevirir; sütun verilmediyse satırdaki ilk başvuru alınır.
func (r *Reader) atQuery(file string, line, col int, kind string) []*Node {
	var n *Node
	if col > 0 {
		_, n = r.At(file, line, col)
	} else {
		for _, i := range r.byFile[file] {
			if o := r.Index.Occurrences[i]; o.Pos.Line == line {
				n = r.byID[o.Node]
				break
			}
		}
	}
	if n == nil || (kind != "" && string(n.Kind) != kind) {
		return nil
	}
	return []*Node{n}
}

// Suggest bulunamayan bir ada yakın düğümleri verir: adı (ya da nitelenmiş adın son parçası ya da
// bir yerleşik adı) sorguyu içerenler önce, sonra Türkçe harf kuralıyla küçük düzenleme
// uzaklığındakiler.
func (r *Reader) Suggest(q string, limit int) []*Node {
	lq := Fold(q)
	maxD := min(4, 1+len([]rune(lq))/4)
	type scored struct {
		n     *Node
		score int
	}
	var found []scored
	for i := range r.Index.Nodes {
		n := &r.Index.Nodes[i]
		if n.Kind == KindParameter || n.Kind == KindOption {
			continue // öneriler bildirimlere gider; parametre ve seçenek adı yazım yanlışı önerisi değildir
		}
		cands := []string{n.Name, n.Qualified}
		if j := strings.LastIndexByte(n.Qualified, '.'); j >= 0 {
			cands = append(cands, n.Qualified[j+1:])
		}
		if k, err := codes.Parse(n.Code); err == nil {
			cands = append(cands, LanguageNames(k)...) // kodlu bildirimin adı kimliğidir; öneri dillerdeki adlarına bakar
		}
		if n.Kind == KindBuiltin {
			for name, ids := range r.builtins {
				for _, id := range ids {
					if id == n.ID {
						cands = append(cands, name)
					}
				}
			}
		}
		best := -1
		for _, c := range cands {
			lc := Fold(c)
			if lc == "" {
				continue
			}
			if strings.Contains(lc, lq) {
				best = 0
				break
			}
			if d := turkish.Distance(lq, lc, maxD); d <= maxD && (best < 0 || d < best) {
				best = d
			}
		}
		if best >= 0 {
			found = append(found, scored{n, best})
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		a, b := found[i], found[j]
		if x, y := r.isIgnored(a.n), r.isIgnored(b.n); x != y {
			return y // depodaki öneri önce
		}
		if a.score != b.score {
			return a.score < b.score
		}
		if len(a.n.Name) != len(b.n.Name) {
			return len(a.n.Name) < len(b.n.Name)
		}
		return a.n.ID < b.n.ID
	})
	if len(found) > limit {
		found = found[:limit]
	}
	out := make([]*Node, len(found))
	for i, f := range found {
		out[i] = f.n
	}
	return out
}
