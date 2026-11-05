package ml

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func writeGGUF(t *testing.T, dir, name string, b *gguftest.Builder) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestLoadMetadataGivesJSON: üstveri TR_JSON'dur; tokenizer.ggml. anahtarları girmez,
// chat_template metindir, sayıların cinsi yazımdan gelir (15.55).
func TestLoadMetadataGivesJSON(t *testing.T) {
	dir := t.TempDir()
	b := (&gguftest.Builder{}).KV("general.architecture", "qwen3").KV("qwen3.block_count", uint32(28)).
		KV("qwen3.rope.freq_base", float32(1000000)).KV("qwen3.attention.layer_norm_rms_epsilon", float32(1e-6)).
		KV("general.tags", []string{"a", "b"}).KV("tokenizer.chat_template", "{{ x }}").
		KV("tokenizer.ggml.tokens", []string{"gizli"}).KV("ok", true).KV("neg", int64(-3)).
		Tensor("w", gguf.TypeQ5_0, []uint64{32}, make([]byte, 22))
	writeGGUF(t, dir, "m.gguf", b)
	out, err := fromProgram(t, dir, `
TR_var m = TR_load_metadata("m.gguf")
TR_print(TR_String(m["general.architecture"]))
TR_print(TR_Int(m["qwen3.block_count"]) + 1)
TR_print(TR_Float(m["qwen3.rope.freq_base"]))
TR_print(TR_Float(m["qwen3.attention.layer_norm_rms_epsilon"]) == 0.0000009999999974752427)
TR_print(m["general.tags"].TR_length)
TR_print(TR_String(m["tokenizer.chat_template"]))
TR_print(TR_Bool(m["ok"]))
TR_print(TR_Int(m["neg"]))
TR_print(m.TR_get("tokenizer.ggml.tokens") == TR_none)
TR_print(m.TR_keys().TR_length)
`)
	if err != nil {
		t.Fatal(err)
	}
	want := "qwen3\n29\n1000000.0\ntrue\n2\n{{ x }}\ntrue\n-3\ntrue\n8\n"
	if out != want {
		t.Errorf("çıktı:\n%s\nbeklenen:\n%s", out, want)
	}
}

func TestLoadMetadataRejects(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		b    *gguftest.Builder
		code messages.Code
	}{
		{"büyük.gguf", (&gguftest.Builder{}).KV("x", uint64(math.MaxInt64)+1), messages.H6685},
		{"nan.gguf", (&gguftest.Builder{}).KV("x", float32(math.NaN())), messages.H6685},
		{"sonsuz.gguf", (&gguftest.Builder{}).KV("x", []float32{float32(math.Inf(1))}), messages.H6685},
	}
	for _, c := range cases {
		writeGGUF(t, dir, c.name, c.b)
		if _, err := fromProgram(t, dir, `TR_load_metadata("`+c.name+`")`); !messages.Has(err, c.code) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "düz.txt"), []byte("merhaba dünya"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := fromProgram(t, dir, `TR_load_metadata("düz.txt")`); !messages.Has(err, messages.H6676) {
		t.Errorf("GGUF değil: %v", err)
	}
	if _, err := fromProgram(t, dir, `TR_load_metadata("yok.gguf")`); !messages.Has(err, messages.H2107) {
		t.Errorf("yok: %v", err)
	}
}
