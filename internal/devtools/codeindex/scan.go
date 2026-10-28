package codeindex

// codeScanner bir satırın kod karakterlerini sırayla verir: çift tırnaklı metinler (kaçışlarıyla)
// ve isteğe bağlı olarak `#` yorumları (2.2; `//` tam bölmedir) atlanır, tırnak işaretleri de verilmez. Parantez sayımı,
// birim (satırın kod noktası konumu) ve ne zaman duracağı çağıranındır; çağıran i ile bir sonraki
// konumu ya da end ile bitişi değiştirebilir (ad atlama, değer kesme).
type codeScanner struct {
	text     []rune
	i, end   int  // bir sonraki konum ve bitiş (dışlayan)
	comments bool // `#` satır sonuna kadar yorum sayılır
	inStr    bool
}

// newCodeScanner text[from:to] aralığını tarar.
func newCodeScanner(text []rune, from, to int, comments bool) *codeScanner {
	return &codeScanner{text: text, i: from, end: to, comments: comments}
}

// next bir sonraki kod karakterini ve konumunu verir; aralık bitince ok false olur.
func (s *codeScanner) next() (pos int, c rune, ok bool) {
	for s.i < s.end {
		j := s.i
		s.i++
		c := s.text[j]
		switch {
		case s.inStr:
			if c == '\\' {
				s.i++
			} else if c == '"' {
				s.inStr = false
			}
		case c == '"':
			s.inStr = true
		case s.comments && c == '#':
			s.i = s.end
		default:
			return j, c, true
		}
	}
	return 0, 0, false
}
