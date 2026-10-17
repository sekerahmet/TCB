package yz

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"tr_dili/pkg/vm"
)

func TestBuiltinsRejectBadSizesWithoutPanic(t *testing.T) {
	_, val, err := runTRWithin(t, 5*time.Second, `sonuç [aralık(-5), aralık(5, 2), aralık(3)]`)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if val.Inspect() != "[[], [], [0, 1, 2]]" {
		t.Fatalf("[[], [], [0, 1, 2]] bekleniyordu, %s alındı", val.Inspect())
	}

	for _, code := range []string{
		`aralık(10000000000)`,
		`sıfırlar(-2, 2)`,
		`sıfırlar(-3)`,
		`birler(0, 4)`,
		`birim_matris(-1)`,
		`rastgele_matris(2, -7)`,
		`sıfırlar(100000, 100000)`,
	} {
		_, _, err := runTRWithin(t, 5*time.Second, code)
		if err == nil {
			t.Errorf("%s: hata bekleniyordu", code)
		} else if strings.Contains(err.Error(), "çökme önlendi") {
			t.Errorf("%s: panik yerine düzgün hata bekleniyordu: %v", code, err)
		}
	}
}

func TestNeuralNetTrainingValidatesInput(t *testing.T) {
	for _, tc := range []struct{ code, want string }{
		{`değişken m = sinir_ağı([2, 3, 1])
		  m.eğit([[0, 1], [1, 0]], [[1], [1]], devir: 0)`, "devir"},
		{`değişken m = sinir_ağı([2, 3, 1])
		  m.eğit([[0, 1], [1, 0]], [[1, 0], [1, 0]], devir: 5)`, "çıkış katmanı"},
		{`değişken m = sinir_ağı([2, 3, 1])
		  m.eğit([[0, 1, 1], [1, 0, 1]], [[1], [1]], devir: 5)`, "giriş katmanı"},
	} {
		_, _, err := runTRWithin(t, 5*time.Second, tc.code)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q içeren hata bekleniyordu, alınan: %v", tc.want, err)
		}
	}
}

func TestModelLoadRejectsCorruptFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"bos.json":      `{"katman_boyutları":[2,1],"aktivasyon":"tanh","ağırlıklar":[[]],"sapmalar":[[[0]]]}`,
		"hic.json":      `{"katman_boyutları":[2,1],"aktivasyon":"tanh","ağırlıklar":[],"sapmalar":[]}`,
		"duzensiz.json": `{"katman_boyutları":[2,1],"aktivasyon":"tanh","ağırlıklar":[[[1],[2,3]]],"sapmalar":[[[0]]]}`,
		"sapma.json":    `{"katman_boyutları":[2,1],"aktivasyon":"tanh","ağırlıklar":[[[1],[2]]],"sapmalar":[]}`,
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		_, _, err := runTRWithin(t, 5*time.Second, fmt.Sprintf("model_yükle(%q)", filepath.ToSlash(path)))
		if err == nil || strings.Contains(err.Error(), "çökme önlendi") {
			t.Errorf("%s: 'Model dosyası bozuk' hatası bekleniyordu, alınan: %v", name, err)
		}
	}
}

// yz_sor eskiden sunucu kapalıyken uydurma bir "yanıt" döndürüyordu.
func TestYzSorReportsUnreachableServer(t *testing.T) {
	port := freePort(t) // dinleyen yok
	_, err := runTR(t, fmt.Sprintf(`sonuç yz_sor("merhaba", adres: "http://127.0.0.1:%d/api/generate")`, port))
	if err == nil || !strings.Contains(err.Error(), "ulaşılamadı") {
		t.Fatalf("ulaşılamadı hatası bekleniyordu, alınan: %v", err)
	}

	notFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"model not found"}`, http.StatusNotFound)
	}))
	defer notFound.Close()
	_, err = runTR(t, fmt.Sprintf(`sonuç yz_sor("merhaba", adres: %q)`, notFound.URL))
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("404 hatası bekleniyordu, alınan: %v", err)
	}

	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"response": "  Merhaba!  "}`)
	}))
	defer ok.Close()
	val, err := runTR(t, fmt.Sprintf(`sonuç yz_sor("merhaba", adres: %q)`, ok.URL))
	if err != nil || val.Inspect() != "Merhaba!" {
		t.Fatalf("\"Merhaba!\" bekleniyordu, alınan: %v, %v", val, err)
	}
}

func TestMatrixIndexingAndAssignment(t *testing.T) {
	code := `
		değişken M = matris([[1, 2], [3, 4]])
		M[0][1] = 10
		M[1][0] += 5
		M[1] = [8, 9]
		sonuç [M[0][1], M[1][0], M[1][1]]
	`
	val, err := runTR(t, code)
	if err != nil {
		t.Fatalf("Matris indeksleme/atama hatası: %v", err)
	}
	if val.Inspect() != "[10.0, 8.0, 9.0]" {
		t.Fatalf("[10.0, 8.0, 9.0] bekleniyordu, %s alındı", val.Inspect())
	}
}

func TestAutogradAndNeuralNet(t *testing.T) {
	code := `
		değişken x = tensör(3.0, türev_izle: doğru)
		değişken y = x * x + 2 * x + 1
		geri_yayılım(y)
		sonuç [y.değer, x.gradyan]
	`
	val, err := runTR(t, code)
	if err != nil {
		t.Fatalf("Autograd hatası: %v", err)
	}
	if val.Inspect() != "[16.0, 8.0]" {
		t.Fatalf("[16.0, 8.0] bekleniyordu, %s alındı", val.Inspect())
	}
}

// json_çöz sözlük anahtarlarının sırasını korumuyordu (Go map'i sırasızdır).
func TestJSONDecodePreservesKeyOrder(t *testing.T) {
	_, val, err := runTRWithin(t, 5*time.Second, `
		değişken d = json_çöz("{\"zeta\": 1, \"alfa\": 2.5, \"orta\": [true, null, \"x\"], \"iç\": {\"b\": 1, \"a\": 2}}")
		sonuç [d.anahtarlar(), d["iç"].anahtarlar(), d["orta"], d["alfa"], json_metin(d)]`)
	if err != nil {
		t.Fatal(err)
	}
	want := `[["zeta", "alfa", "orta", "iç"], ["b", "a"], [doğru, yok, "x"], 2.5, `
	if !strings.HasPrefix(val.Inspect(), want) {
		t.Fatalf("%s… bekleniyordu, %s alındı", want, val.Inspect())
	}
}

// d[1] ile d["1"] eskiden Inspect() ile aynı anahtara düşüyordu.
func TestDictKeysAreTypeAware(t *testing.T) {
	val, err := runTR(t, `
		değişken d = {1: "tam", "1": "metin"}
		d[doğru] = "mantık"
		d["doğru"] = "metin2"
		sonuç [d[1], d["1"], d[doğru], d["doğru"], uzunluk(d), d.içerir(1), d.içerir("2")]
	`)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if val.Inspect() != `["tam", "metin", "mantık", "metin2", 4, doğru, yanlış]` {
		t.Fatalf("beklenmeyen sonuç: %s", val.Inspect())
	}

	// Döngü ve anahtarlar() özgün anahtar tiplerini geri vermeli.
	val, err = runTR(t, `
		değişken d = {1: "a", "b": 2}
		değişken tipler = []
		döngü k, v içinde d { tipler = tipler + [tip(k)] }
		sonuç [tipler, d.anahtarlar(), [tip(k) döngü k içinde d]]
	`)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if val.Inspect() != `[["Tam", "Metin"], [1, "b"], ["Tam", "Metin"]]` {
		t.Fatalf("beklenmeyen sonuç: %s", val.Inspect())
	}

	// Yazdırma ve JSON dönüşümü metin anahtarlar için değişmemeli.
	out, _, err := runTROut(t, `
		değişken d = {"ad": "Ali", 1: "bir"}
		yazdır(d)
		yazdır(json_metin({"ad": "Ali"}))
		değişken j = json_çöz("{\"x\": 1}")
		yazdır(j["x"])
	`)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	want := "{\"ad\": \"Ali\", 1: \"bir\"}\n{\"ad\":\"Ali\"}\n1\n"
	if out != want {
		t.Fatalf("çıktı %q bekleniyordu, %q alındı", want, out)
	}

	// Metin olmayan anahtarlar Go/JSON tarafına görünen hâlleriyle aktarılır.
	d := vm.NewDictVal()
	d.SetValue(&vm.IntVal{Val: 7}, &vm.StringVal{Val: "yedi"})
	m := vm.TRToGoValue(d).(map[string]interface{})
	if m["7"] != "yedi" {
		t.Fatalf("TRToGoValue anahtarı \"7\" olmalı, alınan: %v", m)
	}
}
