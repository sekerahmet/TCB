package checkpoint_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/stdlib"
	"github.com/sekerahmet/TCB/internal/stdlib/checkpoint"
)

// esliYazici bir tamponu birden çok çıkışa paylaştırır.
type esliYazici struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (e *esliYazici) Write(p []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.b.Write(p)
}

func (e *esliYazici) String() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.b.String()
}

// calistir kodu yeni bir VM'de (yeni bir program gibi) çalıştırır; standart çıktıyı ve
// hatayı birlikte verir.
func calistir(t *testing.T, kod string) (string, error) {
	t.Helper()
	m := interp.New()
	w := &esliYazici{}
	m.Out, m.Err = w, w
	_, err := calistirVM(t, m, kod)
	return w.String(), err
}

// calistirVM kodu verilen VM'de bir program olarak çalıştırır (program dosyası depo
// kökündedir) ve motorların kaynaklarını kapatır.
func calistirVM(t *testing.T, m *interp.VM, kod string) (interp.Value, error) {
	t.Helper()
	m.SourceFile = filepath.Join(reporoot.Root(), "egit.tr")
	p := parser.New(lexer.New(kod))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v\n%s", p.Errors(), kod)
	}
	v, err := m.Run(prog)
	m.WaitAsync()
	stdlib.Kapat()
	return v, err
}

func trYol(s string) string { return filepath.ToSlash(s) }

// egitimProgrami küçük bir modeli kontrol noktasıyla dur adıma kadar eğitir (toplam adımlık
// bir koşunun parçası). İki model (AdamW ve momentumlu SGD), örnekleyici, sıralı okuyucu,
// günlük ve düz değerler noktaya girer; sonunda parametreler ve iyileştirici durumları
// karşılaştırma için kaydedilir.
func egitimProgrami(kosu string, dur, toplam int, ekSecenek string) string {
	return fmt.Sprintf(`
değişken KOŞU = %q
değişken V = 4096
değişken D = 16
değişken sayaç = 0
fonksiyon parametre(şekil, ölçek) {
    sayaç += 1
    sonuç tensör(türevsiz(rastgele_normal(şekil, tohum: sayaç) * ölçek), türev_izle: doğru)
}
değişken model = {E: parametre([V, D], 0.1), W: parametre([D, V], 0.1)}
değişken ikinci = [parametre([D], 1.0)]
değişken op = AdamW(parametreler(model), öğrenme_oranı: 0.01)
değişken op2 = SGD(ikinci, öğrenme_oranı: 0.1, momentum: 0.9)
değişken küme = veri_kümesi(%q)
değişken ö = küme.örnekleyici(toplu: 4, bağlam: 8, tohum: 5)
değişken s = küme.sıralı(toplu: 2, bağlam: 8)
değişken g = günlük(yol(KOŞU, "metrics.jsonl"))
değişken izlenen = {en_iyi: 99.0, geçmiş: [], son_sıralı: yok}
değişken kn = kontrol_noktası(KOŞU, {model: model, op: op, ikinci: ikinci, op2: op2, ö: ö, s: s, g: g, izlenen: izlenen}%s)
değişken ç = ilerleme(%d, günlük: g)
döngü adım içinde kn.kaldığı_adım..%d {
    değişken giriş, hedef = ö.sonraki()
    op.sıfırla()
    op2.sıfırla()
    değişken kayıp = çapraz_entropi(gömme(model.E, giriş) @ model.W, hedef) + toplam(ikinci[0] * ikinci[0]) * 0.01
    geri_yayılım(kayıp)
    op.adım()
    op2.adım()
    değişken d = s.sonraki()
    eğer d != yok { izlenen.son_sıralı = d[0].değer[0][0] }
    eğer kayıp.değer < izlenen.en_iyi { izlenen.en_iyi = kayıp.değer }
    izlenen.geçmiş.ekle(adım)
    ç.güncelle({kayıp: kayıp}, adım: adım + 1)
    kn.güncelle(adım + 1)
}
tensör_kaydet(yol(KOŞU, "son.trt"), parametreler([model, ikinci], adlarıyla: doğru))
tensör_kaydet(yol(KOŞU, "op.trt"), op.durum)
tensör_kaydet(yol(KOŞU, "op2.trt"), op2.durum)
yazdır("izlenen:", izlenen)
yazdır("ö:", ö.durum)
`, trYol(kosu), trYol(filepath.Join(reporoot.Root(), "tests", "data", "text")), ekSecenek, toplam, dur)
}

// gecerliOlculer günlüğün geçerli ölçü kayıtlarını (adım → kayıp) okur: bir devam kaydından
// önce gelen ve adımı ondan büyük olan kayıtlar geçersizdir (19, günlük okuyucu kuralı).
func gecerliOlculer(t *testing.T, yol string) []string {
	t.Helper()
	veri, err := os.ReadFile(yol)
	if err != nil {
		t.Fatal(err)
	}
	type olcu struct {
		adim  int64
		kayip string
	}
	var olculer []olcu
	for _, s := range strings.Split(strings.TrimSpace(string(veri)), "\n") {
		var k map[string]json.RawMessage
		if err := json.Unmarshal([]byte(s), &k); err != nil {
			t.Fatalf("geçersiz satır %q", s)
		}
		var adim int64
		_ = json.Unmarshal(k["adım"], &adim)
		var tur string
		_ = json.Unmarshal(k["tür"], &tur)
		switch tur {
		case "devam":
			olculer = slices.DeleteFunc(olculer, func(o olcu) bool { return o.adim > adim })
		case "":
			olculer = append(olculer, olcu{adim, string(k["kayıp"])})
		}
	}
	var out []string
	for _, o := range olculer {
		out = append(out, fmt.Sprintf("%d:%s", o.adim, o.kayip))
	}
	return out
}

// sonuclarAyni iki koşunun parametre, iyileştirici dosyalarının, geçerli kayıp dizisinin ve
// programın son çıktısının (düz değerler, okuyucu durumu) bit düzeyinde aynı olduğunu denetler.
func sonuclarAyni(t *testing.T, a, b, ciktiA, ciktiB string) {
	t.Helper()
	for _, ad := range []string{"son.trt", "op.trt", "op2.trt"} {
		x, err1 := os.ReadFile(filepath.Join(a, ad))
		y, err2 := os.ReadFile(filepath.Join(b, ad))
		if err1 != nil || err2 != nil {
			t.Fatalf("%s okunamadı: %v %v", ad, err1, err2)
		}
		if !bytes.Equal(x, y) {
			t.Errorf("%s bit düzeyinde aynı değil", ad)
		}
	}
	ka, kb := gecerliOlculer(t, filepath.Join(a, "metrics.jsonl")), gecerliOlculer(t, filepath.Join(b, "metrics.jsonl"))
	if !slices.Equal(ka, kb) {
		t.Errorf("geçerli kayıp dizileri farklı:\n%v\n%v", ka, kb)
	}
	son := func(c string) string { return c[strings.LastIndex(c, "izlenen:"):] }
	if son(ciktiA) != son(ciktiB) {
		t.Errorf("son durum farklı:\n%s\n%s", son(ciktiA), son(ciktiB))
	}
}

// Kesintisiz eğitim ile kesilip (aralıklı kayıttan, kaydedilmemiş adımlar yeniden
// hesaplanarak) sürdürülen eğitim aynı son adımda bit düzeyinde aynı parametreleri,
// iyileştirici durumlarını, kayıp dizisini, okuyucu konumlarını ve düz değerleri verir.
func TestKesintisizleSurdurulenAyni(t *testing.T) {
	var simdi time.Time
	t.Cleanup(checkpoint.SaatTaklidi(func() time.Time { simdi = simdi.Add(time.Minute); return simdi }))
	// Her güncelle'de saat 1 dk ilerler: her_dakika: 3 ile yaklaşık üç adımda bir kayıt.
	a := t.TempDir()
	ciktiA, err := calistir(t, egitimProgrami(a, 24, 24, ", her_dakika: 3"))
	if err != nil {
		t.Fatalf("%v\n%s", err, ciktiA)
	}
	b := t.TempDir()
	if c, err := calistir(t, egitimProgrami(b, 13, 24, ", her_dakika: 3")); err != nil {
		t.Fatalf("%v\n%s", err, c)
	}
	ciktiB, err := calistir(t, egitimProgrami(b, 24, 24, ", her_dakika: 3"))
	if err != nil {
		t.Fatalf("%v\n%s", err, ciktiB)
	}
	if !strings.Contains(ciktiB, "Kaldığı yerden devam: adım 1") || strings.Contains(ciktiB, "adım 13 (") {
		t.Fatalf("13. adımdan önceki son aralıklı noktadan sürmeliydi:\n%s", ciktiB)
	}
	sonuclarAyni(t, a, b, ciktiA, ciktiB)
	// tut: 2 → en yeni iki nokta kalır.
	girdiler, _ := os.ReadDir(filepath.Join(b, "kontrol_noktalari"))
	var noktalar []string
	for _, g := range girdiler {
		noktalar = append(noktalar, g.Name())
	}
	if len(noktalar) != 3 || noktalar[2] != "son.json" {
		t.Errorf("iki nokta ve son.json bekleniyordu: %v", noktalar)
	}
}

// Ağ sürücüsü (FUSE) benzetimi: yeniden adlandırma başarısız olunca aralıklı kayıt eğitimi
// durdurmaz (uyarı, sonraki aralıkta yeniden deneme), önceki geçerli nokta bozulmaz, geçici
// klasör kalmaz; art arda üçüncü başarısızlıkta açık hata verilir. Yeniden adlandırma
// düzelince kayıt sürer ve sayaç sıfırlanır.
func TestYenidenAdlandirmaHatasiEgitimiDurdurmaz(t *testing.T) {
	var bozuk atomicBool
	t.Cleanup(checkpoint.YenidenAdlandirmaTaklidi(func(eski, yeni string) error {
		if bozuk.al() && filepath.Base(filepath.Dir(yeni)) == "kontrol_noktalari" && strings.HasPrefix(filepath.Base(yeni), "adim_") {
			return &os.LinkError{Op: "rename", Old: eski, New: yeni, Err: errors.New("Input/output error")}
		}
		return os.Rename(eski, yeni)
	}))
	kosu := t.TempDir()
	program := func(n int, bozukAdimlar string) string {
		return fmt.Sprintf(`
değişken w = tensör([1.0, 2.0], türev_izle: doğru)
değişken op = SGD([w], öğrenme_oranı: 0.1)
değişken kn = kontrol_noktası(%q, {w: w, op: op}, her_dakika: 0)
değişken sonuçlar = []
döngü adım içinde kn.kaldığı_adım..%d {
    geri_yayılım(toplam(w * w))
    op.adım()
    op.sıfırla()
    eğer [%s].içerir(adım + 1) { boz() }
    sonuçlar.ekle(kn.güncelle(adım + 1))
    düzelt()
}
yazdır("sonuçlar:", sonuçlar)
`, trYol(kosu), n, bozukAdimlar)
	}
	kancalar := func(kod string) string {
		return strings.ReplaceAll(strings.ReplaceAll(kod, "boz()", "bozuk_yap(doğru)"), "düzelt()", "bozuk_yap(yanlış)")
	}
	cikti, err := calistirKancali(t, kancalar(program(6, "3, 4")), &bozuk)
	if err != nil {
		t.Fatalf("iki başarısızlık eğitimi durdurmamalıydı: %v\n%s", err, cikti)
	}
	if !strings.Contains(cikti, "sonuçlar: [doğru, doğru, yanlış, yanlış, doğru, doğru]") ||
		strings.Count(cikti, "Uyarı: kontrol_noktası: ") != 2 ||
		!strings.Contains(cikti, "(1/3; eğitim sürüyor") || !strings.Contains(cikti, "Son geçerli nokta: adım 2") ||
		!strings.Contains(cikti, "yerel diske kaydedip") {
		t.Fatalf("beklenen uyarılar ve sonuçlar yok:\n%s", cikti)
	}
	artiklariDenetle(t, kosu)
	cikti, err = calistirKancali(t, kancalar(program(12, "8, 9, 10")), &bozuk)
	if err == nil || !strings.Contains(err.Error(), "art arda 3 kez kaydedilemedi, eğitim durduruldu") || !strings.Contains(err.Error(), "Son geçerli nokta: adım 7") {
		t.Fatalf("üçüncü başarısızlık açık hata vermeliydi: %v\n%s", err, cikti)
	}
	if !strings.Contains(cikti, "Kaldığı yerden devam: adım 6") {
		t.Fatalf("önceki koşunun son noktasından sürmeliydi:\n%s", cikti)
	}
	artiklariDenetle(t, kosu)
	// Önceki geçerli nokta sağlam: sürdürme 7'den başlar.
	cikti, err = calistirKancali(t, kancalar(program(8, "")), &bozuk)
	if err != nil || !strings.Contains(cikti, "Kaldığı yerden devam: adım 7") {
		t.Fatalf("son geçerli noktadan sürmeliydi: %v\n%s", err, cikti)
	}
}

// artiklariDenetle noktalar klasöründe geçici ya da kenara alınmış klasör kalmadığını denetler.
func artiklariDenetle(t *testing.T, kosu string) {
	t.Helper()
	girdiler, _ := os.ReadDir(filepath.Join(kosu, "kontrol_noktalari"))
	for _, g := range girdiler {
		if strings.Contains(g.Name(), ".tmp-") || strings.Contains(g.Name(), ".eski-") {
			t.Errorf("artık klasör kaldı: %s", g.Name())
		}
	}
}

type atomicBool struct {
	mu sync.Mutex
	v  bool
}

func (a *atomicBool) al() bool { a.mu.Lock(); defer a.mu.Unlock(); return a.v }
func (a *atomicBool) ver(v bool) {
	a.mu.Lock()
	a.v = v
	a.mu.Unlock()
}

// calistirKancali calistir gibidir; programa bozuk_yap(Mantık) yerleşiği eklenir (testin
// yeniden adlandırma benzetimini açıp kapatır).
func calistirKancali(t *testing.T, kod string, bozuk *atomicBool) (string, error) {
	t.Helper()
	m := interp.New()
	w := &esliYazici{}
	m.Out, m.Err = w, w
	m.GlobalEnv.Define("bozuk_yap", &interp.BuiltinVal{Name: "bozuk_yap", Fn: func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		bozuk.ver(args[0] == interp.TRUE)
		return interp.NULL, nil
	}}, true)
	_, err := calistirVM(t, m, kod)
	return w.String(), err
}
