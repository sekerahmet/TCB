package main

import (
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/lib"
)

// wavSeed geçerli bir WAV kurar: tohumlar bunlardır (22.4).
func wavSeed(tag, channels, rate, bits uint32, data []byte, extra ...byte) []byte {
	le := binary.LittleEndian
	align := channels * bits / 8
	fmtChunk := le.AppendUint16(nil, uint16(tag))
	fmtChunk = le.AppendUint16(fmtChunk, uint16(channels))
	fmtChunk = le.AppendUint32(fmtChunk, rate)
	fmtChunk = le.AppendUint32(fmtChunk, rate*align)
	fmtChunk = le.AppendUint16(fmtChunk, uint16(align))
	fmtChunk = le.AppendUint16(fmtChunk, uint16(bits))
	body := []byte("WAVEfmt ")
	body = le.AppendUint32(body, uint32(len(fmtChunk)))
	body = append(body, fmtChunk...)
	body = append(body, "data"...)
	body = le.AppendUint32(body, uint32(len(data)))
	body = append(body, data...)
	body = append(body, extra...)
	return append(le.AppendUint32([]byte("RIFF"), uint32(len(body))), body...)
}

// readWav TR_read_wav'ı süreç içinde koşar; hatası döner.
func readWav(t *testing.T, path string) error {
	t.Helper()
	code := "# TR 4\nTR_import \"tr/audio\"\nTR_var s = TR_read_wav(" + strconv.Quote(filepath.ToSlash(path)) + ")\n"
	s := source.Prepare(code, source.Options{TypeCheck: true})
	if len(s.Errors) > 0 {
		t.Fatalf("program denetimden geçmedi: %v", s.Errors)
	}
	machine := interp.New()
	_, err := machine.Run(s.Program)
	return err
}

// FuzzReadWav: WAV dosyası dış girdidir. Her bayt dizisi ya sesi ya H kodlu hatayı verir: panik yok,
// asılma yok (kesinti süre sınırıyla). Normal `go test` tohumları koşar; G4'te:
//
//	go test ./cmd/trlang -run '^$' -fuzz FuzzReadWav -fuzztime 5m
func FuzzReadWav(f *testing.F) {
	f.Add(wavSeed(1, 1, 16000, 16, []byte{0, 0, 0, 64}))
	f.Add(wavSeed(1, 2, 8000, 16, []byte{0, 64, 0, 192}))
	f.Add(wavSeed(3, 1, 16000, 32, []byte{0, 0, 128, 62}))
	f.Add(wavSeed(1, 1, 16000, 16, []byte{0, 0}, []byte("LIST\x03\x00\x00\x00abc\x00")...))
	f.Add(wavSeed(1, 1, 16000, 24, []byte{0, 0, 0}))
	f.Add([]byte("RIFF\xff\xff\xff\xffWAVE"))
	f.Add(append(wavSeed(1, 1, 16000, 16, []byte{0, 0, 0, 64}), []byte("TAG\x00\x00")...))
	f.Add(append(wavSeed(1, 1, 16000, 16, []byte{0, 0, 0, 64}), []byte{1, 2, 1, 2, 1, 2, 1, 2, 1, 2}...))
	interp.SetLibrary(lib.Files)
	path := filepath.Join(f.TempDir(), "ses.wav")
	f.Fuzz(func(t *testing.T, data []byte) {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- readWav(t, path) }()
		select {
		case err := <-done:
			var e *messages.Error
			if err != nil && !errors.As(err, &e) {
				t.Fatalf("H kodu taşımayan hata: %v", err)
			}
		case <-time.After(30 * time.Second):
			t.Fatal("okuyucu asıldı")
		}
	})
}

// İletilerin önerileri: ffmpeg yazımı ve dosyaya yazma (22.4).
func TestWavMessagesGiveTheFix(t *testing.T) {
	cases := []struct {
		text, want string
	}{
		{messages.TextIn("tr", messages.H7061, "a.wav", "pcm", "24 bit"), "`ffmpeg -nostdin -i girdi -c:a pcm_s16le çıktı.wav`"},
		{messages.TextIn("tr", messages.H7067, "a.wav", "data", 8, 2), "sesi dosyaya yazın"},
		{messages.TextIn("tr", messages.H7080, "a.wav", "ab"), "sesi dosyaya yazın"},
		{messages.TextIn("tr", messages.H7074, 0, 3, 1.5), "`TR_clamp(x, TR_min: -1.0, TR_max: 1.0)`"},
	}
	for _, c := range cases {
		if !strings.Contains(c.text, c.want) {
			t.Errorf("%q içinde %q yok", c.text, c.want)
		}
	}
	for _, value := range []float64{math.NaN(), math.Inf(1)} {
		if text := messages.TextIn("tr", messages.H7075, 0, 3, value); strings.Contains(text, "TR_clamp") {
			t.Errorf("NaN ve sonsuz iletisi kırpmayı önermemeli: %s", text)
		}
	}
	if text := messages.TextIn("tr", messages.H7068, "a.wav"); strings.Contains(text, "dosyaya yazın") {
		t.Errorf("boy 0 boru belirtisi değil: %s", text)
	}
}
