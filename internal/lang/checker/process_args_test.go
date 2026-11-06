package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (21.1): TR_run_process'in argüman listesine TR_Path koymak denetim hatasıdır, kod çalışma
// anınınkidir (H7490). Metin listesi, TR_String(yol) ve tipi bilinmeyen öğe geçer (çalışma anında
// yakalanır: TestRunProcessArgumentListErrors).
func TestProcessArgumentListHoldsNoPath(t *testing.T) {
	const y = "TR_var y = TR_Path(\"a.txt\")\n"
	cases := []struct {
		name, src string
		line      int
	}{
		{"çağrı", "TR_print(TR_run_process(\"p\", [TR_Path(\"a\")]))", 1},
		{"karışık", "TR_print(TR_run_process(\"p\", [\"-i\", TR_Path(\"a\")]))", 1},
		{"tipi yazılı ad", "TR_var y: TR_Path = TR_Path(\"a\")\nTR_print(TR_run_process(\"p\", [\"-i\", y]))", 2},
		{"öğe tipi yazılı", y + "TR_var l: TR_List[TR_Path] = [y]\nTR_print(TR_run_process(\"p\", l))", 3},
		{"akışlı çağrı", "TR_var p = TR_spawn_process(\"p\", [TR_Path(\"a\")])", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { singleError(t, tc.src, messages.H7490, tc.line) })
	}
	noErrors(t, y+"TR_print(TR_run_process(\"p\", [\"-i\", TR_String(y)]))\nTR_var l: TR_List[TR_String] = []\nTR_print(TR_run_process(\"p\", l))")
	noErrors(t, y+"TR_print(TR_run_process(\"p\", [y]))")
	// Süreç tipi denetimden geçer: yöntemler, çıkış kodu TR_Int, baytlar TR_Bytes (21.2).
	noErrors(t, "TR_var p: TR_Process = TR_spawn_process(\"p\", [], TR_buffer_bytes: 8)\nTR_var b: TR_Bytes = p.TR_read(4)\np.TR_write(b)\np.TR_close_stdin()\nTR_var k: TR_Int = p.TR_wait(TR_timeout: 1)\nTR_var d: TR_Bool = p.TR_done()\np.TR_cancel()\nTR_print(k, d)")
}
