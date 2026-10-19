package bind

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

func adlar(kodlar ...codes.Kod) []Ad {
	var a []Ad
	for _, k := range kodlar {
		a = append(a, KodAd(k))
	}
	return a
}

// KodAd yalnızca adlı argüman kodlarını (x9xx) kabul eder; yanlış kod başlangıçta panikler.
func TestKodAd(t *testing.T) {
	a := KodAd(codes.T910)
	if a.TR != codes.Ad(codes.T910) || !a.Kabul(a.TR) || a.Yazimlar()[0] != a.TR {
		t.Fatalf("KodAd(T910) = %+v", a)
	}
	defer func() {
		if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), "G100") {
			t.Fatalf("adlı argüman olmayan kod paniklemeli: %v", r)
		}
	}()
	KodAd(codes.G100)
}

// AdliDenetle bilinmeyen adda öneriyi şu sırayla arar: (1) kod listesindeki öneri (ASCII
// yazım, eski ad), yalnızca bu fonksiyonun adlarındansa; (2) başka dillerden alışılmış ad
// (büyük-küçük harf duyarsız), yine yalnızca bu fonksiyonun adlarındansa; (3) yazım
// yakınlığı. Öneri yoksa geçerli adlar listelenir; fonksiyon adlı argüman almıyorsa söylenir.
func TestAdliDenetleOneriSirasi(t *testing.T) {
	ogrenme, eksen, epsilon := codes.Ad(codes.T910), codes.Ad(codes.T902), codes.Ad(codes.T905)
	kaldirilan, ok := codes.OluBul(codes.AlanParametre, "nerede") // kaldırılmış bir adlı argümanın eski adı
	if !ok || !codes.Kaldirildi(kaldirilan) {
		t.Fatal("'nerede' kaldırılmış bir adlı argümanın eski adı olmalı; sınama verisini güncelleyin")
	}
	durumlar := []struct {
		ad      string
		verilen string
		adlar   []Ad
		oneri   string   // boşsa öneri yok
		icerir  []string // mesajda geçmesi gerekenler (öneri yoksa)
		icermez []string
	}{
		{ad: "bilinen ad", verilen: ogrenme, adlar: adlar(codes.T910, codes.T911)},
		{ad: "ASCII yazım", verilen: "ogrenme_orani", adlar: adlar(codes.T910, codes.T911), oneri: ogrenme},
		{ad: "eski ad", verilen: "öğrenme_hızı", adlar: adlar(codes.T910), oneri: ogrenme},
		{ad: "yabancı ad", verilen: "lr", adlar: adlar(codes.T910, codes.T911), oneri: ogrenme},
		{ad: "yabancı ad büyük harfle", verilen: "LR", adlar: adlar(codes.T910), oneri: ogrenme},
		{ad: "yabancı ad yazım yakınlığından önce", verilen: "eps", adlar: adlar(codes.T905, codes.T902), oneri: epsilon},
		{ad: "yazım hatası", verilen: "egsen", adlar: adlar(codes.T902, codes.T903), oneri: eksen},
		{ad: "yabancı adın karşılığı bu fonksiyonda yok", verilen: "lr", adlar: adlar(codes.T902),
			icerir: []string{eksen}, icermez: []string{ogrenme}},
		{ad: "kaldırılmış kodun eski adı önerilmez", verilen: "nerede", adlar: adlar(codes.T902),
			icerir: []string{eksen}, icermez: []string{"'" + codes.Ad(kaldirilan) + "'"}},
		{ad: "adlı argüman almayan fonksiyon", verilen: "eksen", adlar: nil, icerir: []string{"almaz"}},
	}
	for _, d := range durumlar {
		t.Run(d.ad, func(t *testing.T) {
			deger := map[string]interp.Value{d.verilen: &interp.IntVal{Val: 1}}
			err := AdliDenetle("deneme", deger, d.adlar...)
			if d.oneri == "" && d.icerir == nil {
				if err != nil {
					t.Fatalf("bilinen ad reddedildi: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("%q kabul edildi; hata bekleniyordu", d.verilen)
			}
			m := err.Error()
			if !strings.Contains(m, "deneme") || !strings.Contains(m, "'"+d.verilen+"'") {
				t.Errorf("mesaj fonksiyonu ve verilen adı söylemeli: %s", m)
			}
			if d.oneri != "" && !strings.Contains(m, "'"+d.oneri+"'?") {
				t.Errorf("öneri %q bekleniyordu: %s", d.oneri, m)
			}
			if d.oneri == "" && strings.Contains(m, "?") {
				t.Errorf("öneri beklenmiyordu: %s", m)
			}
			for _, p := range d.icerir {
				if !strings.Contains(m, p) {
					t.Errorf("mesajda %q yok: %s", p, m)
				}
			}
			for _, p := range d.icermez {
				if strings.Contains(m, p) {
					t.Errorf("mesajda %q olmamalı: %s", p, m)
				}
			}
		})
	}
}

func TestAdliAl(t *testing.T) {
	a := KodAd(codes.T902)
	v := &interp.IntVal{Val: 3}
	if g, ok := AdliAl(map[string]interp.Value{a.TR: v}, a); !ok || g != v {
		t.Fatalf("AdliAl = %v, %v", g, ok)
	}
	if _, ok := AdliAl(map[string]interp.Value{"başka": v}, a); ok {
		t.Fatal("verilmeyen adlı argüman bulundu")
	}
}

func TestTamSayiVeMantik(t *testing.T) {
	for _, d := range []struct {
		v  interp.Value
		n  int64
		ok bool
	}{
		{&interp.IntVal{Val: -7}, -7, true},
		{&interp.FloatVal{Val: 4}, 4, true},
		{&interp.FloatVal{Val: 4.5}, 0, false},
		{&interp.StringVal{Val: "4"}, 0, false},
	} {
		if n, ok := TamSayi(d.v); n != d.n || ok != d.ok {
			t.Errorf("TamSayi(%s) = %d, %v; %d, %v olmalı", d.v.Inspect(), n, ok, d.n, d.ok)
		}
	}
	if b, err := Mantik("f", "x", &interp.BoolVal{Val: true}); err != nil || !b {
		t.Errorf("Mantik(doğru) = %v, %v", b, err)
	}
	if _, err := Mantik("f", "x", &interp.IntVal{Val: 1}); err == nil || !strings.Contains(err.Error(), "f: x") {
		t.Errorf("Mantik(1) hata vermeli: %v", err)
	}
}

// Kurtar ve Guvenli: Go paniği programı çökertmez, TR'nin yakalayabildiği bir hataya döner.
func TestPanikHatayaCevrilir(t *testing.T) {
	cevir := func(r any) error { return fmt.Errorf("çevrildi: %v", r) }
	if s, err := Kurtar(func() int { return 5 }, cevir); s != 5 || err != nil {
		t.Errorf("paniksiz Kurtar = %d, %v", s, err)
	}
	if s, err := Kurtar(func() int { panic("dizin taştı") }, cevir); s != 0 || err == nil || err.Error() != "çevrildi: dizin taştı" {
		t.Errorf("panikli Kurtar = %d, %v", s, err)
	}

	beklenen := errors.New("sıradan hata")
	if _, err := Guvenli("fn", func() (interp.Value, error) { return nil, beklenen }); err != beklenen {
		t.Errorf("Guvenli fonksiyonun hatasını olduğu gibi döndürmeli: %v", err)
	}
	if v, err := Guvenli("fn", func() (interp.Value, error) { return &interp.IntVal{Val: 2}, nil }); err != nil || v.Inspect() != "2" {
		t.Errorf("Guvenli = %v, %v", v, err)
	}
	_, err := Guvenli("fn", func() (interp.Value, error) {
		var s []int
		_ = s[3]
		return nil, nil
	})
	if err == nil || !strings.HasPrefix(err.Error(), "fn: beklenmeyen iç hata: ") {
		t.Errorf("Guvenli paniği iç hataya çevirmeli: %v", err)
	}
}

// Kesilme (Ctrl+C) bir hata değildir: Kurtar onu yutmaz, yığın çözülmeye devam eder ve
// görev kesilmiş olarak biter.
func TestKurtarKesilmeyiYutmaz(t *testing.T) {
	m := interp.New()
	cevrildi := false
	m.GlobalEnv.Define("dene_kurtar", &interp.BuiltinVal{Name: "dene_kurtar", Fn: func(y *interp.VM, _ []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		y.Kes()
		_, _ = Kurtar(func() int {
			y.Blocking(func() {}) // kilit geri alınınca kesilme paniği atılır
			return 1
		}, func(any) error { cevrildi = true; return errors.New("yutuldu") })
		return interp.NULL, nil
	}}, true)
	_, err := m.Run(parser.New(lexer.New("dene_kurtar()")).ParseProgram())
	if cevrildi {
		t.Fatal("Kurtar kesilmeyi hataya çevirdi")
	}
	if !errors.Is(err, interp.ErrKesildi) {
		t.Fatalf("görev kesilmiş olarak bitmeli: %v", err)
	}
}

// YolCoz göreli yolu önce çalışma klasörüne, sonra programın klasörüne göre çözer; hiçbir
// yerde yoksa çalışma klasörüne göre yol döner (hata mesajı kullanıcının yazdığını göstersin).
func TestYolCoz(t *testing.T) {
	calisma, program := t.TempDir(), t.TempDir()
	t.Chdir(calisma)
	yaz := func(klasor, ad string) {
		if err := os.WriteFile(filepath.Join(klasor, ad), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	yaz(calisma, "ikisinde.txt")
	yaz(program, "ikisinde.txt")
	yaz(program, "yalnız_programda.txt")
	m := interp.New()
	m.SourceFile = filepath.Join(program, "betik.tr")

	if g := YolCoz(m, "ikisinde.txt"); g != "ikisinde.txt" {
		t.Errorf("çalışma klasörü önce gelmeli: %q", g)
	}
	if g := YolCoz(m, "yalnız_programda.txt"); g != filepath.Join(program, "yalnız_programda.txt") {
		t.Errorf("programın klasörüne bakılmalı: %q", g)
	}
	g, bakilan := YolCozBakilan(m, "yok.txt")
	if g != "yok.txt" || len(bakilan) != 2 {
		t.Errorf("olmayan dosya çalışma klasörüne göre dönmeli ve iki yere bakılmalı: %q %v", g, bakilan)
	}
	if g := YolCoz(nil, "yalnız_programda.txt"); g != "yalnız_programda.txt" {
		t.Errorf("VM yoksa yalnızca çalışma klasörüne bakılır: %q", g)
	}
	mutlak := filepath.Join(program, "yalnız_programda.txt")
	if g, b := YolCozBakilan(m, mutlak); g != mutlak || b != nil {
		t.Errorf("mutlak yol olduğu gibi kalmalı: %q %v", g, b)
	}

	_, err := os.Open(YolCoz(m, "yok.txt"))
	if !YokMu(err) || !YokMu(fmt.Errorf("sarılı: %w", err)) || YokMu(fs.ErrPermission) {
		t.Errorf("YokMu yalnızca 'dosya yok' hatasını tanımalı: %v", err)
	}
}

func TestKisaSayiVeBaytMetni(t *testing.T) {
	for n, b := range map[int64]string{
		0: "0", 999: "999", 1000: "1 bin", 12_345: "12,3 bin", 2_500_000: "2,5 milyon",
		56_900_000_000: "56,9 milyar", 3_000_000_000_000: "3 trilyon",
	} {
		if g := KisaSayi(n); g != b {
			t.Errorf("KisaSayi(%d) = %q; %q olmalı", n, g, b)
		}
	}
	for n, b := range map[int64]string{
		0: "0 bayt", 1023: "1023 bayt", 1024: "1 KB", 1536: "1,5 KB",
		5 << 20: "5 MB", 1288490189: "1,2 GB", 2 << 40: "2 TB",
	} {
		if g := BaytMetni(n); g != b {
			t.Errorf("BaytMetni(%d) = %q; %q olmalı", n, g, b)
		}
	}
}
