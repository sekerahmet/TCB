package download

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TestDownloadLockIsExclusive: .part'ın kilidini tutan varken ikinci açış errBusy'dir; indirme H7722
// verir. Kilit kapanınca bırakılır.
func TestDownloadLockIsExclusive(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a.bin")
	p, err := openPart(target + ".part")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openPart(target + ".part"); err != errBusy {
		t.Errorf("ikinci açış errBusy olmalı: %v", err)
	}
	wantCode(t, dl("https://huggingface.co/a", slash(target), "TR_none", "TR_none"), messages.H7722)
	p.close()
	p2, err := openPart(target + ".part")
	if err != nil {
		t.Fatalf("kilit bırakılınca açılmalı: %v", err)
	}
	p2.close()
}

// TestDownloadLockFollowsTheFile: kilitli dosyanın adı başka yere geçince yoldaki yeni dosya ayrı bir
// dosyadır ve kilitlenebilir; openPart'ın verdiği dosya yolun gösterdiği dosyadır.
func TestDownloadLockFollowsTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.part")
	p, err := openPart(path)
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()
	if err := renameLocked(p.f, path, filepath.Join(dir, "baska.part")); err != nil {
		t.Fatal(err)
	}
	q, err := openPart(path)
	if err != nil {
		t.Fatalf("yeni dosya kilitlenmeli: %v", err)
	}
	defer q.close()
	held, _ := q.f.Stat()
	named, _ := os.Stat(path)
	if !os.SameFile(held, named) {
		t.Error("açılan dosya yolun gösterdiği dosya olmalı")
	}
}

// TestDownloadTargetCheckedUnderTheLock: hedefin denetimi kilit alındıktan sonra yinelenir. Aynı
// hedefe iki indirmede öbürü bu çağrının denetimiyle kilidi arasında hedefi yerine koyarsa bu çağrı
// onu TR_overwrite: olmadan ezmez: H7726, hedef öbürünün dosyasıdır (28.10; DENETİM 759 N2).
func TestDownloadTargetCheckedUnderTheLock(t *testing.T) {
	s := fileServer(t, []byte("ikinci"))
	target := filepath.Join(t.TempDir(), "a.bin")
	m := interp.New()
	args := []interp.Value{&interp.StringVal{Val: s.URL + "/a"}, &interp.StringVal{Val: slash(target)}}
	c, err := readCall(m, args, map[string]interp.Value{nameSha256.TR: interp.Null, nameSize.TR: interp.Null})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.checkTarget(); err != nil {
		t.Fatalf("hedef yokken denetim geçmeli: %v", err)
	}
	// Öbür indirme denetimden sonra, kilitten önce bitip hedefi yerine koyar.
	if err := os.WriteFile(target, []byte("birinci"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := c.locked(m); !messages.Has(err, messages.H7726) {
		t.Errorf("kilit alınınca H7726 bekleniyordu, gelen %v", err)
	}
	if b, _ := os.ReadFile(target); string(b) != "birinci" {
		t.Errorf("hedef %q, beklenen öbür indirmenin dosyası", b)
	}
}

// TestDownloadRenamesUnderTheLock: yerine koyma kilit tutulurken yapılır; var olan hedef tek adımda
// ezilir, .part'ın adı kalmaz.
func TestDownloadRenamesUnderTheLock(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a.bin")
	os.WriteFile(target, []byte("eski"), 0o644)
	p, err := openPart(target + ".part")
	if err != nil {
		t.Fatal(err)
	}
	p.f.Write([]byte("yeni"))
	if err := p.place(target); err != nil {
		t.Fatalf("yerine koyma: %v", err)
	}
	if exists(target + ".part") {
		t.Error(".part'ın adı kalmamalı")
	}
	p.close()
	if b, _ := os.ReadFile(target); string(b) != "yeni" {
		t.Errorf("hedef %q, beklenen yeni", b)
	}
}
