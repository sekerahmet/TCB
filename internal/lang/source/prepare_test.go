package source

import (
	"errors"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
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

// Kodlu dosya çevrilmeden hazırlanır. Konum dosyanındır: satır dosyanın satırıdır (başlık 1. satır,
// 2.1), sütun ve hata çerçevesinin metni diskteki metindendir (kimlik yazımıyla, TR_print).
func TestCodedFilePrepares(t *testing.T) {
	s := Prepare(Header+"\nTR_var x = 1\nTR_print(x, tanımsız)\n", Options{TypeCheck: true})
	if s.Stage != StageType || len(s.Errors) != 1 {
		t.Fatalf("tanımsız ad bildirilmeli: %v", s.Errors)
	}
	wantPos(t, s.Errors[0], 3, 13) // dosyadaki konum
	if s.Text != Header+"\nTR_var x = 1\nTR_print(x, tanımsız)\n" {
		t.Errorf("hata çerçevesinin metni dosyanın kendi metni olmalı: %q", s.Text)
	}
	// Yerleştirmedeki kodlar da dosyada yazıldıkları gibi sayılır.
	s = Prepare(Header+"\nTR_print(\"\\(TR_sum([1])) \\(yok_ad)\")\n", Options{TypeCheck: true})
	if len(s.Errors) != 1 {
		t.Fatalf("yerleştirmedeki ad bildirilmeli: %v", s.Errors)
	}
	wantPos(t, s.Errors[0], 2, 28)
	// Gösterimde '$' ile kaçırılacak kullanıcı adı da dosyadaki yerindedir.
	s = Prepare(Header+"\nTR_var eğer = 1 + tanımsız\n", Options{TypeCheck: true})
	if s.Text != Header+"\nTR_var eğer = 1 + tanımsız\n" || len(s.Errors) != 1 {
		t.Fatalf("metin %q, hatalar %v", s.Text, s.Errors)
	}
	wantPos(t, s.Errors[0], 2, 19)
	// Kod olmayan her ad kullanıcının adıdır, anahtar kelime yazımında bile.
	s = Prepare(Header+"\nTR_var eğer = 1\nTR_print(eğer)\n", Options{TypeCheck: true})
	if s.Stage != "" {
		t.Errorf("kodlu dosyada 'eğer' bir kullanıcı adıdır: %v", s.Errors)
	}
}

func TestCodedFileErrors(t *testing.T) {
	cases := map[string]string{
		Header + "\nTR_yok(1)\n": "'TR_yok' bilinen bir kimlik değil",
	}
	for src, part := range cases {
		s := Prepare(src, Options{TypeCheck: true})
		if !strings.Contains(joinErrors(s.Errors), part) {
			t.Errorf("%q: %q içeren hata bekleniyordu: %v", src, part, s.Errors)
		}
	}
	// Başlıksız metin dosya değildir (2.1): hata başlığı gösterir.
	s := Prepare("TR_print(1)\n", Options{})
	if s.Stage != StageSyntax || !strings.Contains(s.Errors[0].Error(), Header) {
		t.Errorf("başlıksız dosya hatası başlığı göstermeli: %v", s.Errors)
	}
	// Ayrıştırılamayan kaynağın görünümü yazılmaz.
	var sh *SyntaxError
	if _, err := Show(Header+"\nTR_if {\n", codes.Source()); !errors.As(err, &sh) {
		t.Errorf("sözdizimi hatası bekleniyordu: %v", err)
	}
}

// Kural (17.7): kabuğa yazılan satır kişinin yazdığı koddur, dosya gibi yalnız kimlik yazımıyla
// okunur (Options.Body); bir yazım dilinin sözcüğü kullanıcının adıdır. Çerçevenin metni satırın
// kendisidir. Neden: kod yalnız kimliklerle yazılır, görünüm koda geri çevrilmez.
func TestTypedLineIsReadWithIdentities(t *testing.T) {
	if s := Prepare("TR_var x = 2\nTR_print(x)", Options{Body: true, TypeCheck: true}); s.Stage != "" {
		t.Errorf("kimlik yazımıyla satır okunmalı: %v", s.Errors)
	}
	s := Prepare("yazdır(1)", Options{Body: true, TypeCheck: true})
	if s.Stage != StageType || len(s.Errors) != 1 || s.Text != "yazdır(1)" {
		t.Fatalf("Türkçe adı tanımsız ad olmalı: %q %v", s.Text, s.Errors)
	}
	wantPos(t, s.Errors[0], 1, 1)
}

// A file with thousands of errors (often not a TR file at all) reports the first MaxErrors and
// how many more there are; reporting every error with a code frame took minutes.
func TestPrepareLimitsErrors(t *testing.T) {
	r := Prepare(strings.Repeat("'tek tırnak'\n", 500), Options{Body: true})
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

// Kural: tanılar dosyadaki sırayla, satır ve sütuna göre verilir (sözcük çözümleyici, ayrıştırıcı ve
// denetleyici buldukları sırayla bildirir). Neden: ajan ve kişi iletileri dosyayı yukarıdan aşağı
// okur gibi izlesin.
func TestErrorsAreInFileOrder(t *testing.T) {
	r := Prepare(Header+"\nTR_var a = 0X1F\nTR_var b = 1_\nTR_var c = 99999999999999999999\nTR_var d = .5\n", Options{})
	if len(r.Errors) != 4 {
		t.Fatalf("dört hata bekleniyordu: %v", r.Errors)
	}
	for i, err := range r.Errors {
		wantPos(t, err, i+2, 12)
	}
}

func joinErrors(errs []error) string { return strings.Join(messages.Texts(errs), "\n") }

// speedFiles hız tabanının metinleridir: büyük, orta ve küçük bir dosya (cmd/tr/e2e'deki
// uçtan uca ölçüm de aynılarını kullanır).
var speedFiles = []string{"lib/layers.tr", "examples/tr/35_transformer.tr", "examples/tr/08_fonksiyonlar.tr"}

// BenchmarkPrepare kodlu dosyanın tip denetimiyle hazırlanmasını ölçer (gösterimin kurulması dahil).
// Madde 15'in tabanıdır:
// go test -run XXX -bench Prepare -count 10 ./internal/lang/source/
func BenchmarkPrepare(b *testing.B) {
	for _, file := range speedFiles {
		data, err := os.ReadFile(reporoot.Path(file))
		if err != nil {
			b.Fatal(err)
		}
		b.Run(path.Base(file), func(b *testing.B) {
			for b.Loop() {
				if r := Prepare(string(data), Options{TypeCheck: true}); r.Stage != "" {
					b.Fatal(r.Errors)
				}
			}
		})
	}
}
