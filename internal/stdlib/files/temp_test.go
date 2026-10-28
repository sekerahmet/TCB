package files

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// tempBase işletim sisteminin geçici klasörünü test bitene kadar yeni, boş bir klasör yapar
// ve onu verir (geçici_klasör()'ün oluşturduklarını saymak için).
func tempBase(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	for _, name := range []string{"TMP", "TEMP", "TMPDIR"} {
		t.Setenv(name, base)
	}
	if resolvedDir(os.TempDir()) != resolvedDir(base) {
		t.Skipf("geçici klasör ortam değişkeniyle değiştirilemedi: %s", os.TempDir())
	}
	return base
}

func baseContent(t *testing.T, base string) []string {
	t.Helper()
	inputs, err := reporoot.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, g := range inputs {
		names = append(names, g.Name())
	}
	return names
}

// 18.14: geçici_klasör() geçici klasörün altında yeni, boş, yalnızca kullanıcıya açık bir
// klasör verir; her çağrı ayrı bir klasördür ve program bitince içindekilerle silinir.
func TestTempDir(t *testing.T) {
	tmpWork(t)
	base := tempBase(t)
	got := mustRunTR(t, `TR_var k = TR_temp_dir()
		TR_var bos = TR_list_dir(k).TR_length
		TR_write_file(TR_path(k, "a.txt"), "x")
		TR_make_dir(TR_path(k, "alt/derin"))
		TR_write_file(TR_path(k, "alt/derin/b.txt"), "y")
		TR_var o = TR_read_lines(TR_path(k, "alt/derin/b.txt"))
		[TR_type(k) == TR_Path, k.TR_is_dir, k.TR_absolute == k, bos, k != TR_temp_dir()]`)
	if got != `[doğru, doğru, doğru, 0, doğru]` {
		t.Fatal(got)
	}
	names := baseContent(t, base)
	if len(names) != 2 {
		t.Fatalf("iki geçici klasör bekleniyordu: %v", names)
	}
	prefix := "tr_" + strconv.Itoa(os.Getpid()) + "_"
	for _, name := range names {
		if !strings.HasPrefix(name, prefix) {
			t.Errorf("%q adı %q ile başlamalı", name, prefix)
		}
		if b, err := os.Stat(filepath.Join(base, name)); err != nil || (runtime.GOOS != "windows" && b.Mode().Perm() != 0o700) {
			t.Errorf("%s yalnızca kullanıcıya açık olmalı: %v %v", name, b.Mode(), err)
		}
	}
	// Açık bir satır okuyucusu da olsa (önce kapatılır) klasörler silinir.
	CloseAll()
	if rest := baseContent(t, base); len(rest) != 0 {
		t.Fatalf("program bitince geçici klasör kalmamalı: %v", rest)
	}
	CloseAll() // ikinci kez çağırmak sorun değildir
	errContains(t, `TR_temp_dir(1)`, "geçici_klasör() 0 argüman bekler, 1 verildi")
}

// Program geçici klasörü kendisi silmiş ya da taşımışsa program sonunda hata yoktur.
func TestTempDirDeletedByProgram(t *testing.T) {
	tmpWork(t)
	base := tempBase(t)
	mustRunTR(t, `TR_var k = TR_temp_dir(); TR_remove(k); TR_move(TR_temp_dir(), "tasindi")`)
	if errs := deleteTempDirs(); len(errs) != 0 {
		t.Fatal(errs)
	}
	if rest := baseContent(t, base); len(rest) != 0 {
		t.Fatalf("taban boş olmalı: %v", rest)
	}
}

// Geçici klasör silinirken bağlantılar izlenmez: bağlantının gösterdiği klasöre dokunulmaz.
func TestTempDirDoesNotFollowLinks(t *testing.T) {
	k := tmpWork(t)
	tempBase(t)
	outer := filepath.Join(k, "dis")
	write(t, filepath.Join(outer, "korunan.txt"), "x")
	s := runTR(t, "", `TR_temp_dir()`)
	if s.failure != nil {
		t.Fatal(s.failure)
	}
	tmp := filepath.FromSlash(s.value.Inspect())
	dirLink(t, outer, filepath.Join(tmp, "baglanti"))
	if errs := deleteTempDirs(); len(errs) != 0 {
		t.Fatal(errs)
	}
	if exists(tmp) {
		t.Error("geçici klasör silinmeli")
	}
	if !exists(filepath.Join(outer, "korunan.txt")) {
		t.Fatal("bağlantının gösterdiği klasördeki dosya silindi")
	}
}

// Salt okunur bir dosya bulunan geçici klasör zorla silinmez (sil'in kuralı); uyarı döner.
func TestTempDirReadOnly(t *testing.T) {
	tmpWork(t)
	tempBase(t)
	s := runTR(t, "", `TR_var k = TR_temp_dir(); TR_write_file(TR_path(k, "s.txt"), "x"); k`)
	if s.failure != nil {
		t.Fatal(s.failure)
	}
	file := filepath.Join(filepath.FromSlash(s.value.Inspect()), "s.txt")
	if err := os.Chmod(file, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(file, 0o644) })
	errs := deleteTempDirs()
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "geçici_klasör(): ") ||
		!strings.Contains(errs[0].Error(), "salt okunur") {
		t.Fatalf("salt okunur dosya uyarısı bekleniyordu: %v", errs)
	}
}
