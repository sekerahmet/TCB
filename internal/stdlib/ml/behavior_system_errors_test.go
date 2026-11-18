package ml

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestUnavailableGPUStatusCarriesNoSystemText(t *testing.T) {
	dir := t.TempDir()
	out, err := fromProgram(t, dir, `
TR_try {
    TR_Tensor([1.0]).TR_to_device("gpu:99")
} TR_catch e {
    TR_print(e.TR_code)
    TR_print(TR_String(e))
    TR_print(e.TR_text("tr"))
    TR_print(e.TR_text("en"))
}
`)
	if err != nil {
		t.Fatalf("beklenmeyen çalışma hatası: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 4 {
		t.Fatalf("4 satır bekleniyordu, gelen (%d): %q", len(lines), out)
	}
	if lines[0] != "H6813" {
		t.Fatalf("kod = %q, H6813 bekleniyordu", lines[0])
	}
	valLine, trLine, enLine := lines[1], lines[2], lines[3]
	if !strings.HasPrefix(valLine, `Message("H6813"`) || !strings.Contains(valLine, `Message("H40`) {
		t.Fatalf("TR_String(e) dilden bağımsız Message ağacı olmalı: %q", valLine)
	}
	if !strings.Contains(trLine, "cihazı kullanılamıyor") || strings.Contains(trLine, "no GPU:") {
		t.Fatalf("TR iletisi tek dilde değil: %q", trLine)
	}
	if !strings.Contains(enLine, "is not available") || strings.Contains(enLine, "GPU yok:") {
		t.Fatalf("EN iletisi tek dilde değil: %q", enLine)
	}
	for _, line := range []string{valLine, trLine, enLine} {
		low := strings.ToLower(line)
		if strings.Contains(low, "the specified module could not be found") || strings.Contains(low, "cannot open shared object") {
			t.Fatalf("H6813 içinde ham sistem metni: %q", line)
		}
	}
}

func TestTensorLoadSystemErrorsCarryNoSystemText(t *testing.T) {
	trBroken := io.MultiReader(strings.NewReader("TRTENSOR"), iotest.ErrReader(os.ErrPermission))
	_, err := decodeFile(trBroken, 64)
	if root, ok := messages.Root(err); !messages.Has(err, messages.H6600) || !ok || root.Code != messages.H2133 || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("TRTENSOR okuma hatası beklenmedik: %v (kök %v)", err, root)
	}
	if tr := strings.ToLower(messages.ErrorIn(err, "tr")); strings.Contains(tr, "permission denied") {
		t.Fatalf("TRTENSOR TR iletisinde ham metin: %q", tr)
	}

	stHdr := []byte(`{"w":{"dtype":"F32","shape":[4],"data_offsets":[0,16]}}`)
	var stPrefix bytes.Buffer
	_ = binary.Write(&stPrefix, binary.LittleEndian, uint64(len(stHdr)))
	stPrefix.Write(stHdr)
	stEOF := io.MultiReader(bytes.NewReader(stPrefix.Bytes()), bytes.NewReader([]byte{1, 2}), iotest.ErrReader(io.ErrUnexpectedEOF))
	_, stErr := decodeTensorFile(stEOF, int64(stPrefix.Len()+16))
	if root, ok := messages.Root(stErr); !messages.Has(stErr, messages.H6653) || !ok || root.Code != messages.H2153 {
		t.Fatalf("safetensors kesik veri hatası beklenmedik: %v (kök %v)", stErr, root)
	}
	if low := strings.ToLower(messages.ErrorIn(stErr, "en")); strings.Contains(low, "unexpected eof") {
		t.Fatalf("safetensors iletisinde ham EOF metni: %q", low)
	}

	diskErr := errors.New("sector fault")
	ggufBytes := (&gguftest.Builder{}).Tensor("w", gguf.TypeF32, []uint64{16}, make([]byte, 64)).Bytes()
	ggufBroken := bufio.NewReader(io.MultiReader(bytes.NewReader(ggufBytes[:len(ggufBytes)-32]), iotest.ErrReader(diskErr)))
	_, gErr := decodeGGUF(ggufBroken, int64(len(ggufBytes)))
	if root, ok := messages.Root(gErr); !messages.Has(gErr, messages.H6653) || !ok || root.Code != messages.H2629 || !errors.Is(gErr, diskErr) {
		t.Fatalf("GGUF tensör okuma hatası beklenmedik: %v (kök %v)", gErr, root)
	}
	if strings.Contains(messages.ErrorIn(gErr, "tr"), "sector fault") {
		t.Fatalf("GGUF iletisinde ham disk hatası: %q", gErr.Error())
	}

	dir := t.TempDir()
	out, runErr := fromProgram(t, dir, `
TR_try {
    TR_load_tensors("`+filepath.ToSlash(dir)+`")
} TR_catch e {
    TR_print(TR_String(e))
}
`)
	if runErr != nil {
		t.Fatalf("beklenmeyen çalışma hatası: %v", runErr)
	}
	if low := strings.ToLower(out); strings.Contains(low, "is a directory") || strings.Contains(low, "access is denied") || strings.Contains(low, "incorrect function") {
		t.Fatalf("TR_load_tensors klasör hatasında ham sistem metni: %q", out)
	}
}
