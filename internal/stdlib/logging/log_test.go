package logging_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	_ "github.com/sekerahmet/TCB/internal/stdlib"
	"github.com/sekerahmet/TCB/internal/stdlib/logging"
	"github.com/sekerahmet/TCB/internal/testkit"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// newVM çıktısı tampona giden bir VM kurar; program dosyası klasördedir.
func newVM(t *testing.T, dir string, arguments ...string) (*interp.VM, *testkit.TeeWriter) {
	t.Helper()
	t.Cleanup(logging.CloseAll)
	m := interp.New()
	w := &testkit.TeeWriter{}
	m.Out, m.Err = w, w
	m.SourceFile = filepath.Join(dir, "egit.tr")
	m.SetArgs(arguments)
	return m, w
}

func run(t *testing.T, m *interp.VM, code string) (interp.Value, error) {
	t.Helper()
	prog := trrun.Parse(t, code)
	return m.Run(prog)
}

// lines günlük dosyasının JSON satırlarını çözer.
func lines(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, s := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		var k map[string]any
		if err := json.Unmarshal([]byte(s), &k); err != nil {
			t.Fatalf("geçersiz JSON satırı %q: %v", s, err)
		}
		out = append(out, k)
	}
	return out
}

func pathText(path string) string { return filepath.ToSlash(path) }

// Günlük: başlangıç kaydı (program, komut satırı ayarları, gizli değer maskeli), ölçü ve
// mesaj kayıtları, NaN ve sonsuz metin olarak, tek elemanlı tensör sayı olarak.
func TestLogRecords(t *testing.T) {
	k := t.TempDir()
	path := filepath.Join(k, "kayit", "egitim.jsonl")
	m, _ := newVM(t, k, "--hf_token", "hf_gizlideger1")
	_, err := run(t, m, `
değişken ayar = komut_satırı({adım: 10, hf_token: ""})
değişken g = günlük("`+pathText(path)+`")
g.ekle({kayıp: tensör([2.5]), nan: ondalık("NaN"), sonsuz: ondalık("inf"), belirteç: ayar.hf_token})
g.ekle("merhaba")
g.ekle({"time": "elle", x: [1, 2]})
`)
	if err != nil {
		t.Fatal(err)
	}
	s := lines(t, path)
	if len(s) != 4 {
		t.Fatalf("4 satır bekleniyordu: %v", s)
	}
	b := s[0]
	if b["kind"] != "start" || b["version"] != 2.0 || b["program"] != "egit.tr" {
		t.Errorf("başlangıç kaydı: %v", b)
	}
	if a, _ := b["settings"].(map[string]any); a["step"] != 10.0 || a["hf_token"] != interp.SecretMask {
		t.Errorf("ayarlar gizli değeri maskelenmiş olarak taşımalı: %v", b["settings"])
	}
	o := s[1]
	if o["loss"] != 2.5 || o["nan"] != "NaN" || o["sonsuz"] != "Infinity" || o["belirteç"] != interp.SecretMask {
		t.Errorf("ölçü kaydı: %v", o)
	}
	if _, ok := o["time"].(string); !ok {
		t.Errorf("zaman eklenmeli: %v", o)
	}
	if s[2]["message"] != "merhaba" || s[3]["time"] != "elle" {
		t.Errorf("mesaj ve elle verilen zaman: %v %v", s[2], s[3])
	}
	if strings.Contains(string(mustRead(t, path)), "hf_gizlideger1") {
		t.Error("gizli değer dosyaya düz yazılmamalı")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Hatalar: uzantı, ayrılmış anahtar, desteklenmeyen değer.
func TestLogErrors(t *testing.T) {
	k := t.TempDir()
	m, _ := newVM(t, k)
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{`günlük("` + pathText(filepath.Join(k, "a.csv")) + `")`, messages.H7319},
		{`günlük("` + pathText(filepath.Join(k, "a.jsonl")) + `").ekle({"kind": "x"})`, messages.H7321},
		{`günlük("` + pathText(filepath.Join(k, "b.jsonl")) + `").ekle({f: yazdır})`, messages.H7300},
	} {
		if _, err := run(t, m, d.code); !messages.Has(err, d.want) {
			t.Errorf("%s: %s hatası bekleniyordu: %v", d.code, d.want, err)
		}
	}
	// Argüman tipi hatası henüz kodlu değil (interp).
	if _, err := run(t, m, `günlük("`+pathText(filepath.Join(k, "c.jsonl"))+`").ekle(5)`); err == nil || !strings.Contains(err.Error(), "1. argüman olarak Sözlük ya da Metin bekler") {
		t.Errorf("argüman tipi hatası bekleniyordu: %v", err)
	}
}

// Var olan dosyaya eklenir; önceki koşu yarım satır bıraktıysa yeni kayıt yeni satırda başlar.
// Kontrol noktası arayüzü: Uzunluk satır sınırıdır, WriteResume devam kaydını yazar.
func TestLogAppendAndResume(t *testing.T) {
	k := t.TempDir()
	path := filepath.Join(k, "e.jsonl")
	if err := os.WriteFile(path, []byte(`{"step":1}`+"\n"+`{"step":2,"ka`), 0o644); err != nil {
		t.Fatal(err)
	}
	m, _ := newVM(t, k)
	v, err := run(t, m, `değişken g = günlük("`+pathText(path)+`")
g.ekle({kayıp: 1})
g`)
	if err != nil {
		t.Fatal(err)
	}
	g, ok := logging.IsLog(v)
	if !ok {
		t.Fatalf("Günlük bekleniyordu: %v", v)
	}
	n, err := g.Len()
	if err != nil || n != int64(len(mustRead(t, path))) {
		t.Errorf("Uzunluk = %d, %v", n, err)
	}
	if err := g.WriteResume(1500); err != nil {
		t.Fatal(err)
	}
	content := string(mustRead(t, path))
	line := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if len(line) != 5 || line[1] != `{"step":2,"ka` || !strings.HasPrefix(line[2], `{"kind":"start"`) ||
		!strings.Contains(line[4], `"kind":"resume"`) || !strings.HasSuffix(line[4], `"step":1500}`) {
		t.Errorf("beklenmeyen günlük:\n%s", content)
	}
	// Kapatılan günlüğe yazılınca dosya yeniden açılır.
	logging.CloseAll()
	if _, err := run(t, m, `g.ekle("yine")`); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(mustRead(t, path)), `"message":"yine"}`+"\n") {
		t.Errorf("kapatılan günlük yeniden açılmalı: %s", mustRead(t, path))
	}
}

// Hiç kayıt yazılmayan günlük kapanırken başlangıç kaydını yazar.
func TestLogEmptyClose(t *testing.T) {
	k := t.TempDir()
	path := filepath.Join(k, "bos.jsonl")
	m, _ := newVM(t, k)
	if _, err := run(t, m, `günlük("`+pathText(path)+`")`); err != nil {
		t.Fatal(err)
	}
	logging.CloseAll()
	if s := lines(t, path); len(s) != 1 || s[0]["kind"] != "start" {
		t.Errorf("tek başlangıç kaydı bekleniyordu: %v", s)
	}
}

// fakeClock elle ilerletilen bir saattir.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (s *fakeClock) now() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.t
}

func (s *fakeClock) advance(d time.Duration) {
	s.mu.Lock()
	s.t = s.t.Add(d)
	s.mu.Unlock()
}

func setClock(t *testing.T) *fakeClock {
	s := &fakeClock{t: time.Date(2026, 10, 18, 14, 0, 0, 0, time.Local)}
	t.Cleanup(logging.FakeClock(s.now))
	return s
}
