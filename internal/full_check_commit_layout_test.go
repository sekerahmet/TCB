package internal

// Commit kipinin arka plandaki düzen adımı (tools/full_check.sh, kip-yakın K2–K7): testler betiğin işlevlerini
// commitPrelude'la çıkarıp sahte go'yla koşar (full_check_commit_test.go).

import (
	"strconv"
	"strings"
	"testing"
)

// layoutPrelude adım ve kalan yazımını kaydın sonuç alanını da dolduracak biçimde sahteler (record_failure gibi: ilk
// kalan adımın adı) ve G4'ü sık düşüren hafif testleri tek sahte pakete (./ct) indirir.
const layoutPrelude = `step() { step_name=$1; printf '== %s\n' "$1"; }
fail() { printf '!! KALDI: %s\n' "$1"; failed=1; [ -n "$commit_failed_step" ] || commit_failed_step=${step_name:-$1}; }
commit_test_pkgs=(./ct)
commit_specrefs=0,not:0
`

const layoutTitle = "Düzen kuralları (./internal, ./internal/reporoot) ve G4'ü sık düşüren hafif testler (5 test, 1 paket)"

// Kural (tools/full_check.sh, --kip=commit; kip-yakın K2–K6; DENETİM 840 Ş2, 841 N1, N3): düzen adımı atıflardan sonra
// arka planda başlar, yakınlar ön planda onunla birlikte koşar; kip sonda onu bekler, çıktısını başlığıyla basar ve
// kalan adımını ana kabukta kaydeder. 120 sn yalnız yakınlarındır: düzen adımı kesilmez (TCB 2026-11-17 16:13). İki
// adım birden kalırsa kaydın sonuç alanı düzen adımıdır. --ilk-kalanda-dur'da arka planın kalanı sonraki parçayı
// başlatmaz, ama koşmuş parçanın geçtisi kalır. Neden: düzen adımı ve atıflar yakınlardan önce 120 sn'nin 65–97
// sn'sini harcıyordu; 4c53d080'de yakınlara 7 sn kaldı.
func TestCommitModeRunsLayoutAlongsideNearTests(t *testing.T) {
	ok := func(pkg string) string { return "ok  \texample.com/fix/" + pkg + "\t0.1s\n" }
	bad := func(pkg, test string) string {
		return "--- FAIL: " + test + " (0.00s)\nFAIL\nFAIL\texample.com/fix/" + pkg + "\t0.1s\n"
	}
	run := "commit_layout_start\ncommit_selection $'%s'\ncommit_layout_finish\n" +
		"echo \"FAILED=$failed STEP=$commit_failed_step SPEC=$commit_specrefs\"\necho \"RAN=${commit_ran[*]}\"\necho \"LEFT=${commit_left[*]}\"\n"
	for _, c := range []struct {
		name, flags, sel string
		files            map[string]string
		want, not        []string
	}{
		{"birlikte", "", "a TestA near",
			map[string]string{"internal.sleep": "4", "internal.out": ok("internal"), "a.out": ok("a"), "ct.out": ok("ct")},
			[]string{"== " + layoutTitle, "FAILED=0 STEP= SPEC=0,not:0", "RAN=./a TestA"}, []string{"!! KALDI"}},
		{"düzen kalır", "", "a TestA near",
			map[string]string{"internal.out": bad("internal", "TestLayout"), "a.out": ok("a"), "ct.out": ok("ct")},
			[]string{"== " + layoutTitle + "\n--- FAIL: TestLayout", "!! KALDI: düzen kuralları", "FAILED=1 STEP=" + layoutTitle + " SPEC=0,not:0", "RAN=./a TestA"}, nil},
		{"ikisi birden", "", "a TestA near",
			map[string]string{"internal.out": bad("internal", "TestLayout"), "a.out": bad("a", "TestA"), "ct.out": ok("ct")},
			[]string{"!! KALDI: değişen dosyaların hafif testleri", "!! KALDI: düzen kuralları", "STEP=" + layoutTitle + " "}, nil},
		{"uzun düzen", "commit_deadline_ms=$(($(commit_now_ms) + 4000))\n", "a TestA near",
			map[string]string{"internal.sleep": "6", "internal.out": ok("internal"), "a.out": ok("a"), "ct.out": ok("ct")},
			[]string{"== " + layoutTitle, "FAILED=0", "RAN=./a TestA"}, []string{"!! KALDI"}},
		{"ilk kalanda", "stop_first=1\n", "a TestA near\\nb TestB far",
			// Düzen adımı 2 sn'de kalır, ilk parça (./a) o sırada koşmaktadır (4 sn): ikinci parça (./b) başlamaz.
			map[string]string{"internal.sleep": "2", "internal.out": bad("internal", "TestLayout"), "a.sleep": "4", "a.out": ok("a"), "b.out": ok("b"), "ct.out": ok("ct")},
			[]string{"!! KALDI: düzen kuralları", "RAN=./a TestA", "LEFT=./b TestB"}, []string{"KALDI: değişen"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir, env := fakeGo(t, c.files)
			deadline := "commit_deadline_ms=$(($(commit_now_ms) + 20000))\n"
			if strings.HasPrefix(c.flags, "commit_deadline_ms") {
				deadline = ""
			}
			src := layoutPrelude + deadline + c.flags + strings.Replace(run, "%s", c.sel, 1) + "echo \"T=$SECONDS\"\n"
			out := runCommitBash(t, dir, env, src)
			for _, w := range c.want {
				if !strings.Contains(out, w) {
					t.Errorf("çıktıda yok: %q\n%s", w, out)
				}
			}
			for _, w := range c.not {
				if strings.Contains(out, w) {
					t.Errorf("çıktıda olmamalı: %q\n%s", w, out)
				}
			}
			args := fakeArgs(t, dir)
			at := func(pkg string) int {
				for i, a := range args {
					if strings.HasSuffix(a, " "+pkg) || strings.Contains(a, " "+pkg+" ") {
						return i
					}
				}
				return -1
			}
			switch c.name {
			case "birlikte":
				// Yakın parça düzen adımı bitmeden başlar: ./ct, düzen paketlerinin 4 sn'lik koşusundan sonra gelir.
				if a, ct := at("./a"), at("./ct"); a < 0 || ct < 0 || a > ct {
					t.Errorf("yakın parça (./a) düzen adımı sürerken başlamalıydı: %q", args)
				}
			case "uzun düzen":
				// Kip düzen adımını kesmez, bekler: sahte paket 6 sn uyur, yakınların süresi 4 sn.
				if secs := lastSeconds(out); secs < 6 {
					t.Errorf("kip düzen adımını beklemeliydi (T=%d):\n%s", secs, out)
				}
			case "ilk kalanda":
				if at("./b") >= 0 {
					t.Errorf("arka plan kalınca sonraki parça (./b) başlamamalıydı: %q", args)
				}
			}
		})
	}
}

// Kural (tools/full_check.sh, --kip=commit; kip-yakın K7; DENETİM 840 Ş2, 841 N2, 845 N3): kip kesilince (TERM) trap
// arka plandaki düzen adımını alt süreçleriyle durdurur ve geçici dosyalarını siler; kip beklerken hemen, ön planda yakın bir parça koşarken en geç o
// parça bitince (bash trap'i ön plandaki komut bitince koşar). Neden: betiği çağıranın timeout'u yalnız betiği keser,
// arka plandaki go test sonraki komutlarla yarışırdı.
func TestCommitModeStopsTheLayoutWhenCut(t *testing.T) {
	for _, c := range []struct {
		name, body string
		min, max   int
	}{
		// Üst sınırlar Windows'ta durdurmanın kendi süresini (ps ve taskkill, 2–3 sn) de taşır: dizüstünde 3 ve 6 sn.
		{"beklerken", "commit_layout_finish\n", 0, 8},
		{"yakın koşarken", "commit_selection $'a TestA near' >/dev/null\ncommit_layout_finish\n", 3, 12},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir, env := fakeGo(t, map[string]string{"internal.sleep": "20", "internal.out": "ok\n", "a.sleep": "3", "a.out": "ok  \texample.com/fix/a\t0.1s\n"})
			src := layoutPrelude + "commit_deadline_ms=$(($(commit_now_ms) + 20000))\nt0=$SECONDS\n" +
				"( me=$BASHPID; commit_layout_start; echo \"$commit_layout_out $commit_layout_res $commit_layout_res.tmp\" >\"$FAKE_GO_DIR/tmpfiles\"; " +
				"(sleep 1; kill -TERM $me) & " + strings.TrimSuffix(strings.ReplaceAll(c.body, "\n", "; "), " ") + " )\n" +
				"echo \"CODE=$? T=$((SECONDS - t0))\"\nsleep 1\n" +
				"for p in $(cat \"$FAKE_GO_DIR/pids\"); do kill -0 \"$p\" 2>/dev/null && echo \"ALIVE $p\"; done\n" +
				"for f in $(cat \"$FAKE_GO_DIR/tmpfiles\"); do [ ! -e \"$f\" ] || echo \"KEPT $f\"; done\necho END\n"
			out := runCommitBash(t, dir, env, src)
			t.Logf("durduruldu: T=%d sn", lastSeconds(out))
			if !strings.Contains(out, "CODE=143 ") {
				t.Errorf("kesilen kip 143 ile bitmeliydi:\n%s", out)
			}
			if strings.Contains(out, "ALIVE ") {
				t.Errorf("arka plandaki sahte go durdurulmalıydı:\n%s", out)
			}
			if strings.Contains(out, "KEPT ") {
				t.Errorf("durdurulan arka planın geçici dosyaları silinmeliydi (DENETİM 845 N3):\n%s", out)
			}
			if secs := lastSeconds(out); secs < c.min || secs > c.max {
				t.Errorf("arka plan %d–%d sn içinde durmalıydı (T=%d):\n%s", c.min, c.max, secs, out)
			}
		})
	}
}

// lastSeconds çıktının son "T=<sn>" değeridir; yoksa -1.
func lastSeconds(out string) int {
	i := strings.LastIndex(out, "T=")
	if i < 0 {
		return -1
	}
	s := out[i+2:]
	if j := strings.IndexAny(s, " \n"); j >= 0 {
		s = s[:j]
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}
