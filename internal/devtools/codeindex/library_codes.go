package codeindex

import (
	"regexp"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Dilin öğesi olan adlar koddur (dil tanımı 17.3): dizin dosyayı kimlikleriyle okur (read.go), bu yüzden adı bir kod
// olan bildirim (kitaplığın yapısı, işlevi, alanı, metodu, parametresi: TR_type_GPT, TR_arg_step; kullanıcı yapısının
// üye koduyla yazılmış alanı) o kodu taşır (Node.Code) ve adı kimliğidir. Kitaplık kodunun ayrı yerleşik düğümü yoktur
// (builtin). Sorgu kodla ya da herhangi bir yazım dilindeki adla bildirime çözülür (Resolve), çıktı adı okuyanın
// dilinde yazar (display), bir metnin kullandığı kitaplık adı koduyla verilir (Snippet), codegraph bildirimi koduna
// bağlar: dördü de düğümün kodunu okur.

// declCode bir bildirimin kodudur: adı (nitelenmiş adın son parçası) bir kimlikse o kod, değilse boş (kullanıcının
// adı). Ad okunan belirteçten gelir; TR_ ile başlayan ad kullanıcının adı olamaz (2.3a), bu yüzden kimlik kodun kendisidir.
func declCode(name string) string {
	if k, err := codes.Parse(name[strings.LastIndex(name, ".")+1:]); err == nil {
		return k.String()
	}
	return ""
}

// IdentityQuery sorgu (harf büyüklüğü ne olursa olsun) çıplak bir kimlik ya da üye yazımı mı (`TR_ctx_init`,
// `tr_ctx_init`, `x.TR_map`): kimlik kodun kendi düğümünü gösterir (yerleşik, kitaplık bildirimi; codegraph'ta ad
// düğümü), bir bildirimin adının parçası değildir. İki aracın tek kuralı.
func IdentityQuery(q string) bool {
	s := strings.TrimPrefix(strings.ToUpper(q), "X.")
	return !strings.Contains(s, ".") && codes.IsReservedName(s)
}

// MemberQuery sorgu `<alıcı>.TR_<ad>` ya da `.TR_<ad>` biçimli bir yazımıysa `x.TR_<ad>` karşılığını verir; değilse boş
// döner. Alıcı tam harfle bilinen bir modül ya da kodlu tip adıysa boş döner ("bulunamadı"). Alıcı kodsuz bir ad olup
// bilinen bir tipin adıysa (`A.TR_shape`, `Model.TR_append`, `Linear.TR_map`, `GPT.TR_map`), `x.TR_<ad>` hedefiyle
// birlikte o tip düğümlerinin indislerini döndürür; `x.TR_<ad>` bir şey bulursa cevap tip ve değer adaylarıyla
// belirsizdir (çıkış 3), bulmazsa "bulunamadı" kalır (çıkış 4). Takma adla sorulan kitaplık bildirimleri
// (`katman.TR_GPT`, `k.TR_Linear`) nitelenmiş ad basamağında yalnızca takma adın gösterdiği modül için çözülür
// (qualifiedSpellings). İki aracın (codeindex ve codegraph) tek kuralı.
func MemberQuery(q string, n int, kindOf func(i int) Kind, names func(i int) [][2]string) (string, []int) {
	recv, member, ok := strings.Cut(q, ".")
	if !ok || strings.EqualFold(recv, "x") || strings.ContainsAny(recv, "/:") || !IdentityQuery("x."+member) {
		return "", nil
	}
	if recv != "" {
		if mods := QualifiedMatch(n, NameKey{Key: recv}, func(i int) [][2]string {
			if kindOf(i) == KindModule {
				return names(i)
			}
			return nil
		}); len(mods) > 0 {
			return "x." + member, mods
		}
		if types := QualifiedMatch(n, NameKey{Key: recv}, func(i int) [][2]string {
			if kindOf(i) == KindStruct {
				return names(i)
			}
			return nil
		}); len(types) > 0 {
			if codes.IsReservedName(recv) {
				return "", nil
			}
			return "x." + member, types
		}
	}
	return "x." + member, nil
}

// moduleImportAliases her modül dosyasını (`lib/layers.tr`) dizinde onu `TR_import … TR_as <ad>` ile aktaran sıralı
// takma adlara eşler; `<takma ad>.TR_<ad>` yalnız takma adın modülünde bildirilmişse bulunur.
func moduleImportAliases(idx *Index) map[string][]string {
	byMod := map[string][]string{}
	for _, n := range idx.Nodes {
		for _, imp := range n.Imports {
			if imp.Alias != "" && imp.Target != "" && !slices.Contains(byMod[imp.Target], imp.Alias) {
				byMod[imp.Target] = append(byMod[imp.Target], imp.Alias)
			}
		}
	}
	for _, list := range byMod {
		slices.Sort(list)
	}
	return byMod
}

// MemberTypeNote, MemberModuleNote ve MemberValueNote tiple ya da modülle aynı adı taşıyan kodsuz alıcıda
// (`A.TR_shape`, `Model.TR_append`, `layers.TR_map`) belirsiz cevabın aday kümelerinin ileti kodu (H8877, H8879, H8878)
// ve etkin dildeki etiket metnidir; *In kardeşleri aynı kodların verilen dildeki ("en": codegraph) metnini döner.
func MemberTypeNote(recv string) (string, string) {
	return messages.H8877.String(), messages.Text(messages.H8877, recv)
}

func MemberModuleNote(recv string) (string, string) {
	return messages.H8879.String(), messages.Text(messages.H8879, recv)
}

func MemberValueNote(recv, member string) (string, string) {
	return messages.H8878.String(), messages.Text(messages.H8878, recv, member)
}

func MemberTypeNoteIn(lang, recv string) (string, string) {
	return messages.H8877.String(), messages.TextIn(lang, messages.H8877, recv)
}

func MemberModuleNoteIn(lang, recv string) (string, string) {
	return messages.H8879.String(), messages.TextIn(lang, messages.H8879, recv)
}

func MemberValueNoteIn(lang, recv, member string) (string, string) {
	return messages.H8878.String(), messages.TextIn(lang, messages.H8878, recv, member)
}

// BuiltinNames bir yerleşik kodun (ya da ad kodunun) sorguda aranan adlarıdır: kimliği, okuyanın dilindeki adı, her
// yazım dilindeki adı (LanguageNames) ve program metnindeki yazımı (SpelledNames: TR_print, x.TR_map). Kitaplık kodunun
// dillerdeki adlarını ve yazımını kitaplıktaki bildirimi taşır (DeclNames), kod yalnız kimliğiyle aranır; iki aracın
// (trlang index ve codegraph) tek kuralı.
func BuiltinNames(k codes.Code) []string {
	if k.IsLibrary() {
		return []string{k.String()}
	}
	out := []string{k.String()}
	for _, s := range append(append([]string{codes.DisplayName(k)}, LanguageNames(k)...), SpelledNames(k)...) {
		if s != "" && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// DeclNames bir bildirimin çıplak sorguda aranan adlarıdır: kodsuz bildirimde kısa adı; kodlu bildirimde her yazım
// dilindeki adı ve kitaplığın kendi bildiriminde (IsLibraryDecl) program metnindeki yazımı (SpelledNames). Kullanıcının
// aynı kodlu üyesi dillerdeki adıyla aranır; iki aracın (trlang index ve codegraph) tek kuralı.
func DeclNames(n *Node) []string {
	k, err := codes.Parse(n.Code)
	if err != nil {
		return []string{shortName(n)}
	}
	names := LanguageNames(k)
	if IsLibraryDecl(n) {
		names = append(names, SpelledNames(k)...)
	}
	return names
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

// SpelledNames kodun program metnindeki yazımıyla sorulduğu adlardır (dil tanımı 2.3a; ajan bu yazımı yazar; codegraph
// aynı adları ad düğümünün ve kodlu bildirimin adları yapar, iki aracın kuralı tektir): öbür
// yerdeki kod `TR_print`, üye `x.TR_map`. Çıplak yazım (`TR_map`, `TR_every_minutes`) daha öncelikli bir yerde aynı
// yazımlı başka kod yoksa kodundur; öncelik öbür, üye, argüman sırasıdır (`TR_min` işlevdir, adlı argüman `TR_min`
// değil). Program metninde yazılmayan kodun (komut, seçenek, ortam değişkeni) yazımı yoktur.
func SpelledNames(k codes.Code) []string {
	p, ok := codes.PlaceOf(k)
	if !ok {
		return nil
	}
	s := codes.Spelling(k)
	var out []string
	if p == codes.PlaceMember {
		out = append(out, "x."+s)
	}
	for q := codes.PlaceOther; q < p; q++ {
		if _, taken := codes.FromSpelling(q, s); taken {
			return out
		}
	}
	return append(out, s)
}

// codeNames kodlu düğümlerin sorguda aranan adlarıdır: her yazım dilindeki adı ve kitaplığın kendi kodunda kimliği ->
// düğüm kimlikleri. Kimlik kodun asıl düğümünü gösterir: kitaplık kodunda bildirimi (yerleşik düğümü yoktur), çekirdekle
// ortak kodda (TR_arg_min, kitaplıkta bir parametrenin de adı) yerleşiği. Kodlu bildirim kısa adıyla (kimliğiyle)
// aranmaz (nameClass); kullanıcının adları yazıldığı gibi aranır.
func codeNames(idx *Index) map[string][]string {
	names := map[string][]string{}
	for _, n := range idx.Nodes {
		if n.Code == "" {
			continue
		}
		spellings := DeclNames(&n)
		if IsLibraryDecl(&n) {
			// kitaplığın kendi bildirimi kimliğiyle ve yazımıyla; kullanıcının aynı kodlu üyesi dillerdeki adıyla
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

// pathIn düğümün modül içindeki yoludur (Name: `TR_type_Linear.TR_mem_forward`), kodlu parçaları spell'in yazımıyla
// (bir dilin adı ya da kodun kendisi): yol sahiplerin (defines kenarları) adlarından kurulur, kullanıcının adı aynen
// kalır.
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

// qualifiedIn düğümün nitelenmiş adıdır (`layers.Doğrusal.ileri`), kodlu parçaları spell'in yazımıyla.
func (r *Reader) qualifiedIn(n *Node, spell func(codes.Code) string) string {
	if n.Qualified == "" || !strings.HasSuffix(n.Qualified, n.Name) {
		return n.Qualified
	}
	return strings.TrimSuffix(n.Qualified, n.Name) + r.pathIn(n, spell)
}

// identWord bir imzadaki ad sözcüğüdür.
var identWord = regexp.MustCompile(`[\p{L}_][\p{L}\p{N}_]*`)

// signatureIn düğümün imzasıdır, dilin öğeleri lang'in yazımıyla (belirteç düzeyinde çeviri): imza okunan
// belirteçlerden kurulur (anahtar sözcük, adlar), kimlik olan her sözcük kodun lang'deki adına çevrilir, kullanıcının
// adları aynen kalır. Parametrenin varsayılan değeri dosyanın metnidir, yazıldığı gibi kalır.
func signatureIn(n *Node, lang *codes.Language) string {
	return identWord.ReplaceAllStringFunc(n.Signature, func(w string) string {
		if k, err := codes.Parse(w); err == nil {
			if g, ok := lang.Entry(k); ok {
				return g.Name
			}
		}
		return w
	})
}

// qualifiedSpellings düğümün nitelenmiş adının dilin öğesi olan parçaları her yazım dilinde yazılmış biçimleridir
// (kimlikle yazılmış Qualified'dan farklı olanlar): `layers.Linear.forward`, `layers.Doğrusal.ileri`, ayrıca görünen
// adlı tipin kod yazımlı üyesi (`layers.Linear.TR_forward`, `layers.Doğrusal.TR_forward`, `layers.GPT.TR_generate`) ve
// üst düzey kitaplık bildiriminde yalnızca o modülü aktaran `TR_as` takma adlarıyla yazılmış biçimleri (`katman.TR_GPT`,
// `k.TR_Linear`). Yolunda kodlu düğüm olmayan düğümde boştur.
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
	spells := []func(codes.Code) string{codes.Spelling} // program metnindeki yazım: layers.TR_Linear.TR_forward
	for _, l := range writingLanguages() {
		spells = append(spells, l.Name)
	}
	for _, spell := range spells {
		add(r.qualifiedIn(n, spell))
	}
	if p := r.byID[r.parent[n.ID]]; p != nil && p.Kind != KindModule && strings.Contains(n.Name, ".") && strings.HasSuffix(n.Qualified, n.Name) {
		if k, err := codes.Parse(n.Code); err == nil {
			prefix := strings.TrimSuffix(n.Qualified, n.Name)
			leaf := "." + codes.Spelling(k)
			for _, l := range writingLanguages() {
				add(prefix + r.pathIn(p, l.Name) + leaf)
			}
		}
	}
	if IsLibraryDecl(n) && !strings.Contains(n.Name, ".") && n.Pos != nil {
		if k, err := codes.Parse(n.Code); err == nil {
			leaf := "." + codes.Spelling(k)
			for _, a := range r.moduleAliases[n.Pos.File] {
				add(a + leaf)
			}
		}
	}
	return out
}

// QualifiedNames düğümün nitelenmiş adının her yazım dilindeki ve kendi modülünün `TR_as` takma adlarıyla yazılmış
// biçimleridir (qualifiedSpellings): codegraph onları düğümün takma adları yapar.
func (r *Reader) QualifiedNames(n *Node) []string {
	qs := r.qualifiedSpellings(n)
	out := make([]string, len(qs))
	for i, q := range qs {
		out[i] = q[0]
	}
	return out
}

// libraryDecl düğüm kitaplığın kodlu bir bildirimi mi: kod taşır ve kitaplıktadır (depodaki lib/ ya da gömülü tr:lib/;
// kitaplığın testleri değil). Kullanıcının aynı kodla yazılmış üyesi kitaplığın bildirimi değildir.
func libraryDecl(n *Node) bool {
	if n.Code == "" || n.Pos == nil {
		return false
	}
	file := strings.TrimPrefix(n.Pos.File, libraryPrefix)
	return strings.HasPrefix(file, "lib/") && !strings.HasSuffix(file, "_test.tr")
}

// IsLibraryDecl düğüm kitaplığın kendi kodlu bildirimi mi: kitaplık kodu taşır ve kitaplıktadır (libraryDecl; çekirdekle
// ortak kod ve kullanıcının aynı kodla yazılmış üyesi değil).
func IsLibraryDecl(n *Node) bool {
	if !libraryDecl(n) {
		return false
	}
	k, err := codes.Parse(n.Code)
	return err == nil && k.IsLibrary()
}
