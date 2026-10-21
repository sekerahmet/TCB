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
		writeFile(t, dir, "egit.tr", coded)
		s := run(t, trBinary, dir, "", env, "egit.tr")
		wait(t, s, 0, nil, nil)
		outputs[i] = s.stdout
		shapes[i] = fileFormatShape(t, dir)
	}
	// Program her yazım dilinde kanonik biçimiyle çalışır (17.7): çıktısı da aynıdır
	// (yazdır(tip(...)) iki dilde aynı ad). TR_DIL yalnızca gösterimi değiştirir: aynı
	// sözdizimi hatası İngilizce başlıkla bildirilir.
	if outputs[0] != outputs[1] {
		t.Fatalf("program çıktısı yazım diline göre değişti: %q / %q", outputs[0], outputs[1])
	}
	bad := t.TempDir()
	writeFile(t, bad, "bozuk.tr", "eğer {\n")
	wait(t, run(t, trBinary, bad, "", []string{"TR_DIL=en"}, "bozuk.tr"), 1, nil, []string{"Syntax Errors"})
	if shapes[0] != shapes[1] {
		t.Fatalf("dosya biçimi yazım diline göre değişti:\nvarsayılan:\n%s\nTR_DIL=en:\n%s", shapes[0], shapes[1])
	}
	for _, key := range []string{"ozet.json", "durum.json", "\"sürüm\"", "\"dosyalar\"", "\"nesneler\"", "metrics.jsonl", "\"tür\""} {
		if !strings.Contains(shapes[0], key) {
			t.Errorf("biçim özeti %s içermeli:\n%s", key, shapes[0])
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
