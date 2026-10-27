package e2e

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Dosya biçimleri sözleşmedir: kontrol noktasının dosya adları ve JSON anahtarları ile günlüğün
// kayıt alanları yazım dilinden bağımsızdır, çevrilmez. Aynı kodlu program varsayılan dilde ve
// TR_DIL=en ile çalıştırılır; ürettiği dosyaların adları ve anahtar kümeleri aynı olmalı.
func TestFileFormatsIndependentOfWritingLanguage(t *testing.T) {
	program := "TR_var w = TR_tensor([1.0, 2.0], TR_requires_grad: TR_true)\n" +
		"TR_var g = TR_logger(\"metrics.jsonl\")\n" +
		"TR_var kn = TR_checkpoint(\"kosu\", {w: w})\n" +
		"kn.güncelle(1)\n" +
		"kn.kaydet()\n" +
		"g.ekle({kayıp: 1.5})\n" +
		"TR_print(TR_type(kn))\n"
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
	for _, key := range []string{"digest.json", "state.json", "\"version\"", "\"files\"", "\"objects\"", "metrics.jsonl", "\"kind\""} {
		if !strings.Contains(shapes[0], key) {
			t.Errorf("biçim özeti %s içermeli:\n%s", key, shapes[0])
		}
	}
}

// Veri biçimleri dosyanın dilinden de bağımsızdır (3, 17.1): aynı program Türkçe (dil=tr) ve
// İngilizce (dil=en) yazıldığında değerlerin metni değişir ama JSON metni, CSV'den okunan değerler
// ve günlük satırı bayt bayt aynıdır.
func TestDataFormatsIndependentOfFileLanguage(t *testing.T) {
	program := "TR_print(TR_json_string({a: TR_true, b: TR_none, c: [1.5, TR_false], d: \"x\"}))\n" +
		"TR_print(TR_json_string(TR_parse_csv(\"a,b\\ndoğru,1\\n\")))\n" +
		"TR_logger(\"metrics.jsonl\").TR_append({hazır: TR_true, boş: TR_none})\n" +
		"TR_print(TR_true)\n"
	coded, err := source.Encode(program)
	if err != nil {
		t.Fatal(err)
	}
	var data, logs [2]string
	for i, language := range []string{"tr", "en"} {
		dir := t.TempDir()
		testkit.WriteFile(t, dir, "p.tr", coded)
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

// Kod taşıyan çıplak sözlük anahtarı diske kodun dilden bağımsız adıyla gider (3.7, 17.3): aynı
// program Türkçe ve İngilizce yazıldığında json_metin, günlük, config.json ve kontrol noktası bayt
// bayt aynı anahtarları taşır ({adım: …} de {step: …} de "step"); komut satırında kodlu seçenek her
// dilin adıyla geçer (--adım, --step; 17.8), yazdır anahtarı dosyanın dilinde ve tırnaksız yazar. Kitaplığın
// (tr/egitim) config.json'u İngilizce tanımlayıcılardır; Türkçe program İngilizce JSON'daki
// "step"'i .adım ile bulur.
func TestDictCodedKeysOnDisk(t *testing.T) {
	program := "TR_import \"tr/egitim\" TR_as e\n" +
		"TR_var ayar = TR_command_line({TR_step: 3, TR_lr: 0.5})\n" +
		"TR_var koşu = e.Koşu(\"kosu\", ayar, {}, TR_num_steps: ayar.TR_step)\n" +
		"koşu.değerlendirildi(1, {kayıp: 0.25})\n" +
		"TR_var kn = TR_checkpoint(\"nokta\", {w: TR_tensor([1.0]), sayaç: {TR_step: 1}})\n" +
		"kn.güncelle(1)\n" +
		"kn.kaydet()\n" +
		"TR_print(TR_json_string(ayar))\n" +
		"TR_print(ayar)\n" +
		"TR_print(TR_json_parse(`{\"step\": 4}`).TR_step)\n"
	coded, err := source.Encode(program)
	if err != nil {
		t.Fatal(err)
	}
	var shapes, logs [2]string
	for i, c := range []struct{ language, option, want string }{
		{"tr", "--step", "{\"step\":2,\"lr\":0.5}\n{adım: 2, öğrenme_oranı: 0.5}\n4\n"},
		{"en", "--adım", "{\"step\":2,\"lr\":0.5}\n{step: 2, lr: 0.5}\n4\n"},
	} {
		dir := t.TempDir()
		testkit.WriteFile(t, dir, "p.tr", coded)
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
		if got := strings.Join(top, " "); got != "config device num_params" {
			t.Errorf("dil=%s: config.json anahtarları %q, beklenen \"config device num_params\"", c.language, got)
		}
		if got := fmt.Sprint(config["config"]); got != "map[lr:0.5 step:2]" {
			t.Errorf("dil=%s: config.json ayarları %s", c.language, got)
		}
		// Değerlendirme kaydının zaman damgasından sonrası.
		_, after, _ := strings.Cut(readFile(t, filepath.Join(dir, "kosu", "metrics.jsonl")), "\"step\":1,")
		logs[i], _, _ = strings.Cut(after, "\n")
		shapes[i] = fileFormatShape(t, filepath.Join(dir, "nokta"))
	}
	if want := "\"loss\":0.25}"; logs[0] != want || logs[1] != want {
		t.Errorf("günlüğün değerlendirme kaydı dile göre değişti ya da yanlış: tr %q, en %q", logs[0], logs[1])
	}
	if shapes[0] != shapes[1] || !strings.Contains(shapes[0], "\"version\"") {
		t.Errorf("kontrol noktasının biçimi dosyanın diline göre değişti:\ntr:\n%s\nen:\n%s", shapes[0], shapes[1])
	}
}

// Fonksiyon, yerleşik ve yapı değerinin metni de dosyanın dilindedir, çalıştıranın diline
// (TR_DIL) bağlı değildir (3): aynı program her makinede aynı metni verir.
func TestFunctionTextFollowsFileLanguage(t *testing.T) {
	program := "TR_func f() { TR_return 1 }\nTR_struct P { a }\nTR_var l = [1]\n" +
		"TR_print(TR_str(f), TR_str(P), TR_str(TR_print), TR_str(l.TR_append), \"\\(f)\", TR_str(x => x))\n"
	sameOutputInEveryLanguage(t, program, map[string]string{
		"tr": "<fonksiyon f> <yapı P> <yerleşik yazdır> <yerleşik ekle> <fonksiyon f> <ok fonksiyonu>\n",
		"en": "<function f> <struct P> <built-in print> <built-in append> <function f> <lambda>\n",
	})
}

// Sonsuz ve tanımsız sayının adı (TR_val_inf, TR_val_nan) sayıda ve tensörde aynıdır, dosyanın dilindedir
// ve çalıştıranın diline (TR_DIL) bağlı değildir (3.2). Önceden tensör yazdırması adları
// iletiden alıyordu: aynı program TR_DIL'e göre farklı yazıyordu ("Tanımsız" / "NaN").
func TestNonFiniteNamesFollowFileLanguage(t *testing.T) {
	program := "TR_var t = TR_tensor([0.0, 1.0, -1.0]) / 0.0\nTR_var s = TR_float(\"inf\")\n" +
		"TR_print(t, -s, s, TR_float(\"nan\"), TR_str(TR_tensor([2.0], TR_requires_grad: TR_true)), \"\\(TR_tensor(s))\")\n"
	sameOutputInEveryLanguage(t, program, map[string]string{
		"tr": "Tensör[3] [    NaN,  sonsuz, -sonsuz] -sonsuz sonsuz NaN Tensör[1] (türev izleniyor) [2.0000] Tensör(sonsuz)\n",
		"en": "Tensor[3] [ nan,  inf, -inf] -inf inf nan Tensor[1] (requires grad) [2.0000] Tensor(inf)\n",
	})
}

// Motor değerlerinin metni (tip adı, ayar adları, içindeki iletiler) dosyanın dilindedir,
// TR_DIL'e bağlı değildir (DENETİM 3): İlerleme, Günlük, iyileştirici, kontrol noktası, satır
// okuyucu. Önceden tip adı kanonik, geri kalanı TR_DIL'deydi.
func TestEngineValueTextFollowsFileLanguage(t *testing.T) {
	program := "TR_var w = TR_tensor([1.0], TR_requires_grad: TR_true)\n" +
		"TR_write_file(\"s.txt\", \"a\\nb\\n\")\n" +
		"TR_print(TR_str(TR_progress(2)))\nTR_print(TR_str(TR_logger(\"m.jsonl\")))\n" +
		"TR_print(TR_str(TR_AdamW([w], TR_lr: 0.1)))\nTR_print(TR_str(TR_checkpoint(\"k\", {w: w})))\n" +
		"TR_print(TR_str(TR_read_lines(\"s.txt\")))\n"
	coded, err := source.Encode(program)
	if err != nil {
		t.Fatal(err)
	}
	for language, want := range map[string][]string{
		"tr": {"İlerleme(adım 0/2)", "Günlük(m.jsonl, 0 kayıt)",
			"TR_AdamW(parametre: 1, TR_lr: 0.1, TR_weight_decay: 0.01, TR_beta1: 0.9, TR_beta2: 0.999, TR_eps: 1e-08, TR_step: 0)",
			"KontrolNoktası(k/checkpoints, adım 0, 1 nesne)", "SatırOkuyucu("},
		"en": {"Progress(step 0/2)", "Logger(m.jsonl, 0 records)",
			"TR_AdamW(parameters: 1, lr: 0.1, weight_decay: 0.01, TR_beta1: 0.9, TR_beta2: 0.999, eps: 1e-08, step: 0)",
			"Checkpoint(k/checkpoints, step 0, 1 object)", "LineReader("},
	} {
		for _, env := range [][]string{{"TR_DIL=tr"}, {"TR_DIL=en"}} {
			dir := t.TempDir()
			testkit.WriteFile(t, dir, "p.tr", coded)
			s := testkit.RunBinary(t, trBinary, dir, "", env, "p.tr")
			lines := strings.Split(s.Stdout, "\n")
			if s.Code != 0 || len(lines) < len(want) {
				t.Fatalf("dil=%s, %s: çıkış %d\n%s%s", language, env[0], s.Code, s.Stdout, s.Stderr)
			}
			for i, w := range want {
				if !strings.HasPrefix(lines[i], w) {
					t.Errorf("dil=%s, %s: %d. satır %q, beklenen %q", language, env[0], i+1, lines[i], w)
				}
			}
		}
	}
}

// Görüntüler kişinin dilindedir (TR_DIL), dosyanın dilinde değil: kabuğun yankısı ve programın
// --yardım'ı (tip adları, varsayılan değerler, kodlu seçeneğin adı: {adım: 3} → --step; 17.8)
// TR_DIL=en ile İngilizce, Türkçe dosyada da. Kodsuz seçenek (--hızlı) yazıldığı gibidir.
func TestDisplayFollowsPersonsLanguage(t *testing.T) {
	repl := testkit.RunBinary(t, trBinary, "", "true\ntype(1)\nexit\n", []string{"TR_DIL=en"})
	testkit.Wait(t, repl, 0, []string{"=> true", "=> Int"}, nil)
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "p.tr", "TR_var ayar = TR_command_line({TR_step: 3, hızlı: TR_true})\n")
	help := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "p.tr", "--help")
	testkit.Wait(t, help, 0, []string{"--step <Int>", "(default: true; to turn it off --hızlı=false)"}, nil)
	if strings.Contains(help.Stdout, "doğru") || strings.Contains(help.Stdout, "<Tam>") {
		t.Errorf("TR_DIL=en yardımında Türkçe değer ya da tip adı:\n%s", help.Stdout)
	}
	// Yardımın gösterdiği yazım okunur (19.3, 17.8): TR_DIL=en'de "true,false" ve "inf".
	testkit.WriteFile(t, dir, "v.tr", "TR_var a = TR_command_line({liste: [TR_true], büyük: 1.5})\nTR_print(a.liste, a.büyük)\n")
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "v.tr", "--liste", "true,false", "--büyük", "inf"), 0, []string{"[doğru, yanlış] sonsuz"}, nil)
}

// İletilerin içindeki değerler ve tip adları iletinin dilindedir (TR_DIL): aynı hata TR_DIL=en
// ile "Bool (true)", Türkçede "Mantık (doğru)" der; ileti ile içindeki değer bir dildedir.
func TestMessageValuesFollowPersonsLanguage(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "p.tr", "TR_print([1].TR_join(TR_true))\n")
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "p.tr"), 1, nil, []string{"expects String as argument 1, Bool (true) given"})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=tr"}, "p.tr"), 1, nil, []string{"TR_Bool (TR_true) verildi"})
	// Motorların iletileri de (değer interp.Shown, tip adı ValueType ile).
	testkit.WriteFile(t, dir, "t.tr", "TR_print(TR_tensor([1.0, TR_true]))\n")
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "t.tr"), 1, nil, []string{"true (Bool) given"})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=tr"}, "t.tr"), 1, nil, []string{"TR_true (TR_Bool) verildi"})
}

// Denetleyici dosyanın dilini bilir (source.Prepare): İngilizce dosyada str(b) == "doğru"
// hiç tutmaz, uyarı verir (H1016); aynı karşılaştırma Türkçe dosyada doğrudur, sessizdir.
func TestBoolTextComparisonWarnsByFileLanguage(t *testing.T) {
	coded, err := source.Encode("TR_const b = TR_true\nTR_print(TR_str(b) == \"doğru\")\n")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "en.tr", coded)
	testkit.WriteFile(t, dir, "tr.tr", coded)
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "en.tr"), 0, []string{"false"}, []string{`"doğru" is the name in another writing language`})
	if s := testkit.RunBinary(t, trBinary, dir, "", nil, "tr.tr"); s.Code != 0 || s.Stdout != "doğru\n" || strings.Contains(s.Stderr, "başka bir yazım dilinin") {
		t.Errorf("Türkçe dosyada uyarı olmamalı: %q %q", s.Stdout, s.Stderr)
	}
}

// İletideki dil adları iletinin dilindedir (4E): kitaplık fonksiyonunun adı, adlı argümanın adı
// ve "Bunu mu demek istediniz" önerisi İngilizce dosyanın yakaladığı hatada (TR_DIL=tr) ve
// TR_DIL=en görüntüsünde İngilizcedir. Önceden ad TR_DIL'de kurulup iletiye metin olarak
// veriliyordu (yakalanan hatada "tensör", "devrik()", "türev_izle" kalıyordu).
func TestMessageNamesFollowTheMessageLanguage(t *testing.T) {
	coded, err := source.Encode("TR_try { TR_tensor([1.0], TR_requires_grad: 5) } TR_catch h { TR_print(h) }\n" +
		"TR_try { TR_print(TR_tensor([[1.0]]).T) } TR_catch h { TR_print(h) }\n" +
		"TR_try { TR_tensor([1.0], türev_izl: TR_true) } TR_catch h { TR_print(h) }\n" +
		"TR_print(TR_tensor([[1.0]]).T)\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"tensor: requires_grad must be true or false", "Did you mean 'transpose()'?",
		"tensor() takes no parameter named 'türev_izl'. Did you mean 'requires_grad'?"}
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "en.tr", coded)
	testkit.WriteFile(t, dir, "tr.tr", coded)
	caught := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=tr"}, "en.tr")
	shown := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "tr.tr")
	for i, w := range want {
		if !strings.Contains(caught.Stdout, w) {
			t.Errorf("İngilizce dosyanın yakaladığı hatada %q yok:\n%s", w, caught.Stdout)
		}
		if i == 1 && !strings.Contains(shown.Stderr, w) {
			t.Errorf("TR_DIL=en görüntüsünde %q yok:\n%s", w, shown.Stderr)
		}
	}
	for _, turkish := range []string{"tensör", "devrik", "türev_izle"} {
		if strings.Contains(caught.Stdout, turkish) {
			t.Errorf("İngilizce dosyanın yakaladığı hatada Türkçe ad %q:\n%s", turkish, caught.Stdout)
		}
	}
}

// Sözdizimi ipucundaki dil adı kişinin dilindedir (DENETİM 4D, bulgu 2): Türkçe dosyada döngü
// dışındaki 'çık', TR_DIL=en ile "to leave the function write 'return'" der. Önceden ad kanonikti
// ('sonuç').
func TestSyntaxHintNamesFollowPersonsLanguage(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "p.tr", "fonksiyon f() {\n    çık\n}\n")
	s := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "p.tr")
	if s.Code == 0 || !strings.Contains(s.Stderr, "to leave the function write 'return'") {
		t.Errorf("TR_DIL=en sözdizimi ipucu: çıkış %d\n%s", s.Code, s.Stderr)
	}
}

// İletiye değer olarak giden hazır metin de iletinin dilindedir (DENETİM 4D, bulgu 3): listede
// çalışmayan tensör işlevinin "yalnızca ..." parçası İngilizce dosyanın yakaladığı hatada
// İngilizcedir. Önceden parça messages.Text ile TR_DIL'de kurulup iletiye metin olarak giriyordu
// ("only on tensörde").
func TestMessagePartsFollowTheMessageLanguage(t *testing.T) {
	coded, err := source.Encode("TR_try { TR_softmax([1.0, 2.0]) } TR_catch h { TR_print(h) }\n" +
		"TR_try { TR_where([1, 2], 1, 2) } TR_catch h { TR_print(h) }\n")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "en.tr", coded)
	s := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=tr"}, "en.tr")
	if strings.Count(s.Stdout, "does not work on lists, only on tensors") != 2 {
		t.Errorf("İngilizce dosyanın yakaladığı hatada parça İngilizce olmalı:\n%s%s", s.Stdout, s.Stderr)
	}
	for _, turkish := range []string{"tensörde", "sayıda", "Mantık"} {
		if strings.Contains(s.Stdout, turkish) {
			t.Errorf("Türkçe parça %q:\n%s", turkish, s.Stdout)
		}
	}
}

// Veri kümesi hataları da iletinin dilindedir (B19): önbellek kotası yetmeyince İngilizce dosyanın
// yakaladığı hata ayrıntısıyla İngilizcedir. Önceden ayrıntı TR_DIL'de metne çevrilip iletiye
// metin olarak giriyordu ("the cache quota is insufficient: kota 107 bayt, ...").
func TestDatasetErrorsFollowTheMessageLanguage(t *testing.T) {
	data := filepath.ToSlash(reporoot.Path("tests/data/text"))
	coded, err := source.Encode("dene {\n    değişken k = veri_kümesi(\"" + data + "\", önbellek: \"c2\", kota_gb: 0.0000001)\n" +
		"    yazdır(k.örnekleyici(toplu: 1, bağlam: 4).sonraki())\n} yakala h { yazdır(h) }\n")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "en.tr", coded)
	s := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=tr"}, "en.tr")
	if !strings.Contains(s.Stdout, "the cache quota is insufficient") || !strings.Contains(s.Stdout, "bytes are needed") {
		t.Errorf("İngilizce iletinin ayrıntısı İngilizce olmalı:\n%s%s", s.Stdout, s.Stderr)
	}
	for _, turkish := range []string{"kota", "bayt", "gerekli", "veri:"} {
		if strings.Contains(s.Stdout, turkish) {
			t.Errorf("Türkçe parça %q:\n%s", turkish, s.Stdout)
		}
	}
}

// fileFormatShape klasördeki dosya adlarını ve her JSON dosyasının (satırının) anahtar
// kümesini, değerleri atarak, sıralı bir özet olarak verir.
func fileFormatShape(t *testing.T, dir string) string {
	t.Helper()
	var lines []string
	err := reporoot.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "kosu/checkpoints/adim_") {
			// Adım klasörünün adı sabit ("step_0000000001"); özet yalnızca dosya adını tutar.
			rel = "kosu/checkpoints/step_N/" + filepath.Base(rel)
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

// sameOutputInEveryLanguage Türkçe yazılmış programı dil=tr ve dil=en kodlu kopyalarıyla,
// TR_DIL=tr ve TR_DIL=en ile çalıştırır: çıktı dosyanın diline göre want'takidir, çalıştıranın
// diline bağlı değildir.
func sameOutputInEveryLanguage(t *testing.T, program string, want map[string]string) {
	t.Helper()
	coded, err := source.Encode(program)
	if err != nil {
		t.Fatal(err)
	}
	for language, want := range want {
		dir := t.TempDir()
		testkit.WriteFile(t, dir, "p.tr", coded)
		for _, env := range [][]string{{"TR_DIL=tr"}, {"TR_DIL=en"}} {
			if s := testkit.RunBinary(t, trBinary, dir, "", env, "p.tr"); s.Code != 0 || s.Stdout != want {
				t.Errorf("dil=%s, %s: %q (çıkış %d, %s), beklenen %q", language, env[0], s.Stdout, s.Code, s.Stderr, want)
			}
		}
	}
}
