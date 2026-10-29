package e2e

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// trProgram TR satırlarını tek bir programa birleştirir (ilk satır başlık).
func trProgram(lines ...string) string {
	return "# TR 4\n" + strings.Join(lines, "\n") + "\n"
}

// TR_exit(kod) programı nazikçe bitirir (1.10-1.14): kod 0-125 arasında bir Tam'dır; ertele
// blokları çalışır, yakala yakalamaz; bir görevden çağrılsa da bütün program biter; bir ertele
// bloğunda çağrılan son çağrı kodu belirler; biterken ertelenen işin ya da görevin hatası çıkışı
// 1 yapar.
func TestExitCodes(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name      string
		code      string
		want      int
		stdout    []string
		notStdout []string
		stderr    []string
	}{
		{"kodla çıkar", trProgram(`TR_print("a")`, "TR_exit(4)", `TR_print("b")`), 4, []string{"a"}, []string{"b"}, nil},
		{"başarı kodu", trProgram("TR_exit(0)"), 0, nil, nil, nil},
		{"en büyük kod", trProgram("TR_exit(125)"), 125, nil, nil, nil},
		{"ertele çalışır", trProgram(`TR_defer { TR_print("ertele") }`, "TR_exit(4)"), 4, []string{"ertele"}, nil, nil},
		{"yakala yakalamaz", trProgram(`TR_try { TR_exit(5) } TR_catch e { TR_print("yakalandı") }`, `TR_print("sürdü")`), 5, nil, []string{"yakalandı", "sürdü"}, nil},
		{"görevden çağrılır", trProgram("TR_async { TR_exit(6) }", "TR_sleep(60)", `TR_print("sürdü")`), 6, nil, []string{"sürdü"}, nil},
		{"ertelede son çağrı kazanır", trProgram("TR_defer { TR_exit(9) }", `TR_defer { TR_print("iç ertele") }`, "TR_exit(3)"), 9, []string{"iç ertele"}, nil, nil},
		{"aralık dışı kod hatadır", trProgram("TR_exit(126)"), 1, nil, nil, []string{"0 ile 125", "(4)"}},
		{"ertelenen işin hatası 1", trProgram(`TR_defer { TR_throw "ertele patladı" }`, "TR_exit(3)"), 1, nil, nil, []string{"ertele patladı"}},
		{"görevin hatası 1", trProgram(`TR_async { TR_throw "görev patladı" }`, "TR_wait_all()"), 1, nil, nil, []string{"görev patladı"}},
		{"yakalanmayan hata 1", trProgram(`TR_throw "patladı"`), 1, nil, nil, []string{"patladı"}},
		{"en dışta return sözdizimi hatası", trProgram("TR_return 1"), 1, nil, nil, []string{"(0)"}},
	} {
		writeTR(t, dir, "program.tr", tc.code)
		got := tr(t, dir, "program.tr")
		testkit.Wait(t, got, tc.want, tc.stdout, tc.stderr)
		for _, absent := range tc.notStdout {
			if strings.Contains(got.Stdout, absent) {
				t.Errorf("%s: standart çıktıda %q olmamalıydı:\n%s", tc.name, absent, got.Stdout)
			}
		}
	}
}

// tr'nin kendi kullanım hatası (eksik dosya, bilinmeyen seçenek) 2 koduyla biter (1.9).
func TestToolUsageErrorsExitTwo(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		{"run"},
		{"check"},
		{"build"},
		{"debug"},
		{"--yok"},
		{"run", "--yok", "a.tr"},
	} {
		if s := tr(t, dir, args...); s.Code != 2 {
			t.Errorf("%v: çıkış kodu %d, 2 bekleniyordu\n%s", args, s.Code, s.Stderr)
		}
	}
}

// Çalışmadan önce program ve aktardığı modüller tek yoldan denetlenir (1.3): hata varsa hiçbir
// satır çalışmaz, çıkış 1. 'tr check', 'tr run' ve 'tr debug' aynı sonucu verir.
func TestModulesAreCheckedBeforeAnyLineRuns(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "bozuk.tr", trProgram("TR_func f() -> TR_Int {", `    TR_return "a"`, "}"))
	writeTR(t, dir, "ana.tr", trProgram(`TR_import "bozuk" TR_as b`, `TR_print("ilk satır")`, "TR_print(b.f())"))
	for _, command := range [][]string{{"ana.tr"}, {"run", "ana.tr"}, {"debug", "ana.tr"}, {"check", "ana.tr"}} {
		s := tr(t, dir, command...)
		testkit.Wait(t, s, 1, nil, []string{"bozuk.tr"})
		if strings.Contains(s.Stdout, "ilk satır") {
			t.Errorf("%v: hatalı modül varken hiçbir satır çalışmamalıydı: %q", command, s.Stdout)
		}
	}
}
