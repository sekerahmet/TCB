package gpu

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestInternalfIsInternalError(t *testing.T) {
	err := internalf("detail %d", 7)
	if !messages.Has(err, messages.H9000) {
		t.Fatalf("iç hata kodu bekleniyordu: %v", err)
	}
	if got := internalDetail(err); got != "detail 7" {
		t.Fatalf("ayrıntı = %q", got)
	}
	if !strings.Contains(err.Error(), "detail 7") {
		t.Fatalf("ileti ayrıntıyı içermeli: %v", err)
	}
}

func TestGPUSystemErrorsAndStatusMessage(t *testing.T) {
	e1 := messages.New(messages.H4001, "nvcuda.dll", messages.SystemError("nvcuda.dll", os.ErrPermission))
	if root, ok := messages.Root(e1); !ok || root.Code != messages.H2133 {
		t.Fatalf("H4001 kök kodu = %v (%v), H2133 bekleniyordu", root, ok)
	}
	if !errors.Is(e1, os.ErrPermission) {
		t.Fatalf("Unwrap özgün hatayı korumadı: %v", e1)
	}
	st := messages.New(messages.H4021, e1)
	if tr := messages.ErrorIn(st, "tr"); !strings.Contains(tr, "GPU yok:") || !strings.Contains(tr, "erişim izni yok") || strings.Contains(strings.ToLower(tr), "permission denied") {
		t.Fatalf("TR durum metni beklenmedik: %q", tr)
	}
	if en := messages.ErrorIn(st, "en"); !strings.Contains(en, "no GPU:") || !strings.Contains(en, "permission denied") || strings.Contains(en, "GPU yok") {
		t.Fatalf("EN durum metni beklenmedik: %q", en)
	}

	rawDl := errors.New("libcuda.so.1: cannot open shared object file: No such file")
	e2 := messages.New(messages.H4004, "cuInit", messages.SystemError("", rawDl))
	if root, ok := messages.Root(e2); !ok || root.Code != messages.H2629 || !errors.Is(e2, rawDl) {
		t.Fatalf("H4004 kök kodu / Unwrap hatalı: %v (%v)", e2, root)
	}
	for _, lang := range []string{"tr", "en"} {
		if got := messages.ErrorIn(e2, lang); strings.Contains(got, "cannot open shared object") {
			t.Fatalf("%s metninde ham yükleyici hatası sızdı: %q", lang, got)
		}
	}

	msg := StatusMessage()
	if msg == nil || Status() != msg.Error() {
		t.Fatalf("Status() ile StatusMessage() uyuşmuyor: %q != %v", Status(), msg)
	}
	for _, lang := range []string{"tr", "en"} {
		text := messages.ErrorIn(msg, lang)
		low := strings.ToLower(text)
		if strings.Contains(low, "the specified module could not be found") || strings.Contains(low, "cannot open shared object") {
			t.Fatalf("%s StatusMessage ham sistem metni taşıyor: %q", lang, text)
		}
	}
}
