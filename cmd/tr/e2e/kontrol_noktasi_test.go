package e2e

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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/dataload"
)

// Kontrol noktasının kabul testi (docs/spec/20_kontrol_noktasi.md): N adım kesintisiz eğitim
// ile K adımda kesilip (ayrı süreçte; Ctrl+C ile ve süreç öldürülerek) sürdürülen eğitim aynı
// N. adımda bit düzeyinde aynı parametreleri, iyileştirici durumunu, geçerli kayıp dizisini
// ve düz değerleri verir. Model (gömme, rms normu, nedensel dikkat, GELU katmanı, çapraz
// entropi, gradyan kırpma, AdamW) CPU çekirdeklerinin paralel yoluna girecek kadar büyüktür
// (çarpım ve [256, 2048] eleman bazlı işler); kesintisiz koşu GOMAXPROCS=1 ve 8 ile, öldürülen
// koşu 8 işçiyle başlayıp 1 işçiyle sürdürülür (indirgemeler işçi sayısından bağımsız olmalı).
//
// Test belirlenimcidir, zamana bağlı değildir: kayıtlar adım sayısıyla yapılır (--kayıt_her N:
// her N adımda kn.kaydet()) ve süreç --dur K ile K. adımın sınırında durup bekler (bekliyor.txt
// yazılır); kesme ve öldürme ancak o zaman gönderilir. Böylece eğitim kesilmeden bitemez ve
// kesmenin hangi adımda, son noktanın hangi adımda olduğu bellidir.

const kontrolEgitimProgrami = `değişken ayar = komut_satırı({adım: 40, koşu: "", veri: "", kayıt_her: 0, dur: 0})
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
değişken kn = kontrol_noktası(ayar.koşu, {model: model, op: op, ö: ö, g: g, izlenen: izlenen}, her_dakika: 60.0)
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
    eğer ayar.kayıt_her > 0 ve (adım + 1) % ayar.kayıt_her == 0 { kn.kaydet() }
    eğer adım + 1 == ayar.dur {
        // Testin kesme ya da öldürme noktası: süreç bu adımın sınırında bekler.
        dosya_yaz(yol(ayar.koşu, "bekliyor.txt"), adım + 1)
        döngü doğru { kn.güncelle(adım + 1) }
    }
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
		if err := dataload.NpyYazU16(&b, v); err != nil {
			t.Fatal(err)
		}
		dosyaYaz(t, d, fmt.Sprintf("parca_%d.npy", i), b.String())
	}
	return d
}

// egitimKosusu eğitim programını ayrı bir süreçte başlatır.
type egitimKosusu struct {
	cmd   *exec.Cmd
	kosu  string // koşu klasörü
	hata  *kilitliTampon
	cikti *kilitliTampon
	bitti chan error
}

// egitimBaslat eğitimi isci işçiyle başlatır; ek komut satırı ayarlarıdır (--dur, --kayıt_her).
func egitimBaslat(t *testing.T, klasor, kosu, veri string, isci int, ek ...string) *egitimKosusu {
	t.Helper()
	cmd := exec.Command(trIkilisi, append([]string{"egit.tr", "--koşu", kosu, "--veri", veri}, ek...)...)
	cmd.Dir = klasor
	cmd.Env = append(os.Environ(), fmt.Sprintf("GOMAXPROCS=%d", isci))
	k := &egitimKosusu{cmd: cmd, kosu: filepath.FromSlash(kosu), hata: &kilitliTampon{}, cikti: &kilitliTampon{}, bitti: make(chan error, 1)}
	cmd.Stdout, cmd.Stderr = k.cikti, k.hata
	sureciHazirla(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("eğitim başlatılamadı: %v", err)
	}
	go func() { k.bitti <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		err := <-k.bitti
		k.bitti <- err
	})
	return k
}

// rapor başarısızlık mesajı için alt sürecin standart hatasını, standart çıktısını, günlüğün
// son satırlarını ve kontrol noktası klasörünün içeriğini verir.
func (k *egitimKosusu) rapor() string {
	var b strings.Builder
	fmt.Fprintf(&b, "--- standart hata:\n%s\n--- standart çıktı:\n%s\n", k.hata.String(), k.cikti.String())
	if g, err := os.ReadFile(filepath.Join(k.kosu, "metrics.jsonl")); err == nil {
		satirlar := strings.Split(strings.TrimRight(string(g), "\n"), "\n")
		fmt.Fprintf(&b, "--- günlük (%d satır, son 12):\n%s\n", len(satirlar), strings.Join(satirlar[max(0, len(satirlar)-12):], "\n"))
	}
	if g, err := os.ReadDir(filepath.Join(k.kosu, "kontrol_noktalari")); err == nil {
		var adlar []string
		for _, e := range g {
			adlar = append(adlar, e.Name())
		}
		fmt.Fprintf(&b, "--- kontrol_noktalari: %v\n", adlar)
	}
	return b.String()
}

// bitmeyiBekle sürecin bitmesini (en çok 2 dk) bekler; çıkış kodunu ve bitmediyse yanlış verir.
func (k *egitimKosusu) bitmeyiBekle() (int, bool, error) {
	select {
	case err := <-k.bitti:
		k.bitti <- err
		var ch *exec.ExitError
		if errors.As(err, &ch) {
			return ch.ExitCode(), true, nil
		}
		return 0, true, err
	case <-time.After(2 * time.Minute):
		return -1, false, nil
	}
}

// bekle sürecin bitmesini (en çok 2 dk) bekler ve çıkış kodunu verir.
func (k *egitimKosusu) bekle(t *testing.T) int {
	t.Helper()
	kod, bitti, err := k.bitmeyiBekle()
	if !bitti {
		t.Fatalf("eğitim 2 dk içinde bitmedi\n%s", k.rapor())
	}
	if err != nil {
		t.Fatalf("eğitim beklenemedi: %v\n%s", err, k.rapor())
	}
	return kod
}

// egit eğitimi baştan sona (ya da kaldığı yerden sona) çalıştırır ve çıktısını verir.
func egit(t *testing.T, klasor, kosu, veri string, isci int, ek ...string) string {
	t.Helper()
	k := egitimBaslat(t, klasor, kosu, veri, isci, ek...)
	if kod := k.bekle(t); kod != 0 {
		t.Fatalf("eğitim %d koduyla bitti\n%s", kod, k.rapor())
	}
	if strings.Contains(k.hata.String(), "Uyarı") {
		t.Errorf("uyarı beklenmiyordu\n%s", k.rapor())
	}
	return k.cikti.String()
}

// durmayiBekle sürecin --dur adımının sınırına gelip beklemeye geçmesini (bekliyor.txt; en çok
// 2 dk) bekler. Süreç bu sırada biterse ya da süre dolarsa test raporla kalır.
func (k *egitimKosusu) durmayiBekle(t *testing.T, adim int) {
	t.Helper()
	isaret := filepath.Join(k.kosu, "bekliyor.txt")
	son := time.Now().Add(2 * time.Minute)
	for {
		if b, err := os.ReadFile(isaret); err == nil && strings.TrimSpace(string(b)) == strconv.Itoa(adim) {
			return
		}
		select {
		case err := <-k.bitti:
			k.bitti <- err
			t.Fatalf("eğitim %d. adımda beklemeden bitti (%v)\n%s", adim, err, k.rapor())
		case <-time.After(5 * time.Millisecond):
		}
		if time.Now().After(son) {
			t.Fatalf("eğitim 2 dk içinde %d. adıma gelmedi\n%s", adim, k.rapor())
		}
	}
}

// olcuSayisi günlükteki ölçü kaydı sayısıdır (yarım son satır sayılmaz).
func olcuSayisi(yol string) int {
	b, _ := os.ReadFile(yol)
	return strings.Count(string(b), `"kayıp":`) - strings.Count(string(b[bytes.LastIndexByte(b, '\n')+1:]), `"kayıp":`)
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

// devamAdimlari günlükteki devam kayıtlarının adımlarıdır.
func devamAdimlari(t *testing.T, yol string) []int64 {
	t.Helper()
	veri, err := os.ReadFile(yol)
	if err != nil {
		t.Fatal(err)
	}
	var out []int64
	for _, s := range strings.Split(string(veri), "\n") {
		var k struct {
			Tur  string `json:"tür"`
			Adim int64  `json:"adım"`
		}
		if json.Unmarshal([]byte(s), &k) == nil && k.Tur == "devam" {
			out = append(out, k.Adim)
		}
	}
	return out
}

// sonNoktaAdimi kontrol_noktalari/son.json'un söylediği adımdır.
func sonNoktaAdimi(t *testing.T, kosu string) int64 {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(kosu, "kontrol_noktalari", "son.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Adim int64 `json:"adım"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("son.json çözülemedi: %v (%s)", err, b)
	}
	return s.Adim
}

// klasorKopyala bir nokta klasörünü kopyalar; yarim > 0 ise her dosyanın yalnızca ilk yarim
// baytını yazar (öldürülen bir kaydın yarım kalmış dosyaları).
func klasorKopyala(t *testing.T, kaynak, hedef string, yarim int64) {
	t.Helper()
	if err := os.Mkdir(hedef, 0o755); err != nil {
		t.Fatal(err)
	}
	girdiler, err := os.ReadDir(kaynak)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range girdiler {
		b, err := os.ReadFile(filepath.Join(kaynak, g.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if yarim > 0 {
			b = b[:min(int64(len(b)), yarim)]
		}
		if err := os.WriteFile(filepath.Join(hedef, g.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
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
		k := egitimBaslat(t, klasor, kosu("kesme"), veri, 8, "--dur", "12")
		k.durmayiBekle(t, 12)
		kesmeGonder(t, k.cmd)
		if kod := k.bekle(t); kod != 3 {
			t.Fatalf("çıkış kodu %d, 3 bekleniyordu\n%s", kod, k.rapor())
		}
		if h := k.hata.String(); !strings.Contains(h, "adım bitince kontrol noktası kaydedilip çıkılacak") || !strings.Contains(h, "Kontrol noktası kaydedildi: adım 12") {
			t.Fatalf("durdurma bildirimleri yok\n%s", k.rapor())
		}
		ayniMi(t, "Ctrl+C ve devam", kosu("kesintisiz8"), kosu("kesme"), ciktiA, egit(t, klasor, kosu("kesme"), veri, 8))
		if d := devamAdimlari(t, filepath.Join(klasor, "kesme", "metrics.jsonl")); !slices.Equal(d, []int64{12}) {
			t.Errorf("günlükte adım 12'de bir devam kaydı bekleniyordu, %v var\n%s", d, k.rapor())
		}
	})

	// oldurVeDevam eğitimi 8 işçiyle 25. adımda (son nokta 20. adımda) öldürür, koşu klasörünü
	// bozucu ile değiştirir (kayıt ortasında öldürülmenin bıraktığı durum) ve 1 işçiyle sürdürür.
	// 21-25. adımlar yeniden hesaplanır: günlükte devamdan önce geçersiz kalan 5 kayıt vardır.
	oldurVeDevam := func(t *testing.T, ad string, bozucu func(noktalar string, pid int)) {
		k := egitimBaslat(t, klasor, kosu(ad), veri, 8, "--dur", "25", "--kayıt_her", "10")
		k.durmayiBekle(t, 25)
		if n := sonNoktaAdimi(t, k.kosu); n != 20 {
			t.Fatalf("öldürmeden önce son nokta adım %d, 20 bekleniyordu\n%s", n, k.rapor())
		}
		if err := k.cmd.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		k.bekle(t)
		if bozucu != nil {
			bozucu(filepath.Join(k.kosu, "kontrol_noktalari"), k.cmd.Process.Pid)
		}
		d := egitimBaslat(t, klasor, kosu(ad), veri, 1, "--kayıt_her", "10")
		if kod := d.bekle(t); kod != 0 || strings.Contains(d.hata.String(), "Uyarı") || !strings.Contains(d.hata.String(), "Kaldığı yerden devam: adım 20 ") {
			t.Fatalf("devam %d koduyla bitti ya da 20. adımdan sürmedi\n%s", kod, d.rapor())
		}
		ayniMi(t, ad+" ve 1 işçiyle devam", kosu("kesintisiz8"), kosu(ad), ciktiA, d.cikti.String())
		if n := olcuSayisi(filepath.Join(k.kosu, "metrics.jsonl")); n != 45 {
			t.Errorf("günlükte %d ölçü kaydı, 45 (25 + yeniden hesaplanan 20) bekleniyordu\n%s", n, d.rapor())
		}
		if a := devamAdimlari(t, filepath.Join(k.kosu, "metrics.jsonl")); !slices.Equal(a, []int64{20}) {
			t.Errorf("günlükte adım 20'de bir devam kaydı bekleniyordu, %v var\n%s", a, d.rapor())
		}
		girdiler, _ := os.ReadDir(filepath.Join(k.kosu, "kontrol_noktalari"))
		for _, g := range girdiler {
			if strings.Contains(g.Name(), ".tmp-") || strings.Contains(g.Name(), ".eski-") {
				t.Errorf("yarım kaydın artığı kaldı: %s\n%s", g.Name(), d.rapor())
			}
		}
	}
	t.Run("öldürme", func(t *testing.T) { oldurVeDevam(t, "oldurme", nil) })
	t.Run("kayıt ortasında öldürme", func(t *testing.T) {
		// 30. adımın kaydı yazılırken öldürülmüş gibi: geçici klasörde yarım dosyalar.
		oldurVeDevam(t, "yarim_kayit", func(noktalar string, pid int) {
			klasorKopyala(t, filepath.Join(noktalar, "adim_0000000020"), filepath.Join(noktalar, "adim_0000000030.tmp-"+strconv.Itoa(pid)), 4096)
		})
	})
	t.Run("aynı adımlı nokta yerine konurken öldürme", func(t *testing.T) {
		// Aynı adımlı noktanın yerine yenisi konurken öldürülmüş gibi: eskisi kenara alınmış,
		// yenisi geçici klasörde (tam), yerine konmamış.
		oldurVeDevam(t, "yerine_koyma", func(noktalar string, pid int) {
			asil := filepath.Join(noktalar, "adim_0000000020")
			klasorKopyala(t, asil, filepath.Join(noktalar, "adim_0000000020.tmp-"+strconv.Itoa(pid)), 0)
			if err := os.Rename(asil, asil+".eski-"+strconv.Itoa(pid)); err != nil {
				t.Fatal(err)
			}
		})
	})
}
