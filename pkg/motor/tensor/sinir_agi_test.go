package tensor

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// sayilar "[0.1, 0.2]" biçimindeki yazdırılmış düz listeyi sayılara çevirir.
func sayilar(t *testing.T, s string) []float64 {
	t.Helper()
	s = strings.NewReplacer("[", "", "]", "", " ", "").Replace(strings.TrimSpace(s))
	var out []float64
	for _, p := range strings.Split(s, ",") {
		f, err := strconv.ParseFloat(p, 64)
		if err != nil {
			t.Fatalf("sayı çözülemedi %q: %v", p, err)
		}
		out = append(out, f)
	}
	return out
}

func xorBekle(t *testing.T, ad, cikti string) {
	t.Helper()
	got := sayilar(t, cikti)
	want := []float64{0, 1, 1, 0}
	if len(got) != len(want) {
		t.Fatalf("%s: %d tahmin bekleniyordu: %q", ad, len(want), cikti)
	}
	for i := range want {
		if d := got[i] - want[i]; d > 0.05 || d < -0.05 {
			t.Fatalf("%s: XOR öğrenilmedi: %v", ad, got)
		}
	}
}

// sinir_ağı XOR'u öğrenir; kayıp düşer, tahminler tensördür ve kaydedip yükleme aynı
// tahminleri verir.
func TestSinirAgiXOR(t *testing.T) {
	yol := filepath.ToSlash(filepath.Join(t.TempDir(), "xor.json"))
	out := mustCalistir(t, `değişken X = [[0, 0], [0, 1], [1, 0], [1, 1]]
değişken Y = [[0], [1], [1], [0]]
değişken model = sinir_ağı([2, 8, 1], aktivasyon: "tanh", tohum: 42)
değişken özet = model.eğit(X, Y, devir: 800, öğrenme_hızı: 0.15)
yazdır(özet["ilk_kayıp"] > 0.1, özet["son_kayıp"] < 0.001, özet["devir"])
değişken t = model.tahmin_et(X)
yazdır(tip(t), t.şekil, tip(model.ağırlıklar[0]), model.ağırlıklar[0].şekil)
yazdır(t.şekillendir([-1]).değer)
model.kaydet("`+yol+`")
yazdır(model_yükle("`+yol+`").tahmin_et(X).şekillendir([-1]).değer == t.şekillendir([-1]).değer)
yazdır(model)`)
	satirlar := strings.Split(strings.TrimSpace(out), "\n")
	if len(satirlar) != 5 {
		t.Fatalf("beklenmeyen çıktı:\n%s", out)
	}
	if satirlar[0] != "doğru doğru 800" || satirlar[1] != "Tensör [4, 1] Tensör [2, 8]" || satirlar[3] != "doğru" ||
		satirlar[4] != `<SinirAğı mimari=[2 -> 8 -> 1] aktivasyon="tanh">` {
		t.Fatalf("beklenmeyen çıktı:\n%s", out)
	}
	xorBekle(t, "eğitilen model", satirlar[2])
}

// Diğer aktivasyonlar ve çok çıkışlı hedef de öğrenir.
func TestSinirAgiAktivasyonlar(t *testing.T) {
	for _, akt := range []string{"relu", "sigmoid"} {
		out := mustCalistir(t, `değişken X = [[0, 0], [0, 1], [1, 0], [1, 1]]
değişken model = sinir_ağı([2, 8, 2], aktivasyon: "`+akt+`", tohum: 3)
değişken özet = model.eğit(X, [[0, 1], [1, 0], [1, 0], [0, 1]], devir: 8000, öğrenme_hızı: 0.5)
yazdır(özet["son_kayıp"] < özet["ilk_kayıp"])
değişken t = model.tahmin_et(X)
yazdır(t.dilim(1, 0, 1).şekillendir([-1]).değer)`)
		satirlar := strings.Split(strings.TrimSpace(out), "\n")
		if satirlar[0] != "doğru" {
			t.Fatalf("%s: kayıp düşmedi:\n%s", akt, out)
		}
		xorBekle(t, akt, satirlar[1])
	}
}

// Eski (matris tabanlı) modelin kaydettiği JSON dosyası yüklenir ve doğru tahmin verir.
func TestSinirAgiEskiModelDosyasi(t *testing.T) {
	out := mustCalistir(t, `değişken m = model_yükle("testdata/xor_modeli_eski.json")
yazdır(m)
yazdır(m.tahmin_et([[0, 0], [0, 1], [1, 0], [1, 1]]).şekillendir([-1]).değer)`)
	satirlar := strings.Split(strings.TrimSpace(out), "\n")
	if satirlar[0] != `<SinirAğı mimari=[2 -> 8 -> 1] aktivasyon="tanh">` {
		t.Fatalf("beklenmeyen model: %s", out)
	}
	xorBekle(t, "eski model", satirlar[1])
}

func TestSinirAgiGirdiDenetimi(t *testing.T) {
	for _, tc := range []struct{ kod, want string }{
		{`değişken m = sinir_ağı([2, 3, 1])
		  m.eğit([[0, 1], [1, 0]], [[1], [1]], devir: 0)`, "devir"},
		{`değişken m = sinir_ağı([2, 3, 1])
		  m.eğit([[0, 1], [1, 0]], [[1, 0], [1, 0]], devir: 5)`, "çıkış katmanı"},
		{`değişken m = sinir_ağı([2, 3, 1])
		  m.eğit([[0, 1, 1], [1, 0, 1]], [[1], [1]], devir: 5)`, "giriş katmanı"},
		{`değişken m = sinir_ağı([2, 3, 1])
		  m.eğit([[0, 1], [1, 0]], [[1]], devir: 5)`, "satır sayısı"},
		{`sinir_ağı([2, 3, 1]).tahmin_et([[1, 2, 3]])`, "giriş katmanı"},
		{`sinir_ağı([2])`, "en az 2 katman"},
		{`sinir_ağı([2, 0])`, "pozitif Tam sayı"},
		{`sinir_ağı([2, 1]).yok_boyle`, "SinirAğı üzerinde \"yok_boyle\""},
		{`kayıp_mse([1, 2], [1, 2, 3])`, "kayıp_mse boyut uyuşmazlığı"},
	} {
		_, err := calistir(t, tc.kod)
		if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "çökme önlendi") {
			t.Errorf("%q içeren hata bekleniyordu, alınan: %v", tc.want, err)
		}
	}
	if out := mustCalistir(t, `yazdır(kayıp_mse([[1, 2], [3, 4]], tensör([[1, 2], [3, 6]])))`); out != "1.0\n" {
		t.Fatalf("kayıp_mse: %q", out)
	}
}

func TestModelYukleBozukDosyalar(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"bos.json":      `{"katman_boyutları":[2,1],"aktivasyon":"tanh","ağırlıklar":[[]],"sapmalar":[[[0]]]}`,
		"hic.json":      `{"katman_boyutları":[2,1],"aktivasyon":"tanh","ağırlıklar":[],"sapmalar":[]}`,
		"duzensiz.json": `{"katman_boyutları":[2,1],"aktivasyon":"tanh","ağırlıklar":[[[1],[2,3]]],"sapmalar":[[[0]]]}`,
		"sapma.json":    `{"katman_boyutları":[2,1],"aktivasyon":"tanh","ağırlıklar":[[[1],[2]]],"sapmalar":[]}`,
		"uyumsuz.json":  `{"katman_boyutları":[2,1],"aktivasyon":"tanh","ağırlıklar":[[[1],[2]],[[1],[2]]],"sapmalar":[[[0]],[[0]]]}`,
		"gecersiz.json": `{"katman_boyutları":`,
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := calistir(t, fmt.Sprintf("model_yükle(%q)", filepath.ToSlash(path)))
		if err == nil || !strings.Contains(err.Error(), "Model dosyası") || strings.Contains(err.Error(), "çökme önlendi") {
			t.Errorf("%s: 'Model dosyası' hatası bekleniyordu, alınan: %v", name, err)
		}
	}
}
