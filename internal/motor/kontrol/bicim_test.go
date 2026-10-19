package kontrol_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Biçim sürümü 2: modelin türev izlemeyen tensörleri (durum) de kaydedilir. Sürüm 1
// noktalarında yalnızca türev izleyen tensörler vardı; onlar da okunur.

// surumuIndir bir noktayı sürüm 1 noktası gibi yapar: durum.json ile ozet.json'un sürümünü 1
// yapar ve ozet.json'daki durum.json özetini yeniler.
func surumuIndir(t *testing.T, nokta string) {
	t.Helper()
	dyol := filepath.Join(nokta, "durum.json")
	d, err := os.ReadFile(dyol)
	if err != nil {
		t.Fatal(err)
	}
	d = []byte(strings.Replace(string(d), `"sürüm": 2`, `"sürüm": 1`, 1))
	if err := os.WriteFile(dyol, d, 0o644); err != nil {
		t.Fatal(err)
	}
	oyol := filepath.Join(nokta, "ozet.json")
	ob, err := os.ReadFile(oyol)
	if err != nil {
		t.Fatal(err)
	}
	var ozet map[string]any
	if err := json.Unmarshal(ob, &ozet); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(d)
	ozet["sürüm"] = 1
	ozet["dosyalar"].(map[string]any)["durum.json"] = map[string]any{"bayt": len(d), "sha256": hex.EncodeToString(h[:])}
	ob, _ = json.Marshal(ozet)
	if err := os.WriteFile(oyol, ob, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestTurevIzlemeyenTensorlerSaklanir: modelin içindeki türev izlemeyen tensörler (TR ile
// yazılmış iyileştiricinin momentumu, ara tensör olmayan her yaprak) kaydedilir ve yüklemede
// var olan tensöre yerinde yazılır; ara sonuçlar kaydedilmez.
func TestTurevIzlemeyenTensorlerSaklanir(t *testing.T) {
	kosu := trYol(t.TempDir())
	if cikti, err := calistir(t, fmt.Sprintf(`
değişken w = tensör([1.0, 2.0], türev_izle: doğru)
değişken model = {w: w, durum: {m: tensör([5.0, 6.0]), adım: tensör(3.0)}}
değişken kn = kontrol_noktası(%q, {model: model, tek: tensör([[1.0], [2.0]], tür: "bf16")})
kn.güncelle(4)
kn.kaydet()
`, kosu)); err != nil {
		t.Fatalf("%v\n%s", err, cikti)
	}
	cikti, err := calistir(t, fmt.Sprintf(`
değişken w = tensör([0.0, 0.0], türev_izle: doğru)
değişken m = tensör([0.0, 0.0])
değişken model = {w: w, durum: {m: m, adım: tensör(0.0)}}
değişken tek = tensör([[0.0], [0.0]], tür: "bf16")
değişken kn = kontrol_noktası(%q, {model: model, tek: tek})
yazdır(kn.kaldığı_adım, w.değer, m.değer, model.durum.m.değer, model.durum.adım.değer, tek.değer, tek.tür)
`, kosu))
	if err != nil || !strings.Contains(cikti, "4 [1.0, 2.0] [5.0, 6.0] [5.0, 6.0] 3.0 [[1.0], [2.0]] bf16") {
		t.Fatalf("durum tensörleri yerinde yüklenmedi: %v\n%s", err, cikti)
	}
	// Kayıtta olan bir durum tensörü modelde yoksa ya da şekli farklıysa açık hata.
	for _, d := range []struct{ model, beklenen string }{
		{`{w: tensör([0.0, 0.0], türev_izle: doğru), durum: {m: tensör([0.0, 0.0])}}`, `kayıttaki "durum.adım" tensörü modelde yok`},
		{`{w: tensör([0.0, 0.0], türev_izle: doğru), durum: {m: tensör([0.0]), adım: tensör(0.0)}}`, `"durum.m" tensörünün şekli modelde [1], kayıtta [2]`},
		{`{w: tensör([0.0, 0.0], türev_izle: doğru), durum: {m: tensör([0.0, 0.0]), adım: tensör(0.0), yeni: tensör([1.0])}}`, `modelin "durum.yeni" durum tensörü (türev izlemeyen) kayıtta yok`},
	} {
		cikti, err := calistir(t, fmt.Sprintf(`kontrol_noktası(%q, {model: %s, tek: tensör([[0.0], [0.0]], tür: "bf16")})`, kosu, d.model))
		if err == nil || !strings.Contains(err.Error(), d.beklenen) {
			t.Errorf("%q bekleniyordu: %v\n%s", d.beklenen, err, cikti)
		}
	}
	// Ara sonuç (türev izleyen bir işlemin sonucu) tek başına verilemez; modelin içindeyse atlanır.
	cikti, err = calistir(t, fmt.Sprintf(`
değişken w = tensör([1.0], türev_izle: doğru)
kontrol_noktası(%q, {ara: w * 2})
`, trYol(t.TempDir())))
	if err == nil || !strings.Contains(err.Error(), "ara tensör") {
		t.Errorf("ara tensör hatası bekleniyordu: %v\n%s", err, cikti)
	}
}

// TestSurum1NoktasiOkunur: sürüm 1 noktasında (yalnızca türev izleyen tensörler) kayıtta
// olmayan türev izlemeyen tensörler olduğu gibi kalır; kayıtta türev izleyen olarak duran bir
// tensör, modelde türev izlemeyen bir duruma dönüşmüşse de yerine yazılır (eski kitaplığın
// türev izleyen durum tensörleri).
func TestSurum1NoktasiOkunur(t *testing.T) {
	kosu := trYol(t.TempDir())
	if cikti, err := calistir(t, fmt.Sprintf(`
değişken model = {w: tensör([1.0, 2.0], türev_izle: doğru), m: tensör([5.0, 6.0], türev_izle: doğru)}
değişken kn = kontrol_noktası(%q, {model: model})
kn.güncelle(3)
kn.kaydet()
`, kosu)); err != nil {
		t.Fatalf("%v\n%s", err, cikti)
	}
	surumuIndir(t, filepath.Join(filepath.FromSlash(kosu), "kontrol_noktalari", "adim_0000000003"))
	cikti, err := calistir(t, fmt.Sprintf(`
değişken model = {w: tensör([0.0, 0.0], türev_izle: doğru), m: tensör([0.0, 0.0]), kos: tensör([7.0])}
değişken kn = kontrol_noktası(%q, {model: model})
yazdır(kn.kaldığı_adım, model.w.değer, model.m.değer, model.kos.değer)
`, kosu))
	if err != nil || !strings.Contains(cikti, "3 [1.0, 2.0] [5.0, 6.0] [7.0]") {
		t.Fatalf("sürüm 1 noktası okunmadı: %v\n%s", err, cikti)
	}
	// Sürüm 1'de de modelin her parametresi kayıtta olmalıdır.
	_, err = calistir(t, fmt.Sprintf(`kontrol_noktası(%q, {model: {w: tensör([0.0, 0.0], türev_izle: doğru), m: tensör([0.0, 0.0]), b: tensör([0.0], türev_izle: doğru)}})`, kosu))
	if err == nil || !strings.Contains(err.Error(), `modelin "b" parametresi kayıtta yok`) {
		t.Errorf("eksik parametre hatası bekleniyordu: %v", err)
	}
}
