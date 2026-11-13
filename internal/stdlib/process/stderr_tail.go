package process

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// stderrTailLimit iletideki standart hata sonunun en çok bayt sayısıdır (21.4). Çıktının geri kalanı
// tutulmaz: program başarıyla biterse hepsi atılır, başarısız biterse iletide son satırlar yeter.
const stderrTailLimit = 4096

// stderrTail standart hata çıktısının sonunu sınırlı bellekte tutar. Gizli değer iletide görünmez
// (19.4): kesilen bir değerin kalanı süzgeçten kaçardı, bu yüzden tampon sınır artı en uzun gizli
// değer kadar tutar; metin önce süzülür, sonra kesilir. Tek yazan vardır (os/exec'in kopyalayıcısı),
// okuma program bittikten sonradır.
type stderrTail struct {
	longest func() int // en uzun gizli değerin boyu
	buf     []byte
	cut     bool // tamponun başından bayt düştü
}

// Write baytları tampona ekler; tampon iki katına varınca başı atılır, kopyalama seyrek olsun.
func (t *stderrTail) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if keep := stderrTailLimit + t.longest(); len(t.buf) > 2*keep {
		t.buf = append(t.buf[:0], t.buf[len(t.buf)-keep:]...)
		t.cut = true
	}
	return len(p), nil
}

// text iletiye girecek metindir: mask gizli değerleri süzer, sonra sınıra kesilir ve yazdırılamayan
// baytlar atılır. Tamponun başı düştüyse ilk baytlar bir gizli değerin kalanı olabilir: onlar da atılır.
// Atılan baş bir adres bölgesinin "://"'sunu da götürmüş olabilir; kalan kimlik kısmı bağlamsız
// kalırdı (19.4): pencerenin ilk bölge sınırına ('/', '?', '#', boşluk) kadarki başı '@' taşıyorsa
// (kimlik kısmı '@'den öncedir) o da atılır.
func (t *stderrTail) text(mask func(string) string) string {
	s := mask(string(t.buf))
	if t.cut {
		s = s[min(t.longest(), len(s)):]
		head := s
		if i := strings.IndexFunc(s, isRegionEnd); i >= 0 {
			head = s[:i]
		}
		if strings.Contains(head, "@") {
			s = s[len(head):]
		}
	}
	if len(s) > stderrTailLimit {
		s = s[len(s)-stderrTailLimit:]
		for len(s) > 0 && !utf8.RuneStart(s[0]) {
			s = s[1:]
		}
	}
	return printableText(s)
}

// isRegionEnd 19.4'ün adres bölgesini bitiren harftir.
func isRegionEnd(r rune) bool { return r == '/' || r == '?' || r == '#' || unicode.IsSpace(r) }

// printableText programın baytlarını iletiye uygun metne çevirir: geçersiz UTF-8 U+FFFD olur, satır
// sonu dışındaki denetim karakterleri uçbirimi ve günlüğü bozmasın diye atılır (sekme boşluk olur).
func printableText(s string) string {
	s = strings.ToValidUTF8(s, "�")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '\n':
			return r
		case r == '\t':
			return ' '
		case r == '\r':
			return '\n'
		case unicode.IsControl(r):
			return -1
		}
		return r
	}, s)
	return strings.TrimRight(s, " \n")
}
