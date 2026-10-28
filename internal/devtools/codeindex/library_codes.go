package codeindex

import (
	"regexp"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
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

// IdentityQuery sorgu (harf büyüklüğü ne olursa olsun) çıplak bir kimlik mi (`TR_ctx_init`, `tr_ctx_init`): kimlik kodun
// kendi düğümünü gösterir (yerleşik, kitaplık bildirimi; codegraph'ta ad düğümü), bir bildirimin adının parçası değildir.
// İki aracın tek kuralı.
func IdentityQuery(q string) bool {
	return !strings.Contains(q, ".") && codes.IsReservedName(strings.ToUpper(q))
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
		k, err := codes.Parse(n.Code)
		if err != nil {
			continue
		}
		spellings := LanguageNames(k)
		if k.IsLibrary() && libraryDecl(&n) {
			// kitaplığın kendi bildirimi kimliğiyle ve yazımıyla; kullanıcının aynı kodlu üyesi dillerdeki adıyla
			spellings = append(append(spellings, n.Code), SpelledNames(k)...)
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

// qualifiedIn düğümün nitelenmiş adıdır (`katman.Doğrusal.ileri`), kodlu parçaları spell'in yazımıyla.
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
// (kimlikle yazılmış Qualified'dan farklı olanlar): `katman.Linear.forward`, `katman.Doğrusal.ileri`. Yolunda kodlu
// düğüm olmayan düğümde boştur.
func (r *Reader) qualifiedSpellings(n *Node) [][2]string {
	coded := false
	for c := n; c != nil && c.Kind != KindModule; c = r.byID[r.parent[c.ID]] {
		coded = coded || c.Code != ""
	}
	if !coded {
		return nil
	}
	var out [][2]string
	spells := []func(codes.Code) string{codes.Spelling} // program metnindeki yazım: katman.TR_Linear.TR_forward
	for _, l := range writingLanguages() {
		spells = append(spells, l.Name)
	}
	for _, spell := range spells {
		if q := r.qualifiedIn(n, spell); q != n.Qualified && !slices.ContainsFunc(out, func(s [2]string) bool { return s[0] == q }) {
			out = append(out, [2]string{q, Fold(q)})
		}
	}
	return out
}

// QualifiedNames düğümün nitelenmiş adının her yazım dilindeki biçimleridir (qualifiedSpellings): codegraph onları
// düğümün takma adları yapar.
func (r *Reader) QualifiedNames(n *Node) []string {
	var out []string
	for _, q := range r.qualifiedSpellings(n) {
		out = append(out, q[0])
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
