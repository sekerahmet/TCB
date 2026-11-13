package interp

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// CheckJSON JSON metnini çözmeden önce bir kez baştan sona tarar (3.7a): RFC 8259'un
// dilbilgisini, UTF-8'i ve kaçışları denetler; ilk sorunu satırı, sütunu ve nedeniyle kodlu hata
// olarak verir (H2570–H2579; 1000'i aşan iç içelik H2500). Metin tek bir değerdir, değerden
// sonra yalnız boşluk gelir. Go'nun çözücüsü eşi olmayan vekil kaçışını ve geçersiz baytı
// sessizce U+FFFD yapar, hatasının metni de Go sürümüyle değişir; bu taramadan geçen metinde
// Go'nun çözücüsü hata vermez (JSONToTR, kontrol noktası, safetensors başlığı).
func CheckJSON(data []byte) error {
	s := jsonScan{data: data}
	return s.run()
}

// jsonScan taramanın durumudur: i sıradaki bayt, open açık kapların ('{', '[') baytlarıdır.
type jsonScan struct {
	data []byte
	i    int
	open []int
}

// Taramanın sıradaki karakterden beklediği.
const (
	wantValue     = iota // değer: en üstte, ':' sonrası, dizide ',' sonrası
	wantArrayOpen        // '[' sonrası: değer ya da ']'
	wantKeyOpen          // '{' sonrası: anahtar ya da '}'
	wantKey              // nesnede ',' sonrası: anahtar
	wantColon            // anahtardan sonra ':'
	wantNext             // değerden sonra: ',' ya da kabın kapanışı; en üstte metnin sonu
)

func (s *jsonScan) run() error {
	s.space()
	if s.i == len(s.data) {
		line, column := jsonPosition(s.data, s.i)
		return messages.New(messages.H2579, line, column)
	}
	want := wantValue
	for {
		s.space()
		if s.i == len(s.data) {
			if want == wantNext && len(s.open) == 0 {
				return nil
			}
			return s.unclosed(s.open[len(s.open)-1])
		}
		if err := s.badByte(s.i); err != nil {
			return err
		}
		var err error
		if want, err = s.step(want); err != nil {
			return err
		}
	}
}

// step sıradaki belirteci okur ve ondan sonra bekleneni verir.
func (s *jsonScan) step(want int) (int, error) {
	c := s.data[s.i]
	switch want {
	case wantValue, wantArrayOpen:
		if want == wantArrayOpen && c == ']' {
			return s.close(), nil
		}
		return s.value(want)
	case wantKeyOpen, wantKey:
		switch {
		case c == '"':
			return wantColon, s.text()
		case want == wantKey:
			return 0, s.unexpected("key")
		case c == '}':
			return s.close(), nil
		}
		return 0, s.inObject("open")
	case wantColon:
		if c != ':' {
			return 0, s.unexpected("colon")
		}
		s.i++
		return wantValue, nil
	}
	if len(s.open) == 0 {
		return 0, s.unexpected("end")
	}
	object := s.data[s.open[len(s.open)-1]] == '{'
	switch {
	case c == ',' && object:
		s.i++
		return wantKey, nil
	case c == ',':
		s.i++
		return wantValue, nil
	case c == '}' && object, c == ']' && !object:
		return s.close(), nil
	case object:
		return 0, s.inObject("next")
	}
	return 0, s.unexpected("array_next")
}

// value değer yerindeki belirteci okur: metin, kap açılışı, sayı ya da ad.
func (s *jsonScan) value(want int) (int, error) {
	c := s.data[s.i]
	r, _ := utf8.DecodeRune(s.data[s.i:])
	switch {
	case c == '"':
		return wantNext, s.text()
	case c == '{' || c == '[':
		if len(s.open) >= maxJSONDepth {
			return 0, messages.New(messages.H2500, maxJSONDepth)
		}
		s.open = append(s.open, s.i)
		s.i++
		if c == '{' {
			return wantKeyOpen, nil
		}
		return wantArrayOpen, nil
	case c == '-' || c == '+' || c == '.' || isASCIIDigit(c):
		return wantNext, s.number()
	case c == '\'' || c == '_' || unicode.IsLetter(r):
		return wantNext, s.word()
	}
	if want == wantArrayOpen {
		return 0, s.unexpected("array_open")
	}
	return 0, s.unexpected("value")
}

func (s *jsonScan) close() int {
	s.open = s.open[:len(s.open)-1]
	s.i++
	return wantNext
}

// text '"' ile başlayan metni sonuna kadar okur.
func (s *jsonScan) text() error {
	start := s.i
	s.i++
	for s.i < len(s.data) {
		c := s.data[s.i]
		switch {
		case c == '"':
			s.i++
			return nil
		case c == '\\':
			if err := s.escape(); err != nil {
				return err
			}
		case c < 0x20:
			line, column := jsonPosition(s.data, s.i)
			return messages.New(messages.H2575, fmt.Sprintf("%04X", c), line, column)
		case c >= utf8.RuneSelf:
			if err := s.badByte(s.i); err != nil {
				return err
			}
			_, size := utf8.DecodeRune(s.data[s.i:])
			s.i += size
		default:
			s.i++
		}
	}
	return s.unclosed(start)
}

// escape metindeki ters bölüyle başlayan kaçışı okur. Yüksek vekil (\uD800–\uDBFF) ancak
// hemen ardından düşük vekille (\uDC00–\uDFFF) bir karakterdir; tek başına vekil H2570'tir.
func (s *jsonScan) escape() error {
	at := s.i
	if at+1 >= len(s.data) {
		s.i = len(s.data) // metin kapanmadı (H2574)
		return nil
	}
	switch s.data[at+1] {
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		s.i += 2
		return nil
	case 'u':
		r, ok := hex4(s.data, at)
		switch {
		case !ok:
			return s.badEscape(at, 6)
		case r >= 0xDC00 && r <= 0xDFFF:
			return s.loneSurrogate(at)
		case r >= 0xD800 && r <= 0xDBFF:
			low, ok := hex4(s.data, at+6)
			if !ok || low < 0xDC00 || low > 0xDFFF {
				return s.loneSurrogate(at)
			}
			s.i += 12
		default:
			s.i += 6
		}
		return nil
	}
	if err := s.badByte(at + 1); err != nil {
		return err
	}
	return s.badEscape(at, 2)
}

// hex4, b[i:]'de \u ve dört onaltılık basamak varsa onların değeridir.
func hex4(b []byte, i int) (rune, bool) {
	if i+6 > len(b) || b[i] != '\\' || b[i+1] != 'u' {
		return 0, false
	}
	var r rune
	for _, c := range b[i+2 : i+6] {
		switch {
		case isASCIIDigit(c):
			r = r<<4 | rune(c-'0')
		case c >= 'a' && c <= 'f':
			r = r<<4 | rune(c-'a'+10)
		case c >= 'A' && c <= 'F':
			r = r<<4 | rune(c-'A'+10)
		default:
			return 0, false
		}
	}
	return r, true
}

// number sayıyı RFC 8259'un dilbilgisiyle okur: isteğe bağlı '-', 0 ya da 0'la başlamayan
// basamaklar, isteğe bağlı kesir ve üs.
func (s *jsonScan) number() error {
	b, start := s.data, s.i
	i := start
	if b[i] == '-' {
		i++
	}
	digits := func() {
		for i < len(b) && isASCIIDigit(b[i]) {
			i++
		}
	}
	switch {
	case i < len(b) && b[i] == '0':
		i++
		if i < len(b) && isASCIIDigit(b[i]) {
			return s.badNumber(start)
		}
	case i < len(b) && b[i] >= '1' && b[i] <= '9':
		digits()
	default:
		return s.badNumber(start)
	}
	if i < len(b) && b[i] == '.' {
		if i++; i >= len(b) || !isASCIIDigit(b[i]) {
			return s.badNumber(start)
		}
		digits()
	}
	if i < len(b) && (b[i] == 'e' || b[i] == 'E') {
		if i++; i < len(b) && (b[i] == '+' || b[i] == '-') {
			i++
		}
		if i >= len(b) || !isASCIIDigit(b[i]) {
			return s.badNumber(start)
		}
		digits()
	}
	s.i = i
	return nil
}

// word değer yerinde harfle ya da tek tırnakla başlayan sözcüğü okur; JSON'un adları true,
// false ve null'dur, öbürü H2578'dir (True, None, NaN, 'a').
func (s *jsonScan) word() error {
	w := s.token(s.i)
	if w == "true" || w == "false" || w == "null" {
		s.i += len(w)
		return nil
	}
	line, column := jsonPosition(s.data, s.i)
	return messages.New(messages.H2578, w, line, column)
}

// token start'tan ilk boşluğa, ayraca (, : [ ] { } ") ya da geçersiz bayta kadarki metindir.
func (s *jsonScan) token(start int) string {
	i := start
	for i < len(s.data) {
		r, size := utf8.DecodeRune(s.data[i:])
		if r == utf8.RuneError && size == 1 || isJSONDelimiter(r) {
			break
		}
		i += size
	}
	return string(s.data[start:i])
}

func isJSONDelimiter(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', ',', ':', '[', ']', '{', '}', '"':
		return true
	}
	return false
}

func isASCIIDigit(c byte) bool { return c >= '0' && c <= '9' }

// space JSON'un boşluklarını atlar.
func (s *jsonScan) space() {
	for s.i < len(s.data) {
		switch s.data[s.i] {
		case ' ', '\t', '\n', '\r':
			s.i++
		default:
			return
		}
	}
}

// badByte, i'deki bayt geçerli bir UTF-8 dizisi başlatmıyorsa H2571'dir (yarım kalan dizi dahil).
func (s *jsonScan) badByte(i int) error {
	if s.data[i] < utf8.RuneSelf {
		return nil
	}
	if r, size := utf8.DecodeRune(s.data[i:]); r != utf8.RuneError || size != 1 {
		return nil
	}
	line, column := jsonPosition(s.data, i)
	return messages.New(messages.H2571, fmt.Sprintf("%02X", s.data[i]), line, column)
}

func (s *jsonScan) loneSurrogate(at int) error {
	line, column := jsonPosition(s.data, at)
	return messages.New(messages.H2570, string(s.data[at:at+6]), line, column)
}

// badEscape geçersiz kaçıştır; ileti kaçışın en çok n karakterini gösterir.
func (s *jsonScan) badEscape(at, n int) error {
	i := at
	for k := 0; k < n && i < len(s.data); k++ {
		r, size := utf8.DecodeRune(s.data[i:])
		if r == utf8.RuneError && size == 1 {
			break
		}
		i += size
	}
	line, column := jsonPosition(s.data, at)
	return messages.New(messages.H2576, string(s.data[at:i]), line, column)
}

func (s *jsonScan) badNumber(start int) error {
	line, column := jsonPosition(s.data, start)
	return messages.New(messages.H2577, s.token(start), line, column)
}

// unexpected sıradaki karakter beklenmiyordu (nesnenin anahtar ve ':' yerleri dahil, H2573).
func (s *jsonScan) unexpected(expected string) error {
	r, _ := utf8.DecodeRune(s.data[s.i:])
	line, column := jsonPosition(s.data, s.i)
	return messages.New(messages.H2573, string(r), expected, line, column)
}

// inObject nesnede '}' beklenen yerde başka bir karakterdir (H2572): after "open" '{'
// sonrası, "next" bir değerden sonrası.
func (s *jsonScan) inObject(after string) error {
	r, _ := utf8.DecodeRune(s.data[s.i:])
	line, column := jsonPosition(s.data, s.i)
	return messages.New(messages.H2572, string(r), after, line, column)
}

// unclosed metin bitti ama at'ta açılan kap ya da metin kapanmadı (H2574).
func (s *jsonScan) unclosed(at int) error {
	open := "string"
	switch s.data[at] {
	case '{':
		open = "object"
	case '[':
		open = "array"
	}
	line, column := jsonPosition(s.data, at)
	return messages.New(messages.H2574, open, line, column)
}

// jsonPosition offset'teki baytın 1'den başlayan satırı ve sütunudur; sütun karakter sayar,
// geçersiz bayt bir karakterdir.
func jsonPosition(data []byte, offset int) (line, column int) {
	before := data[:offset]
	line = 1
	last := -1
	for i, c := range before {
		if c == '\n' {
			line++
			last = i
		}
	}
	return line, utf8.RuneCount(before[last+1:]) + 1
}
