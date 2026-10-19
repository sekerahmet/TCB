package dataset_test

import (
	"bytes"
	"context"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/dataload"
	"github.com/sekerahmet/TCB/internal/devtools/debugger"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	_ "github.com/sekerahmet/TCB/internal/stdlib"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/dataset"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// calistir TR kodunu tam dille (motorlar dahil) çalıştırır; çıktıyı ve VM'yi döndürür.
func calistir(t *testing.T, kd string) (string, *interp.VM, error) {
	t.Helper()
	p := parser.New(lexer.New(kd))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := interp.New()
	var out bytes.Buffer
	m.Out = &out
	_, err := m.Run(prog)
	m.WaitAsync()
	return out.String(), m, err
}

func mustCalistir(t *testing.T, kd string) (string, *interp.VM) {
	t.Helper()
	out, m, err := calistir(t, kd)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v\nkod:\n%s", err, kd)
	}
	return out, m
}

// tr bir yolu TR metin değişmezi içine yazılabilir biçime getirir.
func tr(yol string) string { return strings.ReplaceAll(filepath.ToSlash(yol), `"`, `\"`) }

// kumeKur iki parçalı, değerleri konumunu söyleyen bir küme yazar.
func kumeKur(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	// Windows'ta açık dosyalar silinemez: geçici klasör silinmeden önce kaynaklar kapanır.
	t.Cleanup(func() { dataset.TumunuKapat(time.Second) })
	yaz := func(ad string, v []int32) {
		var b bytes.Buffer
		if err := dataload.NpyYazI32(&b, v); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, ad), b.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for p, n := range []int{300, 500} {
		v := make([]int32, n)
		for i := range v {
			v[i] = int32(p*10000 + i)
		}
		yaz([]string{"p0.npy", "p1.npy"}[p], v)
	}
	return d
}

func degisken(t *testing.T, m *interp.VM, ad string) interp.Value {
	t.Helper()
	v, ok := m.GlobalEnv.Get(ad)
	if !ok {
		t.Fatalf("%s değişkeni yok", ad)
	}
	return v
}

func tensorVerisi(t *testing.T, v interp.Value) ([]float32, []int) {
	t.Helper()
	d, ok := v.(*ml.Deger)
	if !ok {
		t.Fatalf("tensör bekleniyordu, %T", v)
	}
	return d.Tensor().Data(), d.Tensor().Shape()
}

// Örnekleyici, aynı tohumla Go kütüphanesinin örnekleyicisiyle aynı toplu işleri verir;
// girdi [B, T], hedef bir kaydırılmış [B, T] tensördür.
func TestOrnekleyiciGoIleAyni(t *testing.T) {
	d := kumeKur(t)
	_, m := mustCalistir(t, `
değişken k = veri_kümesi("`+tr(d)+`")
değişken ö = k.örnekleyici(toplu: 4, bağlam: 16, tohum: 7)
değişken ilk = ö.sonraki()
değişken ikinci = ö.sonraki()
`)
	gk, err := dataload.KumeAc(d, dataload.KumeSecenekleri{})
	if err != nil {
		t.Fatal(err)
	}
	o, err := gk.Ornekleyici(dataload.OrnekleyiciSecenekleri{B: 4, T: 16, Tohum: 7})
	if err != nil {
		t.Fatal(err)
	}
	defer o.Kapat()
	for _, ad := range []string{"ilk", "ikinci"} {
		var ti dataload.TopluIs
		if err := o.Doldur(context.Background(), &ti); err != nil {
			t.Fatal(err)
		}
		l := degisken(t, m, ad).(*interp.ListVal)
		if len(l.Elements) != 2 {
			t.Fatalf("[girdi, hedef] bekleniyordu: %s", l.Inspect())
		}
		g, gs := tensorVerisi(t, l.Elements[0])
		h, hs := tensorVerisi(t, l.Elements[1])
		if len(gs) != 2 || gs[0] != 4 || gs[1] != 16 || len(hs) != 2 || hs[0] != 4 || hs[1] != 16 {
			t.Fatalf("şekiller %v %v", gs, hs)
		}
		for i := 0; i < 4; i++ {
			for j := 0; j < 16; j++ {
				if g[i*16+j] != float32(ti.Girdi(i)[j]) || h[i*16+j] != float32(ti.Hedef(i)[j]) {
					t.Fatalf("%s: satır %d sütun %d farklı", ad, i, j)
				}
			}
		}
	}
}

// Sıralı okuyucu tüm pencereleri sırayla verir, son toplu iş eksik satırlı olabilir,
// bitince yok döner ve kaynaklarını hemen kapatır.
func TestSiraliBitinceYokVeKapanir(t *testing.T) {
	d := kumeKur(t)
	taban := dataset.AcikKaynakSayisi()
	out, m := mustCalistir(t, `
değişken k = veri_kümesi("`+tr(d)+`")
değişken s = k.sıralı(toplu: 8, bağlam: 32)
değişken n = 0
değişken satır = 0
değişken son = yok
döngü {
    değişken x = s.sonraki()
    eğer x == yok { çık }
    n += 1
    satır += x[0].şekil[0]
    son = x
}
yazdır(n, satır, s.sonraki())
`)
	// p0: (300-33)/32+1 = 9 pencere, p1: (500-33)/32+1 = 15 pencere → 24 pencere, 3 toplu iş.
	if strings.TrimSpace(out) != "3 24 yok" {
		t.Fatalf("çıktı %q", out)
	}
	son := degisken(t, m, "son").(*interp.ListVal)
	g, gs := tensorVerisi(t, son.Elements[0])
	if gs[0] != 8 || gs[1] != 32 || g[0] != float32(10000+7*32) {
		t.Fatalf("son toplu iş: şekil %v ilk %v", gs, g[0])
	}
	if n := dataset.AcikKaynakSayisi(); n != taban {
		t.Fatalf("bitmiş sıralı okuyucunun kaynağı açık kaldı: %d > %d", n, taban)
	}
}

// Kullanılmayan okuyucular çöp toplayıcıyla, kalanlar TumunuKapat ile kapanır.
func TestKaynaklarKapanir(t *testing.T) {
	d := kumeKur(t)
	dataset.TumunuKapat(time.Second)
	taban := dataset.AcikKaynakSayisi()
	_, m := mustCalistir(t, `
değişken k = veri_kümesi("`+tr(d)+`")
fonksiyon geçici() {
    değişken ö = k.örnekleyici(toplu: 2, bağlam: 8)
    ö.sonraki()
}
geçici()
geçici()
değişken kalıcı = k.örnekleyici(toplu: 2, bağlam: 8)
kalıcı.sonraki()
`)
	son := time.Now().Add(5 * time.Second)
	for dataset.AcikKaynakSayisi() > taban+1 {
		if time.Now().After(son) {
			t.Fatalf("erişilemez okuyucular kapanmadı: açık %d, beklenen %d", dataset.AcikKaynakSayisi(), taban+1)
		}
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	runtime.KeepAlive(m)
	dataset.TumunuKapat(time.Second)
	if n := dataset.AcikKaynakSayisi(); n != taban {
		t.Fatalf("TumunuKapat sonrası açık kaynak %d, beklenen %d", n, taban)
	}
	// Kapatılmış okuyucu açık bir hata verir, çökmez.
	_, err := m.ApplyFunction(mustUye(t, degisken(t, m, "kalıcı"), "sonraki"), nil, nil, 1)
	if err == nil || !strings.Contains(err.Error(), "kapatılmış") {
		t.Fatalf("kapatılmış okuyucu hatası bekleniyordu: %v", err)
	}
}

func mustUye(t *testing.T, v interp.Value, ad string) interp.Value {
	t.Helper()
	pg, ok := v.(interp.PropertyGetter)
	if !ok {
		t.Fatalf("%T üyesi yok", v)
	}
	u, err := pg.GetProperty(codes.UyeKodu(ad), ad)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// Aynı okuyucuyu kullanan eşzamanlı görevler toplu işleri paylaşır; hiçbiri kaybolmaz.
func TestEszamanliGorevler(t *testing.T) {
	d := kumeKur(t)
	out, _ := mustCalistir(t, `
değişken k = veri_kümesi("`+tr(d)+`")
değişken s = k.sıralı(toplu: 1, bağlam: 16)
değişken sayaç = [0, 0, 0]
döngü g içinde 0..3 {
    eşzamanlı {
        döngü s.sonraki() != yok { sayaç[g] += 1 }
    }
}
bekle_hepsi()
yazdır(sayaç[0] + sayaç[1] + sayaç[2])
`)
	// p0: (300-17)/16+1 = 18, p1: (500-17)/16+1 = 31.
	if strings.TrimSpace(out) != "49" {
		t.Fatalf("çıktı %q", out)
	}
}

// .sonraki() beklerken yorumlayıcı kilidi bırakılır: kilit tutulsaydı başka bir görev
// bu sırada ilerleyemezdi. Önden yükleme gorutini kilidi hiç almaz.
func TestSonrakiKilidiBirakir(t *testing.T) {
	d := kumeKur(t)
	out, _ := mustCalistir(t, `
değişken k = veri_kümesi("`+tr(d)+`")
değişken s = k.sıralı(toplu: 1, bağlam: 16)
değişken adım = 0
eşzamanlı {
    döngü i içinde 0..200 { adım += 1 }
}
döngü s.sonraki() != yok { }
bekle_hepsi()
yazdır(adım)
`)
	if strings.TrimSpace(out) != "200" {
		t.Fatalf("çıktı %q", out)
	}
}

// Tensörler gömme ve çapraz_entropi'ye doğrudan verilebilir.
func TestGommeVeCaprazEntropi(t *testing.T) {
	d := kumeKur(t)
	out, _ := mustCalistir(t, `
değişken k = veri_kümesi("`+tr(d)+`")
değişken ö = k.örnekleyici(toplu: 2, bağlam: 8, tohum: 1)
değişken girdi, hedef = ö.sonraki()
değişken E = rastgele_normal([20000, 4], tohum: 1)
değişken W = rastgele_normal([4, 20000], tohum: 2)
değişken çıktı = gömme(E, girdi) @ W
yazdır(çıktı.şekil, tip(çapraz_entropi(çıktı, hedef)), girdi.şekil, hedef.şekil)
`)
	if strings.TrimSpace(out) != "[2, 8, 20000] Tensör [2, 8] [2, 8]" {
		t.Fatalf("çıktı %q", out)
	}
}

func TestOnbellek(t *testing.T) {
	d := kumeKur(t)
	ob := filepath.Join(t.TempDir(), "onb")
	out, _ := mustCalistir(t, `
değişken k = veri_kümesi("`+tr(d)+`", önbellek: "`+tr(ob)+`", kota_gb: 0.001)
değişken x = k.örnekleyici(toplu: 2, bağlam: 8).sonraki()
yazdır(x[0].şekil)
`)
	if strings.TrimSpace(out) != "[2, 8]" {
		t.Fatalf("çıktı %q", out)
	}
	_, _, err := calistir(t, `veri_kümesi("`+tr(d)+`", önbellek: "`+tr(ob)+`", kota_gb: 0.000000001).örnekleyici(toplu: 2, bağlam: 8)`)
	if err == nil || !strings.Contains(err.Error(), "önbellek kotası yetersiz") {
		t.Fatalf("kota hatası bekleniyordu: %v", err)
	}
	dataset.TumunuKapat(time.Second)
}

func TestHataMesajlari(t *testing.T) {
	d := kumeKur(t)
	bos := t.TempDir()
	bozuk := filepath.Join(bos, "bozuk.npy")
	if err := os.WriteFile(bozuk, []byte("merhaba"), 0o644); err != nil {
		t.Fatal(err)
	}
	durumlar := []struct{ kod, beklenen string }{
		{`veri_kümesi("bu_klasor_yok_123")`, "açılamadı: böyle bir dosya ya da klasör yok"},
		{`veri_kümesi("` + tr(t.TempDir()) + `")`, ".npy veri dosyası yok"},
		{`veri_kümesi("` + tr(bozuk) + `")`, "geçerli bir .npy veri dosyası değil"},
		{`veri_kümesi("` + tr(d) + `", kota_gb: 1)`, "yalnızca önbellek: ile birlikte"},
		{`veri_kümesi("` + tr(d) + `", önbellek: "x")`, "kota_gb: de verilmelidir"},
		{`veri_kümesi("` + tr(d) + `", onbelek: "x")`, "Bunu mu demek istediniz: 'önbellek'"},
		{`veri_kümesi("` + tr(d) + `").örnekleyici(toplu: 2)`, "bağlam: verilmedi"},
		{`veri_kümesi("` + tr(d) + `").örnekleyici(toplu: 0, bağlam: 4)`, "toplu pozitif bir tam sayı"},
		{`veri_kümesi("` + tr(d) + `").örnekleyici(toplu: 2, bağlam: 4, tohum: -1)`, "tohum negatif olmayan"},
		{`veri_kümesi("` + tr(d) + `").sıralı(toplu: 2, bağlam: 4, tohum: 1)`, "'tohum' adında bir parametre almaz"},
		{`veri_kümesi("` + tr(d) + `").örnekleyici(toplu: 2, bağlam: 600)`, "en uzun veri dosyası 500 parça"},
		{`veri_kümesi("` + tr(d) + `").karıştır()`, "üyeler: örnekleyici, sıralı"},
	}
	for _, dd := range durumlar {
		_, _, err := calistir(t, dd.kod)
		if err == nil || !strings.Contains(err.Error(), dd.beklenen) {
			t.Errorf("%s\n hata: %v\n beklenen: %q", dd.kod, err, dd.beklenen)
		}
	}
	// Hatalar dene/yakala ile yakalanabilir.
	out, _ := mustCalistir(t, `dene { veri_kümesi("bu_klasor_yok_123") } yakala h { yazdır("yakalandı") }`)
	if strings.TrimSpace(out) != "yakalandı" {
		t.Fatalf("çıktı %q", out)
	}
}

// Göreli yollar çalışma klasöründe yoksa programın klasörüne göre çözülür.
func TestGoreliYolProgramKlasoru(t *testing.T) {
	d := kumeKur(t)
	p := parser.New(lexer.New(`değişken k = veri_kümesi("` + filepath.Base(d) + `")`))
	prog := p.ParseProgram()
	m := interp.New()
	m.Out = &bytes.Buffer{}
	m.SourceFile = filepath.Join(filepath.Dir(d), "program.tr")
	if _, err := m.Run(prog); err != nil {
		t.Fatal(err)
	}
}

func TestIzlemeMasasi(t *testing.T) {
	d := kumeKur(t)
	_, m := mustCalistir(t, `
değişken k = veri_kümesi("`+tr(d)+`")
değişken ö = k.örnekleyici(toplu: 2, bağlam: 8)
değişken s = k.sıralı(toplu: 2, bağlam: 8)
`)
	beklenen := map[string]string{
		"k": "Veri kümesi (2 veri dosyası, 800 parça)",
		"ö": "Örnekleyici (2×8, dönem 0, 0 toplu iş)",
		"s": "Sıralı okuyucu (2×8, 0 toplu iş okundu)",
	}
	for ad, etiket := range beklenen {
		iz, ok := degisken(t, m, ad).(debugger.IzlemeDegeri)
		if !ok {
			t.Fatalf("%s IzlemeDegeri değil", ad)
		}
		if iz.IzlemeEtiketi() != etiket {
			t.Errorf("%s: etiket %q, beklenen %q", ad, iz.IzlemeEtiketi(), etiket)
		}
	}
	cols, rows, err := degisken(t, m, "k").(debugger.IzlemeDegeri).TabloVerisi()
	if err != nil || len(cols) != 3 || len(rows) != 2 || rows[1][0] != "p1.npy" || rows[1][1] != "500" {
		t.Fatalf("tablo: %v %v %v", cols, rows, err)
	}
	if s := bind.KisaSayi(56_900_000_000); s != "56,9 milyar" {
		t.Fatalf("KisaSayi: %q", s)
	}
	dataset.TumunuKapat(time.Second)
}
