package messages

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
)

// Şablon dili ICU MessageFormat'ın küçük bir alt kümesidir; çevirmen araçları bu biçimi tanır:
//
//	{ad}                         değer
//	{ad|q}                       Go'nun %q biçimiyle tırnaklı (denetim karakterleri kaçırılır)
//	{ad|acc} {ad|dat} {ad|loc}   hâl eki: dilin çekimcisi ekler (tr: 'f'yi, 'f'ye, 'f'de ...);
//	{ad|abl} {ad|gen}            çekimcisi olmayan dilde ad olduğu gibi kalır
//	{ad|list}                    liste değeri, dilin "ya da" kalıbıyla (OrList): a, b ya da c
//	{@ad} {@ad|dat}              ad yuvası: tanımın "slots" tablosundaki kitaplık adı (kimlik),
//	                             iletinin dilindeki adıyla; TR kaynağı adı geçirmez, katalog anar
//	{#ad}                        kod yuvası: aynı "slots" tablosundaki kimliğin programdaki yazımı
//	{ad, plural, one {..} other {..}}  sayıya göre biçim (CLDR çoğul kuralı); dalda (içindeki
//	                                   seçim dallarında da) # sayıdır, "=N" tam eşleşme (N
//	                                   sayının tek yazımı: =0, =1.5) kategoriden önce gelir
//	{ad, select, x {..} other {..}}    metin değerine göre dal; boş değer `empty` dalını seçer
//	                                   ({ad, select, empty {} other {.. {ad} ..}}: isteğe bağlı parça)
//	{{ }}                        düz süslü parantez (dal içinde '}' her zaman dalı kapatır)
//
// Go kodu sözcük çekimlemez ve sayıya bakıp sözcük seçmez: bunlar dilin işidir, şablondadır.
// Her parametre ya dalların dışında ya her dalda geçer: bir dalda unutulan değer o dal
// seçildiğinde kullanıcıya ulaşmazdı.

// segKind şablon parçasının türüdür.
type segKind uint8

const (
	segText     segKind = iota // düz metin
	segArg                     // {ad}, {ad|biçim}
	segPlural                  // {ad, plural, ...}
	segSelect                  // {ad, select, ...}
	segNumber                  // çoğul dalındaki #
	segSlot                    // {@ad}: kimliği iletinin dilindeki adıyla yazar
	segCodeSlot                // {#ad}: kimliği programdaki yazımıyla yazar
)

// segment bir ileti şablonunun parçasıdır.
type segment struct {
	kind     segKind
	text     string               // segText
	arg      int                  // parametrenin sırası
	mod      string               // segArg, segSlot: "", "q", "decimal", "list" ya da hâl adı
	branches map[string][]segment // segPlural, segSelect: dal anahtarı → dal
}

// caseMods şablonda geçerli hâl biçimleridir.
var caseMods = map[string]bool{"acc": true, "dat": true, "loc": true, "abl": true, "gen": true}

// decimalSeparators dillerin ondalık ayırıcısıdır (CLDR "decimal" sembolü: tr ",", en "."). Go
// sayıları noktayla yazar; {ad|decimal} süzgeci ve Decimal metindeki noktayı iletinin ya da
// görüntünün diline çevirir (ICU'nun {n, number}'ı gibi, ama yalnız ayırıcı: binlik ayırıcı yok).
var decimalSeparators = map[string]string{"tr": ",", "en": "."}

// localDecimal metindeki ondalık noktalarını lang dilinin ayırıcısıyla yazar.
func localDecimal(text, lang string) string {
	sep, ok := decimalSeparators[lang]
	if !ok || sep == "." {
		return text
	}
	return strings.ReplaceAll(text, ".", sep)
}

// Decimal writes the decimal points of a number's text (Go's "1.5", "2.3 GB") with the person's
// writing language's separator (TR_LANG: "1,5" in Turkish), for display that is not a message (a
// progress line, a profile table). In a message the template's {name|decimal} does the same in
// the message's language.
func Decimal(text string) string { return localDecimal(text, activeCatalog().Language) }

// pluralCategories CLDR çoğul kategorileridir.
var pluralCategories = map[string]bool{"zero": true, "one": true, "two": true, "few": true, "many": true, "other": true}

// pluralOperands CLDR çoğul kurallarının işlenenleridir (UTS #35, "Plural Operand Meanings"):
// n mutlak değer, i tam kısım, v görünen ondalık basamak sayısı, f o basamakların değeri.
// i tam sayı türünden kayıpsız gelir: 2^53 üstünde de son basamaklar (n%10 kuralları) doğrudur.
type pluralOperands struct {
	n    float64
	i, f uint64
	v    int
}

// pluralRules dilin CLDR çoğul kuralıdır (CLDR plurals.xml). Kuralı olmayan dilde (yeni bir dil
// eklenirken) her sayı "other"dır; yeni dilin kuralı buraya eklenir.
var pluralRules = map[string]func(o pluralOperands) string{
	// tr: one → n = 1 (1.0 da)
	"tr": func(o pluralOperands) string {
		if o.n == 1 {
			return "one"
		}
		return "other"
	},
	// en: one → i = 1 and v = 0 (1.0 değil)
	"en": func(o pluralOperands) string {
		if o.i == 1 && o.v == 0 {
			return "one"
		}
		return "other"
	},
}

var (
	inflectorsMu sync.RWMutex
	inflectors   = map[string]func(word, grammarCase string) string{}
)

// RegisterInflector sets the function that adds grammatical case suffixes ({name|dat} ...) for a
// language; nil removes it. messages imports no other TR package, so the language's grammar
// package registers itself (internal/lang/turkish for tr).
func RegisterInflector(lang string, f func(word, grammarCase string) string) {
	inflectorsMu.Lock()
	defer inflectorsMu.Unlock()
	if f == nil {
		delete(inflectors, lang)
		return
	}
	inflectors[lang] = f
}

func inflect(lang, word, grammarCase string) string {
	inflectorsMu.RLock()
	f := inflectors[lang]
	inflectorsMu.RUnlock()
	if f == nil {
		return word
	}
	return f(word, grammarCase)
}

// compile şablonu parçalara ayırır ve her parametrenin ve ad yuvasının kullanılıp kullanılmadığını
// verir.
func compile(text string, params []string, slots map[string]string) ([]segment, []bool, map[string]bool, error) {
	p := &parser{text: text, params: params, slots: slots, used: make([]bool, len(params)), slotsUsed: map[string]bool{}}
	segs, err := p.sequence(false, false)
	if err != nil {
		return nil, nil, nil, err
	}
	if p.i < len(p.text) {
		return nil, nil, nil, fmt.Errorf("unmatched '}' (write }} for a literal brace)")
	}
	for i, name := range params {
		if p.used[i] && !covers(segs, i) {
			return nil, nil, nil, fmt.Errorf("{%s} is missing from some branches (use it in every branch or outside them)", name)
		}
	}
	return segs, p.used, p.slotsUsed, nil
}

// covers parametrenin seçilebilecek her dal yolunda görünüp görünmediğidir: dal dışında bir
// {ad}, bir çoğul ya da seçim yapısının kendi değeri (değer dalı seçer) ya da yapının bütün
// dalları.
func covers(segs []segment, arg int) bool {
	for _, s := range segs {
		switch s.kind {
		case segArg:
			if s.arg == arg {
				return true
			}
		case segPlural, segSelect:
			if s.arg == arg {
				return true
			}
			all := true
			for _, b := range s.branches {
				if !covers(b, arg) {
					all = false
					break
				}
			}
			if all {
				return true
			}
		}
	}
	return false
}

// parser şablonu soldan sağa okur; çoğul ve seçim dalları iç içe şablonlardır.
type parser struct {
	text   string
	i      int
	params []string
	used   []bool
	slots  map[string]string // ad yuvası → kimlik
	// slotsUsed şablonda geçen ad yuvalarıdır.
	slotsUsed map[string]bool
}

// sequence bir şablonu ya da (inBranch ise) bir dalın içini '}'ye kadar okur.
func (p *parser) sequence(inBranch, inPlural bool) ([]segment, error) {
	var segs []segment
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			segs = append(segs, segment{kind: segText, text: lit.String()})
			lit.Reset()
		}
	}
	for p.i < len(p.text) {
		ch := p.text[p.i]
		next := byte(0)
		if p.i+1 < len(p.text) {
			next = p.text[p.i+1]
		}
		switch {
		case (ch == '{' && next == '{') || (ch == '}' && next == '}' && !inBranch):
			lit.WriteByte(ch)
			p.i += 2
		case ch == '{':
			flush()
			s, err := p.placeholder(inPlural)
			if err != nil {
				return nil, err
			}
			segs = append(segs, s)
		case ch == '}':
			if !inBranch {
				return nil, fmt.Errorf("unmatched '}' (write }} for a literal brace)")
			}
			flush()
			return segs, nil
		case ch == '#' && inPlural:
			flush()
			segs = append(segs, segment{kind: segNumber})
			p.i++
		default:
			lit.WriteByte(ch)
			p.i++
		}
	}
	if inBranch {
		return nil, fmt.Errorf("unclosed branch: %q", p.text)
	}
	flush()
	return segs, nil
}

// placeholder '{' ile başlayan bir parametreyi okur; inPlural bir çoğul dalının içinde olunduğudur.
func (p *parser) placeholder(inPlural bool) (segment, error) {
	start := p.i
	p.i++ // {
	end := strings.IndexAny(p.text[p.i:], ",}")
	if end < 0 {
		return segment{}, fmt.Errorf("unclosed parameter: %q", p.text[start:])
	}
	head := p.text[p.i : p.i+end]
	p.i += end
	if p.text[p.i] == '}' {
		p.i++
		if strings.HasPrefix(head, "@") {
			return p.slot(head[1:])
		}
		if strings.HasPrefix(head, "#") {
			return p.codeSlot(head[1:])
		}
		name, mod, _ := strings.Cut(head, "|")
		n, err := p.param(name)
		if err != nil {
			return segment{}, err
		}
		if mod != "" && mod != "q" && mod != "decimal" && mod != "list" && !caseMods[mod] {
			return segment{}, fmt.Errorf("{%s|%s}: unknown format (q, decimal, list, acc, dat, loc, abl, gen)", name, mod)
		}
		return segment{kind: segArg, arg: n, mod: mod}, nil
	}
	return p.choice(strings.TrimSpace(head), inPlural)
}

// slot "{@ad}" ve "{@ad|dat}" biçimini (süslü parantezler okunmuş) çözer.
func (p *parser) slot(head string) (segment, error) {
	name, mod, _ := strings.Cut(head, "|")
	identity, ok := p.slots[name]
	if !ok {
		return segment{}, fmt.Errorf("{@%s}: no such name slot in the definition (slots: %v)", name, slices.Sorted(maps.Keys(p.slots)))
	}
	if mod != "" && !caseMods[mod] {
		return segment{}, fmt.Errorf("{@%s|%s}: unknown format (acc, dat, loc, abl, gen)", name, mod)
	}
	p.slotsUsed[name] = true
	return segment{kind: segSlot, text: identity, mod: mod}, nil
}

// codeSlot "{#ad}" biçimini (süslü parantezler okunmuş) çözer; kod yuvası hâl ya da biçim eki almaz.
func (p *parser) codeSlot(head string) (segment, error) {
	if strings.Contains(head, "|") {
		name, _, _ := strings.Cut(head, "|")
		return segment{}, fmt.Errorf("{#%s}: code slot takes no format or case suffix", name)
	}
	identity, ok := p.slots[head]
	if !ok {
		return segment{}, fmt.Errorf("{#%s}: no such name slot in the definition (slots: %v)", head, slices.Sorted(maps.Keys(p.slots)))
	}
	p.slotsUsed[head] = true
	return segment{kind: segCodeSlot, text: identity}, nil
}

// choice "{ad, plural|select, anahtar {dal} ...}" biçimini virgülden sonrasından okur. Bir
// çoğulun içindeki seçim dallarında # dıştaki çoğulun sayısıdır (ICU'daki gibi).
func (p *parser) choice(name string, inPlural bool) (segment, error) {
	n, err := p.param(name)
	if err != nil {
		return segment{}, err
	}
	p.i++ // ,
	rest := p.text[p.i:]
	kindEnd := strings.IndexByte(rest, ',')
	if kindEnd < 0 {
		return segment{}, fmt.Errorf("{%s, ...}: a kind and branches were expected", name)
	}
	kindName := strings.TrimSpace(rest[:kindEnd])
	p.i += kindEnd + 1
	s := segment{arg: n, branches: map[string][]segment{}}
	switch kindName {
	case "plural":
		s.kind = segPlural
	case "select":
		s.kind = segSelect
	default:
		return segment{}, fmt.Errorf("{%s, %s}: unknown kind (plural or select)", name, kindName)
	}
	for {
		p.skipSpace()
		if p.i >= len(p.text) {
			return segment{}, fmt.Errorf("{%s, %s}: unclosed", name, kindName)
		}
		if p.text[p.i] == '}' {
			p.i++
			break
		}
		keyEnd := strings.IndexAny(p.text[p.i:], " {}")
		if keyEnd <= 0 || p.text[p.i+keyEnd] == '}' {
			return segment{}, fmt.Errorf("{%s, %s}: key without a branch", name, kindName)
		}
		key := p.text[p.i : p.i+keyEnd]
		p.i += keyEnd
		p.skipSpace()
		if p.i >= len(p.text) || p.text[p.i] != '{' {
			return segment{}, fmt.Errorf("{%s, %s}: key %q has no branch", name, kindName, key)
		}
		if s.kind == segPlural && !pluralCategories[key] && !isExact(key) {
			return segment{}, fmt.Errorf("{%s, plural}: %q is not a CLDR category or =N (N in canonical form: =0, =1.5)", name, key)
		}
		p.i++ // {
		branch, err := p.sequence(true, inPlural || s.kind == segPlural)
		if err != nil {
			return segment{}, err
		}
		p.i++ // }
		s.branches[key] = branch
	}
	if _, ok := s.branches["other"]; !ok {
		return segment{}, fmt.Errorf("{%s, %s}: the \"other\" branch is required", name, kindName)
	}
	return s, nil
}

func (p *parser) param(name string) (int, error) {
	for i, q := range p.params {
		if q == name {
			p.used[i] = true
			return i, nil
		}
	}
	return 0, fmt.Errorf("{%s}: no such parameter in the definition (parameters: %v)", name, p.params)
}

func (p *parser) skipSpace() {
	for p.i < len(p.text) && p.text[p.i] == ' ' {
		p.i++
	}
}

// render şablonu değerlerle doldurur. Eksik değer {ad?} olarak görünür (denetim testi çağrı
// yerlerindeki değer sayısını tanımla karşılaştırır; bu yalnızca bir güvence).
func (c *Catalog) render(code Code, args []any) string {
	segs, ok := c.templates[code]
	if !ok {
		if c.Language != "tr" {
			return catalogs["tr"].render(code, args)
		}
		return fmt.Sprintf("%s %v", code, args) // tanımsız kod: derleme sabitleriyle olmamalı
	}
	var b strings.Builder
	c.write(&b, code, segs, args, nil)
	return b.String()
}

// write parçaları b'ye yazar; number çoğul dalındaki # yerine geçen sayıdır.
func (c *Catalog) write(b *strings.Builder, code Code, segs []segment, args []any, number any) {
	for _, s := range segs {
		if s.kind == segSlot || s.kind == segCodeSlot {
			c.writeSlot(b, s)
			continue
		}
		if s.kind != segText && s.kind != segNumber && s.arg >= len(args) {
			fmt.Fprintf(b, "{%s?}", c.params[code][s.arg])
			continue
		}
		switch s.kind {
		case segText:
			b.WriteString(s.text)
		case segNumber:
			fmt.Fprint(b, number)
		case segArg:
			a := args[s.arg]
			text, coded := c.codedText(a)
			if !coded && s.mod != "q" {
				text = fmt.Sprint(a)
			}
			switch {
			case s.mod == "q" && coded:
				fmt.Fprintf(b, "%q", text)
			case s.mod == "q":
				fmt.Fprintf(b, "%q", a)
			case s.mod == "decimal":
				b.WriteString(localDecimal(text, c.Language))
			case s.mod == "list":
				b.WriteString(c.listText(a, text))
			case s.mod != "":
				b.WriteString(inflect(c.Language, text, s.mod))
			default:
				b.WriteString(text)
			}
		case segPlural:
			c.write(b, code, s.branches[c.pluralKey(s.branches, args[s.arg])], args, args[s.arg])
		case segSelect:
			key := fmt.Sprint(args[s.arg])
			if key == "" {
				key = "empty" // boş değer `empty` dalını seçer (isteğe bağlı parça)
			}
			branch, ok := s.branches[key]
			if !ok {
				branch = s.branches["other"]
			}
			c.write(b, code, branch, args, number)
		}
	}
}

// writeSlot ad yuvasını ({@ad}) iletinin dilindeki adıyla, kod yuvasını ({#ad}) programdaki
// kimlik yazımıyla yazar.
func (c *Catalog) writeSlot(b *strings.Builder, s segment) {
	if s.kind == segCodeSlot {
		b.WriteString(codeSpelling(s.text))
		return
	}
	name := s.text
	if IdentityNames != nil {
		name = IdentityNames(name, c.Language)
	}
	if s.mod != "" {
		name = inflect(c.Language, name, s.mod)
	}
	b.WriteString(name)
}

// codeSpelling tanımın slots tablosundaki kimliğin (TR_mem_length) programdaki yazımıdır (TR_length).
func codeSpelling(identity string) string {
	if IdentitySpelling != nil {
		return IdentitySpelling(identity)
	}
	if rest, ok := strings.CutPrefix(identity, "TR_"); ok {
		if _, short, ok := strings.Cut(rest, "_"); ok {
			return "TR_" + short
		}
	}
	return identity
}

// Listed is a parameter value that is a list of alternatives ({name|list}): the catalog writes
// the items with the message language's "or" list pattern (OrListIn). A value that is not Listed
// is written as it is.
type Listed interface {
	Items(lang string) []string
}

// listText {ad|list} için değerin listesidir; liste olmayan değer olduğu gibi yazılır.
func (c *Catalog) listText(a any, text string) string {
	if l, ok := a.(Listed); ok {
		return OrListIn(c.Language, l.Items(c.Language)...)
	}
	return text
}

// IdentityNames metindeki kimlikleri lang'deki adlarıyla değiştirir; codes paketi kurar.
var IdentityNames func(text, lang string) string

// IdentitySpelling tanımın slots tablosundaki kimliğin (TR_mem_length) programdaki yazımını
// (TR_length) verir; codes paketi kurar.
var IdentitySpelling func(identity string) string

// codedText değer bir kodlu hataysa onun bu katalogdaki metnidir: iç ileti dıştakiyle aynı
// dilde yazılır (catalogFor ikisinin de bu dilde metni olduğunu denetlemiştir). Dile bağlı
// değer (Localized) de bu katalogun dilinde yazılır.
func (c *Catalog) codedText(a any) (string, bool) {
	switch e := a.(type) {
	case string:
		// Metindeki kimlikler (TR_fn_print) iletinin dilindeki adıyla
		// yazılır; kimlik kullanıcı adı olamaz (2.3a), bu yüzden değiştirmek güvenlidir.
		if IdentityNames != nil && strings.Contains(e, "TR_") {
			return IdentityNames(e, c.Language), true
		}
		return "", false
	case Localized:
		return e.MessageText(c.Language), true
	case *Error:
		if e == nil {
			return "", true // yok olan ileti parçası (ör. öneri yoksa) boş yazılır
		}
		return c.render(e.Code, e.Args), true
	case *PosError:
		if e == nil {
			return "", false
		}
		if code, args, ok := e.message(); ok {
			return c.render(code, args), true
		}
		if e.Err == nil {
			return "", true
		}
		if text, ok := c.codedText(e.Err); ok {
			return text, true
		}
		return e.Err.Error(), true
	}
	return "", false
}
