package codes

import (
	_ "embed"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

//go:embed tr.json
var trJSON []byte

//go:embed en.json
var enJSON []byte

// languages ikiliye gömülü yazım dilleridir (ISO 639-1 kodu → dil dosyası). Diller yalnız
// görüntüdür: iletiler, yardım ve kodlu dosyanın salt okunur görünümü (docs/spec/17_kodlar.md, 17.7);
// kod kimliklerle yazılır.
var languages = map[string][]byte{"tr": trJSON, "en": enJSON}

// LanguageEnvVar selects the process's active writing language (empty: tr). The lexer's word
// tables are built once from the active language while packages initialize, so the language
// can be chosen only at process start.
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
	// Foreign holds names familiar from other languages (or libraries) (lr, seed, axis):
	// they are not accepted; the error message only suggests Name. Every operation has one
	// name; a synonym or a second spelling is not accepted. Language-specific: each file
	// records its own readers' habits.
	Foreign []string
	// Family is a short description for people (the group; for members, the types).
	Family string
	// Description is one or two sentences on what the code does (docs/codes.md is generated
	// from it; the example is language-independent, Definition.Example).
	Description string
}

// foreignMatch bir yabancı adın hangi koda ait olduğu ve adın, yabancı bir adın türetilmiş
// ASCII biçimi olup olmadığıdır.
type foreignMatch struct {
	code  Code
	ascii bool
}

// Language is the code list built from a language file (tr.json). It never changes after it
// is built and is safe for concurrent reads.
type Language struct {
	// Code is the language's ISO 639-1 code ("tr"; with a region when needed: "pt-BR").
	Code string

	entries map[Code]*Entry
	// nameArr kod → addır; Name çalışma anında harita araması yapmasın diye dizi.
	nameArr      []string
	codeList     []Code // artan sırada
	spellings    map[Code][]string
	names        [namespaceCount]map[string]Code
	foreignNames [namespaceCount]map[string][]foreignMatch
	canonical    [namespaceCount]map[string]string // ad alanı başına ad ve ASCII biçimi → ad
}

// Load reads and validates a language file. On format errors, duplicate codes or clashing
// names in a namespace (in Turkish or ASCII form, foreign names included) it returns one error
// listing all of them. The codes with a common name (definitions.json "common_name") get it as
// their name in this language; naming such a code in the language file is an error.
func Load(langCode string, data []byte) (*Language, error) {
	return load(langCode, data, commonNames())
}

// load, ortak adları verilmiş Load'dur (testler yapay ortak adlar verir).
func load(langCode string, data []byte, common map[Code]string) (*Language, error) {
	var errs []string
	errorf := func(form string, a ...any) { errs = append(errs, fmt.Sprintf(form, a...)) }

	raw, err := parseFile(data)
	if err != nil {
		return nil, messages.Internalf("language list %s: the file could not be read (it must be a JSON object { \"TR_fn_print\": { \"name\": ... }, ... }): %v", langCode, err)
	}
	raw = withCommonNames(raw, common, errorf)
	familiar := familiarNames()

	d := &Language{Code: langCode, entries: make(map[Code]*Entry, len(raw)), spellings: make(map[Code][]string, len(raw))}
	var nsCount [namespaceCount]int // ad alanı başına yazım sayısı (haritaların boyu için)
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
			errorf("%s: class %q is not known", k, k.Class())
			continue
		}
		if g.name == "" || strings.IndexFunc(g.name, unicode.IsSpace) >= 0 {
			errorf("%s: the name must not be empty or contain spaces (%q)", k, g.name)
			continue
		}
		gr := &Entry{Code: k, Name: g.name, Family: g.family, Description: g.description}
		// Tanıdık adlar dilden bağımsızdır (definitions.json "familiar"); bu dilin kendi adına eşit
		// olan (İngilizcede max) bu dilde tanıdık ad değildir.
		for _, e := range familiar[k] {
			if e == "" || strings.IndexFunc(e, unicode.IsSpace) >= 0 {
				errorf("%s: a familiar name must not be empty or contain spaces (%q)", k, e)
				continue
			}
			if e != g.name && turkish.ASCII(e) != turkish.ASCII(g.name) {
				gr.Foreign = append(gr.Foreign, e)
			}
		}
		d.entries[k] = gr
		if IsReservedName(g.name) && k.Namespace() != NamespaceEnvironment {
			// TR_ ile başlayan adlar kimliklere ayrılmıştır (2.3a); ortam değişkenleri programın adı değildir.
			errorf("%s: the name %q starts with %s; such names are reserved for identities", k, g.name, IdentityPrefix)
			continue
		}
		d.codeList = append(d.codeList, k)
		nsCount[k.Namespace()] += 2
	}
	sort.Slice(d.codeList, func(i, j int) bool { return d.codeList[i] < d.codeList[j] })
	d.nameArr = make([]string, len(identities))

	// Ad alanı tabloları. Kod sırasıyla kurulur ki hata mesajları kararlı olsun.
	for i := range d.names {
		d.names[i] = make(map[string]Code, nsCount[i])
		d.foreignNames[i] = map[string][]foreignMatch{}
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
		ascii := turkish.ASCII(name)
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
		d.nameArr[k] = g.Name
		for _, y := range write {
			d.names[ns][y] = k
		}
	}
	// Yabancı adlar: kabul edilmez, hata mesajında asıl adı önerir (FindForeign, Suggest).
	for _, k := range d.codeList {
		g := d.entries[k]
		ns := k.Namespace()
		for _, e := range g.Foreign {
			spellings := []string{e}
			if a := turkish.ASCII(e); a != e {
				spellings = append(spellings, a)
			}
			for i, name := range spellings {
				if o, ok := owners[ns][turkish.ASCII(name)]; ok && (name == o.name || name == turkish.ASCII(o.name)) {
					continue // bu dilde başka bir kodun adı: o dilde tanıdık ad değil, kendi adıdır
				}
				for _, o := range d.foreignNames[ns][name] {
					if o.code != k {
						errorf("the foreign name %q is registered for both %s and %s; the hint would be ambiguous", name, o.code, k)
					}
				}
				d.foreignNames[ns][name] = append(d.foreignNames[ns][name], foreignMatch{code: k, ascii: i == 1})
			}
		}
	}
	if len(errs) > 0 {
		return nil, messages.Internalf("code list %s:\n  %s", langCode, strings.Join(errs, "\n  "))
	}
	return d, nil
}

// withCommonNames her ortak adı, hiçbir denetimden önce, kodun ham satırına bu dilin adı olarak
// yazar (dosyada satırı yoksa yeni satır açar): ortak ad dilin kendi adlarıyla aynı denetimlerden
// geçer (boş ad, yabancı ad, TR_ öneki ve ortam değişkeni istisnası, çakışma). Ortak adlı koda dil
// dosyasında ad vermek hatadır.
func withCommonNames(raw []rawEntry, common map[Code]string, errorf func(string, ...any)) []rawEntry {
	seen := map[Code]bool{}
	for i := range raw {
		k, err := Parse(raw[i].key)
		name, ok := common[k]
		if err != nil || !ok {
			continue
		}
		seen[k] = true
		if raw[i].name != "" {
			errorf("%s has the common name %q (definitions.json common_name); the language file must not name it (%q)", k, name, raw[i].name)
			continue
		}
		raw[i].name = name
	}
	added := make([]Code, 0, len(common))
	for k := range common {
		if !seen[k] {
			added = append(added, k)
		}
	}
	sort.Slice(added, func(i, j int) bool { return added[i] < added[j] })
	for _, k := range added {
		raw = append(raw, rawEntry{key: k.String(), name: common[k]})
	}
	return raw
}

// Codes returns every code in the language in ascending order.
func (d *Language) Codes() []Code { return append([]Code(nil), d.codeList...) }

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
	if int(k) < len(d.nameArr) && d.nameArr[k] != "" {
		return d.nameArr[k]
	}
	panic(fmt.Sprintf("code list %s: code %s is not in the language file", d.Code, k))
}

// Spellings returns every accepted spelling of the code. Since every operation has one
// spelling this is just the name (if AcceptASCII were on, the ASCII form would follow). The
// returned slice must not be modified.
func (d *Language) Spellings(k Code) []string {
	y, ok := d.spellings[k]
	if !ok {
		panic(fmt.Sprintf("code list %s: code %s is not in the language file", d.Code, k))
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

// FindForeign returns the code to suggest when name is a foreign name (lr, axis) in the given
// namespace; if accept is not nil only codes with accept(code) true are considered. ascii
// reports that name is the derived ASCII form of a foreign name.
func (d *Language) FindForeign(ns Namespace, name string, accept func(Code) bool) (code Code, ascii bool, ok bool) {
	if ns >= namespaceCount {
		return None, false, false
	}
	for _, e := range d.foreignNames[ns][name] {
		if accept == nil || accept(e.code) {
			return e.code, e.ascii, true
		}
	}
	return None, false, false
}

// CanonicalNameIn returns the real Turkish form of a name (written in Turkish or ASCII) in
// the given namespace: CanonicalNameIn(NamespaceGlobal, "yazdir") → "yazdır". ok is false
// when the name is not in that namespace. The same ASCII spelling can belong to different
// names in different namespaces, so it is always asked with a namespace.
func (d *Language) CanonicalNameIn(ns Namespace, name string) (string, bool) {
	if ns >= namespaceCount {
		return "", false
	}
	a, ok := d.canonical[ns][name]
	return a, ok
}

// Suggest returns the code to suggest for a spelling that is not valid in the given
// namespace: when the spelling is the ASCII-folded form of a name there (yazdir → yazdır),
// or a foreign name, or its ASCII form (lr → öğrenme_oranı). If uygun is not nil
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
	if k, _, ok := d.FindForeign(ns, spelling, accept); ok {
		return k, true
	}
	return None, false
}

// LoadLanguage returns the code list of an embedded writing language. Each language is built
// once (lists are read-only): a running program asks for the language of every file it loads
// (17.1), and for the canonical and the display language it returns the one already built.
func LoadLanguage(langCode string) (*Language, error) {
	if langCode == SourceLanguage && activeErr != nil {
		return nil, activeErr
	}
	if active != nil && active.Code == langCode {
		return active, nil
	}
	if display != nil && display.Code == langCode {
		return display, nil
	}
	if l, ok := loaded.Load(langCode); ok {
		return l.(*Language), nil
	}
	l, err := buildLanguage(languages, langCode)
	if err != nil {
		return nil, err
	}
	stored, _ := loaded.LoadOrStore(langCode, l)
	return stored.(*Language), nil
}

// loaded LoadLanguage'ın kurduğu öbür dillerin kod listeleridir (dil kodu → *Language).
var loaded sync.Map

// Display returns the person's writing language (LanguageEnvVar): the language of diagnostics,
// help and command-line output, and of a coded file's view.
func Display() *Language { return display }

// DisplayName returns k's name in the display language; a code that is not written in program
// text (17.9: commands, subcommands, options, shell/debugger commands, environment variables)
// returns its identity name (CommandLineName), and a code the display language does not name
// keeps its canonical name.
func DisplayName(k Code) string {
	if _, ok := PlaceOf(k); !ok && k != None {
		return CommandLineName(k)
	}
	if g, ok := display.Entry(k); ok {
		return g.Name
	}
	return active.Name(k)
}

// OptionFlag returns the command-line form of an option code (17.9): "--"
// plus its identity name with underscores as hyphens (--lang, --go-profile).
func OptionFlag(k Code) string {
	return "--" + strings.ReplaceAll(DisplayName(k), "_", "-")
}

func buildLanguage(langs map[string][]byte, langCode string) (*Language, error) {
	data, ok := langs[langCode]
	if !ok {
		return nil, messages.New(messages.H2508, langCode, strings.Join(Languages(), ", "))
	}
	return Load(langCode, data)
}

// FindCommand finds a tr command or a shell command (cmd, repl) by its command-line identity
// name (17.9: run, index, exit). Display-language names are rejected and suggested via
// SuggestCommandLine.
func FindCommand(name string) (Code, bool) { return findCommandLine(NamespaceCommand, name) }

// FindSubcommand finds a subcommand of 'tr index' (sub) by its command-line identity name (17.9),
// as FindCommand does. The names are valid only after 'tr index'.
func FindSubcommand(name string) (Code, bool) { return findCommandLine(NamespaceSubcommand, name) }

// FindOption finds a command-line option (opt) by its identity name without the leading "--"
// (17.9), as FindCommand does.
func FindOption(name string) (Code, bool) { return findCommandLine(NamespaceOption, name) }

// FindDebuggerCommand finds a debugger command (dbg) by its command-line identity name (17.9),
// as FindCommand does.
func FindDebuggerCommand(name string) (Code, bool) {
	return findCommandLine(NamespaceDebugger, name)
}

// SuggestCommandLine returns the command-line code to suggest when spelling is a display-language
// name (çalıştır → TR_cmd_run, tanım → TR_sub_definition, dil → TR_opt_lang, çıkış → TR_repl_exit,
// adım → TR_dbg_step, TR_DIL → TR_env_LANG) or its ASCII/familiar form (calistir, tanim, cikis, adim).
func SuggestCommandLine(ns Namespace, spelling string, accept func(Code) bool) (Code, bool) {
	if k, ok := findCommandLine(ns, spelling); ok && (accept == nil || accept(k)) {
		return None, false
	}
	if k, ok := findInLanguages(ns, spelling); ok && (accept == nil || accept(k)) {
		return k, true
	}
	return Display().Suggest(ns, spelling, accept)
}

// findCommandLine adı ad alanındaki kodların yalnız kimlik adında (CommandLineName, 17.9) arar.
func findCommandLine(ns Namespace, name string) (Code, bool) {
	for _, k := range active.Codes() {
		if k.Namespace() == ns && CommandLineName(k) == name {
			return k, true
		}
	}
	return None, false
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

// WritingNames returns k's names in every embedded writing language, sorted by language code and
// without repeats: no language's name comes first.
func WritingNames(k Code) []string {
	var names []string
	for _, lc := range Languages() {
		if l, err := LoadLanguage(lc); err == nil {
			if g, ok := l.Entry(k); ok && !slices.Contains(names, g.Name) {
				names = append(names, g.Name)
			}
		}
	}
	return names
}

// Active programın okunduğu ve çalıştığı kimlik listesidir (IdentityLanguage): hiçbir yazım dili
// değildir; kişinin yazım dili Display'dir.
func Active() *Language { return active }

// Source Türkçe yazım dilidir. Bugün başlıksız metnin dili (17.7) ve Türkçe kalan metinlerin
// (Inspect, metin araçlarının Türkçe görünümü) dilidir; bunlar Türkçeye ayrıcalık tanıyan geçici
// istisnalardır. Program içeride kimliklerle çalışır (Active).
func Source() *Language {
	if l, err := LoadLanguage(SourceLanguage); err == nil {
		return l
	}
	return active
}

// TRData returns the embedded Turkish language file (for tests and tools).
func TRData() []byte { return trJSON }

// Name, Spellings, Find, FindForeign, Suggest, CanonicalNameIn, Codes and the other functions
// below are the same-named methods of the active language.

func Name(k Code) string                       { return active.Name(k) }
func Spellings(k Code) []string                { return active.Spellings(k) }
func Find(ns Namespace, y string) (Code, bool) { return active.Find(ns, y) }
func Codes() []Code                            { return active.Codes() }

// CanonicalNameIn, FindForeign and Suggest are for a text whose writing language is not known:
// every embedded writing language is asked in language-code order and the first answer wins (no
// language comes first). A caller that knows the text's language asks that language's method.
func CanonicalNameIn(ns Namespace, name string) (string, bool) {
	for _, lc := range Languages() {
		if l, err := LoadLanguage(lc); err == nil {
			if a, ok := l.CanonicalNameIn(ns, name); ok {
				return a, true
			}
		}
	}
	return "", false
}
func FindForeign(ns Namespace, name string, accept func(Code) bool) (Code, bool, bool) {
	for _, lc := range Languages() {
		if l, err := LoadLanguage(lc); err == nil {
			if k, ascii, ok := l.FindForeign(ns, name, accept); ok {
				return k, ascii, true
			}
		}
	}
	return None, false, false
}
func Suggest(ns Namespace, spelling string, accept func(Code) bool) (Code, bool) {
	for _, lc := range Languages() {
		if l, err := LoadLanguage(lc); err == nil {
			if k, ok := l.Suggest(ns, spelling, accept); ok {
				return k, true
			}
		}
	}
	return None, false
}

// MemberCode returns the code of a method or property name (x.name), or None when it is not
// a member. The compiler calls it once at compile time; at run time only codes are compared.
func MemberCode(name string) Code {
	return active.names[NamespaceMember][name]
}

// Names returns the names of the codes in the same order (for suggestion lists).
func Names(codeList ...Code) []string {
	out := make([]string, len(codeList))
	for i, k := range codeList {
		out[i] = active.Name(k)
	}
	return out
}
