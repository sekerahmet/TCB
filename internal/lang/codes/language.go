package codes

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"
)

//go:embed tr.json
var trJSON []byte

//go:embed en.json
var enJSON []byte

// diller ikiliye gömülü yazım dilleridir (ISO 639-1 kodu → dil dosyası). Türkçe dilin asıl
// dilidir: kodsuz (düz) yazılmış dosyalar Türkçedir; öbür diller kodlu dosyaların gösterimi
// ve o dilde yazılmış metnin kodlanması içindir (docs/spec/17_kodlar.md, 17.6).
var diller = map[string][]byte{"tr": trJSON, "en": enJSON}

// LanguageEnvVar selects the process's active writing language (empty: tr). The lexer's word
// tables are built once from the active language while packages initialize, so the language
// can be chosen only at process start; 'tr kodla --dil en' re-executes itself with this
// variable.
const LanguageEnvVar = "TR_DIL"

// AcceptASCII is the SINGLE switch that decides whether the ASCII-folded forms of names
// (yazdir, sonuc, degisken) are accepted like the names themselves, and it is OFF: every
// operation has one spelling, and the language's words are valid only in their Turkish form.
// The lexer, builtin registration, member resolution, named arguments and type annotations
// get the accepted spellings only through Spellings/Find; the ASCII form is accepted nowhere
// else. ASCII forms are ordinary (undefined) names; Suggest recognizes them only to suggest
// the real Turkish name when an error occurs.
const AcceptASCII = false

// Entry is one line of a language file.
type Entry struct {
	Code Code
	// Name is the code's name in this language.
	Name string
	// Old holds removed names: they are not accepted; the error message only suggests Name.
	// Every operation has one name; a synonym or a second spelling is not accepted. The
	// ASCII forms of old names with Turkish letters (işlev → islev) also count as old.
	Old []string
	// Foreign holds names familiar from other languages (or libraries) (lr, seed, axis):
	// they were never in the language and are not accepted; like Old, the error message
	// only suggests Name. Language-specific: each file records its own readers' habits.
	Foreign []string
	// Family is a short description for people (the group; for members, the types).
	Family string
	// Description is one or two sentences on what the code does; Example is a one-line TR
	// snippet (docs/kodlar.md is generated from them).
	Description, Example string
	// MovedTo is the new code when this code was removed and its job moved to another code
	// (another form); otherwise None (see MovedTo).
	MovedTo Code
	// Removed is true when the code was removed from the language with no replacement
	// (see Removed).
	Removed bool
}

// Dead reports that the code no longer exists in the language: it moved or was removed (its
// name is not accepted and appears only in error messages; its number is never reused).
func (g Entry) Dead() bool { return g.MovedTo != None || g.Removed }

// eskiEslesme bir eski adın hangi koda ait olduğu ve adın, eski bir adın türetilmiş
// ASCII biçimi olup olmadığıdır.
type eskiEslesme struct {
	kod   Code
	ascii bool
}

// Language is the code list built from a language file (tr.json). It never changes after it
// is built and is safe for concurrent reads.
type Language struct {
	// Code is the language's ISO 639-1 code ("tr"; with a region when needed: "pt-BR").
	Code string

	girdiler map[Code]*Entry
	// adDizi aile başına sayı → addır; Ad çalışma anında da harita araması yapmasın diye dizi.
	adDizi   [len(aileHarfleri)][]string
	kodlar   []Code // artan sırada
	yazimlar map[Code][]string
	adlar    [alanSayisi]map[string]Code
	eskiler  [alanSayisi]map[string][]eskiEslesme
	asil     [alanSayisi]map[string]string // ad alanı başına ad ve ASCII biçimi → ad
	// olu taşınmış ya da kaldırılmış kodlardır (artan sırada); oluAd ad alanı başına böyle
	// bir kodun adı, eski adları ve bunların ASCII biçimleri → kod.
	olu   []Code
	oluAd [alanSayisi]map[string]Code
}

// Load reads and validates a language file. On format errors, duplicate codes or clashing
// names in a namespace (in Turkish or ASCII form, old names included) it returns one error
// listing all of them.
func Load(kodu string, veri []byte) (*Language, error) {
	var hatalar []string
	hata := func(bicim string, a ...any) { hatalar = append(hatalar, fmt.Sprintf(bicim, a...)) }

	ham, err := dosyaCoz(veri)
	if err != nil {
		return nil, fmt.Errorf("kod listesi %s: dosya okunamadı (bir JSON nesnesi { \"G100\": { \"ad\": ... }, ... } olmalı): %v", kodu, err)
	}

	d := &Language{Code: kodu, girdiler: make(map[Code]*Entry, len(ham)), yazimlar: make(map[Code][]string, len(ham))}
	var alanBoyu [alanSayisi]int // ad alanı başına yazım sayısı (haritaların boyu için)
	var enBuyuk [len(aileHarfleri)]int
	for _, g := range ham {
		k, err := Parse(g.anahtar)
		if err != nil {
			hata("%v", err)
			continue
		}
		if _, var_ := d.girdiler[k]; var_ {
			hata("%s iki kez tanımlı", k)
			continue
		}
		if k.Namespace() == NamespaceNone {
			hata("%s: %c ailesi henüz kullanılmıyor", k, k.Letter())
			continue
		}
		if g.ad == "" || strings.IndexFunc(g.ad, unicode.IsSpace) >= 0 {
			hata("%s: ad boş olamaz ve boşluk içeremez (%q)", k, g.ad)
			continue
		}
		gr := &Entry{Code: k, Name: g.ad, Old: g.eski, Foreign: g.yabanci, Family: g.aile,
			Description: g.aciklama, Example: g.ornek, MovedTo: tasinanlar[k], Removed: kaldirilanlar[k]}
		for _, e := range append(append([]string(nil), g.eski...), g.yabanci...) {
			if e == "" || strings.IndexFunc(e, unicode.IsSpace) >= 0 {
				hata("%s: eski ya da yabancı ad boş olamaz ve boşluk içeremez (%q)", k, e)
			}
			if e == g.ad || asciiBicim(e) == asciiBicim(g.ad) {
				hata("%s: eski ya da yabancı ad %q adın kendisi (ya da ASCII biçimi) olamaz", k, e)
			}
		}
		d.girdiler[k] = gr
		enBuyuk[int(k)/1000-1] = max(enBuyuk[int(k)/1000-1], k.Number())
		if gr.Dead() {
			d.olu = append(d.olu, k)
			continue
		}
		if IsCodeForm(g.ad) {
			// Kod biçimli adlar ayrılmıştır (2.3a); yalnızca adı yeniden kullanılmış ölü bir kodun
			// ipucusuz adı olabilir (bkz. docs/spec/17_kodlar.md, Ölü kodlar).
			hata("%s: ad %q kod biçiminde (büyük harf ve üç rakam); bu biçim ayrılmıştır", k, g.ad)
			continue
		}
		d.kodlar = append(d.kodlar, k)
		alanBoyu[k.Namespace()] += 2
	}
	sort.Slice(d.kodlar, func(i, j int) bool { return d.kodlar[i] < d.kodlar[j] })
	sort.Slice(d.olu, func(i, j int) bool { return d.olu[i] < d.olu[j] })
	for i, n := range enBuyuk {
		if n > 0 {
			d.adDizi[i] = make([]string, n+1)
		}
	}

	// Ad alanı tabloları. Kod sırasıyla kurulur ki hata mesajları kararlı olsun.
	for i := range d.adlar {
		d.adlar[i] = make(map[string]Code, alanBoyu[i])
		d.eskiler[i] = map[string][]eskiEslesme{}
		d.asil[i] = make(map[string]string, alanBoyu[i])
	}
	// sahibi: ad alanında ASCII biçime göre adın sahibi kod ve Türkçe biçimi.
	type sahip struct {
		kod Code
		ad  string
	}
	sahibi := [alanSayisi]map[string]sahip{}
	for i := range sahibi {
		sahibi[i] = make(map[string]sahip, alanBoyu[i]/2)
	}
	for _, k := range d.kodlar {
		g := d.girdiler[k]
		al := k.Namespace()
		ad := g.Name
		ascii := asciiBicim(ad)
		if o, ok := sahibi[al][ascii]; ok {
			hata("%s %q ile %s %q aynı ad alanında (%s) çakışıyor (ASCII biçimleri: %q)", o.kod, o.ad, k, ad, al, ascii)
			continue
		}
		sahibi[al][ascii] = sahip{k, ad}
		yaz := []string{ad}
		if AcceptASCII && ascii != ad {
			yaz = append(yaz, ascii)
		}
		d.asil[al][ad], d.asil[al][ascii] = ad, ad
		d.yazimlar[k] = yaz
		d.adDizi[int(k)/1000-1][k.Number()] = g.Name
		for _, y := range yaz {
			d.adlar[al][y] = k
		}
	}
	// Eski ve yabancı adlar: kabul edilmez, hata mesajında asıl adı önerir (EskiBul, Oneri).
	for _, k := range d.kodlar {
		g := d.girdiler[k]
		al := k.Namespace()
		for _, e := range append(append([]string(nil), g.Old...), g.Foreign...) {
			yazimlar := []string{e}
			if a := asciiBicim(e); a != e {
				yazimlar = append(yazimlar, a)
			}
			for i, ad := range yazimlar {
				if o, ok := sahibi[al][asciiBicim(ad)]; ok && (ad == o.ad || ad == asciiBicim(o.ad)) {
					hata("%s'in eski adı %q, %s %q adıyla (ya da ASCII biçimiyle) çakışıyor", k, ad, o.kod, o.ad)
					continue
				}
				for _, o := range d.eskiler[al][ad] {
					if o.kod != k {
						hata("eski ad %q hem %s hem %s için kayıtlı; ipucu belirsiz olur", ad, o.kod, k)
					}
				}
				d.eskiler[al][ad] = append(d.eskiler[al][ad], eskiEslesme{kod: k, ascii: i == 1})
			}
		}
	}
	// Taşınmış kodlar: adları (ve eski adları) artık kabul edilmez; yalnızca hata
	// mesajında yeni biçimi önermek için tutulur. Canlı bir adla ya da eski bir adla
	// çakışırlarsa ipucu belirsiz olurdu.
	for i := range d.oluAd {
		d.oluAd[i] = map[string]Code{}
	}
	for _, k := range d.olu {
		g := d.girdiler[k]
		al := k.Namespace()
		d.adDizi[int(k)/1000-1][k.Number()] = g.Name
		tur := "kaldırılmış"
		if g.MovedTo != None {
			tur = "taşınmış"
		}
		if h, ok := d.girdiler[g.MovedTo]; g.MovedTo != None && (!ok || h.Dead()) {
			hata("%s %s'e taşındı, ama %s dil dosyasında canlı bir kod değil", k, g.MovedTo, g.MovedTo)
		}
		adlar := []string{}
		for _, ad := range append([]string{g.Name}, g.Old...) {
			adlar = append(adlar, ad)
			if a := asciiBicim(ad); a != ad {
				adlar = append(adlar, a)
			}
		}
		for _, ad := range adlar {
			if o, ok := sahibi[al][asciiBicim(ad)]; ok && (ad == o.ad || ad == asciiBicim(o.ad)) {
				hata("%s %s'in adı %q, %s %q adıyla (ya da ASCII biçimiyle) çakışıyor", tur, k, ad, o.kod, o.ad)
				continue
			}
			if e := d.eskiler[al][ad]; len(e) > 0 {
				hata("%s %s'in adı %q, %s'in eski adıyla çakışıyor; ipucu belirsiz olur", tur, k, ad, e[0].kod)
				continue
			}
			if o, ok := d.oluAd[al][ad]; ok && o != k {
				hata("%q hem %s hem %s kodlarının (taşınmış ya da kaldırılmış) adı; ipucu belirsiz olur", ad, o, k)
				continue
			}
			d.oluAd[al][ad] = k
		}
	}
	if len(hatalar) > 0 {
		return nil, errors.New("kod listesi " + kodu + ":\n  " + strings.Join(hatalar, "\n  "))
	}
	return d, nil
}

// Codes returns every live code in the language in ascending order (moved codes excluded;
// see MovedCodes).
func (d *Language) Codes() []Code { return append([]Code(nil), d.kodlar...) }

// DeadCodes returns the moved or removed codes in ascending order. Their names are not
// accepted; Name and Entry still work for them (for error messages).
func (d *Language) DeadCodes() []Code { return append([]Code(nil), d.olu...) }

// MovedCodes returns, in ascending order, the codes whose job moved to another code.
func (d *Language) MovedCodes() []Code {
	return d.oluSuz(func(g *Entry) bool { return g.MovedTo != None })
}

// RemovedCodes returns, in ascending order, the codes removed with no replacement.
func (d *Language) RemovedCodes() []Code { return d.oluSuz(func(g *Entry) bool { return g.Removed }) }

func (d *Language) oluSuz(uygun func(*Entry) bool) []Code {
	var out []Code
	for _, k := range d.olu {
		if uygun(d.girdiler[k]) {
			out = append(out, k)
		}
	}
	return out
}

// FindDead returns the code when ad is, in the given namespace, the name or an old name of a
// moved or removed code, or the ASCII form of either (the new code, if any, is
// Entry(k).MovedTo).
func (d *Language) FindDead(al Namespace, ad string) (Code, bool) {
	if al >= alanSayisi {
		return None, false
	}
	k, ok := d.oluAd[al][ad]
	return k, ok
}

// Entry returns the code's line in the language file.
func (d *Language) Entry(k Code) (Entry, bool) {
	g, ok := d.girdiler[k]
	if !ok {
		return Entry{}, false
	}
	return *g, true
}

// Name is the code's name in this language (an array index, no map lookup). A code missing
// from the language file is a developer error and panics (codes are constants in Go; if a
// line was deleted from the file the program stops at startup).
func (d *Language) Name(k Code) string {
	if f, n := int(k)/1000-1, k.Number(); f >= 0 && f < len(d.adDizi) && n < len(d.adDizi[f]) {
		if a := d.adDizi[f][n]; a != "" {
			return a
		}
	}
	panic(fmt.Sprintf("kod listesi %s: %s kodu dil dosyasında yok", d.Code, k))
}

// Spellings returns every accepted spelling of the code. Since every operation has one
// spelling this is just the name (if AcceptASCII were on, the ASCII form would follow). The
// returned slice must not be modified.
func (d *Language) Spellings(k Code) []string {
	y, ok := d.yazimlar[k]
	if !ok {
		panic(fmt.Sprintf("kod listesi %s: %s kodu dil dosyasında yok", d.Code, k))
	}
	return y
}

// Find returns the code of a spelling (the name or an accepted ASCII form) in the given
// namespace.
func (d *Language) Find(al Namespace, yazim string) (Code, bool) {
	if al >= alanSayisi {
		return None, false
	}
	k, ok := d.adlar[al][yazim]
	return k, ok
}

// FindOld returns the replacing code when ad is a removed (old) name in the given namespace;
// if uygun is not nil only codes with uygun(code) true are considered. ascii reports that ad
// is the derived ASCII form of an old name (e.g. islev); a hint may then suggest the ASCII
// form of the new name.
func (d *Language) FindOld(al Namespace, ad string, uygun func(Code) bool) (kod Code, ascii bool, ok bool) {
	if al >= alanSayisi {
		return None, false, false
	}
	for _, e := range d.eskiler[al][ad] {
		if uygun == nil || uygun(e.kod) {
			return e.kod, e.ascii, true
		}
	}
	return None, false, false
}

// CanonicalNameIn returns the real Turkish form of a name (written in Turkish or ASCII) in
// the given namespace: CanonicalNameIn(NamespaceGlobal, "yazdir") → "yazdır". ok is false
// when the name is not in that namespace. The same ASCII spelling can belong to different
// names in different namespaces ("tur" is the old name of G201 tip among global names and
// the ASCII form of Y402 tür among members), so it is always asked with a namespace.
func (d *Language) CanonicalNameIn(al Namespace, ad string) (string, bool) {
	if al >= alanSayisi {
		return "", false
	}
	a, ok := d.asil[al][ad]
	return a, ok
}

// Suggest returns the code to suggest for a spelling that is not valid in the given
// namespace: when the spelling is the ASCII-folded form of a name there (yazdir → yazdır),
// or a removed (old) name, or its ASCII form (işlev, islev → fonksiyon). If uygun is not nil
// only codes with uygun(code) true are suggested. The suggestion is always the code's name
// (its Turkish form); the ASCII form is never suggested. There is no suggestion when the
// spelling is a valid name in the namespace.
func (d *Language) Suggest(al Namespace, yazim string, uygun func(Code) bool) (Code, bool) {
	if al >= alanSayisi {
		return None, false
	}
	if _, gecerli := d.adlar[al][yazim]; gecerli {
		return None, false
	}
	if ad, ok := d.asil[al][yazim]; ok && ad != yazim {
		if k, ok := d.adlar[al][ad]; ok && (uygun == nil || uygun(k)) {
			return k, true
		}
	}
	if k, _, ok := d.FindOld(al, yazim, uygun); ok {
		return k, true
	}
	return None, false
}

// etkin programın kullandığı dildir (LanguageEnvVar; varsayılan Türkçe).
var etkin = func() *Language {
	kodu := os.Getenv(LanguageEnvVar)
	if kodu == "" {
		kodu = "tr"
	}
	d, err := dilKur(kodu)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hata: %s=%s: %v\n", LanguageEnvVar, kodu, err)
		os.Exit(2)
	}
	return d
}()

// LoadLanguage builds the code list of an embedded writing language; for the active language
// it returns the one already built.
func LoadLanguage(kodu string) (*Language, error) {
	if etkin != nil && etkin.Code == kodu {
		return etkin, nil
	}
	return dilKur(kodu)
}

func dilKur(kodu string) (*Language, error) {
	veri, ok := diller[kodu]
	if !ok {
		return nil, fmt.Errorf("'%s' yazım dili yok; var olanlar: %s", kodu, strings.Join(Languages(), ", "))
	}
	return Load(kodu, veri)
}

// Languages returns the codes of the embedded writing languages, sorted.
func Languages() []string {
	var r []string
	for k := range diller {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}

// Active returns the language the program uses.
func Active() *Language { return etkin }

// TRData returns the embedded Turkish language file (for tests and tools).
func TRData() []byte { return trJSON }

// Name, Spellings, Find, FindOld, Suggest, CanonicalNameIn, Codes and the other functions
// below are the same-named methods of the active language.

func Name(k Code) string                       { return etkin.Name(k) }
func Spellings(k Code) []string                { return etkin.Spellings(k) }
func Find(al Namespace, y string) (Code, bool) { return etkin.Find(al, y) }
func Codes() []Code                            { return etkin.Codes() }
func MovedCodes() []Code                       { return etkin.MovedCodes() }
func RemovedCodes() []Code                     { return etkin.RemovedCodes() }
func DeadCodes() []Code                        { return etkin.DeadCodes() }
func FindDead(al Namespace, ad string) (Code, bool) {
	return etkin.FindDead(al, ad)
}

// Removed reports whether k was removed from the language with no replacement
// (definitions.json, "kaldırıldı"); it reads the generated kaldirilanlar table.
func Removed(k Code) bool { return kaldirilanlar[k] }

// MovedTo returns the new code when k was removed and its job moved to another code (another
// form: from function to method or back). This is language independent (definitions.json,
// "taşındı"); it reads the generated tasinanlar table.
func MovedTo(k Code) (Code, bool) {
	y, ok := tasinanlar[k]
	return y, ok
}
func CanonicalNameIn(al Namespace, ad string) (string, bool) {
	return etkin.CanonicalNameIn(al, ad)
}
func FindOld(al Namespace, ad string, uygun func(Code) bool) (Code, bool, bool) {
	return etkin.FindOld(al, ad, uygun)
}
func Suggest(al Namespace, yazim string, uygun func(Code) bool) (Code, bool) {
	return etkin.Suggest(al, yazim, uygun)
}

// MemberCode returns the code of a method or property name (x.name), or None when it is not
// a member. The compiler calls it once at compile time; at run time only codes are compared.
func MemberCode(ad string) Code {
	k, _ := etkin.adlar[NamespaceMember][ad]
	return k
}

// Names returns the names of the codes in the same order (for suggestion lists).
func Names(kodlar ...Code) []string {
	out := make([]string, len(kodlar))
	for i, k := range kodlar {
		out[i] = etkin.Name(k)
	}
	return out
}
