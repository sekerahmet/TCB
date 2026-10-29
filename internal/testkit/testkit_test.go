package testkit

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// Kural: WriteFile ara klasörleri açar, bölü çizgili adı işletim sisteminin ayracına çevirir ve
// yazdığı dosyanın yolunu verir.
func TestWriteFileCreatesParentFolders(t *testing.T) {
	dir := t.TempDir()
	path := WriteFile(t, dir, "a/b/c.tr", "TR_print(1)\n")
	if want := filepath.Join(dir, "a", "b", "c.tr"); path != want {
		t.Fatalf("yol %q, beklenen %q", path, want)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "TR_print(1)\n" {
		t.Fatalf("dosya içeriği %q, hata %v", data, err)
	}
}

// Kural: HasJSONName boş olmayan bir json adı ya da "-" etiketi arar; adsız etiket ("omitempty"
// tek başına) ve etiketsiz alan yoktur.
func TestHasJSONNameNeedsANameOrDash(t *testing.T) {
	src := "package p\ntype T struct {\n\tA int `json:\"a\"`\n\tB int `json:\"-\"`\n\tC int `json:\",omitempty\"`\n\tD int\n\tE int `yaml:\"e\"`\n}\n"
	f, err := parser.ParseFile(token.NewFileSet(), "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []bool
	ast.Inspect(f, func(n ast.Node) bool {
		if field, ok := n.(*ast.Field); ok && len(field.Names) > 0 {
			got = append(got, HasJSONName(field))
		}
		return true
	})
	want := []bool{true, true, false, false, false}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("alan %d: %v, beklenen %v", i, got[i], want[i])
		}
	}
}

// Kural: RunBinary çıktıları ve çıkış kodunu verir; sıfırdan farklı çıkış testi düşürmez (Wait
// denetler).
func TestRunBinaryReportsOutputsAndExitCode(t *testing.T) {
	exe, err := os.Executable() // bu test ikilisinin kendisi: -test.run ile var olmayan teste çalışır
	if err != nil {
		t.Fatal(err)
	}
	ok := RunBinary(t, exe, "", "", nil, "-test.run=^$")
	Wait(t, ok, 0, []string{"PASS"}, nil)
	bad := RunBinary(t, exe, "", "", nil, "-test.bilinmeyen-bayrak")
	if bad.Code == 0 || bad.Stderr == "" {
		t.Fatalf("geçersiz bayrak sıfırdan farklı kodla ve hata çıktısıyla dönmeli: %+v", bad)
	}
}

// Kural: GlobFiles kalıp sırasını korur ve depo köküne göre çalışır; eşleşmeyen kalıp boş kalır.
// Neden: checker ve başka paketlerin testleri aynı glob döngüsünü kendileri yazıyordu.
func TestGlobFilesKeepsPatternOrder(t *testing.T) {
	got := GlobFiles(t, "tests/*.tr", "no/such/dir/*.tr", "examples/tr/*.tr")
	if len(got) < 2 {
		t.Fatalf("depo programları bulunamadı: %v", got)
	}
	if !strings.Contains(filepath.ToSlash(got[0]), "/tests/") || !strings.Contains(filepath.ToSlash(got[len(got)-1]), "/examples/tr/") {
		t.Fatalf("kalıp sırası bozuldu: ilk %s, son %s", got[0], got[len(got)-1])
	}
}

// Kural: CopyDir klasörü alt klasörleriyle ve dosya içerikleriyle birlikte kopyalar.
// Neden: codeindex ve e2e testleri aynı yürüyüş döngüsünü ayrı ayrı yazıyordu.
func TestCopyDirCopiesTreeAndContents(t *testing.T) {
	src, dst := t.TempDir(), filepath.Join(t.TempDir(), "kopya")
	WritePath(t, filepath.Join(src, "a.txt"), "bir")
	WritePath(t, filepath.Join(src, "alt", "b.txt"), "iki")
	CopyDir(t, src, dst)
	for name, want := range map[string]string{"a.txt": "bir", filepath.Join("alt", "b.txt"): "iki"} {
		got, err := os.ReadFile(filepath.Join(dst, name))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; beklenen %q", name, got, err, want)
		}
	}
}

// recorder hataları toplayan sahte testing.TB'dir.
type recorder struct {
	testing.TB
	errors []string
}

func (r *recorder) Helper() {}
func (r *recorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func TestMustContainReportsEveryMissingPart(t *testing.T) {
	r := &recorder{TB: t}
	MustContain(r, "alpha beta", "alpha", "gamma", "beta", "delta")
	if len(r.errors) != 2 || !strings.Contains(r.errors[0], `"gamma"`) || !strings.Contains(r.errors[1], `"delta"`) {
		t.Errorf("one report per missing part: %v", r.errors)
	}
	MustContain(r, "x", "x")
	if len(r.errors) != 2 {
		t.Error("a present part is not reported")
	}
}

// Kural: ScalesLinearly'nin oranı doğrusal işi karesel işten ayırır (sınır 8: doğrusal ~4,
// karesel ~16). İş, ölçüm çözünürlüğünün çok üstünde olsun diye büyük tutulur.
func TestLinearRatioSeparatesLinearFromQuadratic(t *testing.T) {
	var sink int
	work := func(steps int) func() (int, error) {
		return func() (int, error) {
			s := 0
			for i := 0; i < steps; i++ {
				s += i ^ (s >> 3)
			}
			sink += s
			return steps, nil
		}
	}
	linear := func(n int) func() (int, error) { return work(n * 20000) }
	quadratic := func(n int) func() (int, error) { return work(n * n * 100) }
	if small, large := linearRuns(200, linear); large.ratio(small) > linearLimit {
		t.Errorf("doğrusal iş oranı %.1f, en çok %g olmalı", large.ratio(small), linearLimit)
	}
	if small, large := linearRuns(200, quadratic); large.ratio(small) <= linearLimit {
		t.Errorf("karesel iş oranı %.1f, %g'i aşmalı", large.ratio(small), linearLimit)
	}
	_ = sink
}

// Kural: ScalesLinearly karesel büyüyen işte kalır; deneme sayısı, dönüşümlü ölçüm ve çöp toplama
// onu gizlemez. İş, tarihteki gerileme gibi doğrusal ve karesel payın toplamıdır (codeindex'te CI'da
// oran 9-10): n'de karesel pay doğrusalın dört katı, oran (4+64)/(1+4) ≈ 13,6, saf karesel işin
// ~16'sının altında. Pay küçüldükçe sınama yükte kararsızlaşır: eşit pay (oran 10) boş makinede 8,9'a,
// iki kat pay (oran 12) aşırı yükte 8,0'a indi. İş ölçülen işlevde büyür: ScalesLinearly'nin dışarıdan
// gördüğü yol sınanır. Küçük boy ~20 ms sürer: çekirdekler aşırı yüklüyken zamanlayıcının bir diliminde
// kesilmeden biten birkaç ms'lik iş, hep kesilen büyük boydan ayrıcalıklı ölçülür ve oran bozulur
// (6 ms'lik işle 40 koşuda bir 4,1).
func TestScalesLinearlyFailsQuadraticWork(t *testing.T) {
	var sink int
	r := &recorder{TB: t}
	ScalesLinearly(r, "karesel", 100, func(n int) func() (int, error) {
		steps := n*40000 + n*n*1600
		return func() (int, error) {
			s := 0
			for i := 0; i < steps; i++ {
				s += i ^ (s >> 3)
			}
			sink += s
			return n, nil
		}
	})
	if len(r.errors) != 1 || !strings.Contains(r.errors[0], "does not grow linearly") {
		t.Errorf("karesel iş kalmalı: %v", r.errors)
	}
	_ = sink
}

// Kural: ScalesLinearly süreyi ancak işin iki boyu da hatasız ve tam bittiyse karşılaştırır: iş hata
// verirse ya da büyük iş küçüğün 3,5–4,5 katı birim işlemezse (iş yarıda kaldı, girdi büyümedi) test kalır.
func TestScalesLinearlyChecksTheWorkIsDone(t *testing.T) {
	for _, c := range []struct {
		name string
		work func(n int) (int, error)
		ok   bool
	}{
		{"tam", func(n int) (int, error) { return n, nil }, true},
		{"TR_const pay", func(n int) (int, error) { return n + 10, nil }, true},
		{"büyümeyen", func(n int) (int, error) { return 100, nil }, false},
		{"yarıda kalan", func(n int) (int, error) { return min(n, 150), nil }, false},
		{"hatalı", func(n int) (int, error) { return n, errors.New("bozuk") }, false},
	} {
		r := &recorder{TB: t}
		ScalesLinearly(r, c.name, 100, func(n int) func() (int, error) { return func() (int, error) { return c.work(n) } })
		if ok := len(r.errors) == 0; ok != c.ok {
			t.Errorf("%s: geçti %v, beklenen %v (%v)", c.name, ok, c.ok, r.errors)
		}
	}
}

// Kural: GoStringLiterals her metin sabitini çözülmüş değeri ve satırıyla verir; yapı etiketleri
// ve içe aktarma yolları atlanır.
func TestGoStringLiteralsSkipsTagsAndImports(t *testing.T) {
	path := WriteFile(t, t.TempDir(), "a.go", "package a\n\nimport \"strings\"\n\ntype T struct {\n\tA int `json:\"a\"`\n}\n\nvar x = \"merhaba\"\nvar y = `ham`\nvar _ = strings.ToUpper\n")
	var got []string
	if err := GoStringLiterals(path, func(s string, line int) { got = append(got, strconv.Itoa(line)+":"+s) }); err != nil {
		t.Fatal(err)
	}
	if want := []string{"9:merhaba", "10:ham"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Kural: PanicError hata paniğini olduğu gibi, başka değeri metne çevirerek verir; panik yoksa nil.
func TestPanicError(t *testing.T) {
	sentinel := errors.New("hata")
	if err := PanicError(func() { panic(sentinel) }); err != sentinel {
		t.Errorf("error paniği: %v", err)
	}
	if err := PanicError(func() { panic(42) }); err == nil || err.Error() != "42" {
		t.Errorf("error olmayan panik: %v", err)
	}
	if err := PanicError(func() {}); err != nil {
		t.Errorf("panik yok: %v", err)
	}
}

// Kural: WaitGoroutineBlocked, kanalda bekleyen gorutin görünene kadar bekler ve döner.
func TestWaitGoroutineBlocked(t *testing.T) {
	c := make(chan struct{})
	go blockedOnChannel(c)
	WaitGoroutineBlocked(t, true, []string{"[chan receive"}, "testkit.blockedOnChannel")
	close(c)
}

func blockedOnChannel(c chan struct{}) { <-c }

// Kural: FirstLineDiff ilk ayrılan satırı numarası ve etiketleriyle gösterir; aynı metinde "(fark yok)".
func TestFirstLineDiff(t *testing.T) {
	if got := FirstLineDiff("a\nb\nc", "a\nx\nc", "özgün", "geri"); got != "line 2:\n  özgün: b\n  geri : x" {
		t.Errorf("fark: %q", got)
	}
	if got := FirstLineDiff("a\nb", "a\nb\nc", "1", "2"); got != "line 3:\n  1: \n  2: c" {
		t.Errorf("uzunluk farkı: %q", got)
	}
	if got := FirstLineDiff("a", "a", "1", "2"); got != "(fark yok)" {
		t.Errorf("aynı: %q", got)
	}
}
