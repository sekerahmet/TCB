package internal

// Commit kipinin değişen dosyalar adımı ve kaydı (tools/full_check.sh, D1 codegraph dilim 3): testler betiğin
// işlevlerini çıkarıp bash'te sahte bir go komutuyla koşar; seçim metin olarak verilir.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// commitPrelude betiğin commit kipi işlevlerini, dizilerini ve kayıt değişkenlerini bash öncülü olarak verir; adım ve
// kalan yazımı sahtedir (step, fail), izleyici ve sonuç işlevleri betiğin kendisidir.
func commitPrelude(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(reporoot.Path("tools/full_check.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := strings.ReplaceAll(string(data), "\r\n", "\n")
	parts := []string{"set -o pipefail", "stop_first=\"\"", "seen_fail=$(mktemp)", "p_flag=()", "failed=0",
		`step() { printf '== %s\n' "$1"; }`, `fail() { printf '!! KALDI: %s\n' "$1"; failed=1; }`}
	for _, pat := range []string{
		`(?ms)^commit_heavy_tests=\(\n.*?^\)$`, `(?ms)^commit_late_packages=\(\n.*?^\)$`,
		`(?ms)^commit_seconds=.*?^commit_failed_step=""$`, `(?m)^commit_now_ms\(\) \{ .*\}$`,
	} {
		m := regexp.MustCompile(pat).FindString(script)
		if m == "" {
			t.Fatalf("tools/full_check.sh: %s bulunamadı", pat)
		}
		parts = append(parts, m)
	}
	for _, name := range []string{"commit_plan", "commit_run_chunks", "commit_settle", "commit_selection", "commit_record",
		"commit_counts", "commit_kind", "commit_passed_line", "watch_failures", "watched_ok", "spec_refs_check", "commit_light_tests"} {
		m := regexp.MustCompile(`(?ms)^` + name + `\(\) \{\n.*?^\}$`).FindString(script)
		if m == "" {
			t.Fatalf("tools/full_check.sh: %s işlevi bulunamadı", name)
		}
		parts = append(parts, m)
	}
	return strings.Join(parts, "\n") + "\n"
}

// fakeGo test klasörüne go.mod ve sahte bir go komutu yazar: argümanlarını ve süreç kimliğini kaydeder, argümandaki her
// ./paket için <paket>.out'u basar (önce <paket>.sleep kadar bekler); durdurulunca (TERM) paketlerin <paket>.kill'ini
// basar. Çıktılar out'taki anahtarlardır ("a.out", "b.sleep" ...). Ortam değişkenleri ve klasör döner.
func fakeGo(t *testing.T, out map[string]string) (dir string, env []string) {
	t.Helper()
	dir = t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(filepath.Join(dir, "fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module example.com/fix\n",
		"bin/go": "#!/usr/bin/env bash\nprintf '%s\\n' \"$*\" >>\"$FAKE_GO_DIR/args\"\necho $$ >>\"$FAKE_GO_DIR/pids\"\n" +
			"keys=()\nfor a in \"$@\"; do case $a in ./*) k=${a#./}; keys+=(\"${k//\\//_}\") ;; esac; done\n" +
			"trap 'for k in \"${keys[@]}\"; do cat \"$FAKE_GO_DIR/$k.kill\" 2>/dev/null; done; kill $! 2>/dev/null; exit 1' TERM\n" +
			"code=0\nfor k in \"${keys[@]}\"; do\n" +
			"  if [ -f \"$FAKE_GO_DIR/$k.sleep\" ]; then sleep \"$(cat \"$FAKE_GO_DIR/$k.sleep\")\" & wait $!; fi\n" +
			"  cat \"$FAKE_GO_DIR/$k.out\" 2>/dev/null && grep -q '^FAIL' \"$FAKE_GO_DIR/$k.out\" && code=1\n" +
			"done\nexit $code\n",
	}
	for name, body := range out {
		files["fake/"+name] = body
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	env = append(os.Environ(), "FAKE_GO_DIR="+filepath.ToSlash(filepath.Join(dir, "fake")),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir, env
}

// runCommitBash öncülün ardından src'yi klasörde bash'te koşar ve çıktıyı verir.
func runCommitBash(t *testing.T, dir string, env []string, src string) string {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash yok")
	}
	// Metin dosyadan okunur: Windows'un komut satırından geçen uzun `bash -c` metni bozuluyordu (sözdizimi hatası).
	file := filepath.Join(t.TempDir(), "commit.sh")
	if err := os.WriteFile(file, []byte(commitPrelude(t)+src), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bash, filepath.ToSlash(file))
	cmd.WaitDelay = time.Second
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	return string(out)
}

// fakeArgs sahte go'nun aldığı komut satırlarıdır (parça başına bir satır).
func fakeArgs(t *testing.T, dir string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "fake", "args"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var lines []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// Kural (tools/full_check.sh, --kip=commit; D1 codegraph dilim 3): commit kipi değişen dosyaların hafif testlerini
// koşar (codegraph changed --go-tests'in seçimi). ./internal ve ./internal/reporoot seçimden çıkar (kip onları bütün
// olarak koşar); ağır listesindeki ad, başka paketteyse de koşmaz (-skip paketten bağımsızdır), "G4'e kaldı"da adıyla
// yazılır ve sonuç satırı kalan sayısını taşır: kalan test sessiz kalmaz (DENETİM 669 Ş2, 681 N2).
func TestCommitModeRunsTheLightSelection(t *testing.T) {
	dir, env := fakeGo(t, map[string]string{"a.out": "ok  \texample.com/fix/a\t0.1s\n"})
	out := runCommitBash(t, dir, env, "commit_deadline_ms=$(($(commit_now_ms) + 60000))\n"+
		"commit_selection $'a TestA1\\na TestA2\\ninternal TestLayout\\ninternal/reporoot TestRoot\\n"+
		"tools/codegraph TestPlanningQuestionsAreAnswered far\\nb TestPlanningQuestionsAreAnswered far\\na helperNotATest'\n"+
		"echo \"RAN=${commit_ran[*]}\"\ncommit_passed_line\n")
	for _, want := range []string{
		"== Değişen dosyaların hafif testleri, yakından uzağa (2 test, 1 paket, 2 yakın; codegraph changed --go-tests --by-distance; ağır listesindeki ve kipin 120 sn'si (kipin başından, derleme dahil) dolunca bitmemiş testler G4'e kalır)",
		"G4'e kaldı (2 test: 2 ağır listesinde, 0 test 120 sn'de kesilen 0 pakette): TestPlanningQuestionsAreAnswered, TestPlanningQuestionsAreAnswered",
		"RAN=./internal TestLayout ./internal/reporoot TestRoot ./a TestA1 ./a TestA2",
		"✅ Commit öncesi denetim geçti (tam denetim değildir; yakın testlerin hepsi koştu; 2 uzak test G4'e kaldı)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("çıktıda yok: %q\n%s", want, out)
		}
	}
	args := fakeArgs(t, dir)
	if len(args) != 1 {
		t.Fatalf("tek parça beklenirdi, go %d kez koştu: %q", len(args), args)
	}
	line := args[0]
	for _, want := range []string{"-short -count=1 -run ^(TestA1|TestA2)$ -skip ^(", "TestPlanningQuestionsAreAnswered", " ./a"} {
		if !strings.Contains(line, want) {
			t.Errorf("go'nun argümanlarında yok: %q\n%s", want, line)
		}
	}
	if strings.Contains(line, "./internal") || strings.Contains(line, "./tools/codegraph") || strings.Contains(line, "./b") {
		t.Errorf("koşmaması gereken paket go'ya verildi: %s", line)
	}
	empty := runCommitBash(t, dir, env, "commit_selection ''\ncommit_passed_line\n")
	if strings.Contains(empty, "==") || !strings.Contains(empty, "ℹ️ Commit öncesi denetim bitti, geçti değil: değişen dosyalar için hiç test seçilmedi; düzen kuralları ve hafif testler koştu.\n") {
		t.Errorf("boş seçimde adım olmamalı ve sonuç satırı ℹ️ olmalı:\n%s", empty)
	}
	// Seçime girmeyen codegraph testleri (D6-3, S7): kalan varsa aynı satırda, yoksa kendi satırında sayılır.
	self := runCommitBash(t, dir, env, "commit_self_out=7\ncommit_deadline_ms=$(($(commit_now_ms) + 60000))\n"+
		"commit_selection $'b TestPlanningQuestionsAreAnswered'\ncommit_left=()\ncommit_left_heavy=0\ncommit_selection ''\n")
	for _, want := range []string{
		"G4'e kaldı (1 test: 1 ağır listesinde, 0 test 120 sn'de kesilen 0 pakette; seçime girmeyen 7 codegraph testi): TestPlanningQuestionsAreAnswered\n",
		"G4'e kaldı (seçime girmeyen 7 codegraph testi)\n",
	} {
		if !strings.Contains(self, want) {
			t.Errorf("çıktıda yok: %q\n%s", want, self)
		}
	}
}

// Kural (tools/full_check.sh, --kip=commit; D1 codegraph dilim 6, D6-2): commit kipi tekrar adaylarını aramaz (dups
// --changed commit'siz değişikliği görmüyor, kipin her kaydında aday:0 çıkıyordu); codegraph'ı dil tanımı atıfları
// adımı derler ve değişen dosyaların hafif testleri aynı ikiliyi koşar: kipte codegraph bir kez derlenir.
func TestCommitModeRunsNoDupsAndBuildsCodegraphOnce(t *testing.T) {
	data, err := os.ReadFile(reporoot.Path("tools/full_check.sh"))
	if err != nil {
		t.Fatal(err)
	}
	branch := regexp.MustCompile(`(?ms)^\tcommit\)\n.*?^\t\t;;$`).FindString(strings.ReplaceAll(string(data), "\r\n", "\n"))
	if branch == "" {
		t.Fatal("tools/full_check.sh: commit) kolu bulunamadı")
	}
	if regexp.MustCompile(`(?m)^\s*dups_check\s*$`).MatchString(branch) {
		t.Errorf("commit kipi dups_check koşmamalı:\n%s", branch)
	}
	if spec, light := strings.Index(branch, "\t\tspec_refs_check\n"), strings.Index(branch, "\t\tcommit_light_tests\n"); spec < 0 || light < spec {
		t.Errorf("commit kipinde spec_refs_check commit_light_tests'ten önce koşmalı:\n%s", branch)
	}
	dir, env := fakeGo(t, map[string]string{"tools_edithook.out": "345 item(s), 7956 mention(s), 0 error(s), 0 note(s)\n"})
	built := filepath.ToSlash(filepath.Join(dir, "bin", "codegraph-built"))
	files := map[string]string{
		"bin/codegraph-built": "#!/usr/bin/env bash\nprintf '%s\\n' \"$*\" >>\"$FAKE_GO_DIR/cg_args\"\n" +
			"echo '  not run by these commands: 7 codegraph tests (the full check on Colab runs them)' >&2\n",
		".codegraph/graph.json":     "{}\n",
		".codegraph/last-codegraph": built + "\n",
	}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	out := runCommitBash(t, dir, env, "spec_refs_check\ncommit_light_tests\necho \"SPECREFS=$commit_specrefs\"\n")
	if strings.Contains(out, "!! KALDI") || !strings.Contains(out, "SPECREFS=0,not:0") {
		t.Errorf("iki adım da geçmeliydi:\n%s", out)
	}
	if args := fakeArgs(t, dir); len(args) != 1 || !strings.Contains(args[0], "run ./tools/edithook codegraph spec-refs") {
		t.Errorf("go yalnız bir kez (spec-refs için) koşmalıydı: %q", args)
	}
	cgArgs, err := os.ReadFile(filepath.Join(dir, "fake", "cg_args"))
	if err != nil || strings.TrimSpace(string(cgArgs)) != "changed --go-tests --by-distance --without-codegraph-self-tests --base HEAD HEAD" {
		t.Errorf("hafif testlerin seçimi spec-refs'in derlediği ikiliden alınmalıydı: %q %v\n%s", cgArgs, err, out)
	}
	// Seçime girmeyen codegraph testlerinin sayısı codegraph'ın standart hatasından "G4'e kaldı" satırına gelir (D6-3, S7).
	if want := "G4'e kaldı (seçime girmeyen 7 codegraph testi)"; !strings.Contains(out, want) {
		t.Errorf("çıktıda yok: %q\n%s", want, out)
	}
}

// Kural (tools/full_check.sh, --kip=commit; D1 codegraph dilim 3, DENETİM 681 N3): 120 sn kipin baştan sona süresidir.
// Süre dolmadan gelen FAIL düşüştür, --- FAIL satırı olmasa da (TestMain, os.Exit); paket ancak zamanlayıcı dolduysa
// kesilir: sonucu gelmeyen ya da FAIL'i süre dolduktan sonra gelen (go test, test ikilisi durdurulunca böyle basar)
// paketin bütün seçilmiş testleri G4'e kalır, hiçbiri geçti sayılmaz; süre dolduktan sonra gelen --- FAIL de düşüş
// sayılmaz (durdurma testin alt sürecini öldürür, test onu düşüş diye yazar; codegraph dilim 6, TCB 14:53). Süre bitince sonraki parça başlamaz ve
// durdurulan komut süreç olarak kalmaz (DENETİM 670 N2).
func TestCommitModeCutsAtTheLimitByPackage(t *testing.T) {
	dir, env := fakeGo(t, map[string]string{
		"a.out":   "--- FAIL: TestA (0.00s)\nFAIL\nFAIL\texample.com/fix/a\t0.1s\n",
		"c.out":   "FAIL\texample.com/fix/c\t0.1s\n",
		"b.sleep": "30",
		"d.sleep": "30",
		// Durdurulan paket testin alt sürecini de öldürür; test süre dolduktan sonra --- FAIL basar (codegraph dilim 6,
		// TestTimeCeilingsListsConstantCeilings'in `go list: exit status 143`'ü): bu satır düşüş sayılmaz.
		"d.kill": "--- FAIL: TestD (3.00s)\n    d_test.go:1: go list: exit status 143\nFAIL\nFAIL\texample.com/fix/d\t3.0s\n",
	})
	out := runCommitBash(t, dir, env, "commit_chunk_chars=24\ncommit_deadline_ms=$(($(commit_now_ms) + 3000))\n"+
		"commit_selection $'a TestA\\nc TestC\\nb TestB\\nd TestD\\ne TestE'\n"+
		"echo \"RAN=${commit_ran[*]}\"\necho \"LEFT=${commit_left[*]}\"\n")
	for _, want := range []string{
		"G4'e kaldı (3 test: 0 ağır listesinde, 3 test 120 sn'de kesilen 3 pakette): TestB, TestD, TestE",
		"RAN=./a TestA ./c TestC",
		"LEFT=./b TestB ./d TestD ./e TestE",
		"!! KALDI: değişen dosyaların hafif testleri",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("çıktıda yok: %q\n%s", want, out)
		}
	}
	if args := fakeArgs(t, dir); len(args) != 1 {
		t.Errorf("süre bitince ikinci parça başlamamalı; go %d kez koştu: %q", len(args), args)
	}
	pids, err := os.ReadFile(filepath.Join(dir, "fake", "pids"))
	if err != nil {
		t.Fatal(err)
	}
	if alive := runCommitBash(t, dir, env, "kill -0 "+strings.TrimSpace(string(pids))+" 2>/dev/null && echo SÜRÜYOR\ntrue\n"); strings.Contains(alive, "SÜRÜYOR") {
		t.Errorf("kesilen go komutu hâlâ süreç olarak duruyor (%s)", strings.TrimSpace(string(pids)))
	}
}

// Kural (tools/full_check.sh, --kip=commit): süreyi timeout belirler, duvar saati değil. timeout parçayı durdurunca
// sonraki parça başlamaz ve durdurmadan sonra gelen FAIL kesilmiş sayılır, `date` süreden geri kalsa da. CI'da
// TestCommitModeCutsAtTheLimitByPackage'ta timeout ile `date` birbirini tutmadı: ikinci parça başladı, durdurulan d'nin
// FAIL'i düşüş sayıldı (CI koşuları 43581840682, 43589647100, 43625941738; hangi saatin kaydığı ölçülmedi, [hipotez]). Burada saat hiç ilerlemez: eski
// betik aynı biçimde düşüyordu.
func TestCommitModeTrustsTheTimeoutOverTheClock(t *testing.T) {
	dir, env := fakeGo(t, map[string]string{
		"a.out":   "ok  \texample.com/fix/a\t0.1s\n",
		"c.out":   "ok  \texample.com/fix/c\t0.1s\n",
		"b.sleep": "30",
		"d.sleep": "30",
		"d.kill":  "--- FAIL: TestD (1.00s)\nFAIL\nFAIL\texample.com/fix/d\t1.0s\n",
	})
	// İlk parça a, c, b, d (24 karakterin altı); e ikinci parçadadır.
	out := runCommitBash(t, dir, env, "commit_chunk_chars=24\ncommit_now_ms() { echo 1000; }\ncommit_deadline_ms=2000\n"+
		"commit_selection $'a TestA\\nc TestC\\nb TestB\\nd TestD\\ne TestE'\n"+
		"echo \"RAN=${commit_ran[*]}\"\necho \"LEFT=${commit_left[*]}\"\n")
	for _, want := range []string{"RAN=./a TestA ./c TestC\n", "LEFT=./b TestB ./d TestD ./e TestE\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("çıktıda yok: %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "timeout: ") {
		t.Errorf("timeout'un satırı çıktıya girmemeli:\n%s", out)
	}
	if args := fakeArgs(t, dir); len(args) != 1 {
		t.Errorf("timeout durdurunca ikinci parça başlamamalı; go %d kez koştu: %q", len(args), args)
	}
}

// Kural (tools/full_check.sh, --kip=commit; D1 codegraph dilim 3, DENETİM 681 N2): seçim komut satırı sınırının
// altında parçalara bölünür. Paketin adları sığdığı sürece tek parçada; tek başına sınırı aşan paketin adları birden
// çok parçada; ağır paketler (commit_late_packages) yeni bir parçadan başlayarak sonda; her parçanın komutu ağır listesini
// -skip ile dışarıda tutar. Tek komut büyük commit'te Windows'un sınırını aşıp hiç başlamıyordu (çıkış 126).
func TestCommitModeSplitsTheSelectionUnderTheCommandLineLimit(t *testing.T) {
	ok := func(pkg string) string { return "ok  \texample.com/fix/" + pkg + "\t0.1s\n" }
	dir, env := fakeGo(t, map[string]string{"a.out": ok("a"), "b.out": ok("b"), "x.out": ok("x"), "tools_codegraph.out": ok("tools/codegraph")})
	sel := "a TestA1\na TestA2\nb TestB11111111\nb TestB22222222\nb TestB33333333\nb TestB44444444\ntools/codegraph TestLight1\nx TestX"
	out := runCommitBash(t, dir, env, "commit_chunk_chars=30\nprintf '%s\\n' $'"+strings.ReplaceAll(sel, "\n", "\\n")+"' | commit_plan\n"+
		"commit_deadline_ms=$(($(commit_now_ms) + 60000))\ncommit_selection $'"+strings.ReplaceAll(sel, "\n", "\\n")+"'\necho \"RAN=${#commit_ran[@]}\"\n")
	plan := "1\t./a\tTestA1\n1\t./a\tTestA2\n2\t./b\tTestB11111111\n2\t./b\tTestB22222222\n3\t./b\tTestB33333333\n" +
		"3\t./b\tTestB44444444\n4\t./x\tTestX\n5\t./tools/codegraph\tTestLight1\n"
	if !strings.HasPrefix(out, plan) {
		t.Errorf("parçalar beklenen gibi değil:\n%s\nbeklenen:\n%s", out, plan)
	}
	if !strings.Contains(out, "RAN=8") {
		t.Errorf("sekiz testin hepsi koşulmalıydı:\n%s", out)
	}
	args := fakeArgs(t, dir)
	if len(args) != 5 {
		t.Fatalf("beş parça beklenirdi, go %d kez koştu: %q", len(args), args)
	}
	for i, line := range args {
		if !strings.Contains(line, " -skip ^(") {
			t.Errorf("parça %d: -skip yok: %s", i+1, line)
		}
	}
}

// Kural (tools/full_check.sh, --kip=commit): ağır listesindeki her ad kendi paketinde tanımlıdır; taşınan ya da yeniden
// adlanan ağır test -skip'ten kurtulup commit kipinde koşardı (TestCommitTestsExist'in örneği). Son parçaya konan ağır
// paketler codegraph'ın heavyTestPackages listesiyle aynıdır (iki listeyi ana oturum eşit tutar).
func TestCommitHeavyTestsExist(t *testing.T) {
	data, err := os.ReadFile(reporoot.Path("tools/full_check.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := strings.ReplaceAll(string(data), "\r\n", "\n")
	heavy := shellArray(t, script, "commit_heavy_tests")
	if len(heavy) == 0 {
		t.Fatal("commit_heavy_tests boş")
	}
	for _, entry := range heavy {
		pkg, name, found := strings.Cut(entry, ":")
		if !found {
			t.Errorf("commit_heavy_tests: %q paket:ad biçiminde değil", entry)
			continue
		}
		files, err := reporoot.Glob(filepath.Join(reporoot.Path(strings.TrimPrefix(pkg, "./")), "*_test.go"))
		if err != nil {
			t.Fatal(err)
		}
		defined := false
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			defined = defined || strings.Contains(string(b), "func "+name+"(t *testing.T)")
		}
		if !defined {
			t.Errorf("commit_heavy_tests'teki %s, %s paketinde tanımlı değil", name, pkg)
		}
	}
	src, err := os.ReadFile(reporoot.Path("tools/codegraph/query_run_listed.go"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`var heavyTestPackages = \[\]string\{([^}]*)\}`).FindSubmatch(src)
	if m == nil {
		t.Fatal("tools/codegraph/query_run_listed.go: heavyTestPackages bulunamadı")
	}
	var want []string
	for _, q := range regexp.MustCompile(`"([^"]+)"`).FindAllSubmatch(m[1], -1) {
		want = append(want, "./"+string(q[1]))
	}
	if late := shellArray(t, script, "commit_late_packages"); !slices.Equal(late, want) {
		t.Errorf("commit_late_packages %v, codegraph'ın heavyTestPackages'ı %v", late, want)
	}
}

// Kural (tools/full_check.sh, --kip=commit; D1 codegraph dilim 3, DENETİM 681 Ş1 ve N1): her koşu ortak git klasörüne
// kendi dosyasını yazar (codegraph-kip/<UTC saat>-<ağaç>.tsv, tek satır). Ağaç kipin sınadığı çalışma ağacıdır:
// değişiklik eklenmeden kip koşup sonra eklenip commit edilirse kaydın ağacı commit'inkine eşittir; gerçek index
// değişmez. Satır kimlik taşımaz: mutlak yol, ev klasörü ve modül yolu yok, paketler ./ ile (TCB 01:48).
func TestCommitModeRecordsItsRun(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.txt", "bir\n")
	git("add", "a.txt")
	git("commit", "-q", "-m", "ilk")
	write("a.txt", "iki\n")
	write("b.txt", "yeni\n")
	status := git("status", "--porcelain") // git status index'i tazeler: bayt karşılaştırması ondan sonra alınır
	index := filepath.Join(dir, ".git", "index")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	run := "commit_dups=0\ncommit_specrefs=0,not:0\ncommit_ran=('./a TestA1' './a TestA2')\ncommit_left=('./b TestB')\n" +
		"commit_near=(['./a TestA1']=1 ['./b TestB']=1)\ncommit_record\n"
	runCommitBash(t, dir, os.Environ(), run)
	after, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before, after) {
		t.Error("commit_record gerçek index'i değiştirdi")
	}
	if git("status", "--porcelain") != status {
		t.Error("commit_record çalışma ağacının durumunu değiştirdi")
	}
	git("add", "-A")
	git("commit", "-q", "-m", "iki")
	tree := git("rev-parse", "HEAD^{tree}")
	// Dosya adı saniyelik UTC saat ve ağaçtır; iki koşu aynı ağacı sınadığı için ikincisi bir sonraki saniyeyi bekler.
	runCommitBash(t, dir, os.Environ(), "sleep 1\nfailed=1\ncommit_failed_step='Biçim (gofmt)'\ncommit_record\n")
	recDir := filepath.Join(dir, ".git", "codegraph-kip")
	entries, err := reporoot.ReadDir(recDir) // ada göre sıralı; depo dışında standart kitaplığın ReadDir'i
	if err != nil || len(entries) != 2 {
		t.Fatalf("iki koşu iki dosya yazmalıydı: %v %v", entries, err)
	}
	var files []string
	for _, e := range entries {
		files = append(files, filepath.Join(recDir, e.Name()))
	}
	name := regexp.MustCompile(`^[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}\.tsv$`)
	var lines []string
	for _, f := range files {
		if !name.MatchString(filepath.Base(f)) {
			t.Errorf("dosya adı <UTC saat>-<ağaç>.tsv değil: %s", filepath.Base(f))
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(b), "\n") != 1 {
			t.Errorf("%s tek satır olmalı:\n%s", filepath.Base(f), b)
		}
		lines = append(lines, strings.TrimSuffix(string(b), "\n"))
	}
	first := strings.Split(lines[0], "\t") // dosyalar saate göre sıralı: ilki eklenmemiş değişiklikle koşan kip
	if len(first) != 9 || first[1] != tree {
		t.Fatalf("ilk kaydın ağacı commit'in ağacına (%s) eşit değil:\n%s", tree, strings.Join(lines, "\n"))
	}
	// Sonuç alanı satırın türüdür (geçti, uyarı, bilgi; DENETİM 748 N2), yakın: alanı koşan ve seçilen yakın testlerdir (B3).
	if want := []string{"main", "geçti", "aday:0", "hata:0,not:0", "yakın:1/2", "./a TestA1,./a TestA2", "./b TestB"}; !slices.Equal(first[2:], want) {
		t.Errorf("alanlar %q, beklenen %q", first[2:], want)
	}
	all := strings.Join(lines, "\n")
	if !strings.Contains(all, "\tBiçim (gofmt)\t") {
		t.Errorf("düşen koşunun kaydı düşen adımı yazmalı:\n%s", all)
	}
	for _, leak := range []string{":/", ":\\", "/Users/", "/home/", "github.com/"} {
		if strings.Contains(all, leak) {
			t.Errorf("kayıt yol ya da kimlik taşıyor (%q):\n%s", leak, all)
		}
	}
}

// Kural (tools/full_check.sh, --kip=commit; codegraph 7 B2): kip seçimi codegraph changed --go-tests --by-distance'tan
// alır ve önce bütün yakınları, sonra uzakları parçalar; bir paket iki parçada görünebilir, ağır paketler kendi bandının
// sonunda kalır. Süre dolunca G4'e kalan uzak testlerdir: alfabetik sırada değişen paketin 0,03 sn'lik testi kipin
// 120 sn'sine sığmamıştı (Dilim A, commit 1).
func TestCommitModeRunsNearTestsFirst(t *testing.T) {
	ok := func(pkg string) string { return "ok  \texample.com/fix/" + pkg + "\t0.1s\n" }
	dir, env := fakeGo(t, map[string]string{"a.out": ok("a"), "z.out": ok("z"), "tools_codegraph.out": ok("tools/codegraph")})
	sel := "a TestAFar far\na TestANear near\ntools/codegraph TestCgNear near\nz TestZNear near\ntools/codegraph TestCgFar far"
	out := runCommitBash(t, dir, env, "printf '%s\\n' $'"+strings.ReplaceAll(sel, "\n", "\\n")+"' | commit_plan\n")
	plan := "1\t./a\tTestANear\n1\t./z\tTestZNear\n2\t./tools/codegraph\tTestCgNear\n3\t./a\tTestAFar\n4\t./tools/codegraph\tTestCgFar\n"
	if out != plan {
		t.Errorf("yakınlar önce, ağır paket bandının sonunda olmalı:\n%s\nbeklenen:\n%s", out, plan)
	}
}

// Kural (tools/full_check.sh, --kip=commit; codegraph 7 B3, S6; DENETİM 748 Ş1): sonuç satırı dürüsttür, beş kolu var
// ve her kol metnini ve çıkış kodunu taşır: yakın testlerin hepsi koştu (uzak kalan varsa sayısıyla; 0); bir kısmı
// kaldı (0); düzen paketleri dışında yakın seçilip hiçbiri koşmadı (⚠️, 3); yalnız uzaklar seçildi (ℹ️, 0); hiç seçim yok
// (ℹ️, 0). Kaydın türü aynı ayrımı yazar (geçti, uyarı, bilgi; DENETİM 748 N2). ./internal'ın seçilen testleri düzen
// adımında koşar, koşulmuş sayılır; ⚠️'nün ölçütüne girmezler (DENETİM 840 Ş1): yakınların hepsi oradaysa ✅. Neden: kip 52 kaydın 12'sinde hiçbir test koşmadan "✅ geçti" diyordu (R276).
func TestCommitModeSaysWhenNearTestsAreLeft(t *testing.T) {
	dir, env := fakeGo(t, map[string]string{"a.out": "ok  \texample.com/fix/a\t0.1s\n", "b.out": "ok  \texample.com/fix/b\t0.1s\n",
		"c.sleep": "30"})
	for _, c := range []struct {
		name, sel, line, kind string
		code                  int
	}{
		{"hepsi", "a TestA near\nb TestB near", "✅ Commit öncesi denetim geçti (tam denetim değildir; yakın testlerin hepsi koştu)\n", "geçti", 0},
		{"hepsi, uzak kalan", "a TestA near\ntools/codegraph TestPlanningQuestionsAreAnswered far",
			"✅ Commit öncesi denetim geçti (tam denetim değildir; yakın testlerin hepsi koştu; 1 uzak test G4'e kaldı)\n", "geçti", 0},
		{"internal", "internal TestLayout near", "✅ Commit öncesi denetim geçti (tam denetim değildir; yakın testlerin hepsi koştu)\n", "geçti", 0},
		// T1 4c53d080: koşan yakınların hepsi düzen adımındandı (24/1060), kip ✅ diyordu (DENETİM 840 Ş1).
		{"internal ve kesilen", "internal TestLayout near\nc TestC near",
			"⚠️ Commit öncesi denetim değişen dosyaların düzen adımı dışındaki hiçbir yakın testini koşamadı (0 ağır listesinde, 1 süreye sığmadı; 1 test G4'e kaldı). Commit'ten önce dosyaların tests-for run: satırlarını koş.\n", "uyarı", 3},
		{"bir kısmı", "a TestA near\nc TestC near\nb TestB far",
			"✅ Commit öncesi denetim geçti (tam denetim değildir; 1 yakın test koşmadı: 0 ağır listesinde, 1 süreye sığmadı; 2 test G4'e kaldı)\n", "geçti", 0},
		// Kipin bu daldaki ilk gerçek koşusu: kalan tek yakın test ağır listesindeydi, satır "süreye sığmadı" diyordu.
		{"ağır yakın", "a TestA near\ntools/codegraph TestPlanningQuestionsAreAnswered near",
			"✅ Commit öncesi denetim geçti (tam denetim değildir; 1 yakın test koşmadı: 1 ağır listesinde, 0 süreye sığmadı; 1 test G4'e kaldı)\n", "geçti", 0},
		{"hiçbiri", "c TestC near\nb TestB far",
			"⚠️ Commit öncesi denetim değişen dosyaların düzen adımı dışındaki hiçbir yakın testini koşamadı (0 ağır listesinde, 1 süreye sığmadı; 2 test G4'e kaldı). Commit'ten önce dosyaların tests-for run: satırlarını koş.\n", "uyarı", 3},
		{"yalnız uzak", "a TestA far\nb TestB far",
			"ℹ️ Commit öncesi denetim bitti, geçti değil: değişen dosyalar için yakın test seçilmedi; 2 uzak test koştu, 0 test G4'e kaldı.\n", "bilgi", 0},
		{"seçim yok", "",
			"ℹ️ Commit öncesi denetim bitti, geçti değil: değişen dosyalar için hiç test seçilmedi; düzen kuralları ve hafif testler koştu.\n", "bilgi", 0},
	} {
		src := "commit_deadline_ms=$(($(commit_now_ms) + 2000))\ncommit_selection $'" + strings.ReplaceAll(c.sel, "\n", "\\n") + "' >/dev/null\n" +
			"echo \"KIND=$(commit_kind)\"\ncommit_passed_line\necho \"CODE=$?\"\n"
		out := runCommitBash(t, dir, env, src)
		if want := "KIND=" + c.kind + "\n" + c.line + "CODE=" + strconv.Itoa(c.code) + "\n"; !strings.HasSuffix(out, want) {
			t.Errorf("%s: çıktının sonu beklenen gibi değil:\n%s\nbeklenen sonu:\n%s", c.name, out, want)
		}
	}
}
