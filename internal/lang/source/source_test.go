package source

import (
	"errors"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Başlık (2.1): ilk satır tam olarak '# TR 4'tür, '#!' ile başlayan ilk satırdan sonra ikinci satır.
// Dosyanın dili yoktur. Başka bir sürümün başlığı (aynı önekle başlayıp başlık olmayan satır) okunmaz
// ve satırıyla bildirilir; başlıksız metin dosya değildir, çalışmaz, ileti kodlama komutunu söyler.
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

func hasPos(err error, line, col int) bool {
	l, c, ok := messages.Pos(err)
	return ok && l == line && c == col
}

// Kural: IdentityLexer gövdeyi yorumlayıcının okuduğu gibi verir: dilin öğesi kimliğiyle gelir (kodlu gövdede yazıldığı
// gibi, başlıksız Türkçe gövdede önce kodlanarak), kullanıcının adı yazıldığı gibi; sütunlar diskteki metne göredir.
func TestIdentityLexerReadsIdentitiesAtDiskPositions(t *testing.T) {
	for _, c := range []struct {
		body  string
		paren int // `(` belirtecinin diskteki sütunu
	}{{"TR_print(adet)", 9}, {"yazdır(adet)", 7}} {
		l := Parts{Body: c.body, Coded: strings.HasPrefix(c.body, "TR_")}.IdentityLexer()
		name, open, user := l.NextToken(), l.NextToken(), l.NextToken()
		if name.Code != codes.FnPrint || open.Column != c.paren || user.Literal != "adet" {
			t.Errorf("%q: %+v %+v %+v; want the identity TR_print, `(` at column %d, the user name adet", c.body, name, open, user, c.paren)
		}
	}
}
