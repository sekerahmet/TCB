package uctanuca

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// kilitliTampon eşzamanlı yazılıp okunabilen bir çıktı tamponudur.
type kilitliTampon struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (k *kilitliTampon) Write(p []byte) (int, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.b.Write(p)
}

func (k *kilitliTampon) String() string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.b.String()
}

// kesilecekSurec programı ayrı bir süreçte başlatır ve kosul doğru olana kadar (en çok 30
// sn) bekler; ardından kesme isteğini (Ctrl+C eşdeğeri) gönderir.
type kesilecekSurec struct {
	t      *testing.T
	cmd    *exec.Cmd
	hata   *kilitliTampon
	klasor string
	bitti  chan error
}

func kesilecekBaslat(t *testing.T, kod string) *kesilecekSurec {
	t.Helper()
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "program.tr", kod)
	cmd := exec.Command(trIkilisi, "program.tr")
	cmd.Dir = klasor
	hata := &kilitliTampon{}
	cmd.Stdout, cmd.Stderr = &kilitliTampon{}, hata
	sureciHazirla(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	s := &kesilecekSurec{t: t, cmd: cmd, hata: hata, klasor: klasor, bitti: make(chan error, 1)}
	go func() { s.bitti <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-s.bitti
	})
	return s
}

// bekleKosul kosul doğru olana kadar bekler.
func (s *kesilecekSurec) bekleKosul(ne string, kosul func() bool) {
	s.t.Helper()
	son := time.Now().Add(30 * time.Second)
	for !kosul() {
		if time.Now().After(son) {
			s.t.Fatalf("%s 30 sn içinde olmadı; standart hata: %s", ne, s.hata.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (s *kesilecekSurec) dosyaVar(ad string) bool {
	_, err := os.Stat(filepath.Join(s.klasor, ad))
	return err == nil
}

// cikisKodu sürecin bitmesini (en çok 30 sn) bekler ve çıkış kodunu verir.
func (s *kesilecekSurec) cikisKodu() int {
	s.t.Helper()
	select {
	case err := <-s.bitti:
		s.bitti <- err // Cleanup için geri koy
		var ch *exec.ExitError
		if errors.As(err, &ch) {
			return ch.ExitCode()
		}
		if err != nil {
			s.t.Fatal(err)
		}
		return 0
	case <-time.After(30 * time.Second):
		s.t.Fatalf("süreç kesme isteğinden sonra 30 sn içinde bitmedi; standart hata: %s", s.hata.String())
		return -1
	}
}

// Ctrl+C (Windows'ta Ctrl+Break olayı, Unix'te SIGINT) programı nazikçe durdurur: döngü
// kesilir, dene/yakala kesilmeyi yakalamaz, fonksiyonun ve programın ertelenen işleri
// çalışır, çıkış kodu 3'tür.
func TestKesmeNazikCikis(t *testing.T) {
	s := kesilecekBaslat(t, strings.Join([]string{
		`ertele { dosya_yaz("ust_ertele.txt", "çalıştı") }`,
		`fonksiyon iş() {`,
		`    ertele { dosya_yaz("ertele.txt", "çalıştı") }`,
		`    dene {`,
		`        dosya_yaz("hazir.txt", "1")`,
		`        döngü { }`,
		`    } yakala h {`,
		`        dosya_yaz("yakalandi.txt", h)`,
		`    }`,
		`}`,
		`iş()`,
		`dosya_yaz("devam_etti.txt", "1")`,
	}, "\n")+"\n")
	s.bekleKosul("program hazır", func() bool { return s.dosyaVar("hazir.txt") })
	kesmeGonder(t, s.cmd)
	if kod := s.cikisKodu(); kod != 3 {
		t.Fatalf("çıkış kodu %d, 3 bekleniyordu; standart hata: %s", kod, s.hata.String())
	}
	for _, ad := range []string{"ertele.txt", "ust_ertele.txt"} {
		if !s.dosyaVar(ad) {
			t.Errorf("%s yok: ertelenen iş çalışmadı", ad)
		}
	}
	for _, ad := range []string{"yakalandi.txt", "devam_etti.txt"} {
		if s.dosyaVar(ad) {
			t.Errorf("%s oluştu: kesilme yakalanmamalı ve program sürmemeli", ad)
		}
	}
	if h := s.hata.String(); !strings.Contains(h, "Durduruluyor") || !strings.Contains(h, "Program durduruldu") {
		t.Errorf("durdurma bildirimleri bekleniyordu: %s", h)
	}
}

// Kesme isteği bekleme sırasında gelirse bekleme hemen biter.
func TestKesmeBeklemeyiUyandirir(t *testing.T) {
	s := kesilecekBaslat(t, "dosya_yaz(\"hazir.txt\", \"1\")\nbekle(120)\ndosya_yaz(\"devam_etti.txt\", \"1\")\n")
	s.bekleKosul("program hazır", func() bool { return s.dosyaVar("hazir.txt") })
	bas := time.Now()
	kesmeGonder(t, s.cmd)
	if kod := s.cikisKodu(); kod != 3 {
		t.Fatalf("çıkış kodu %d, 3 bekleniyordu; standart hata: %s", kod, s.hata.String())
	}
	if sure := time.Since(bas); sure > 20*time.Second {
		t.Errorf("bekleme %v sonra bitti; hemen uyanmalıydı", sure)
	}
	if s.dosyaVar("devam_etti.txt") {
		t.Error("program bekledikten sonra sürmemeli")
	}
}

// Ertelenen iş bitmezse ikinci kesme isteği süreci hemen bitirir (çıkış kodu 130).
func TestIkinciKesmeHemenCikar(t *testing.T) {
	s := kesilecekBaslat(t, "ertele { döngü { } }\ndosya_yaz(\"hazir.txt\", \"1\")\ndöngü { }\n")
	s.bekleKosul("program hazır", func() bool { return s.dosyaVar("hazir.txt") })
	kesmeGonder(t, s.cmd)
	s.bekleKosul("ilk istek işlendi", func() bool { return strings.Contains(s.hata.String(), "Durduruluyor") })
	kesmeGonder(t, s.cmd)
	if kod := s.cikisKodu(); kod != 130 {
		t.Fatalf("çıkış kodu %d, 130 bekleniyordu; standart hata: %s", kod, s.hata.String())
	}
}
