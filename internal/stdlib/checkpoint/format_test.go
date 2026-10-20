package checkpoint_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Biçim sürümü 2: modelin türev izlemeyen tensörleri (durum) de kaydedilir. Sürüm 1
// noktalarında yalnızca türev izleyen tensörler vardı; onlar da okunur.

// downgradeVersion bir noktayı sürüm 1 noktası gibi yapar: durum.json ile ozet.json'un sürümünü 1
// yapar ve ozet.json'daki durum.json özetini yeniler.
func downgradeVersion(t *testing.T, ckpt string) {
	t.Helper()
	sPath := filepath.Join(ckpt, "durum.json")
	d, err := os.ReadFile(sPath)
	if err != nil {
		t.Fatal(err)
	}
	d = []byte(strings.Replace(string(d), `"sürüm": 2`, `"sürüm": 1`, 1))
	if err := os.WriteFile(sPath, d, 0o644); err != nil {
		t.Fatal(err)
	}
	oPath := filepath.Join(ckpt, "ozet.json")
	cache, err := os.ReadFile(oPath)
	if err != nil {
		t.Fatal(err)
	}
	var digest map[string]any
	if err := json.Unmarshal(cache, &digest); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(d)
	digest["sürüm"] = 1
	digest["dosyalar"].(map[string]any)["durum.json"] = map[string]any{"bayt": len(d), "sha256": hex.EncodeToString(h[:])}
	cache, _ = json.Marshal(digest)
	if err := os.WriteFile(oPath, cache, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestNoGradTensorsSaved: modelin içindeki türev izlemeyen tensörler (TR ile
// yazılmış iyileştiricinin momentumu, ara tensör olmayan her yaprak) kaydedilir ve yüklemede
// var olan tensöre yerinde yazılır; ara sonuçlar kaydedilmez.
func TestNoGradTensorsSaved(t *testing.T) {
	runDir := trPath(t.TempDir())
	if output, err := run(t, fmt.Sprintf(`
değişken w = tensör([1.0, 2.0], türev_izle: doğru)
değişken model = {w: w, durum: {m: tensör([5.0, 6.0]), adım: tensör(3.0)}}
değişken kn = kontrol_noktası(%q, {model: model, tek: tensör([[1.0], [2.0]], tür: "bf16")})
kn.güncelle(4)
kn.kaydet()
`, runDir)); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	output, err := run(t, fmt.Sprintf(`
değişken w = tensör([0.0, 0.0], türev_izle: doğru)
değişken m = tensör([0.0, 0.0])
değişken model = {w: w, durum: {m: m, adım: tensör(0.0)}}
değişken tek = tensör([[0.0], [0.0]], tür: "bf16")
değişken kn = kontrol_noktası(%q, {model: model, tek: tek})
yazdır(kn.kaldığı_adım, w.değer, m.değer, model.durum.m.değer, model.durum.adım.değer, tek.değer, tek.tür)
`, runDir))
	if err != nil || !strings.Contains(output, "4 [1.0, 2.0] [5.0, 6.0] [5.0, 6.0] 3.0 [[1.0], [2.0]] bf16") {
		t.Fatalf("durum tensörleri yerinde yüklenmedi: %v\n%s", err, output)
	}
	// Kayıtta olan bir durum tensörü modelde yoksa ya da şekli farklıysa açık hata.
	for _, d := range []struct {
		model string
		want  messages.Code
	}{
		{`{w: tensör([0.0, 0.0], türev_izle: doğru), durum: {m: tensör([0.0, 0.0])}}`, messages.H7453},
		{`{w: tensör([0.0, 0.0], türev_izle: doğru), durum: {m: tensör([0.0]), adım: tensör(0.0)}}`, messages.H7450},
		{`{w: tensör([0.0, 0.0], türev_izle: doğru), durum: {m: tensör([0.0, 0.0]), adım: tensör(0.0), yeni: tensör([1.0])}}`, messages.H7449},
	} {
		output, err := run(t, fmt.Sprintf(`kontrol_noktası(%q, {model: %s, tek: tensör([[0.0], [0.0]], tür: "bf16")})`, runDir, d.model))
		if !messages.Has(err, d.want) {
			t.Errorf("%s bekleniyordu: %v\n%s", d.want, err, output)
		}
	}
	// Ara sonuç (türev izleyen bir işlemin sonucu) tek başına verilemez; modelin içindeyse atlanır.
	output, err = run(t, fmt.Sprintf(`
değişken w = tensör([1.0], türev_izle: doğru)
kontrol_noktası(%q, {ara: w * 2})
`, trPath(t.TempDir())))
	if !messages.Has(err, messages.H7437) {
		t.Errorf("ara tensör hatası bekleniyordu: %v\n%s", err, output)
	}
}

// TestVersion1CheckpointReads: sürüm 1 noktasında (yalnızca türev izleyen tensörler) kayıtta
// olmayan türev izlemeyen tensörler olduğu gibi kalır; kayıtta türev izleyen olarak duran bir
// tensör, modelde türev izlemeyen bir duruma dönüşmüşse de yerine yazılır (eski kitaplığın
// türev izleyen durum tensörleri).
func TestVersion1CheckpointReads(t *testing.T) {
	runDir := trPath(t.TempDir())
	if output, err := run(t, fmt.Sprintf(`
değişken model = {w: tensör([1.0, 2.0], türev_izle: doğru), m: tensör([5.0, 6.0], türev_izle: doğru)}
değişken kn = kontrol_noktası(%q, {model: model})
kn.güncelle(3)
kn.kaydet()
`, runDir)); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	downgradeVersion(t, filepath.Join(filepath.FromSlash(runDir), "kontrol_noktalari", "adim_0000000003"))
	output, err := run(t, fmt.Sprintf(`
değişken model = {w: tensör([0.0, 0.0], türev_izle: doğru), m: tensör([0.0, 0.0]), kos: tensör([7.0])}
değişken kn = kontrol_noktası(%q, {model: model})
yazdır(kn.kaldığı_adım, model.w.değer, model.m.değer, model.kos.değer)
`, runDir))
	if err != nil || !strings.Contains(output, "3 [1.0, 2.0] [5.0, 6.0] [7.0]") {
		t.Fatalf("sürüm 1 noktası okunmadı: %v\n%s", err, output)
	}
	// Sürüm 1'de de modelin her parametresi kayıtta olmalıdır.
	_, err = run(t, fmt.Sprintf(`kontrol_noktası(%q, {model: {w: tensör([0.0, 0.0], türev_izle: doğru), m: tensör([0.0, 0.0]), b: tensör([0.0], türev_izle: doğru)}})`, runDir))
	if !messages.Has(err, messages.H7449) {
		t.Errorf("eksik parametre hatası bekleniyordu: %v", err)
	}
}

// Kayıttaki bir komut satırı ayarı yeni koşuda yoksa da fark olarak bildirilir.
func TestRemovedSettingReported(t *testing.T) {
	runDir := trPath(t.TempDir())
	program := `değişken ayar = komut_satırı(%s)
değişken w = tensör([1.0], türev_izle: doğru)
değişken kn = kontrol_noktası(%q, {w: w})
kn.kaydet()`
	if output, err := run(t, fmt.Sprintf(program, "{oran: 0.1, katman: 2}", runDir)); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	output, err := run(t, fmt.Sprintf(program, "{oran: 0.1}", runDir))
	if err != nil || !strings.Contains(output, "katman: 2 → yok") {
		t.Fatalf("kaldırılan ayar bildirilmedi: %v\n%s", err, output)
	}
}
