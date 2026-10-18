package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"tr_dili/internal/hesap/veriyolu"
)

// Kontrol noktasının kabul testi (docs/dil/20_kontrol_noktasi.md): N adım kesintisiz eğitim
// ile K adımda kesilip (ayrı süreçte; Ctrl+C ile ve süreç öldürülerek) sürdürülen eğitim aynı
// N. adımda bit düzeyinde aynı parametreleri, iyileştirici durumunu, geçerli kayıp dizisini
// ve düz değerleri verir. Model (gömme, rms normu, nedensel dikkat, GELU katmanı, çapraz
// entropi, gradyan kırpma, AdamW) CPU çekirdeklerinin paralel yoluna girecek kadar büyüktür
// (çarpım ve [256, 2048] eleman bazlı işler); kesintisiz koşu GOMAXPROCS=1 ve 8 ile, öldürülen
// koşu 8 işçiyle başlayıp 1 işçiyle sürdürülür (indirgemeler işçi sayısından bağımsız olmalı).

const kontrolEgitimProgrami = `değişken ayar = komut_satırı({adım: 40, koşu: "", veri: "", her_dakika: 60.0})
değişken V = 2048
değişken D = 64
değişken H = 128
değişken sayaç = 0
fonksiyon param(şekil, ölçek) {
    sayaç += 1
    sonuç tensör(türevsiz(rastgele_normal(şekil, tohum: sayaç) * ölçek), türev_izle: doğru)
}
değişken model = {E: param([V, D], 0.05), g: tensör(birler([D]), türev_izle: doğru), Wq: param([D, D], 0.1), Wk: param([D, D], 0.1),
    Wv: param([D, D], 0.1), W1: param([D, H], 0.1), W2: param([H, V], 0.1)}
değişken op = AdamW(parametreler(model), öğrenme_oranı: 0.003)
fonksiyon başlar(x) { sonuç x.şekillendir([8, 32, 4, 16]).eksenleri_sırala([0, 2, 1, 3]) }
fonksiyon ileri(giriş) {
    değişken x = gömme(model.E, giriş)
    değişken h = rms_normu(x, model.g)
    değişken y = dikkat(başlar(h @ model.Wq), başlar(h @ model.Wk), başlar(h @ model.Wv), nedensel: doğru)
    x = x + y.eksenleri_sırala([0, 2, 1, 3]).şekillendir([8, 32, D])
    sonuç gelu(x @ model.W1) @ model.W2
}
değişken ö = veri_kümesi(ayar.veri).örnekleyici(toplu: 8, bağlam: 32, tohum: 11)
değişken g = günlük(yol(ayar.koşu, "metrics.jsonl"))
değişken izlenen = {en_iyi: 99.0, adım_sayısı: 0}
değişken kn = kontrol_noktası(ayar.koşu, {model: model, op: op, ö: ö, g: g, izlenen: izlenen}, her_dakika: ayar.her_dakika)
değişken ç = ilerleme(ayar.adım, günlük: g)
döngü adım içinde kn.kaldığı_adım..ayar.adım {
    değişken giriş, hedef = ö.sonraki()
    op.sıfırla()
    değişken kayıp = çapraz_entropi(ileri(giriş), hedef)
    geri_yayılım(kayıp)
    gradyan_kırp(parametreler(model), en_fazla: 1.0)
    op.adım()
    eğer kayıp.değer < izlenen.en_iyi { izlenen.en_iyi = kayıp.değer }
    izlenen.adım_sayısı += 1
    ç.güncelle({kayıp: kayıp}, adım: adım + 1)
    kn.güncelle(adım + 1)
}
tensör_kaydet(yol(ayar.koşu, "son.trt"), parametreler(model, adlarıyla: doğru))
tensör_kaydet(yol(ayar.koşu, "op.trt"), op.durum)
yazdır("izlenen:", izlenen, ö.durum)
`

// kontrolVerisi deterministik parça kimliklerinden dört .npy veri dosyası yazar.
func kontrolVerisi(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	r := rand.New(rand.NewPCG(1, 2))
	for i := range 4 {
		v := make([]uint16, 20000)
		for j := range v {
			v[j] = uint16(r.IntN(2048))
		}
		var b bytes.Buffer
		if err := veriyolu.NpyYazU16(&b, v); err != nil {
			t.Fatal(err)
		}
		dosyaYaz(t, d, fmt.Sprintf("parca_%d.npy", i), b.String())
	}
	return d
}

// egitimKosusu eğitim programını ayrı bir süreçte başlatır.
type egitimKosusu struct {
	cmd   *exec.Cmd
	hata  *kilitliTampon
	cikti *kilitliTampon
	bitti chan error
}

func egitimBaslat(t *testing.T, klasor, kosu, veri string, isci int, herDakika string) *egitimKosusu {
	t.Helper()
	cmd := exec.Command(trIkilisi, "egit.tr", "--koşu", kosu, "--veri", veri, "--her_dakika", herDakika)
	cmd.Dir = klasor
	cmd.Env = append(os.Environ(), fmt.Sprintf("GOMAXPROCS=%d", isci))
	k := &egitimKosusu{cmd: cmd, hata: &kilitliTampon{}, cikti: &kilitliTampon{}, bitti: make(chan error, 1)}
	cmd.Stdout, cmd.Stderr = k.cikti, k.hata
	sureciHazirla(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { k.bitti <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		err := <-k.bitti
		k.bitti <- err
	})
	return k
}

// bekle sürecin bitmesini (en çok 2 dk) bekler ve çıkış kodunu verir.
func (k *egitimKosusu) bekle(t *testing.T) int {
	t.Helper()
	select {
	case err := <-k.bitti:
		k.bitti <- err
		var ch *exec.ExitError
		if errors.As(err, &ch) {
			return ch.ExitCode()
		}
		if err != nil {
			t.Fatal(err)
		}
		return 0
	case <-time.After(2 * time.Minute):
		t.Fatalf("eğitim 2 dk içinde bitmedi; standart hata:\n%s", k.hata.String())
		return -1
	}
}

// egit eğitimi baştan sona (ya da kaldığı yerden sona) çalıştırır ve çıktısını verir.
func egit(t *testing.T, klasor, kosu, veri string, isci int) string {
	t.Helper()
	k := egitimBaslat(t, klasor, kosu, veri, isci, "60")
	if kod := k.bekle(t); kod != 0 {
		t.Fatalf("eğitim %d koduyla bitti:\n%s\n%s", kod, k.cikti.String(), k.hata.String())
	}
	if strings.Contains(k.hata.String(), "Uyarı") {
		t.Errorf("uyarı beklenmiyordu:\n%s", k.hata.String())
	}
	return k.cikti.String()
}

// olcuSayisi günlükteki ölçü kaydı sayısıdır (yarım son satır sayılmaz).
func olcuSayisi(yol string) int {
	b, _ := os.ReadFile(yol)
	return strings.Count(string(b), `"kayıp":`) - strings.Count(string(b[bytes.LastIndexByte(b, '\n')+1:]), `"kayıp":`)
}

// bekleKadar kosul doğru olana kadar (en çok 1 dk) bekler.
func bekleKadar(t *testing.T, k *egitimKosusu, ne string, kosul func() bool) {
	t.Helper()
	son := time.Now().Add(time.Minute)
	for !kosul() {
		select {
		case <-k.bitti:
			t.Fatalf("%s olmadan eğitim bitti:\n%s", ne, k.hata.String())
		default:
		}
		if time.Now().After(son) {
			t.Fatalf("%s 1 dk içinde olmadı:\n%s", ne, k.hata.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// gecerliKayiplar günlüğün geçerli ölçü kayıtlarını "adım:kayıp" olarak verir: bir devam
// kaydından önce gelen ve adımı ondan büyük olan kayıtlar geçersizdir (19, günlük okuyucu kuralı).
func gecerliKayiplar(t *testing.T, yol string) []string {
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
		if json.Unmarshal([]byte(s), &k) != nil {
			continue // öldürülen sürecin yarım kalmış son satırı
		}
		var adim int64
		var tur string
		_ = json.Unmarshal(k["adım"], &adim)
		_ = json.Unmarshal(k["tür"], &tur)
		switch {
		case tur == "devam":
			olculer = slices.DeleteFunc(olculer, func(o olcu) bool { return o.adim > adim })
		case tur == "" && k["kayıp"] != nil:
			olculer = append(olculer, olcu{adim, string(k["kayıp"])})
		}
	}
	var out []string
	for _, o := range olculer {
		out = append(out, fmt.Sprintf("%d:%s", o.adim, o.kayip))
	}
	return out
}

// ayniMi iki koşunun sonucunu bit düzeyinde karşılaştırır.
func ayniMi(t *testing.T, ne, a, b, ciktiA, ciktiB string) {
	t.Helper()
	for _, ad := range []string{"son.trt", "op.trt"} {
		x, err1 := os.ReadFile(filepath.Join(a, ad))
		y, err2 := os.ReadFile(filepath.Join(b, ad))
		if err1 != nil || err2 != nil || !bytes.Equal(x, y) {
			t.Errorf("%s: %s bit düzeyinde aynı değil (%v %v)", ne, ad, err1, err2)
		}
	}
	ka, kb := gecerliKayiplar(t, filepath.Join(a, "metrics.jsonl")), gecerliKayiplar(t, filepath.Join(b, "metrics.jsonl"))
	if len(ka) != 40 || !slices.Equal(ka, kb) {
		t.Errorf("%s: geçerli kayıp dizileri farklı (%d, %d kayıt):\n%v\n%v", ne, len(ka), len(kb), ka, kb)
	}
	if ciktiA != ciktiB {
		t.Errorf("%s: son durum farklı:\n%s\n%s", ne, ciktiA, ciktiB)
	}
}

func TestKontrolNoktasiBitDuzeyindeDevam(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "egit.tr", kontrolEgitimProgrami)
	veri := kontrolVerisi(t)
	kosu := func(ad string) string { return filepath.ToSlash(filepath.Join(klasor, ad)) }

	ciktiA := egit(t, klasor, kosu("kesintisiz8"), veri, 8)
	ayniMi(t, "GOMAXPROCS=1 ve 8", kosu("kesintisiz8"), kosu("kesintisiz1"), ciktiA, egit(t, klasor, kosu("kesintisiz1"), veri, 1))

	t.Run("Ctrl+C", func(t *testing.T) {
		k := egitimBaslat(t, klasor, kosu("kesme"), veri, 8, "60")
		bekleKadar(t, k, "12 adım", func() bool { return olcuSayisi(filepath.Join(klasor, "kesme", "metrics.jsonl")) >= 12 })
		kesmeGonder(t, k.cmd)
		if kod := k.bekle(t); kod != 3 {
			t.Fatalf("çıkış kodu %d, 3 bekleniyordu:\n%s", kod, k.hata.String())
		}
		if h := k.hata.String(); !strings.Contains(h, "adım bitince kontrol noktası kaydedilip çıkılacak") || !strings.Contains(h, "Kontrol noktası kaydedildi: adım") {
			t.Fatalf("durdurma bildirimleri yok:\n%s", h)
		}
		ayniMi(t, "Ctrl+C ve devam", kosu("kesintisiz8"), kosu("kesme"), ciktiA, egit(t, klasor, kosu("kesme"), veri, 8))
		if b, _ := os.ReadFile(filepath.Join(klasor, "kesme", "metrics.jsonl")); strings.Count(string(b), `"tür":"devam"`) != 1 {
			t.Errorf("günlükte bir devam kaydı bekleniyordu:\n%s", b)
		}
	})
	t.Run("öldürme", func(t *testing.T) {
		// Yaklaşık 120 ms'de bir kayıt: öldürülen sürecin son noktasından sonraki adımlar
		// yeniden hesaplanır (günlükte devamdan önce geçersiz kalan kayıtlar).
		k := egitimBaslat(t, klasor, kosu("oldurme"), veri, 8, "0.002")
		bekleKadar(t, k, "bir nokta ve 20 adım", func() bool {
			_, err := os.Stat(filepath.Join(klasor, "oldurme", "kontrol_noktalari", "son.json"))
			return err == nil && olcuSayisi(filepath.Join(klasor, "oldurme", "metrics.jsonl")) >= 20
		})
		if err := k.cmd.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		k.bekle(t)
		cikti := egit(t, klasor, kosu("oldurme"), veri, 1)
		ayniMi(t, "öldürme ve 1 işçiyle devam", kosu("kesintisiz8"), kosu("oldurme"), ciktiA, cikti)
		if n := olcuSayisi(filepath.Join(klasor, "oldurme", "metrics.jsonl")); n <= 40 {
			t.Logf("öldürme son noktadan hemen sonra oldu (%d ölçü kaydı); yeniden hesaplanan adım yok", n)
		}
	})
}
