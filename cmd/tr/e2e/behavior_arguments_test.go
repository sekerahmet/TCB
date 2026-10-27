package e2e

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Programlar: readsProgram komut satırını en dışta okur; readsViaFunction onu bir fonksiyonda
// okur ve fonksiyonu çağırır; readsNothing hiç okumaz; readsInAFunctionNeverCalled yalnız
// çağrılmayan bir fonksiyonda okur.
var (
	readsProgram = strings.Join([]string{
		"# TR 4",
		`TR_var ayar = TR_command_line({"adim": 1})`,
		`TR_print("adım: \(ayar["adim"])")`,
	}, "\n") + "\n"
	readsViaFunction = strings.Join([]string{
		"# TR 4",
		"TR_func oku() {",
		`    TR_var ayar = TR_command_line({"adim": 1})`,
		`    TR_print("adım: \(ayar["adim"])")`,
		"}",
		"oku()",
	}, "\n") + "\n"
	readsNothing                = "# TR 4\nTR_print(\"çalıştı\")\n"
	readsInAFunctionNeverCalled = strings.Join([]string{
		"# TR 4",
		"TR_func oku() {",
		`    TR_var ayar = TR_command_line({"adim": 1})`,
		"}",
		`TR_print("çalıştı")`,
	}, "\n") + "\n"
)

// Betiğe dosya adından sonra verilen argümanlar programındır ve her çalıştırma yolunda aynıdır
// (1.4): komut satırını okuyan program onları alır.
func TestScriptArgumentsPassed(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "okur.tr", readsProgram)
	testkit.WriteFile(t, dir, "dolayli.tr", readsViaFunction)
	for _, command := range [][]string{
		{"okur.tr", "--adim", "5"},
		{"çalıştır", "okur.tr", "--adim", "5"},
		{"hata_ayıkla", "okur.tr", "--adim", "5"},
		{"dolayli.tr", "--adim", "5"},
	} {
		testkit.Wait(t, tr(t, dir, command...), 0, []string{"adım: 5"}, nil)
	}
}

// Argüman verildiği hâlde programın en dışından ulaşılan bir komut_satırı çağrısı yoksa program
// çalışmaz: kullanım hatası (çıkış 2), hiçbir satır çalışmaz; ileti verilen seçeneği ve eklenecek
// çağrıyı söyler (1.4). Aynı kural her çalıştırma yolunda geçerlidir.
func TestArgumentsToAProgramThatReadsNoneAreAUsageError(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "okumaz.tr", readsNothing)
	testkit.WriteFile(t, dir, "cagrilmaz.tr", readsInAFunctionNeverCalled)
	for _, command := range [][]string{
		{"okumaz.tr", "--adim", "5"},
		{"çalıştır", "okumaz.tr", "--adim", "5"},
		{"hata_ayıkla", "okumaz.tr", "--adim", "5"},
		{"cagrilmaz.tr", "--adim", "5"},
	} {
		s := tr(t, dir, command...)
		testkit.Wait(t, s, 2, nil, []string{"--adim", "komut_satırı"})
		if s.Stdout != "" {
			t.Errorf("%v: hiçbir satır çalışmamalıydı: %q", command, s.Stdout)
		}
	}
	// Argümansız çalıştırmada kural yok
	testkit.Wait(t, tr(t, dir, "okumaz.tr"), 0, []string{"çalıştı"}, nil)
}

// Komut satırını okumayan programa yalnız -h verilirse programın seçeneği olmadığı yazılır,
// çıkış 0 (1.4). Başlatıcıların argümanları (ROS 2'nin --ros-args blokları, --local-rank=N)
// programın sayılmaz: programa argüman verilmemiş gibi çalışır.
func TestHelpAndLauncherArgumentsOfAProgramThatReadsNone(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "okumaz.tr", readsNothing)
	testkit.Wait(t, tr(t, dir, "okumaz.tr", "-h"), 0, []string{"seçeneği almıyor"}, nil)
	for _, launcher := range [][]string{
		{"--ros-args", "-r", "__node:=x", "-p", "a:=1", "--"},
		{"--ros-args", "-r", "__node:=x"},
		{"--local-rank=3"},
		{"--local_rank=3"},
	} {
		testkit.Wait(t, tr(t, dir, append([]string{"okumaz.tr"}, launcher...)...), 0, []string{"çalıştı"}, nil)
	}
	// ROS 2 bloğundan sonra gelen argüman programındır
	testkit.WriteFile(t, dir, "okur.tr", readsProgram)
	testkit.Wait(t, tr(t, dir, "okur.tr", "--ros-args", "-r", "a:=b", "--", "--adim", "7", "--local-rank=1"), 0, []string{"adım: 7"}, nil)
}

// 'tr derle' ile üretilen uygulamanın bütün argümanları programındır (1.4); kural onda da
// geçerlidir.
func TestBuiltAppArguments(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "okur.tr", readsProgram)
	testkit.WriteFile(t, dir, "okumaz.tr", readsNothing)
	readsExe := filepath.Join(t.TempDir(), "okur.exe")
	readsNothingExe := filepath.Join(t.TempDir(), "okumaz.exe")
	testkit.Wait(t, tr(t, dir, "derle", "okur.tr", readsExe), 0, nil, nil)
	testkit.Wait(t, tr(t, dir, "derle", "okumaz.tr", readsNothingExe), 0, nil, nil)
	testkit.Wait(t, testkit.RunBinary(t, readsExe, dir, "", nil, "--adim", "5"), 0, []string{"adım: 5"}, nil)
	testkit.Wait(t, testkit.RunBinary(t, readsNothingExe, dir, "", nil, "--adim", "5"), 2, nil, []string{"--adim"})
}
