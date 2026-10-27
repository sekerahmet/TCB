package checkpoint_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// formatKeys bir JSON belgesinin biçime ait anahtarlarıdır (noktalı yol, sıralı); programın
// verdiği içerik (ayarlar, düz değerler, dosya listesinin adları) gezilmez.
func formatKeys(t *testing.T, path string, user ...string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var walk func(v any, prefix string)
	walk = func(v any, prefix string) {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				p := prefix + k
				seen[p] = true
				if !slices.Contains(user, p) {
					walk(e, p+".")
				}
			}
		case []any:
			for _, e := range x {
				walk(e, prefix)
			}
		}
	}
	walk(doc, "")
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Kontrol noktasının dosya ve klasör adları, JSON anahtarları ve kayıt türleri dilden
// bağımsız İngilizce tanımlayıcılardır; iyileştiricinin adı kodudur, iyileştirici durumunun
// tensör adları İngilizcedir (dil tanımı 17.3, 20). Programın verdiği adlar (nesne adları,
// ayarlar, düz değerlerin anahtarları) programındır: kodlu çıplak anahtar kodun dilden bağımsız
// adıyla ({günlük: g} → "logger"), kodsuz ve tırnaklı ad yazıldığı gibi ("sayaç").
func TestCheckpointFormatIsLanguageIndependent(t *testing.T) {
	runDir := trPath(t.TempDir())
	if output, err := run(t, fmt.Sprintf(`
değişken w = tensör([1.0, 2.0], türev_izle: doğru)
değişken op = AdamW([w], öğrenme_oranı: 0.01)
değişken g = günlük(yol(%[1]q, "metrics.jsonl"))
değişken kn = kontrol_noktası(%[1]q, {model: {w: w}, op: op, günlük: g, sayaç: {"en_iyi": 1.5}})
geri_yayılım(toplam(w * w))
op.adım()
g.ekle({"x": 1})
kn.güncelle(3)
kn.kaydet()
`, runDir)); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	dir := filepath.Join(filepath.FromSlash(runDir), "checkpoints")
	ckpt := filepath.Join(dir, "step_0000000003")
	for _, name := range []string{"latest.json", "step_0000000003/state.json", "step_0000000003/digest.json", "step_0000000003/tensors.trt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("checkpoints/%s yok: %v", name, err)
		}
		testkit.FormatNames(t, filepath.Base(name))
	}
	cases := []struct {
		file string
		user []string
		want []string
	}{
		{filepath.Join(dir, "latest.json"), nil, []string{"dir", "step", "version"}},
		{filepath.Join(ckpt, "digest.json"), []string{"files"}, []string{"files", "step", "version"}},
		{filepath.Join(ckpt, "state.json"), []string{"settings", "objects.value"},
			[]string{"objects", "objects.kind", "objects.length", "objects.name", "objects.optimizer", "objects.param_count", "objects.value",
				"program", "step", "time", "tr", "version", "workers"}},
	}
	for _, c := range cases {
		got := formatKeys(t, c.file, c.user...)
		testkit.FormatNames(t, got...)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s anahtarları %v, beklenen %v", filepath.Base(c.file), got, c.want)
		}
	}
	b, err := os.ReadFile(filepath.Join(ckpt, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Objects []struct{ Name, Kind, Optimizer string } `json:"objects"`
	}
	if err := json.Unmarshal(b, &state); err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, o := range state.Objects {
		kinds[o.Name] = o.Kind
		if o.Name == "op" && o.Optimizer != "TR_AdamW" {
			t.Errorf("iyileştiricinin adı kodu olmalı: %q", o.Optimizer)
		}
	}
	if want := map[string]string{"model": "parameters", "op": "optimizer", "logger": "log", "sayaç": "values"}; fmt.Sprint(kinds) != fmt.Sprint(want) {
		t.Errorf("kayıt türleri %v, beklenen %v", kinds, want)
	}
	tensors, err := os.ReadFile(filepath.Join(ckpt, "tensors.trt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"op/step", "op/step.0", "op/m.0", "op/v.0", "model/w"} {
		if !bytes.Contains(tensors, []byte(name)) {
			t.Errorf("tensors.trt'de %q yok", name)
		}
	}
}
