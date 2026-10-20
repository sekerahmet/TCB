package messages

import (
	"fmt"
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
//	{ad, plural, one {..} other {..}}  sayıya göre biçim (CLDR çoğul kuralı); dalda # sayıdır,
//	                                   "=N" tam eşleşme kategoriden önce gelir
//	{ad, select, x {..} other {..}}    metin değerine göre dal
//	{{ }}                        düz süslü parantez (dal içinde '}' her zaman dalı kapatır)
//
// Go kodu sözcük çekimlemez ve sayıya bakıp sözcük seçmez: bunlar dilin işidir, şablondadır.

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

// pluralRules dilin CLDR çoğul kuralıdır: tam sayı n için kategori. Kuralı olmayan dilde
// (yeni bir dil eklenirken) her sayı "other"dır.
var pluralRules = map[string]func(n float64, integer bool) string{
	"tr": func(n float64, integer bool) string { return oneIfOne(n, integer) },
	"en": func(n float64, integer bool) string { return oneIfOne(n, integer) },
}

// oneIfOne Türkçe ve İngilizcenin kuralıdır: tam sayı 1 "one", gerisi "other".
func oneIfOne(n float64, integer bool) string {
	if integer && n == 1 {
		return "one"
	}
	return "other"
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
		return nil, nil, fmt.Errorf("eşi olmayan '}' (düz parantez için }} yazın)")
	}
	return segs, p.used, nil
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
			s, err := p.placeholder()
			if err != nil {
				return nil, err
			}
			segs = append(segs, s)
		case ch == '}':
			if !inBranch {
				return nil, fmt.Errorf("eşi olmayan '}' (düz parantez için }} yazın)")
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
		return nil, fmt.Errorf("kapanmamış dal: %q", p.text)
	}
	flush()
	return segs, nil
}

// placeholder '{' ile başlayan bir parametreyi okur.
func (p *parser) placeholder() (segment, error) {
	start := p.i
	p.i++ // {
	end := strings.IndexAny(p.text[p.i:], ",}")
	if end < 0 {
		return segment{}, fmt.Errorf("kapanmamış parametre: %q", p.text[start:])
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
			return segment{}, fmt.Errorf("{%s|%s}: bilinmeyen biçim (q, acc, dat, loc, abl, gen)", name, mod)
		}
		return segment{kind: segArg, arg: n, mod: mod}, nil
	}
	return p.choice(strings.TrimSpace(head))
}

// choice "{ad, plural|select, anahtar {dal} ...}" biçimini virgülden sonrasından okur.
func (p *parser) choice(name string) (segment, error) {
	n, err := p.param(name)
	if err != nil {
		return segment{}, err
	}
	p.i++ // ,
	rest := p.text[p.i:]
	kindEnd := strings.IndexByte(rest, ',')
	if kindEnd < 0 {
		return segment{}, fmt.Errorf("{%s, ...}: tür ve dallar bekleniyordu", name)
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
		return segment{}, fmt.Errorf("{%s, %s}: bilinmeyen tür (plural ya da select)", name, kindName)
	}
	for {
		p.skipSpace()
		if p.i >= len(p.text) {
			return segment{}, fmt.Errorf("{%s, %s}: kapanmamış", name, kindName)
		}
		if p.text[p.i] == '}' {
			p.i++
			break
		}
		keyEnd := strings.IndexAny(p.text[p.i:], " {}")
		if keyEnd <= 0 || p.text[p.i+keyEnd] == '}' {
			return segment{}, fmt.Errorf("{%s, %s}: dalsız anahtar", name, kindName)
		}
		key := p.text[p.i : p.i+keyEnd]
		p.i += keyEnd
		p.skipSpace()
		if p.i >= len(p.text) || p.text[p.i] != '{' {
			return segment{}, fmt.Errorf("{%s, %s}: %q anahtarının dalı yok", name, kindName, key)
		}
		if s.kind == segPlural && !pluralCategories[key] && !isExact(key) {
			return segment{}, fmt.Errorf("{%s, plural}: %q bir CLDR kategorisi ya da =N değil", name, key)
		}
		p.i++ // {
		branch, err := p.sequence(true, s.kind == segPlural)
		if err != nil {
			return segment{}, err
		}
		p.i++ // }
		s.branches[key] = branch
	}
	if _, ok := s.branches["other"]; !ok {
		return segment{}, fmt.Errorf("{%s, %s}: \"other\" dalı zorunlu", name, kindName)
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
	return 0, fmt.Errorf("{%s}: tanımda böyle bir parametre yok (parametreler: %v)", name, p.params)
}

func (p *parser) skipSpace() {
	for p.i < len(p.text) && p.text[p.i] == ' ' {
		p.i++
	}
}

func isExact(key string) bool {
	if len(key) < 2 || key[0] != '=' {
		return false
	}
	_, err := strconv.ParseFloat(key[1:], 64)
	return err == nil
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
			switch {
			case s.mod == "q":
				fmt.Fprintf(b, "%q", args[s.arg])
			case s.mod != "":
				b.WriteString(inflect(c.Language, fmt.Sprint(args[s.arg]), s.mod))
			default:
				fmt.Fprint(b, args[s.arg])
			}
		case segPlural:
			c.write(b, code, s.branches[c.pluralKey(s.branches, args[s.arg])], args, args[s.arg])
		case segSelect:
			branch, ok := s.branches[fmt.Sprint(args[s.arg])]
			if !ok {
				branch = s.branches["other"]
			}
			c.write(b, code, branch, args, number)
		}
	}
}

// pluralKey değerin dalını seçer: önce "=N" tam eşleşme, sonra dilin CLDR kategorisi, yoksa
// "other". Sayı olmayan değer "other"dır.
func (c *Catalog) pluralKey(branches map[string][]segment, v any) string {
	n, integer, ok := number(v)
	if !ok {
		return "other"
	}
	if _, exact := branches["="+strconv.FormatFloat(n, 'f', -1, 64)]; exact {
		return "=" + strconv.FormatFloat(n, 'f', -1, 64)
	}
	rule := pluralRules[c.Language]
	if rule == nil {
		return "other"
	}
	if k := rule(n, integer); branches[k] != nil {
		return k
	}
	return "other"
}

// number değeri sayıya çevirir; integer tam sayı türünden gelip gelmediğidir.
func number(v any) (n float64, integer, ok bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true, true
	case int64:
		return float64(x), true, true
	case int32:
		return float64(x), true, true
	case uint:
		return float64(x), true, true
	case uint64:
		return float64(x), true, true
	case float64:
		return x, false, true // CLDR: ondalık yazılan sayı (1.0) "one" değildir
	case float32:
		return float64(x), false, true
	}
	return 0, false, false
}
