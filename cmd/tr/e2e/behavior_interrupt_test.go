package e2e

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// interruptibleProcess programı ayrı bir süreçte başlatır ve kosul doğru olana kadar (en çok 30
// sn) bekler; ardından kesme isteğini (Ctrl+C eşdeğeri) gönderir.
type interruptibleProcess struct {
	t      *testing.T
	cmd    *exec.Cmd
	stderr *testkit.TeeWriter
	dir    string
	done   chan error
}

func startInterruptible(t *testing.T, code string) *interruptibleProcess {
	t.Helper()
	dir := t.TempDir()
	writeTR(t, dir, "program.tr", code)
	cmd := exec.Command(trBinary, "program.tr")
	cmd.Dir = dir
	stderr := &testkit.TeeWriter{}
	cmd.Stdout, cmd.Stderr = &testkit.TeeWriter{}, stderr
	prepareProcess(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	s := &interruptibleProcess{t: t, cmd: cmd, stderr: stderr, dir: dir, done: make(chan error, 1)}
	go func() { s.done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-s.done
	})
	return s
}

// waitCond kosul doğru olana kadar bekler.
func (s *interruptibleProcess) waitCond(ne string, cond func() bool) {
	s.t.Helper()
	last := time.Now().Add(30 * time.Second)
	for !cond() {
		if time.Now().After(last) {
			s.t.Fatalf("%s 30 sn içinde olmadı; standart hata: %s", ne, s.stderr.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (s *interruptibleProcess) fileExists(ad string) bool {
	_, err := os.Stat(filepath.Join(s.dir, ad))
	return err == nil
}

// exitCode sürecin bitmesini (en çok 30 sn) bekler ve çıkış kodunu verir.
func (s *interruptibleProcess) exitCode() int {
	s.t.Helper()
	select {
	case err := <-s.done:
		s.done <- err // Cleanup için geri koy
		var ch *exec.ExitError
		if errors.As(err, &ch) {
			return ch.ExitCode()
		}
		if err != nil {
			s.t.Fatal(err)
		}
		return 0
	case <-time.After(30 * time.Second):
		s.t.Fatalf("süreç kesme isteğinden sonra 30 sn içinde bitmedi; standart hata: %s", s.stderr.String())
		return -1
	}
}

// endedByInterrupt sürecin bitmesini (en çok 30 sn) bekler ve isteğin sinyaliyle bittiğini
// söyler (1.9): Unix'te SIGINT'le ölür (kabuk 130 görür), Windows'ta çıkış kodu
// STATUS_CONTROL_C_EXIT'tir.
func (s *interruptibleProcess) endedByInterrupt() bool {
	s.t.Helper()
	select {
	case err := <-s.done:
		s.done <- err // Cleanup için geri koy
		return interruptEnd(err)
	case <-time.After(30 * time.Second):
		s.t.Fatalf("süreç kesme isteğinden sonra 30 sn içinde bitmedi; standart hata: %s", s.stderr.String())
		return false
	}
}

// Ctrl+C (Windows'ta Ctrl+Break olayı, Unix'te SIGINT) programı nazikçe durdurur: döngü
// kesilir, dene/yakala kesilmeyi yakalamaz, fonksiyonun ve programın ertelenen işleri
// çalışır, süreç isteğin sinyaliyle biter (1.5, 1.9).
func TestInterruptGracefulExit(t *testing.T) {
	s := startInterruptible(t, strings.Join([]string{
		`TR_defer { TR_write_file("ust_ertele.txt", "çalıştı") }`,
		`TR_func iş() {`,
		`    TR_defer { TR_write_file("ertele.txt", "çalıştı") }`,
		`    TR_try {`,
		`        TR_write_file("hazir.txt", "1")`,
		`        TR_for { }`,
		`    } TR_catch h {`,
		`        TR_write_file("yakalandi.txt", h)`,
		`    }`,
		`}`,
		`iş()`,
		`TR_write_file("devam_etti.txt", "1")`,
	}, "\n")+"\n")
	s.waitCond("program hazır", func() bool { return s.fileExists("hazir.txt") })
	sendInterrupt(t, s.cmd)
	if !s.endedByInterrupt() {
		t.Fatalf("süreç isteğin sinyaliyle bitmeliydi; standart hata: %s", s.stderr.String())
	}
	for _, ad := range []string{"ertele.txt", "ust_ertele.txt"} {
		if !s.fileExists(ad) {
			t.Errorf("%s yok: ertelenen iş çalışmadı", ad)
		}
	}
	for _, ad := range []string{"yakalandi.txt", "devam_etti.txt"} {
		if s.fileExists(ad) {
			t.Errorf("%s oluştu: kesilme yakalanmamalı ve program sürmemeli", ad)
		}
	}
	if h := s.stderr.String(); !strings.Contains(h, "Durduruluyor") || !strings.Contains(h, "Program durduruldu") {
		t.Errorf("durdurma bildirimleri bekleniyordu: %s", h)
	}
}

// Kesme isteği bekleme sırasında gelirse bekleme hemen biter. Tavan beklemenin yarısıdır: uyanmayan
// bir bekleme onu aşar, yüklü makinede de (boşta ~1 sn sürer) payı geniştir.
func TestInterruptWakesWait(t *testing.T) {
	s := startInterruptible(t, "TR_write_file(\"hazir.txt\", \"1\")\nTR_sleep(600)\nTR_write_file(\"devam_etti.txt\", \"1\")\n")
	s.waitCond("program hazır", func() bool { return s.fileExists("hazir.txt") })
	start := time.Now()
	sendInterrupt(t, s.cmd)
	if !s.endedByInterrupt() {
		t.Fatalf("süreç isteğin sinyaliyle bitmeliydi; standart hata: %s", s.stderr.String())
	}
	if duration := time.Since(start); duration > 300*time.Second {
		t.Errorf("bekleme %v sonra bitti; hemen uyanmalıydı", duration)
	}
	if s.fileExists("devam_etti.txt") {
		t.Error("program bekledikten sonra sürmemeli")
	}
}

// Ertelenen iş bitmezse ikinci kesme isteği süreci hemen SIGINT ile bitirir (kabuk 130 görür, 1.7).
func TestSecondInterruptExitsImmediately(t *testing.T) {
	s := startInterruptible(t, "TR_defer { TR_for { } }\nTR_write_file(\"hazir.txt\", \"1\")\nTR_for { }\n")
	s.waitCond("program hazır", func() bool { return s.fileExists("hazir.txt") })
	sendInterrupt(t, s.cmd)
	s.waitCond("ilk istek işlendi", func() bool { return strings.Contains(s.stderr.String(), "Durduruluyor") })
	sendInterrupt(t, s.cmd)
	if !s.endedByInterrupt() {
		t.Fatalf("süreç isteğin sinyaliyle hemen bitmeliydi; standart hata: %s", s.stderr.String())
	}
}
