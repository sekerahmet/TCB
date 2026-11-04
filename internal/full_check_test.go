package internal

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

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

// Kural (tools/full_check.sh, --taban): taban tek bir commit'e çözülmezse betik çıkış 2 ile durur.
// Birden çok commit'e uyan kısa kimlik "ortak ata yok" diye okunup değişen dosyaların testleri ve dups
// sessizce atlanıyordu (D1'in 13.1'i, 2026-11-03). Ortak önek depodan aranır: sabit bir önek yalnız yerel
// depoda belirsizdi, taze tam klonda çözülüp gerçek ön denetimi başlatıyordu (DENETİM istek 104). Sığ
// klonda önek çıkmaz, olmayan kimlik yine sınanır.
func TestBaseThatIsNotOneCommitStops(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash yok")
	}
	root := reporoot.Path(".")
	bases := []string{"0000000000000000"}
	if p := sharedCommitPrefix(t, root); p != "" {
		bases = append(bases, p)
	} else {
		t.Log("ortak önek yok (sığ klon): yalnız olmayan kimlik sınanıyor")
	}
	for _, base := range bases {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, bash, filepath.Join("tools", "full_check.sh"), "--kip=on", "--taban="+base)
		cmd.WaitDelay = time.Second
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 2 || !strings.Contains(string(out), "tek bir commit'e çözülmüyor") {
			t.Errorf("--taban=%s: çıkış 2 ve çözülmüyor iletisi beklenirdi, gelen %v: %s", base, err, out)
		}
	}
}

// sharedCommitPrefix iki commit'in ortak dört harfli önekini verir; yoksa boş döner.
func sharedCommitPrefix(t *testing.T, root string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-list", "--all")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	ids := strings.Fields(string(out))
	slices.Sort(ids)
	for i := 1; i < len(ids); i++ {
		if ids[i][:4] == ids[i-1][:4] {
			return ids[i][:4]
		}
	}
	return ""
}

// Kural (tools/full_check.sh, --kip=on): ön denetimin bütün Go adımları önbelleksiz koşar. Depo testleri
// dosyaları `git ls-files` ile bulur; çalışma ağacında Go'nun önbelleği yeni izlenen dosyayı görmez ve eski
// sonucu verir (DENETİM istek 106, 110).
func TestPreCheckGoStepsSkipTheCache(t *testing.T) {
	data, err := os.ReadFile(reporoot.Path("tools/full_check.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := strings.ReplaceAll(string(data), "\r\n", "\n")
	goTest := regexp.MustCompile(`^\s*(?:\w+=\S*\s+)*go test `)
	for _, part := range [][2]string{{"selected_tests() {\n", "\n}\n"}, {"\ton)\n\t\tquick_static_checks\n", "\n\t\t;;\n"}} {
		_, rest, found := strings.Cut(script, part[0])
		if !found {
			t.Fatalf("tools/full_check.sh: %q bulunamadı", part[0])
		}
		body, _, _ := strings.Cut(rest, part[1])
		count := 0
		for line := range strings.SplitSeq(body, "\n") {
			if !goTest.MatchString(line) {
				continue
			}
			count++
			if !strings.Contains(line, "-count=1") {
				t.Errorf("-count=1 yok: %s", strings.TrimSpace(line))
			}
		}
		if count == 0 {
			t.Errorf("%q içinde go test satırı bulunamadı", part[0])
		}
	}
}
