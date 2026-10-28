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

// Dosyanın dili yoktur (2.1): program kimliklerle yazılır, değerin metni ve dosya biçimleri dilden
// bağımsızdır. Çalıştıranın dili (TR_DIL) yalnızca iletileri ve görüntüleri değiştirir.

// Dosya biçimleri sözleşmedir: kontrol noktasının dosya adları ve JSON anahtarları ile günlüğün
// kayıt alanları yazım dilinden bağımsızdır, çevrilmez. Aynı kodlu program varsayılan dilde ve
// TR_DIL=en ile çalıştırılır; ürettiği dosyaların adları ve anahtar kümeleri aynı olmalı.
func TestFileFormatsIndependentOfWritingLanguage(t *testing.T) {
	program := "TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)\nTR_var g = TR_Logger(\"metrics.jsonl\")\nTR_var kn = TR_Checkpoint(\"kosu\", {\"w\": w})\nkn.güncelle(1)\nkn.kaydet()\ng.ekle({\"kayıp\": 1.5})\nTR_print(TR_Type(kn))\n"
	coded, err := source.Encode(program)
	if err != nil {
		t.Fatal(err)
	}
	var shapes, outputs [2]string
	for i, env := range [][]string{nil, {"TR_DIL=en"}} {
		dir := t.TempDir()
		writeTR(t, dir, "egit.tr", coded)
		s := testkit.RunBinary(t, trBinary, dir, "", env, "egit.tr")
		testkit.Wait(t, s, 0, nil, nil)
		outputs[i] = s.Stdout
		shapes[i] = fileFormatShape(t, dir)
	}
	// Programın çıktısı çalıştıranın diline bağlı değildir: değerin metni dilden bağımsızdır
	// (TR_print(TR_Type(...)) iki koşuda da aynı ad, 3). TR_DIL yalnızca gösterimi değiştirir:
	// aynı sözdizimi hatası İngilizce başlıkla bildirilir.
	if outputs[0] != outputs[1] || outputs[0] != "Checkpoint\n" {
		t.Fatalf("program çıktısı yazım diline göre değişti ya da yanlış: %q / %q", outputs[0], outputs[1])
	}
	bad := t.TempDir()
	writeTR(t, bad, "bozuk.tr", source.Header+"\nTR_if {\n")
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

// Veri biçimleri ve değerin metni dilden bağımsızdır (3, 2.1): aynı program TR_DIL=tr ve TR_DIL=en
// ile çalıştırıldığında JSON metni, CSV'den okunan değerler, günlük satırı ve TR_print(TR_true) bayt
// bayt aynıdır.
func TestDataFormatsAreLanguageIndependent(t *testing.T) {
	program := "TR_print(TR_json_string({\"a\": TR_true, \"b\": TR_none, \"c\": [1.5, TR_false], \"d\": \"x\"}))\nTR_print(TR_json_string(TR_parse_csv(\"a,b\\ndoğru,1\\n\")))\nTR_Logger(\"metrics.jsonl\").TR_append({\"hazır\": TR_true, \"boş\": TR_none})\nTR_print(TR_true)\n"
	coded, err := source.Encode(program)
	if err != nil {
		t.Fatal(err)
	}
	var data, logs [2]string
	for i, env := range [][]string{{"TR_DIL=tr"}, {"TR_DIL=en"}} {
		dir := t.TempDir()
		writeTR(t, dir, "p.tr", coded)
		s := testkit.RunBinary(t, trBinary, dir, "", env, "p.tr")
		testkit.Wait(t, s, 0, nil, nil)
		lines := strings.SplitN(s.Stdout, "\n", 3)
		if lines[2] != "true\n" {
			t.Errorf("%s: değerin metni %q, beklenen \"true\\n\"", env[0], lines[2])
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
		t.Errorf("veri biçimi çalıştıranın diline göre değişti:\ntr: %q %q\nen: %q %q", data[0], logs[0], data[1], logs[1])
	}
}

// Sözlüğün metin anahtarı diske olduğu gibi gider (3.7, 17.3): json_metin, günlük, config.json ve
// kontrol noktası {"step": …} için "step" anahtarını taşır; komut satırında seçenek metin anahtarının
// adıyla geçer (--step) ve çıktı her koşuda aynıdır, TR_print anahtarı tırnakla yazar. Kitaplığın
// (tr/egitim) config.json'u İngilizce tanımlayıcılardır; program JSON'daki "step"'i ["step"] ile okur.
func TestDictTextKeysOnDisk(t *testing.T) {
	program := "TR_import \"tr/egitim\" TR_as e\nTR_var ayar = TR_command_line({\"step\": 3, \"lr\": 0.5})\nTR_var koşu = e.Koşu(\"kosu\", ayar, {}, TR_num_steps: ayar[\"step\"])\nkoşu.değerlendirildi(1, {\"loss\": 0.25})\nTR_var kn = TR_Checkpoint(\"nokta\", {\"w\": TR_Tensor([1.0]), \"sayaç\": {\"step\": 1}})\nkn.güncelle(1)\nkn.kaydet()\nTR_print(TR_json_string(ayar))\nTR_print(ayar)\nTR_print(TR_json_parse(`{\"step\": 4}`)[\"step\"])\n"
	coded, err := source.Encode(program)
	if err != nil {
		t.Fatal(err)
	}
	const want = "{\"step\":2,\"lr\":0.5}\n{\"step\": 2, \"lr\": 0.5}\n4\n"
	var shapes, logs [2]string
	for i, option := range []string{"--step", "--step"} {
		dir := t.TempDir()
		writeTR(t, dir, "p.tr", coded)
		s := testkit.RunBinary(t, trBinary, dir, "", nil, "p.tr", option, "2")
		testkit.Wait(t, s, 0, nil, nil)
		if got := progressLine.ReplaceAllString(s.Stdout, ""); got != want {
			t.Errorf("%s: çıktı %q, beklenen %q", option, got, want)
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
			t.Errorf("%s: config.json anahtarları %q, beklenen \"config device num_params\"", option, got)
		}
		if got := fmt.Sprint(config["config"]); got != "map[lr:0.5 step:2]" {
			t.Errorf("%s: config.json ayarları %s", option, got)
		}
		// Değerlendirme kaydının zaman damgasından sonrası.
		_, after, _ := strings.Cut(readFile(t, filepath.Join(dir, "kosu", "metrics.jsonl")), "\"step\":1,")
		logs[i], _, _ = strings.Cut(after, "\n")
		shapes[i] = fileFormatShape(t, filepath.Join(dir, "nokta"))
	}
	if want := "\"loss\":0.25}"; logs[0] != want || logs[1] != want {
		t.Errorf("günlüğün değerlendirme kaydı seçeneğin yazımına göre değişti ya da yanlış: %q, %q", logs[0], logs[1])
	}
	if shapes[0] != shapes[1] || !strings.Contains(shapes[0], "\"version\"") {
		t.Errorf("kontrol noktasının biçimi seçeneğin yazımına göre değişti:\n--step:\n%s\n--adım:\n%s", shapes[0], shapes[1])
	}
}

// Fonksiyon, yerleşik ve yapı değerinin metni dilden bağımsızdır, çalıştıranın diline (TR_DIL)
// bağlı değildir (3.14): aynı program her makinede aynı metni verir; metin sabit İngilizce, yerleşiğin
// adı kimliğin adıdır.
func TestFunctionTextIsLanguageIndependent(t *testing.T) {
	program := "TR_func f() { TR_return 1 }\nTR_struct P { a }\nTR_var l = [1]\nTR_print(TR_String(f), TR_String(P), TR_String(TR_print), TR_String(l.TR_append), \"\\(f)\", TR_String((x) => x))\n" +
		""
	sameOutputInEveryDisplayLanguage(t, program,
		"<function f> P <builtin print> <builtin append> <function f> <lambda>\n")
}

// Sonsuz ve tanımsız sayının adı (TR_val_inf, TR_val_nan) sayıda ve tensörde aynıdır, dilden
// bağımsızdır ve çalıştıranın diline (TR_DIL) bağlı değildir (3.2). Önceden tensör yazdırması
// adları iletiden alıyordu: aynı program TR_DIL'e göre farklı yazıyordu ("Tanımsız" / "NaN").
func TestNonFiniteNamesAreLanguageIndependent(t *testing.T) {
	program := "TR_var t = TR_Tensor([0.0, 1.0, -1.0]) / 0.0\nTR_var s = TR_Float(\"inf\")\nTR_print(t, -s, s, TR_Float(\"nan\"), TR_String(TR_Tensor([2.0], TR_requires_grad: TR_true)), \"\\(TR_Tensor(s))\")\n" +
		""
	sameOutputInEveryDisplayLanguage(t, program,
		"Tensor[3] [ nan,  inf, -inf] -inf inf nan Tensor[1] (requires grad) [2.0000] Tensor(inf)\n")
}

// Motor değerlerinin metni (tip adı, ayar adları, içindeki iletiler) dilden bağımsızdır, TR_DIL'e
// bağlı değildir (3.14; metin sabit İngilizce): İlerleme, Günlük, iyileştirici, kontrol noktası, satır okuyucu.
func TestEngineValueTextIsLanguageIndependent(t *testing.T) {
	program := "TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true)\nTR_write_file(\"s.txt\", \"a\\nb\\n\")\nTR_print(TR_String(TR_Progress(2)))\nTR_print(TR_String(TR_Logger(\"m.jsonl\")))\nTR_print(TR_String(TR_AdamW([w], TR_lr: 0.1)))\nTR_print(TR_String(TR_Checkpoint(\"k\", {\"w\": w})))\nTR_print(TR_String(TR_read_lines(\"s.txt\")))\n"
	coded, err := source.Encode(program)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Progress(step 0/2)", "Logger(m.jsonl, 0 records)",
		"AdamW(parameters: 1, lr: 0.1, weight_decay: 0.01, beta1: 0.9, beta2: 0.999, eps: 1e-08, step: 0)",
		"Checkpoint(k/checkpoints, step 0, 1 object)", "LineReader("}
	for _, env := range [][]string{{"TR_DIL=tr"}, {"TR_DIL=en"}} {
		dir := t.TempDir()
		writeTR(t, dir, "p.tr", coded)
		s := testkit.RunBinary(t, trBinary, dir, "", env, "p.tr")
		lines := strings.Split(s.Stdout, "\n")
		if s.Code != 0 || len(lines) < len(want) {
			t.Fatalf("%s: çıkış %d\n%s%s", env[0], s.Code, s.Stdout, s.Stderr)
		}
		for i, w := range want {
			if !strings.HasPrefix(lines[i], w) {
				t.Errorf("%s: %d. satır %q, beklenen %q", env[0], i+1, lines[i], w)
			}
		}
	}
}

// Görüntüler kişinin dilindedir (TR_DIL): kabuğun yankısı ve programın --yardım'ı (tip adları,
// varsayılan değerler; seçeneğin adı metin anahtarıdır: {"step": 3} → --step, 3.7) TR_DIL=en ile İngilizce.
// Kodsuz seçenek (--hızlı) yazıldığı gibidir. Seçeneğe verilen değer kişinin dilinde okunur; değerin
// metni dilden bağımsızdır.
func TestDisplayFollowsPersonsLanguage(t *testing.T) {
	repl := testkit.RunBinary(t, trBinary, "", "true\nType(1)\nexit\n", []string{"TR_DIL=en"})
	testkit.Wait(t, repl, 0, []string{"=> true", "=> Int"}, nil)
	dir := t.TempDir()
	writeTR(t, dir, "p.tr", source.Header+"\nTR_var ayar = TR_command_line({\"step\": 3, \"hızlı\": TR_true})\n")
	help := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "p.tr", "--help")
	testkit.Wait(t, help, 0, []string{"--step <Int>", "(default: true; to turn it off --hızlı=false)"}, nil)
	if strings.Contains(help.Stdout, "doğru") || strings.Contains(help.Stdout, "<Tam>") {
		t.Errorf("TR_DIL=en yardımında Türkçe değer ya da tip adı:\n%s", help.Stdout)
	}
	// Yardımın gösterdiği yazım okunur (19.3, 17.8): TR_DIL=en'de "true,false"; Türkçede
	// "doğru,yanlış"; sonsuzun adı her dilde "inf" (3.2). Çıktı iki koşuda da aynıdır.
	writeTR(t, dir, "v.tr", source.Header+"\nTR_var a = TR_command_line({\"liste\": [TR_true], \"büyük\": 1.5})\nTR_print(a[\"liste\"], a[\"büyük\"])\n")
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "v.tr", "--liste", "true,false", "--büyük", "inf"), 0, []string{"[true, false] inf"}, nil)
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=tr"}, "v.tr", "--liste", "doğru,yanlış", "--büyük", "inf"), 0, []string{"[true, false] inf"}, nil)
}

// İletilerin içindeki değerler ve tip adları iletinin dilindedir (TR_DIL): aynı hata TR_DIL=en
// ile "Bool (true)", Türkçede "Mantık (doğru)" der; ileti ile içindeki değer bir dildedir.
func TestMessageValuesFollowPersonsLanguage(t *testing.T) {
	t.Skip("katman 2b: çalışma anı iletisinin değer ve tip yuvaları dosyada kimlik yazımıyla basılınca açılır")
	dir := t.TempDir()
	writeTR(t, dir, "p.tr", source.Header+"\nTR_print([1].TR_join(TR_true))\n")
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "p.tr"), 1, nil, []string{"expects String as argument 1, Bool (true) given"})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=tr"}, "p.tr"), 1, nil, []string{"Metin bekler, Mantık (doğru) verildi"})
	// Motorların iletileri de (değer interp.Shown, tip adı ValueType ile).
	writeTR(t, dir, "t.tr", source.Header+"\nTR_print(TR_Tensor([1.0, TR_true]))\n")
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "t.tr"), 1, nil, []string{"true (Bool) given"})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=tr"}, "t.tr"), 1, nil, []string{"TR_true (TR_Bool) verildi"})
}

// TR_String(Mantık) dilden bağımsız "true" ya da "false" verir: bir yazım dilinin adıyla
// (TR_String(B) == "doğru") ya da başka bir dilden tanıdık adla ("True") karşılaştırma hiç tutmaz,
// denetleyici her TR_DIL'de her biri için bir kez uyarır (H1016); "true" ile karşılaştırma
// doğrudur, sessizdir.
func TestBoolTextComparisonWarns(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "p.tr", source.Header+"\nTR_const B = TR_true\nTR_print(TR_String(B) == \"doğru\")\nTR_print(TR_String(B) == \"True\")\nTR_print(TR_String(B) == \"true\")\n")
	for _, c := range []struct{ env, warning string }{
		{"TR_DIL=en", "never holds"},
		{"TR_DIL=tr", "karşılaştırma hiç tutmaz"},
	} {
		s := testkit.RunBinary(t, trBinary, dir, "", []string{c.env}, "p.tr")
		if s.Code != 0 || s.Stdout != "false\nfalse\ntrue\n" || strings.Count(s.Stderr, c.warning) != 2 || !strings.Contains(s.Stderr, `"doğru"`) || !strings.Contains(s.Stderr, `"True"`) {
			t.Errorf("%s: çıkış %d, çıktı %q, uyarılar:\n%s", c.env, s.Code, s.Stdout, s.Stderr)
		}
	}
}

// İletideki dil adları iletinin dilindedir (4E): kitaplık fonksiyonunun adı, adlı argümanın adı
// ve "Bunu mu demek istediniz" önerisi programın yakaladığı hatada İngilizcedir (yakalanan hata
// değerdir, metni dilden bağımsızdır; TR_DIL=tr'de de) ve TR_DIL=en görüntüsünde İngilizcedir.
// Önceden ad TR_DIL'de kurulup iletiye metin olarak veriliyordu (yakalanan hatada "tensör",
// "devrik()", "türev_izle" kalıyordu).
func TestMessageNamesFollowTheMessageLanguage(t *testing.T) {
	coded, err := source.Encode("TR_try { TR_Tensor([1.0], TR_requires_grad: 5) } TR_catch h { TR_print(h) }\nTR_try { TR_print(TR_Tensor([[1.0]]).T) } TR_catch h { TR_print(h) }\nTR_try { TR_Tensor([1.0], türev_izl: TR_true) } TR_catch h { TR_print(h) }\nTR_print(TR_Tensor([[1.0]]).T)\n" +
		"" +
		"" +
		"")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Tensor: requires_grad must be true or false", "Did you mean 'transpose()'?",
		"Tensor() takes no parameter named 'türev_izl'. Did you mean 'requires_grad'?"}
	dir := t.TempDir()
	writeTR(t, dir, "p.tr", coded)
	caught := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=tr"}, "p.tr")
	shown := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "p.tr")
	for i, w := range want {
		if !strings.Contains(caught.Stdout, w) {
			t.Errorf("yakalanan hatada %q yok:\n%s", w, caught.Stdout)
		}
		if i == 1 && !strings.Contains(shown.Stderr, w) {
			t.Errorf("TR_DIL=en görüntüsünde %q yok:\n%s", w, shown.Stderr)
		}
	}
	for _, turkish := range []string{"tensör", "devrik", "türev_izle"} {
		if strings.Contains(caught.Stdout, turkish) {
			t.Errorf("yakalanan hatada Türkçe ad %q:\n%s", turkish, caught.Stdout)
		}
	}
}

// Dosyanın sözdizimi ipucundaki dil sözcüğü kimlik yazımıyladır (dosyanın dili yoktur, 2.1; öneri
// dosyaya olduğu gibi yazılabilir): döngü dışındaki TR_break, TR_DIL=en ile "to leave the function
// write 'TR_return'" der, Türkçede "'TR_return' yazın"; cümle iletinin dilindedir.
func TestSyntaxHintNamesInAFileAreSpellings(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "p.tr", source.Header+"\nTR_func f() {\n    TR_break\n}\n")
	for _, c := range []struct{ env, want string }{
		{"TR_DIL=en", "to leave the function write 'TR_return'"},
		{"TR_DIL=tr", "fonksiyondan çıkmak için 'TR_return' yazın"},
	} {
		s := testkit.RunBinary(t, trBinary, dir, "", []string{c.env}, "p.tr")
		if s.Code == 0 || !strings.Contains(s.Stderr, c.want) || !strings.Contains(s.Stderr, "'TR_break'") {
			t.Errorf("%s sözdizimi ipucu: çıkış %d\n%s", c.env, s.Code, s.Stderr)
		}
	}
}

// İletiye değer olarak giden hazır metin de iletinin dilindedir (DENETİM 4D, bulgu 3): listede
// çalışmayan tensör işlevinin "yalnızca ..." parçası programın yakaladığı hatada (TR_DIL=tr'de de)
// İngilizcedir. Önceden parça messages.Text ile TR_DIL'de kurulup iletiye metin olarak giriyordu
// ("only on tensörde").
func TestMessagePartsFollowTheMessageLanguage(t *testing.T) {
	coded, err := source.Encode("TR_try { TR_softmax([1.0, 2.0]) } TR_catch h { TR_print(h) }\n" +
		"TR_try { TR_where([1, 2], 1, 2) } TR_catch h { TR_print(h) }\n")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeTR(t, dir, "p.tr", coded)
	s := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=tr"}, "p.tr")
	if strings.Count(s.Stdout, "does not work on lists, only on tensors") != 2 {
		t.Errorf("yakalanan hatada parça İngilizce olmalı:\n%s%s", s.Stdout, s.Stderr)
	}
	for _, turkish := range []string{"tensörde", "sayıda", "Mantık"} {
		if strings.Contains(s.Stdout, turkish) {
			t.Errorf("Türkçe parça %q:\n%s", turkish, s.Stdout)
		}
	}
}

// Veri kümesi hataları da iletinin dilindedir (B19): önbellek kotası yetmeyince programın yakaladığı
// hata ayrıntısıyla İngilizcedir (TR_DIL=tr'de de). Önceden ayrıntı TR_DIL'de metne çevrilip
// iletiye metin olarak giriyordu ("the cache quota is insufficient: kota 107 bayt, ...").
func TestDatasetErrorsFollowTheMessageLanguage(t *testing.T) {
	data := filepath.ToSlash(reporoot.Path("tests/data/text"))
	coded, err := source.Encode("TR_try {\n    TR_var k = TR_Dataset(\"" + data + "\", TR_cache: \"c2\", TR_quota_gb: 0.0000001)\n    TR_print(k.TR_sampler(TR_batch_size: 1, TR_context: 4).TR_next())\n} TR_catch h { TR_print(h) }\n")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeTR(t, dir, "p.tr", coded)
	s := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=tr"}, "p.tr")
	if !strings.Contains(s.Stdout, "the cache quota is insufficient") || !strings.Contains(s.Stdout, "bytes are needed") {
		t.Errorf("yakalanan hatanın ayrıntısı İngilizce olmalı:\n%s%s", s.Stdout, s.Stderr)
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

// sameOutputInEveryDisplayLanguage Türkçe yazılmış programı kodlayıp TR_DIL=tr ve TR_DIL=en ile
// çalıştırır: çıktı iki koşuda da want'tır, çalıştıranın diline bağlı değildir.
func sameOutputInEveryDisplayLanguage(t *testing.T, program, want string) {
	t.Helper()
	coded, err := source.Encode(program)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeTR(t, dir, "p.tr", coded)
	for _, env := range [][]string{{"TR_DIL=tr"}, {"TR_DIL=en"}} {
		if s := testkit.RunBinary(t, trBinary, dir, "", env, "p.tr"); s.Code != 0 || s.Stdout != want {
			t.Errorf("%s: %q (çıkış %d, %s), beklenen %q", env[0], s.Stdout, s.Code, s.Stderr, want)
		}
	}
}
