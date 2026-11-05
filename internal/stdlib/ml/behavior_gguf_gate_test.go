package ml

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
	"github.com/sekerahmet/TCB/lib"
)

// Kural (15.55, DENETİM 161): bağlam uzunluğu 0 ya da eksi olan dosya kapıda kodlu hata verir;
// konum tablosu boş kurulup uzaktaki bir çarpımda düşmez. Pozitif ya da yazılmamış bağlam kapıdan
// geçer: sıradaki eksik anahtar H6699 verir.
func TestGGUFContextLengthGate(t *testing.T) {
	interp.SetLibrary(lib.Files)
	defer interp.SetLibrary(nil)
	cases := []struct {
		name  string
		value any
		want  messages.Code
	}{
		{"sıfır", uint32(0), messages.H7155},
		{"eksi", int32(-1), messages.H7155},
		{"pozitif", uint32(1), messages.H6699},
		{"yok", nil, messages.H6699},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := (&gguftest.Builder{}).KV("general.architecture", "llama")
			if c.value != nil {
				b.KV("llama.context_length", c.value)
			}
			path := filepath.Join(t.TempDir(), "m.gguf")
			if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			m := interp.New()
			m.GlobalEnv.Define("yol", &interp.StringVal{Val: path}, true)
			_, err := m.Run(trrun.Parse(t, `TR_import "tr/pretrained"
TR_pretrained_gguf(yol)`))
			if !messages.Has(err, c.want) {
				t.Errorf("%v, %s bekleniyordu", err, c.want)
			}
		})
	}
}
