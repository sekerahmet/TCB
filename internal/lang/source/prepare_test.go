package source

import (
	"errors"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// wantPos hatanın konumunu metinden değil yapıdan (messages.Pos) okuyup bekleneniyle karşılaştırır.
func wantPos(t *testing.T, err error, line, col int) {
	t.Helper()
	if l, c, ok := messages.Pos(err); !ok || l != line || c != col {
		t.Errorf("konum %d:%d bekleniyordu, gelen %d:%d (var: %v): %v", line, col, l, c, ok, err)
	}
}

// Kodlu dosya çevrilmeden hazırlanır; konumlar kişinin gösterimine göredir (17.5; TR_DIL yokken Türkçe).
func TestCodedFilePrepares(t *testing.T) {
	s := Prepare(Header+"tr\nTR_kw_var x = 1\nTR_fn_print(x, tanımsız)\n", Options{TypeCheck: true})
	if s.Stage != StageType || len(s.Errors) != 1 {
		t.Fatalf("tanımsız ad bildirilmeli: %v", s.Errors)
	}
	wantPos(t, s.Errors[0], 2, 11) // Türkçe gösterimdeki konum
	if s.Text != "değişken x = 1\nyazdır(x, tanımsız)\n" {
		t.Errorf("hata çerçevesinin metni Türkçe gösterim olmalı: %q", s.Text)
	}
	// Yerleştirmedeki kodlar da sütunu kaydırır.
	s = Prepare(Header+"tr\nTR_fn_print(\"{TR_fn_sum([1])} {yok_ad}\")\n", Options{TypeCheck: true})
	if len(s.Errors) != 1 {
		t.Fatalf("yerleştirmedeki ad bildirilmeli: %v", s.Errors)
	}
	wantPos(t, s.Errors[0], 1, 24)
	// Gösterimde '$' ile kaçırılan kullanıcı adı da sütunu kaydırır: konum gösterimdeki metne göredir.
	s = Prepare(Header+"tr\nTR_kw_var eğer = 1 + tanımsız\n", Options{TypeCheck: true})
	if s.Text != "değişken $eğer = 1 + tanımsız\n" || len(s.Errors) != 1 {
		t.Fatalf("gösterim %q, hatalar %v", s.Text, s.Errors)
	}
	wantPos(t, s.Errors[0], 1, 22)
	// Kod olmayan her ad kullanıcının adıdır, anahtar kelime yazımında bile.
	s = Prepare(Header+"tr\nTR_kw_var eğer = 1\nTR_fn_print(eğer)\n", Options{TypeCheck: true})
	if s.Stage != "" {
		t.Errorf("kodlu dosyada 'eğer' bir kullanıcı adıdır: %v", s.Errors)
	}
}

func TestCodedFileErrors(t *testing.T) {
	cases := map[string]string{
		Header + "tr\nTR_fn_yok(1)\n": "'TR_fn_yok' bilinen bir kimlik değil",
	}
	for src, part := range cases {
		s := Prepare(src, Options{TypeCheck: true})
		if !strings.Contains(joinErrors(s.Errors), part) {
			t.Errorf("%q: %q içeren hata bekleniyordu: %v", src, part, s.Errors)
		}
	}
	// Türkçe yazılmış dosyada TR_ ile başlayan ad hatadır ve başlığı gösterir (2.3a).
	s := Prepare("TR_fn_print(1)\n", Options{})
	if s.Stage != StageSyntax || !strings.Contains(s.Errors[0].Error(), Header) {
		t.Errorf("ayrılmış ad hatası başlığı göstermeli: %v", s.Errors)
	}
	// Ayrıştırılamayan kaynak çevrilmez.
	var sh *SyntaxError
	if _, err := Encode("eğer {\n"); !errors.As(err, &sh) {
		t.Errorf("sözdizimi hatası bekleniyordu: %v", err)
	}
}

// A file with thousands of errors (often not a TR file at all) reports the first MaxErrors and
// how many more there are; reporting every error with a code frame took minutes.
func TestPrepareLimitsErrors(t *testing.T) {
	r := Prepare(strings.Repeat("# yorum değil\n", 500), Options{})
	if len(r.Errors) != MaxErrors+1 {
		t.Fatalf("%d errors; %d expected", len(r.Errors), MaxErrors+1)
	}
	last := r.Errors[MaxErrors]
	if _, _, ok := messages.Pos(last); ok || !messages.Has(last, messages.H2513) {
		t.Errorf("summary line must be H2513 without a position: %v", last)
	}
	// Başka hatalar konumludur.
	wantPos(t, r.Errors[0], 1, 1)
}

func joinErrors(errs []error) string { return strings.Join(messages.Texts(errs), "\n") }

// speedFiles hız tabanının metinleridir: büyük, orta ve küçük bir dosya (cmd/tr/e2e'deki
// uçtan uca ölçüm de aynılarını kullanır).
var speedFiles = []string{"lib/katman.tr", "examples/tr/35_transformer.tr", "examples/tr/08_fonksiyonlar.tr"}

// BenchmarkPrepare aynı metnin kodsuz ve kodlu biçiminin tip denetimiyle hazırlanmasını ölçer;
// kodlu biçimin ek işi (Türkçe görünüm ve ikinci çözümleme) farkta görünür. Madde 15'in
// tabanıdır, ad kuralından ve görünümün kaldırılmasından sonra aynen koşar:
// go test -run XXX -bench Prepare -count 10 ./internal/lang/source/
func BenchmarkPrepare(b *testing.B) {
	for _, file := range speedFiles {
		data, err := os.ReadFile(reporoot.Path(file))
		if err != nil {
			b.Fatal(err)
		}
		coded, err := Encode(string(data))
		if err != nil {
			b.Fatal(err)
		}
		for _, form := range []struct{ name, text string }{{"plain", string(data)}, {"coded", coded}} {
			b.Run(path.Base(file)+"/"+form.name, func(b *testing.B) {
				for b.Loop() {
					if r := Prepare(form.text, Options{TypeCheck: true}); r.Stage != "" {
						b.Fatal(r.Errors)
					}
				}
			})
		}
	}
}
