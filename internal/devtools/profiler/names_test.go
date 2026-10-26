package profiler

import (
	"os"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// opNamesOut yardımcı testin çıktı dosyasını veren ortam değişkenidir (yalnız alt süreçte).
const opNamesOut = "TR_PROFILE_OP_NAMES_OUT"

// TestOpNamesHelper birkaç işlemin rapor adını dosyaya yazar; yalnızca
// TestOpNamesFollowPersonsLanguage tarafından TR_DIL=tr/en alt süreçlerinde çalışır.
func TestOpNamesHelper(t *testing.T) {
	out := os.Getenv(opNamesOut)
	if out == "" {
		t.Skip("yalnızca TestOpNamesFollowPersonsLanguage'ın alt süreci")
	}
	var names []string
	for _, is := range []tensor.OpKind{tensor.OpKindSum, tensor.OpKindAdamW, tensor.OpKindToDevice} {
		names = append(names, opName(is))
	}
	if err := os.WriteFile(out, []byte(strings.Join(names, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Profil raporundaki işlem adları kişinin gösterim dilindedir (TR_DIL): TR_DIL=en ile "sum",
// "AdamW.step", ".to_device"; TR_DIL=tr ile "toplam", "AdamW.adım", ".cihaza". Önceden adlar her
// dilde kanonik Türkçeydi. TR_DIL süreç başında okunduğu için her dil ayrı bir alt süreçte koşar.
func TestOpNamesFollowPersonsLanguage(t *testing.T) {
	for lang, want := range map[string]string{"en": "sum\nAdamW.step\n.to_device", "tr": "toplam\nAdamW.adım\n.cihaza"} {
		if got := string(testkit.RunWithLanguageFile(t, "TestOpNamesHelper", opNamesOut, lang)); got != want {
			t.Errorf("TR_DIL=%s: %q, beklenen %q", lang, got, want)
		}
	}
}

// TestOpNameCoversEveryOp: her tensör işleminin raporda görünen adı bir ad koduna (opCode),
// bir ileti koduna (opLabel) ya da opName'in işleç/metot dalına bağlıdır; hiçbiri değilse
// rapor tensör paketinin ASCII anahtarını yazardı.
func TestOpNameCoversEveryOp(t *testing.T) {
	special := map[tensor.OpKind]bool{
		tensor.OpKindAdd: true, tensor.OpKindSub: true, tensor.OpKindMul: true, tensor.OpKindDiv: true,
		tensor.OpKindLess: true, tensor.OpKindGreater: true, tensor.OpKindLessEq: true,
		tensor.OpKindGreaterEq: true, tensor.OpKindMatMul: true, tensor.OpKindToDevice: true,
		tensor.OpKindCast: true, tensor.OpKindAdamW: true, tensor.OpKindSGD: true,
	}
	for i := tensor.OpKind(0); i < tensor.OpKindCount; i++ {
		_, a := opCode[i]
		_, b := opLabel[i]
		if !a && !b && !special[i] {
			t.Errorf("%d. işlemin (%s) rapor adı yok: ad koduna ya da opLabel'a bağlayın", i, i.Code())
		}
		if opName(i) == "" {
			t.Errorf("%s: rapor adı boş", i.Code())
		}
	}
}
