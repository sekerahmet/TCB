package e2e

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Dosya biçimleri sözleşmedir: kontrol noktasının dosya adları ve JSON anahtarları ile günlüğün
// kayıt alanları yazım dilinden bağımsızdır, çevrilmez. Aynı kodlu program varsayılan dilde ve
// TR_DIL=en ile çalıştırılır; ürettiği dosyaların adları ve anahtar kümeleri aynı olmalı.
func TestFileFormatsIndependentOfWritingLanguage(t *testing.T) {
	program := "değişken w = tensör([1.0, 2.0], türev_izle: doğru)\n" +
		"değişken g = günlük(\"metrics.jsonl\")\n" +
		"değişken kn = kontrol_noktası(\"kosu\", {w: w})\n" +
		"kn.güncelle(1)\n" +
		"kn.kaydet()\n" +
		"g.ekle({kayıp: 1.5})\n" +
		"yazdır(tip(kn))\n"
	coded, err := source.Encode(program)
	if err != nil {
		t.Fatal(err)
	}
	var shapes, outputs [2]string
	for i, env := range [][]string{nil, {"TR_DIL=en"}} {
		dir := t.TempDir()
		testkit.WriteFile(t, dir, "egit.tr", coded)
		s := testkit.RunBinary(t, trBinary, dir, "", env, "egit.tr")
		testkit.Wait(t, s, 0, nil, nil)
		outputs[i] = s.Stdout
		shapes[i] = fileFormatShape(t, dir)
	}
	// Programın çıktısı çalıştıranın diline bağlı değildir: değerin metni dosyanın dilindedir
	// (dil=tr; yazdır(tip(...)) iki koşuda da Türkçe ad, 3). TR_DIL yalnızca gösterimi
	// değiştirir: aynı sözdizimi hatası İngilizce başlıkla bildirilir.
	if outputs[0] != outputs[1] {
		t.Fatalf("program çıktısı yazım diline göre değişti: %q / %q", outputs[0], outputs[1])
	}
	bad := t.TempDir()
	testkit.WriteFile(t, bad, "bozuk.tr", "eğer {\n")
	testkit.Wait(t, testkit.RunBinary(t, trBinary, bad, "", []string{"TR_DIL=en"}, "bozuk.tr"), 1, nil, []string{"Syntax Errors"})
	if shapes[0] != shapes[1] {
		t.Fatalf("dosya biçimi yazım diline göre değişti:\nvarsayılan:\n%s\nTR_DIL=en:\n%s", shapes[0], shapes[1])
	}
	for _, key := range []string{"ozet.json", "durum.json", "\"sürüm\"", "\"dosyalar\"", "\"nesneler\"", "metrics.jsonl", "\"tür\""} {
		if !strings.Contains(shapes[0], key) {
			t.Errorf("biçim özeti %s içermeli:\n%s", key, shapes[0])
		}
	}
}

// Veri biçimleri dosyanın dilinden de bağımsızdır (3, 17.1): aynı program Türkçe (dil=tr) ve
// İngilizce (dil=en) yazıldığında değerlerin metni değişir ama JSON metni, CSV'den okunan değerler
// ve günlük satırı bayt bayt aynıdır.
func TestDataFormatsIndependentOfFileLanguage(t *testing.T) {
	program := "yazdır(json_metin({a: doğru, b: yok, c: [1.5, yanlış], d: \"x\"}))\n" +
		"yazdır(json_metin(csv_çöz(\"a,b\\ndoğru,1\\n\")))\n" +
		"günlük(\"metrics.jsonl\").ekle({hazır: doğru, boş: yok})\n" +
		"yazdır(doğru)\n"
	coded, err := source.Encode(program)
	if err != nil {
		t.Fatal(err)
	}
	var data, logs [2]string
	for i, language := range []string{"tr", "en"} {
		dir := t.TempDir()
		testkit.WriteFile(t, dir, "p.tr", inLanguage(coded, language))
		s := testkit.RunBinary(t, trBinary, dir, "", nil, "p.tr")
		testkit.Wait(t, s, 0, nil, nil)
		lines := strings.SplitN(s.Stdout, "\n", 3)
		if want := map[string]string{"tr": "doğru\n", "en": "true\n"}[language]; lines[2] != want {
			t.Errorf("dil=%s: değerin metni %q, beklenen %q", language, lines[2], want)
		}
		data[i] = lines[0] + "\n" + lines[1]
		// Günlüğün kaydı (zaman damgasından sonrası).
		_, logs[i], _ = strings.Cut(readFile(t, filepath.Join(dir, "metrics.jsonl")), "\"hazır\"")
	}
	if want := ":true,\"boş\":null}\n"; logs[0] != want {
		t.Errorf("günlük kaydı %q, beklenen %q", logs[0], want)
	}
	if want := "{\"a\":true,\"b\":null,\"c\":[1.5,false],\"d\":\"x\"}\n[[\"a\",\"b\"],[\"doğru\",1]]"; data[0] != want {
		t.Errorf("JSON ve CSV: %q, beklenen %q", data[0], want)
	}
	if data[0] != data[1] || logs[0] != logs[1] {
		t.Errorf("veri biçimi dosyanın diline göre değişti:\ntr: %q %q\nen: %q %q", data[0], logs[0], data[1], logs[1])
	}
}

// Kod taşıyan çıplak sözlük anahtarı dosyanın dilindedir (3.7, 17.3): İngilizce betikte
// komut_satırı({step: …}) seçeneği --step, json_metin anahtarı "step" olur; Türkçe program
// İngilizce yazılmış JSON'daki "step"'i .adım ile bulur. Kitaplığın ve motorların kendi dosya
// anahtarları (config.json, kontrol noktası) Türkçe kalır.
func TestDictKeysFollowFileLanguage(t *testing.T) {
	program := "aktar \"tr/egitim\" olarak e\n" +
		"değişken ayar = komut_satırı({adım: 3, öğrenme_oranı: 0.5})\n" +
		"değişken koşu = e.Koşu(\"kosu\", ayar, {}, adım_sayısı: ayar.adım)\n" +
		"değişken kn = kontrol_noktası(\"nokta\", {w: tensör([1.0])})\n" +
		"kn.güncelle(1)\n" +
		"kn.kaydet()\n" +
		"yazdır(json_metin(ayar))\n" +
		"yazdır(json_çöz(`{\"step\": 4}`).adım)\n"
	coded, err := source.Encode(program)
	if err != nil {
		t.Fatal(err)
	}
	var shapes [2]string
	for i, c := range []struct{ language, option, want string }{
		{"tr", "--adım", "{\"adım\":2,\"öğrenme_oranı\":0.5}\n4\n"},
		{"en", "--step", "{\"step\":2,\"lr\":0.5}\n4\n"},
	} {
		dir := t.TempDir()
		testkit.WriteFile(t, dir, "p.tr", inLanguage(coded, c.language))
		s := testkit.RunBinary(t, trBinary, dir, "", nil, "p.tr", c.option, "2")
		testkit.Wait(t, s, 0, nil, nil)
		if got := progressLine.ReplaceAllString(s.Stdout, ""); got != c.want {
			t.Errorf("dil=%s: çıktı %q, beklenen %q", c.language, got, c.want)
		}
		var config map[string]any
		if err := json.Unmarshal([]byte(readFile(t, filepath.Join(dir, "kosu", "config.json"))), &config); err != nil {
			t.Fatal(err)
		}
		var top []string
		for k := range config {
			top = append(top, k)
		}
		sort.Strings(top)
		if got := strings.Join(top, " "); got != "ayar cihaz parametre_sayısı" {
			t.Errorf("dil=%s: config.json anahtarları %q, kitaplığın Türkçe anahtarları kalmalı", c.language, got)
		}
		shapes[i] = fileFormatShape(t, filepath.Join(dir, "nokta"))
	}
	if shapes[0] != shapes[1] || !strings.Contains(shapes[0], "\"sürüm\"") {
		t.Errorf("kontrol noktasının biçimi dosyanın diline göre değişti:\ntr:\n%s\nen:\n%s", shapes[0], shapes[1])
	}
}

// Fonksiyon, yerleşik ve yapı değerinin metni de dosyanın dilindedir, çalıştıranın diline
// (TR_DIL) bağlı değildir (3): aynı program her makinede aynı metni verir.
func TestFunctionTextFollowsFileLanguage(t *testing.T) {
	program := "fonksiyon f() { sonuç 1 }\nyapı P { a }\ndeğişken l = [1]\n" +
		"yazdır(metin(f), metin(P), metin(yazdır), metin(l.ekle), \"{f}\")\n"
	coded, err := source.Encode(program)
	if err != nil {
		t.Fatal(err)
	}
	for language, want := range map[string]string{
		"tr": "<fonksiyon f> <yapı P> <yerleşik yazdır> <yerleşik ekle> <fonksiyon f>\n",
		"en": "<function f> <struct P> <built-in print> <built-in append> <function f>\n",
	} {
		dir := t.TempDir()
		testkit.WriteFile(t, dir, "p.tr", inLanguage(coded, language))
		for _, env := range [][]string{{"TR_DIL=tr"}, {"TR_DIL=en"}} {
			if s := testkit.RunBinary(t, trBinary, dir, "", env, "p.tr"); s.Code != 0 || s.Stdout != want {
				t.Errorf("dil=%s, %s: %q (çıkış %d, %s), beklenen %q", language, env[0], s.Stdout, s.Code, s.Stderr, want)
			}
		}
	}
}

// Sonsuz ve tanımsız sayının adı (A110, A111) sayıda ve tensörde aynıdır, dosyanın dilindedir
// ve çalıştıranın diline (TR_DIL) bağlı değildir (3.2). Önceden tensör yazdırması adları
// iletiden alıyordu: aynı program TR_DIL'e göre farklı yazıyordu ("Tanımsız" / "NaN").
func TestNonFiniteNamesFollowFileLanguage(t *testing.T) {
	program := "değişken t = tensör([0.0, 1.0, -1.0]) / 0.0\ndeğişken s = ondalık(\"inf\")\n" +
		"yazdır(t, -s, s, ondalık(\"nan\"), metin(tensör([2.0], türev_izle: doğru)), \"{tensör(s)}\")\n"
	coded, err := source.Encode(program)
	if err != nil {
		t.Fatal(err)
	}
	for language, want := range map[string]string{
		"tr": "Tensör[3] [    NaN,  sonsuz, -sonsuz] -sonsuz sonsuz NaN Tensör[1] (türev izleniyor) [2.0000] Tensör(sonsuz)\n",
		"en": "Tensor[3] [ nan,  inf, -inf] -inf inf nan Tensor[1] (requires grad) [2.0000] Tensor(inf)\n",
	} {
		dir := t.TempDir()
		testkit.WriteFile(t, dir, "p.tr", inLanguage(coded, language))
		for _, env := range [][]string{{"TR_DIL=tr"}, {"TR_DIL=en"}} {
			if s := testkit.RunBinary(t, trBinary, dir, "", env, "p.tr"); s.Code != 0 || s.Stdout != want {
				t.Errorf("dil=%s, %s: %q (çıkış %d, %s), beklenen %q", language, env[0], s.Stdout, s.Code, s.Stderr, want)
			}
		}
	}
}

// fileFormatShape klasördeki dosya adlarını ve her JSON dosyasının (satırının) anahtar
// kümesini, değerleri atarak, sıralı bir özet olarak verir.
func fileFormatShape(t *testing.T, dir string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "kosu/kontrol_noktalari/adim_") {
			// Adım klasörünün adı sabit ("adim_0000000001"); özet yalnızca dosya adını tutar.
			rel = "kosu/kontrol_noktalari/adim_N/" + filepath.Base(rel)
		}
		lines = append(lines, "dosya "+rel)
		if strings.HasSuffix(rel, "egit.tr") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		switch {
		case strings.HasSuffix(rel, ".json"):
			lines = append(lines, rel+" "+jsonKeys(t, data))
		case strings.HasSuffix(rel, ".jsonl"):
			for _, l := range strings.Split(strings.TrimSpace(string(data)), "\n") {
				lines = append(lines, rel+" "+jsonKeys(t, []byte(l)))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// jsonKeys bir JSON nesnesinin anahtarlarını (iç içe nesneler dahil) sıralı bir metin olarak
// verir; dizi elemanları ve değerler atılır, kullanıcı anahtarları (kayıp, w) süzülmez.
func jsonKeys(t *testing.T, data []byte) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("geçersiz JSON %q: %v", data, err)
	}
	var keys []string
	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				keys = append(keys, prefix+"\""+k+"\"")
				walk(prefix+"\""+k+"\".", e)
			}
		case []any:
			for _, e := range x {
				walk(prefix+"[].", e)
			}
		}
	}
	walk("", v)
	sort.Strings(keys)
	return strings.Join(keys, " ")
}
