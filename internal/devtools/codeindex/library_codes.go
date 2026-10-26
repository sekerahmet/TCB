package codeindex

import (
	"regexp"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// Kitaplığın dışa açık adları koddur ve kitaplığın dili yoktur (dil tanımı 17.3): lib/ altındaki bir yapı, işlev,
// sabit, alan, metot ya da parametre bildiriminin adı bir kodsa (kitaplığın kendi kodu ya da çekirdekle ortak kod:
// TR_type_GPT, TR_arg_step) bildirim o kodu taşır (Node.Code); kitaplık kodunun ayrı yerleşik düğümü yoktur (builtin).
// Sorgu kodla ya da herhangi bir yazım dilindeki adla bildirime çözülür (Resolve), çıktı adı okuyanın dilinde yazar
// (display), bir metnin kullandığı kitaplık adı koduyla verilir (Snippet), codegraph bildirimi koduna bağlar: dördü de
// düğümün kodunu okur.

// libraryNamespace kitaplık bildiriminin türünden adının ad alanıdır.
var libraryNamespace = map[Kind]codes.Namespace{
	KindStruct: codes.NamespaceType, KindFunction: codes.NamespaceGlobal, KindConstant: codes.NamespaceGlobal,
	KindVariable: codes.NamespaceGlobal, KindField: codes.NamespaceMember, KindMethod: codes.NamespaceMember,
	KindParameter: codes.NamespaceParam,
}

// libraryCode f'deki bir kitaplık bildiriminin kodudur (değilse boş). Kitaplık depodaki lib/ klasörüdür ya da gömülü
// kitaplık (tr:lib/...); kitaplığın test dosyaları (_test.tr) kitaplığın adlarını bildirmez. Dizin dosyayı Türkçe
// yazımında okur (readTurkish): kodlu dosyanın adı o yazıma koddan çevrilmiştir, aynı yazımda aranınca kodu geri verir.
func (x *indexer) libraryCode(f *file, kind Kind, name string) string {
	space, ok := libraryNamespace[kind]
	file := strings.TrimPrefix(f.rel, libraryPrefix)
	if !ok || !strings.HasPrefix(file, "lib/") || strings.HasSuffix(file, "_test.tr") {
		return ""
	}
	if k, ok := x.lang.Find(space, name[strings.LastIndex(name, ".")+1:]); ok {
		return k.String()
	}
	return ""
}

// LanguageNames kodun her yazım dilindeki adıdır (yinelenen bir kez): kodlu bildirim (Node.Code) bu adlarla da aranır;
// codegraph aynı adları bildirimin takma adları yapar, iki aracın çıplak ad kuralı tektir.
func LanguageNames(k codes.Code) []string {
	var out []string
	for _, l := range writingLanguages() {
		if name := l.Name(k); !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	return out
}

// codeNames kodlu düğümlerin sorguda aranan adlarıdır: her yazım dilindeki adı ve kitaplığın kendi kodunda kimliği ->
// düğüm kimlikleri. Kimlik kodun asıl düğümünü gösterir: kitaplık kodunda bildirimi (yerleşik düğümü yoktur), çekirdekle
// ortak kodda (TR_arg_min, kitaplıkta bir parametrenin de adı) yerleşiği. Kullanıcının adları yalnız yazıldığı gibi
// aranır (nameClass).
func codeNames(idx *Index) map[string][]string {
	names := map[string][]string{}
	for _, n := range idx.Nodes {
		k, err := codes.Parse(n.Code)
		if err != nil {
			continue
		}
		spellings := LanguageNames(k)
		if k.IsLibrary() {
			spellings = append(spellings, n.Code)
		}
		for _, s := range spellings {
			if !slices.Contains(names[s], n.ID) {
				names[s] = append(names[s], n.ID)
			}
		}
	}
	return names
}

// pathIn düğümün modül içindeki yoludur (Name: `Doğrusal.ileri`), kodlu parçaları spell'in yazımıyla (bir dilin adı ya
// da kodun kendisi): yol sahiplerin (defines kenarları) adlarından kurulur, kullanıcının adı aynen kalır.
func (r *Reader) pathIn(n *Node, spell func(codes.Code) string) string {
	seg := shortName(n)
	if k, err := codes.Parse(n.Code); err == nil {
		seg = spell(k)
	}
	if p := r.byID[r.parent[n.ID]]; p != nil && p.Kind != KindModule && strings.Contains(n.Name, ".") {
		return r.pathIn(p, spell) + "." + seg
	}
	return seg
}

// qualifiedIn düğümün nitelenmiş adıdır (`katman.Doğrusal.ileri`), kodlu parçaları spell'in yazımıyla.
func (r *Reader) qualifiedIn(n *Node, spell func(codes.Code) string) string {
	if n.Qualified == "" || !strings.HasSuffix(n.Qualified, n.Name) {
		return n.Qualified
	}
	return strings.TrimSuffix(n.Qualified, n.Name) + r.pathIn(n, spell)
}

// identWord bir imzadaki ad sözcüğüdür.
var identWord = regexp.MustCompile(`[\p{L}_][\p{L}\p{N}_]*`)

// signatureIn düğümün imzasıdır, dilin öğeleri lang'in yazımıyla (belirteç düzeyinde çeviri): baştaki anahtar sözcük
// (`fonksiyon`, `yapı`, `test`), düğümün ve parametrelerinin kodlu adları, parametrenin varsayılan değerindeki dil adları
// (`yok`, `doğru`). Kullanıcının adları aynen kalır; anahtar sözcük ve varsayılan değer dışındaki bir sözcük, dilin bir
// adıyla aynı yazılmış olsa da kullanıcının adı sayılır.
func (r *Reader) signatureIn(n *Node, lang *codes.Language) string {
	own := map[string]string{} // Türkçe yazım -> lang'deki yazım
	for _, id := range append([]string{n.ID}, r.children[n.ID]...) {
		if c := r.byID[id]; c != nil && (id == n.ID || c.Kind == KindParameter) {
			if k, err := codes.Parse(c.Code); err == nil {
				own[shortName(c)] = lang.Name(k)
			}
		}
	}
	keyword := n.Kind == KindFunction || n.Kind == KindMethod || n.Kind == KindStruct || n.Kind == KindTest
	value := strings.Index(n.Signature, " = ") // parametrenin varsayılan değeri
	if n.Kind != KindParameter {
		value = -1
	}
	var b strings.Builder
	last := 0
	for i, at := range identWord.FindAllStringIndex(n.Signature, -1) {
		w := n.Signature[at[0]:at[1]]
		b.WriteString(n.Signature[last:at[0]])
		last = at[1]
		if s, ok := own[w]; ok {
			w = s
		} else if i == 0 && keyword || value >= 0 && at[0] > value {
			if k, ok := source.Turkish().Find(codes.NamespaceGlobal, w); ok {
				w = lang.Name(k)
			}
		}
		b.WriteString(w)
	}
	b.WriteString(n.Signature[last:])
	return b.String()
}

// qualifiedSpellings düğümün nitelenmiş adının dilin öğesi olan parçaları her yazım dilinde ve kodla yazılmış
// biçimleridir (Türkçe yazımdan farklı olanlar): `katman.Linear.forward`, `katman.TR_type_Linear.TR_mem_forward`.
// Yolunda kodlu düğüm olmayan düğümde boştur.
func (r *Reader) qualifiedSpellings(n *Node) [][2]string {
	coded := false
	for c := n; c != nil && c.Kind != KindModule; c = r.byID[r.parent[c.ID]] {
		coded = coded || c.Code != ""
	}
	if !coded {
		return nil
	}
	var out [][2]string
	add := func(q string) {
		if q != n.Qualified && !slices.ContainsFunc(out, func(s [2]string) bool { return s[0] == q }) {
			out = append(out, [2]string{q, Fold(q)})
		}
	}
	for _, l := range writingLanguages() {
		add(r.qualifiedIn(n, l.Name))
	}
	add(r.qualifiedIn(n, codes.Code.String))
	return out
}
