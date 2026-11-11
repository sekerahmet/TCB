package e2e

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural (15.55, durma kimlikleri): üretimin TR_eos:'u bir listedir; tek sayı verilince ileti adı
// ve kopyalanınca çalışan yazımı söyler (`TR_eos: [4]`). Adlı argümanın tipi bugün denetlenmediği
// için (docs/status.md, 6.5) hata çalışırken gelir. Önerilen yazımın çalıştığı TR testindedir
// (lib/layers_test.tr).
func TestGenerateEosSingleNumberSuggestsAList(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "eos.tr", "TR_import \"tr/layers\"\nTR_var m = TR_GPT(6, TR_context: 8, TR_embed_dim: 8, TR_num_layers: 1, TR_num_heads: 2, TR_seed: 2)\n"+
		"TR_print(m.TR_generate([1], 2, TR_temperature: 0, TR_eos: 4))\n")
	s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "eos.tr"))
	if !strings.Contains(s.Stdout+s.Stderr, "`TR_eos: [4]`") {
		t.Errorf("ileti önerilen yazımı demiyor:\n%s%s", s.Stdout, s.Stderr)
	}
}
