package messages

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"sync"
)

// Şablon dili ICU MessageFormat'ın küçük bir alt kümesidir; çevirmen araçları bu biçimi tanır:
//
//	{ad}                         değer
//	{ad|q}                       Go'nun %q biçimiyle tırnaklı (denetim karakterleri kaçırılır)
//	{ad|acc} {ad|dat} {ad|loc}   hâl eki: dilin çekimcisi ekler (tr: 'f'yi, 'f'ye, 'f'de ...);
//	{ad|abl} {ad|gen}            çekimcisi olmayan dilde ad olduğu gibi kalır
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
	segText   segKind = iota // düz metin
	segArg                   // {ad}, {ad|biçim}
	segPlural                // {ad, plural, ...}
	segSelect                // {ad, select, ...}
	segNumber                // çoğul dalındaki #
)

// segment bir ileti şablonunun parçasıdır.
type segment struct {
	kind     segKind
	text     string               // segText
	arg      int                  // parametrenin sırası
	mod      string               // segArg: "", "q" ya da hâl adı
	branches map[string][]segment // segPlural, segSelect: dal anahtarı → dal
}

// caseMods şablonda geçerli hâl biçimleridir.
var caseMods = map[string]bool{"acc": true, "dat": true, "loc": true, "abl": true, "gen": true}

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

// compile şablonu parçalara ayırır ve her parametrenin kullanılıp kullanılmadığını verir.
func compile(text string, params []string) ([]segment, []bool, error) {
	p := &parser{text: text, params: params, used: make([]bool, len(params))}
	segs, err := p.sequence(false, false)
	if err != nil {
		return nil, nil, err
	}
	if p.i < len(p.text) {
		return nil, nil, fmt.Errorf("unmatched '}' (write }} for a literal brace)")
	}
	for i, name := range params {
		if p.used[i] && !covers(segs, i) {
			return nil, nil, fmt.Errorf("{%s} is missing from some branches (use it in every branch or outside them)", name)
		}
	}
	return segs, p.used, nil
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
		name, mod, _ := strings.Cut(head, "|")
		n, err := p.param(name)
		if err != nil {
			return segment{}, err
		}
		if mod != "" && mod != "q" && !caseMods[mod] {
			return segment{}, fmt.Errorf("{%s|%s}: unknown format (q, acc, dat, loc, abl, gen)", name, mod)
		}
		return segment{kind: segArg, arg: n, mod: mod}, nil
	}
	return p.choice(strings.TrimSpace(head), inPlural)
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

// isExact "=N" anahtarıdır; N sayının tek (kanonik) yazımıdır: =0.0 ya da =+1 hiçbir değerle
// eşleşmezdi.
func isExact(key string) bool {
	if len(key) < 2 || key[0] != '=' {
		return false
	}
	f, err := strconv.ParseFloat(key[1:], 64)
	return err == nil && strconv.FormatFloat(f, 'f', -1, 64) == key[1:]
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

// codedText değer bir kodlu hataysa onun bu katalogdaki metnidir: iç ileti dıştakiyle aynı
// dilde yazılır (catalogFor ikisinin de bu dilde metni olduğunu denetlemiştir).
func (c *Catalog) codedText(a any) (string, bool) {
	switch e := a.(type) {
	case *Error:
		if e == nil {
			return "", false
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

// pluralKey değerin dalını seçer: önce "=N" tam eşleşme, sonra dilin CLDR kategorisi, yoksa
// "other". Sayı olmayan değer "other"dır.
func (c *Catalog) pluralKey(branches map[string][]segment, v any) string {
	text, ok := exactText(v)
	if !ok {
		return "other"
	}
	if _, exact := branches["="+text]; exact {
		return "=" + text
	}
	rule := pluralRules[c.Language]
	if rule == nil {
		return "other"
	}
	o, _ := operands(v)
	if k := rule(o); branches[k] != nil {
		return k
	}
	return "other"
}

// exactText sayının "=N" anahtarındaki yazımıdır; değer sayı değilse ok false'tur.
func exactText(v any) (string, bool) {
	if x, ok := integerValue(v); ok {
		return x.String(), true
	}
	switch x := v.(type) {
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32), true
	}
	return "", false
}

// operands değerin CLDR işlenenleridir. Tam sayı türünde v = 0'dır; ondalık türü (float) en
// az bir ondalık basamaklı sayılır: CLDR'de 1.0 yazılan sayı İngilizcede "one" değildir.
func operands(v any) (pluralOperands, bool) {
	if x, ok := integerValue(v); ok {
		x.Abs(x)
		n, _ := new(big.Float).SetInt(x).Float64()
		return pluralOperands{n: n, i: x.Uint64()}, true
	}
	var text string
	switch x := v.(type) {
	case float64:
		text = strconv.FormatFloat(math.Abs(x), 'f', -1, 64)
	case float32:
		text = strconv.FormatFloat(math.Abs(float64(x)), 'f', -1, 32)
	default:
		return pluralOperands{}, false
	}
	whole, frac, _ := strings.Cut(text, ".")
	if frac == "" {
		frac = "0"
	}
	o := pluralOperands{v: len(frac)}
	o.n, _ = strconv.ParseFloat(text, 64)
	o.i, _ = strconv.ParseUint(whole, 10, 64)
	o.f, _ = strconv.ParseUint(frac, 10, 64)
	return o, true
}

// integerValue Go'nun bütün tam sayı türlerini kayıpsız okur.
func integerValue(v any) (*big.Int, bool) {
	switch x := v.(type) {
	case int:
		return big.NewInt(int64(x)), true
	case int8:
		return big.NewInt(int64(x)), true
	case int16:
		return big.NewInt(int64(x)), true
	case int32:
		return big.NewInt(int64(x)), true
	case int64:
		return big.NewInt(x), true
	case uint:
		return new(big.Int).SetUint64(uint64(x)), true
	case uint8:
		return new(big.Int).SetUint64(uint64(x)), true
	case uint16:
		return new(big.Int).SetUint64(uint64(x)), true
	case uint32:
		return new(big.Int).SetUint64(uint64(x)), true
	case uint64:
		return new(big.Int).SetUint64(x), true
	}
	return nil, false
}
