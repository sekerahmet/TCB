package codes

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

//go:embed tr.json
var trJSON []byte

//go:embed en.json
var enJSON []byte

// languages ikiliye gömülü yazım dilleridir (ISO 639-1 kodu → dil dosyası). Türkçe dilin asıl
// dilidir: kodsuz (düz) yazılmış dosyalar Türkçedir; öbür diller kodlu dosyaların gösterimi
// ve o dilde yazılmış metnin kodlanması içindir (docs/spec/17_kodlar.md, 17.6).
var languages = map[string][]byte{"tr": trJSON, "en": enJSON}

// LanguageEnvVar selects the process's active writing language (empty: tr). The lexer's word
// tables are built once from the active language while packages initialize, so the language
// can be chosen only at process start; 'tr kodla --dil en' re-executes itself with this
// variable.
const LanguageEnvVar = messages.LanguageEnvVar

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
	// snippet (docs/codes.md is generated from them).
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

// oldMatch bir eski adın hangi koda ait olduğu ve adın, eski bir adın türetilmiş
// ASCII biçimi olup olmadığıdır.
type oldMatch struct {
	code  Code
	ascii bool
}

// Language is the code list built from a language file (tr.json). It never changes after it
// is built and is safe for concurrent reads.
type Language struct {
	// Code is the language's ISO 639-1 code ("tr"; with a region when needed: "pt-BR").
	Code string

	entries map[Code]*Entry
	// nameArr aile başına sayı → addır; Ad çalışma anında da harita araması yapmasın diye dizi.
	nameArr   [len(familyLetters)][]string
	codeList  []Code // artan sırada
	spellings map[Code][]string
	names     [namespaceCount]map[string]Code
	oldNames  [namespaceCount]map[string][]oldMatch
	canonical [namespaceCount]map[string]string // ad alanı başına ad ve ASCII biçimi → ad
	// dead taşınmış ya da kaldırılmış kodlardır (artan sırada); oluAd ad alanı başına böyle
	// bir kodun adı, eski adları ve bunların ASCII biçimleri → kod.
	dead      []Code
	deadNames [namespaceCount]map[string]Code
}

// Load reads and validates a language file. On format errors, duplicate codes or clashing
// names in a namespace (in Turkish or ASCII form, old names included) it returns one error
// listing all of them.
func Load(langCode string, data []byte) (*Language, error) {
	var errs []string
	errorf := func(form string, a ...any) { errs = append(errs, fmt.Sprintf(form, a...)) }

	raw, err := parseFile(data)
	if err != nil {
		return nil, internalf("language list %s: the file could not be read (it must be a JSON object { \"G100\": { \"ad\": ... }, ... }): %v", langCode, err)
	}

	d := &Language{Code: langCode, entries: make(map[Code]*Entry, len(raw)), spellings: make(map[Code][]string, len(raw))}
	var nsCount [namespaceCount]int // ad alanı başına yazım sayısı (haritaların boyu için)
	var largest [len(familyLetters)]int
	for _, g := range raw {
		k, err := Parse(g.key)
		if err != nil {
			errorf("%v", err)
			continue
		}
		if _, var_ := d.entries[k]; var_ {
			errorf("%s is defined twice", k)
			continue
		}
		if k.Namespace() == NamespaceNone {
			errorf("%s: family %c is not used yet", k, k.Letter())
			continue
		}
		if g.name == "" || strings.IndexFunc(g.name, unicode.IsSpace) >= 0 {
			errorf("%s: the name must not be empty or contain spaces (%q)", k, g.name)
			continue
		}
		gr := &Entry{Code: k, Name: g.name, Old: g.old, Foreign: g.foreign, Family: g.family,
			Description: g.description, Example: g.example, MovedTo: movedMap[k], Removed: removedSet[k]}
		for _, e := range append(append([]string(nil), g.old...), g.foreign...) {
			if e == "" || strings.IndexFunc(e, unicode.IsSpace) >= 0 {
				errorf("%s: an old or foreign name must not be empty or contain spaces (%q)", k, e)
			}
			if e == g.name || asciiForm(e) == asciiForm(g.name) {
				errorf("%s: the old or foreign name %q must not be the name itself (or its ASCII form)", k, e)
			}
		}
		d.entries[k] = gr
		largest[int(k)/1000-1] = max(largest[int(k)/1000-1], k.Number())
		if gr.Dead() {
			d.dead = append(d.dead, k)
			continue
		}
		if IsCodeForm(g.name) {
			// Kod biçimli adlar ayrılmıştır (2.3a); yalnızca adı yeniden kullanılmış ölü bir kodun
			// ipucusuz adı olabilir (bkz. docs/spec/17_kodlar.md, Ölü kodlar).
			errorf("%s: the name %q has the code form (a capital letter and three digits); this form is reserved", k, g.name)
			continue
		}
		d.codeList = append(d.codeList, k)
		nsCount[k.Namespace()] += 2
	}
	sort.Slice(d.codeList, func(i, j int) bool { return d.codeList[i] < d.codeList[j] })
	sort.Slice(d.dead, func(i, j int) bool { return d.dead[i] < d.dead[j] })
	for i, n := range largest {
		if n > 0 {
			d.nameArr[i] = make([]string, n+1)
		}
	}

	// Ad alanı tabloları. Kod sırasıyla kurulur ki hata mesajları kararlı olsun.
	for i := range d.names {
		d.names[i] = make(map[string]Code, nsCount[i])
		d.oldNames[i] = map[string][]oldMatch{}
		d.canonical[i] = make(map[string]string, nsCount[i])
	}
	// sahibi: ad alanında ASCII biçime göre adın sahibi kod ve Türkçe biçimi.
	type owner struct {
		code Code
		name string
	}
	owners := [namespaceCount]map[string]owner{}
	for i := range owners {
		owners[i] = make(map[string]owner, nsCount[i]/2)
	}
	for _, k := range d.codeList {
		g := d.entries[k]
		ns := k.Namespace()
		name := g.Name
		ascii := asciiForm(name)
		if o, ok := owners[ns][ascii]; ok {
			errorf("%s %q and %s %q clash in the same namespace (%s) (ASCII forms: %q)", o.code, o.name, k, name, ns, ascii)
			continue
		}
		owners[ns][ascii] = owner{k, name}
		write := []string{name}
		if AcceptASCII && ascii != name {
			write = append(write, ascii)
		}
		d.canonical[ns][name], d.canonical[ns][ascii] = name, name
		d.spellings[k] = write
		d.nameArr[int(k)/1000-1][k.Number()] = g.Name
		for _, y := range write {
			d.names[ns][y] = k
		}
	}
	// Eski ve yabancı adlar: kabul edilmez, hata mesajında asıl adı önerir (EskiBul, Oneri).
	for _, k := range d.codeList {
		g := d.entries[k]
		ns := k.Namespace()
		for _, e := range append(append([]string(nil), g.Old...), g.Foreign...) {
			spellings := []string{e}
			if a := asciiForm(e); a != e {
				spellings = append(spellings, a)
			}
			for i, name := range spellings {
				if o, ok := owners[ns][asciiForm(name)]; ok && (name == o.name || name == asciiForm(o.name)) {
					errorf("the old name %q of %s clashes with %s %q (or its ASCII form)", name, k, o.code, o.name)
					continue
				}
				for _, o := range d.oldNames[ns][name] {
					if o.code != k {
						errorf("the old name %q is registered for both %s and %s; the hint would be ambiguous", name, o.code, k)
					}
				}
				d.oldNames[ns][name] = append(d.oldNames[ns][name], oldMatch{code: k, ascii: i == 1})
			}
		}
	}
	// Taşınmış kodlar: adları (ve eski adları) artık kabul edilmez; yalnızca hata
	// mesajında yeni biçimi önermek için tutulur. Canlı bir adla ya da eski bir adla
	// çakışırlarsa ipucu belirsiz olurdu.
	for i := range d.deadNames {
		d.deadNames[i] = map[string]Code{}
	}
	for _, k := range d.dead {
		g := d.entries[k]
		ns := k.Namespace()
		d.nameArr[int(k)/1000-1][k.Number()] = g.Name
		kind := "kaldırılmış"
		if g.MovedTo != None {
			kind = "taşınmış"
		}
		if h, ok := d.entries[g.MovedTo]; g.MovedTo != None && (!ok || h.Dead()) {
			errorf("%s moved to %s, but %s is not a live code in the language file", k, g.MovedTo, g.MovedTo)
		}
		names := []string{}
		for _, name := range append([]string{g.Name}, g.Old...) {
			names = append(names, name)
			if a := asciiForm(name); a != name {
				names = append(names, a)
			}
		}
		for _, name := range names {
			if o, ok := owners[ns][asciiForm(name)]; ok && (name == o.name || name == asciiForm(o.name)) {
				errorf("the name %q of %s %s clashes with %s %q (or its ASCII form)", name, kind, k, o.code, o.name)
				continue
			}
			if e := d.oldNames[ns][name]; len(e) > 0 {
				errorf("the name %q of %s %s clashes with the old name of %s; the hint would be ambiguous", name, kind, k, e[0].code)
				continue
			}
			if o, ok := d.deadNames[ns][name]; ok && o != k {
				errorf("%q is the (moved or removed) name of both %s and %s; the hint would be ambiguous", name, o, k)
				continue
			}
			d.deadNames[ns][name] = k
		}
	}
	if len(errs) > 0 {
		return nil, internalf("code list %s:\n  %s", langCode, strings.Join(errs, "\n  "))
	}
	return d, nil
}

// Codes returns every live code in the language in ascending order (moved codes excluded;
// see MovedCodes).
func (d *Language) Codes() []Code { return append([]Code(nil), d.codeList...) }

// DeadCodes returns the moved or removed codes in ascending order. Their names are not
// accepted; Name and Entry still work for them (for error messages).
func (d *Language) DeadCodes() []Code { return append([]Code(nil), d.dead...) }

// MovedCodes returns, in ascending order, the codes whose job moved to another code.
func (d *Language) MovedCodes() []Code {
	return d.deadWhere(func(g *Entry) bool { return g.MovedTo != None })
}

// RemovedCodes returns, in ascending order, the codes removed with no replacement.
func (d *Language) RemovedCodes() []Code {
	return d.deadWhere(func(g *Entry) bool { return g.Removed })
}

func (d *Language) deadWhere(accept func(*Entry) bool) []Code {
	var out []Code
	for _, k := range d.dead {
		if accept(d.entries[k]) {
			out = append(out, k)
		}
	}
	return out
}

// FindDead returns the code when ad is, in the given namespace, the name or an old name of a
// moved or removed code, or the ASCII form of either (the new code, if any, is
// Entry(k).MovedTo).
func (d *Language) FindDead(ns Namespace, name string) (Code, bool) {
	if ns >= namespaceCount {
		return None, false
	}
	k, ok := d.deadNames[ns][name]
	return k, ok
}

// Entry returns the code's line in the language file.
func (d *Language) Entry(k Code) (Entry, bool) {
	g, ok := d.entries[k]
	if !ok {
		return Entry{}, false
	}
	return *g, true
}

// Name is the code's name in this language (an array index, no map lookup). A code missing
// from the language file is a developer error and panics (codes are constants in Go; if a
// line was deleted from the file the program stops at startup).
func (d *Language) Name(k Code) string {
	if f, n := int(k)/1000-1, k.Number(); f >= 0 && f < len(d.nameArr) && n < len(d.nameArr[f]) {
		if a := d.nameArr[f][n]; a != "" {
			return a
		}
	}
	panic(fmt.Sprintf("kod listesi %s: %s kodu dil dosyasında yok", d.Code, k))
}

// Spellings returns every accepted spelling of the code. Since every operation has one
// spelling this is just the name (if AcceptASCII were on, the ASCII form would follow). The
// returned slice must not be modified.
func (d *Language) Spellings(k Code) []string {
	y, ok := d.spellings[k]
	if !ok {
		panic(fmt.Sprintf("kod listesi %s: %s kodu dil dosyasında yok", d.Code, k))
	}
	return y
}

// Find returns the code of a spelling (the name or an accepted ASCII form) in the given
// namespace.
func (d *Language) Find(ns Namespace, spelling string) (Code, bool) {
	if ns >= namespaceCount {
		return None, false
	}
	k, ok := d.names[ns][spelling]
	return k, ok
}

// FindOld returns the replacing code when ad is a removed (old) name in the given namespace;
// if uygun is not nil only codes with uygun(code) true are considered. ascii reports that ad
// is the derived ASCII form of an old name (e.g. islev); a hint may then suggest the ASCII
// form of the new name.
func (d *Language) FindOld(ns Namespace, name string, accept func(Code) bool) (code Code, ascii bool, ok bool) {
	if ns >= namespaceCount {
		return None, false, false
	}
	for _, e := range d.oldNames[ns][name] {
		if accept == nil || accept(e.code) {
			return e.code, e.ascii, true
		}
	}
	return None, false, false
}

// CanonicalNameIn returns the real Turkish form of a name (written in Turkish or ASCII) in
// the given namespace: CanonicalNameIn(NamespaceGlobal, "yazdir") → "yazdır". ok is false
// when the name is not in that namespace. The same ASCII spelling can belong to different
// names in different namespaces ("tur" is the old name of G201 tip among global names and
// the ASCII form of Y402 tür among members), so it is always asked with a namespace.
func (d *Language) CanonicalNameIn(ns Namespace, name string) (string, bool) {
	if ns >= namespaceCount {
		return "", false
	}
	a, ok := d.canonical[ns][name]
	return a, ok
}

// Suggest returns the code to suggest for a spelling that is not valid in the given
// namespace: when the spelling is the ASCII-folded form of a name there (yazdir → yazdır),
// or a removed (old) name, or its ASCII form (işlev, islev → fonksiyon). If uygun is not nil
// only codes with uygun(code) true are suggested. The suggestion is always the code's name
// (its Turkish form); the ASCII form is never suggested. There is no suggestion when the
// spelling is a valid name in the namespace.
func (d *Language) Suggest(ns Namespace, spelling string, accept func(Code) bool) (Code, bool) {
	if ns >= namespaceCount {
		return None, false
	}
	if _, valid := d.names[ns][spelling]; valid {
		return None, false
	}
	if name, ok := d.canonical[ns][spelling]; ok && name != spelling {
		if k, ok := d.names[ns][name]; ok && (accept == nil || accept(k)) {
			return k, true
		}
	}
	if k, _, ok := d.FindOld(ns, spelling, accept); ok {
		return k, true
	}
	return None, false
}

// active programın kullandığı dildir (LanguageEnvVar; varsayılan Türkçe). Değişken geçersiz
// bir dil seçiyorsa paket süreci kapatmaz (paketi kullanan dil sunucusu, testler ya da TR'yi
// gömen bir program da kapanırdı): Türkçeye düşülür ve hata LanguageError ile okunur; tr
// komutu başlarken onu bildirip çıkar.
var active = func() *Language {
	langCode := messages.ActiveLanguage()
	d, err := buildLanguage(langCode)
	if err != nil {
		activeErr = messages.New(messages.H2509, LanguageEnvVar, langCode, err)
		if d, err = buildLanguage("tr"); err != nil {
			panic(fmt.Sprintf("codes: gömülü Türkçe kod listesi kurulamadı: %v", err))
		}
	}
	return d
}()

// activeErr LanguageEnvVar'ın seçtiği dil kurulamadıysa hatasıdır (dil Türkçeye düşmüştür).
var activeErr error

// LanguageError reports why the language selected by LanguageEnvVar could not be built (the
// active language then fell back to Turkish); nil when the selection is valid.
func LanguageError() error { return activeErr }

// LoadLanguage builds the code list of an embedded writing language; for the active language
// it returns the one already built.
func LoadLanguage(langCode string) (*Language, error) {
	if active != nil && active.Code == langCode {
		return active, nil
	}
	return buildLanguage(langCode)
}

func buildLanguage(langCode string) (*Language, error) {
	data, ok := languages[langCode]
	if !ok {
		return nil, messages.New(messages.H2508, langCode, strings.Join(Languages(), ", "))
	}
	return Load(langCode, data)
}

// FindCommand finds a tr command or a shell command (K0xx, K2xx) by its name in any embedded
// writing language: the active language first, then the others. Command lines and scripts then
// work on every machine whatever TR_DIL is (unlike the language's source, a command line has no
// coded form).
func FindCommand(name string) (Code, bool) { return findInLanguages(NamespaceCommand, name) }

// FindOption finds a command-line option (K1xx) by its name without the leading "--", in any
// embedded writing language, as FindCommand does.
func FindOption(name string) (Code, bool) { return findInLanguages(NamespaceOption, name) }

// FindDebuggerCommand finds a debugger command (K3xx) by its name in any embedded writing
// language, as FindCommand does.
func FindDebuggerCommand(name string) (Code, bool) {
	return findInLanguages(NamespaceDebugger, name)
}

// FindRemovedCommandLine finds a removed or moved command-line name (a K code of namespace ns
// that is dead) in any embedded writing language, so the command line can name what replaced it.
func FindRemovedCommandLine(ns Namespace, name string) (Code, bool) {
	return searchLanguages(func(l *Language) (Code, bool) { return l.FindDead(ns, name) })
}

// findInLanguages adı önce etkin dilde, sonra öbür gömülü dillerde arar.
func findInLanguages(ns Namespace, name string) (Code, bool) {
	return searchLanguages(func(l *Language) (Code, bool) { return l.Find(ns, name) })
}

// searchLanguages find'ı önce etkin dilde, sonra öbür gömülü dillerde dener.
func searchLanguages(find func(*Language) (Code, bool)) (Code, bool) {
	if k, ok := find(active); ok {
		return k, true
	}
	for _, lc := range Languages() {
		if lc == active.Code {
			continue
		}
		if l, err := LoadLanguage(lc); err == nil {
			if k, ok := find(l); ok {
				return k, true
			}
		}
	}
	return None, false
}

// Languages returns the codes of the embedded writing languages, sorted.
func Languages() []string {
	var r []string
	for k := range languages {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}

// Active returns the language the program uses.
func Active() *Language { return active }

// TRData returns the embedded Turkish language file (for tests and tools).
func TRData() []byte { return trJSON }

// Name, Spellings, Find, FindOld, Suggest, CanonicalNameIn, Codes and the other functions
// below are the same-named methods of the active language.

func Name(k Code) string                       { return active.Name(k) }
func Spellings(k Code) []string                { return active.Spellings(k) }
func Find(ns Namespace, y string) (Code, bool) { return active.Find(ns, y) }
func Codes() []Code                            { return active.Codes() }
func MovedCodes() []Code                       { return active.MovedCodes() }
func RemovedCodes() []Code                     { return active.RemovedCodes() }
func DeadCodes() []Code                        { return active.DeadCodes() }
func FindDead(ns Namespace, name string) (Code, bool) {
	return active.FindDead(ns, name)
}

// Removed reports whether k was removed from the language with no replacement
// (definitions.json, "kaldırıldı"); it reads the generated kaldirilanlar table.
func Removed(k Code) bool { return removedSet[k] }

// MovedTo returns the new code when k was removed and its job moved to another code (another
// form: from function to method or back). This is language independent (definitions.json,
// "taşındı"); it reads the generated tasinanlar table.
func MovedTo(k Code) (Code, bool) {
	y, ok := movedMap[k]
	return y, ok
}
func CanonicalNameIn(ns Namespace, name string) (string, bool) {
	return active.CanonicalNameIn(ns, name)
}
func FindOld(ns Namespace, name string, accept func(Code) bool) (Code, bool, bool) {
	return active.FindOld(ns, name, accept)
}
func Suggest(ns Namespace, spelling string, accept func(Code) bool) (Code, bool) {
	return active.Suggest(ns, spelling, accept)
}

// MemberCode returns the code of a method or property name (x.name), or None when it is not
// a member. The compiler calls it once at compile time; at run time only codes are compared.
func MemberCode(name string) Code {
	k, _ := active.names[NamespaceMember][name]
	return k
}

// Names returns the names of the codes in the same order (for suggestion lists).
func Names(codeList ...Code) []string {
	out := make([]string, len(codeList))
	for i, k := range codeList {
		out[i] = active.Name(k)
	}
	return out
}
