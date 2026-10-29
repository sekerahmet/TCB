package source

import (
	"errors"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Başlık (2.1): ilk satır tam olarak '# TR 4'tür, '#!' ile başlayan ilk satırdan sonra ikinci satır.
// Dosyanın dili yoktur. Başka bir sürümün başlığı (aynı önekle başlayıp başlık olmayan satır) okunmaz
// ve satırıyla bildirilir; başlıksız metin dosya değildir, çalışmaz, ileti başlığı söyler.
func TestHeaderRules(t *testing.T) {
	for _, m := range []string{Header + "\n", Header, "\xef\xbb\xbf" + Header + "\r\nTR_var x = 1", "#!/usr/bin/env tr\n" + Header + "\nTR_var x = 1\n"} {
		if b, err := Split(m); err != nil || !b.Coded || b.Join() != m {
			t.Errorf("%q: kodlu okunmalı ve geri birleşmeli (%+v, %v)", m, b, err)
		}
	}
	// Başlıksız metin, başlığın başka satırda olduğu metin ve eski biçimin başlığı başlıksızdır.
	for _, m := range []string{"TR_var x = 1\n", "\n" + Header + "\n", "# yorum\n" + Header + "\n", "//! tr kodlu 3 dil=tr\nTR_kw_var x = 1\n", "# TR\n"} {
		if b, err := Split(m); err != nil || b.Coded {
			t.Errorf("%q: başlıksız sayılmalı (%+v, %v)", m, b, err)
		}
		s := Prepare(m, Options{})
		if s.Stage != StageSyntax || len(s.Errors) != 1 || !messages.Has(s.Errors[0], messages.H2540) || !hasPos(s.Errors[0], 1, 1) {
			t.Errorf("%q: başlıksız dosya H2540 (1:1) vermeli: %v", m, s.Errors)
		}
	}
	// Başka bir sürümün başlığı okunmaz; hata başlığın satırındadır.
	for m, line := range map[string]int{"# TR 3\n": 1, "# TR 5\nTR_var x = 1\n": 1, Header + " \n": 1, "# TR 4 # yorum\n": 1, "#!/usr/bin/env tr\n# TR 3\n": 2} {
		var bh *HeaderError
		if _, err := Split(m); !errors.As(err, &bh) || !messages.Has(err, messages.H2512) || bh.At != line {
			t.Errorf("%q: H2512 başlık hatası (satır %d) bekleniyordu, alınan %v", m, line, err)
		}
		if s := Prepare(m, Options{}); s.Stage != StageSyntax || !hasPos(s.Errors[0], line, 1) {
			t.Errorf("%q: hazırlık başlık hatasını sözdizimi hatası olarak bildirmeli: %+v", m, s.Errors)
		}
	}
	// Başlıksız kodlu gövde (bir testin ya da aracın parçası) Body ile okunur.
	if s := Prepare("TR_var x = 1\nTR_print(x)\n", Options{Body: true, TypeCheck: true}); s.Stage != "" {
		t.Errorf("başlıksız kodlu gövde okunmalı: %v", s.Errors)
	}
}

// Kural (17.7): görünüm dosyası işaretini taşır. 'tr show' başlığa görünümün dilini yazar ('# TR 4
// view=tr'; satır sayısı değişmez); görünüm çalışmaz ve koda geri çevrilmez: ileti (H2541, başlığın
// satırında) kodlu dosyayı çalıştırmayı söyler. Bilinmeyen dilin işareti okunamayan başlıktır (H2512).
// Neden: dosyanın türü dosyanın kendisinde olsun; görünüm yalnız okunur.
func TestViewHeader(t *testing.T) {
	coded := "#!/usr/bin/env tr\n" + Header + "\nTR_var x = 1\nTR_print(x)\n"
	shown, err := Show(coded, codes.Source())
	if want := "#!/usr/bin/env tr\n" + Header + " view=tr\ndeğişken x = 1\nyazdır(x)\n"; err != nil || shown != want {
		t.Fatalf("görünüm %q (%v), beklenen %q", shown, err, want)
	}
	var ve *ViewError
	if b, err := Split(shown); !errors.As(err, &ve) || b.View != "tr" || b.Coded || ve.At != 2 || HeaderLines(shown) != 2 {
		t.Errorf("görünüm başlığı tanınmalı: %+v, %v, %d", b, err, HeaderLines(shown))
	}
	for _, body := range []bool{false, true} {
		if s := Prepare(shown, Options{TypeCheck: true, Body: body}); s.Stage != StageSyntax || !messages.Has(s.Errors[0], messages.H2541) || !hasPos(s.Errors[0], 2, 1) {
			t.Errorf("görünüm çalışmamalı (H2541, 2:1): %v", s.Errors)
		}
	}
	if _, err := Show(shown, codes.Source()); !errors.As(err, &ve) {
		t.Errorf("görünüm yeniden gösterilmemeli: %v", err)
	}
	var bh *HeaderError
	if _, err := Split(Header + " view=xx\n"); !errors.As(err, &bh) {
		t.Errorf("bilinmeyen dilin işareti okunamayan başlıktır: %v", err)
	}
}

func hasPos(err error, line, col int) bool {
	l, c, ok := messages.Pos(err)
	return ok && l == line && c == col
}

// Kural (17.7): Lexer gövdeyi yorumlayıcının okuduğu gibi verir: dilin öğesi kimliğiyle gelir, kullanıcının
// adı yazıldığı gibi; sütunlar diskteki metne göredir. Başlıksız gövde de kimlik yazımıyla okunur: bir yazım
// dilinin sözcüğü (yazdır) kullanıcının adıdır.
func TestLexerReadsIdentitiesAtDiskPositions(t *testing.T) {
	for _, coded := range []bool{true, false} {
		l := Parts{Body: "TR_print(adet)", Coded: coded}.Lexer()
		name, open, user := l.NextToken(), l.NextToken(), l.NextToken()
		if name.Code != codes.FnPrint || open.Column != 9 || user.Literal != "adet" {
			t.Errorf("coded %v: %+v %+v %+v; want the identity TR_print, `(` at column 9, the user name adet", coded, name, open, user)
		}
	}
	if tok := (Parts{Body: "yazdır(adet)"}).Lexer().NextToken(); tok.Code != codes.None || tok.Literal != "yazdır" {
		t.Errorf("a writing language's word is a user name: %+v", tok)
	}
}
