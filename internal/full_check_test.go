package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural (tools/full_check.sh, --ilk-kalanda-dur): izlenen bir adım, izleyicinin gördüğü kalan satırla
// kalır; yalnız durdurulan sürecin çıkış koduna bakılmaz. Kalan satırını yazıp 0 dönen süreç bayrakla
// ✅ veriyordu (DENETİM istek 51, S1). Test betiğin kendi watch_failures ve watched_ok işlevlerini
// alır, sahte adımlarla bash'te koşar. Bayraksız yol bilerek eskisi gibidir: gerçek go test kalan
// paketle her zaman sıfırdan farklı çıkar.
func TestStopAtFirstFailureCountsTheSeenFailureLine(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash yok")
	}
	script, err := os.ReadFile(filepath.Join(reporoot.Path("."), "tools", "full_check.sh"))
	if err != nil {
		t.Fatal(err)
	}
	var funcs []string
	for _, name := range []string{"watch_failures", "watched_ok"} {
		m := regexp.MustCompile(`(?ms)^` + name + `\(\) \{.*?^\}`).Find(script)
		if m == nil {
			t.Fatalf("tools/full_check.sh: %s işlevi bulunamadı", name)
		}
		funcs = append(funcs, string(m))
	}
	// Sonuçtan sonra DURDU: izleyici kardeş süreçleri durdurdu; SONRAKİ: kalan satırdan sonraki satır da geçti.
	cases := []struct {
		flag, output, exit, want string
	}{
		{"1", `printf 'ok\tpkg/b\nFAIL\tpkg/a\nok\tpkg/c\n'`, "0", "KALDI DURDU"}, // istek 51'in senaryosu
		{"1", `printf 'ok\tpkg/b\n'`, "0", "GEÇTİ"},
		{"1", `printf 'ok\tpkg/b\n'`, "1", "KALDI"},
		{"1", `printf '!! KALDI: örnek\nok\tpkg/c\n'`, "0", "KALDI DURDU"},
		{"", `printf 'ok\tpkg/b\nFAIL\tpkg/a\nok\tpkg/c\n'`, "0", "GEÇTİ SONRAKİ"}, // bayraksız eski yol, bilerek
		{"", `printf 'ok\tpkg/b\n'`, "0", "GEÇTİ"},
		{"", `printf 'ok\tpkg/b\n'`, "1", "KALDI"},
	}
	for _, c := range cases {
		src := "set -o pipefail\nstop_siblings() { echo DURDU >>\"$durdu\"; }\n" + strings.Join(funcs, "\n") + "\n" +
			"stop_first=" + c.flag + "\nseen_fail=$(mktemp)\ndurdu=$(mktemp)\nseen=$(mktemp)\n" +
			"( " + c.output + "; exit " + c.exit + " ) | watch_failures >\"$seen\"\n" +
			`if watched_ok "${PIPESTATUS[0]}"; then echo GEÇTİ; else echo KALDI; fi` + "\n" +
			`if [ -s "$durdu" ]; then echo DURDU; fi` + "\n" +
			`if grep -q 'pkg/c' "$seen"; then echo SONRAKİ; fi` + "\n" +
			`rm -f "$seen_fail" "$durdu" "$seen"` + "\n"
		out, err := exec.Command(bash, "-c", src).CombinedOutput()
		if got := strings.Join(strings.Fields(string(out)), " "); err != nil || got != c.want {
			t.Errorf("bayrak %q, çıktı %s, çıkış %s: %q beklenirdi, gelen %q (%v)", c.flag, c.output, c.exit, c.want, got, err)
		}
	}
}
