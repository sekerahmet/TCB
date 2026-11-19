package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Sürecin bellek tavanı, TR_MEMORY_LIMIT (19.12a): geçersiz değer programı başlatmaz, birimli değerin
// önerisi kopyalanınca çalışır, Go çalışma zamanının değişkenleri tek satırla söylenir, dış program
// aynı tavanı alır.

// trEnv yorumlayıcıyı ortama env eklenmiş olarak çalıştırır.
func trEnv(t *testing.T, dir string, env []string, args ...string) result {
	t.Helper()
	return testkit.RunBinary(t, trBinary, dir, "", env, args...)
}

// limitProgram sürecin .TR_memory_limit_bytes'ını yazar.
const limitProgram = "TR_print(TR_machine().TR_memory_limit_bytes)\n"

// machineLimit programın yazdığı sınırdır; sayı değilse (TR_none) sınır yoktur.
func machineLimit(t *testing.T, out string) (int64, bool) {
	t.Helper()
	n, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	return n, err == nil
}

// expectedLimit tavanla işletim sisteminin sınırının küçüğüdür: osLimit tavansız koşunun çıktısıdır.
func expectedLimit(t *testing.T, osLimit string, memCap int64) int64 {
	t.Helper()
	if n, ok := machineLimit(t, osLimit); ok {
		return min(n, memCap)
	}
	return memCap
}

// Kural (19.12a): geçersiz değer kullanım hatasıdır: çıkış 2, program başlamaz, ileti değeri ve
// geçerli yazımı söyler.
func TestInvalidMemoryLimitStopsTheProgram(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "program.tr", "TR_write_file(\"calisti.txt\", \"x\")\n")
	for value, want := range map[string]string{
		"abc":                 `TR_MEMORY_LIMIT="abc" geçersiz`,
		"0":                   `TR_MEMORY_LIMIT="0" geçersiz`,
		"-5":                  `TR_MEMORY_LIMIT="-5" geçersiz`,
		"1.5":                 `TR_MEMORY_LIMIT="1.5" geçersiz`,
		"9223372036854775808": `TR_MEMORY_LIMIT="9223372036854775808" çok büyük`,
		"6GiB":                `TR_MEMORY_LIMIT="6GiB" birim almaz`,
		"6GB":                 `TR_MEMORY_LIMIT="6GB" birim almaz`,
	} {
		r := trEnv(t, dir, []string{"TR_MEMORY_LIMIT=" + value}, "program.tr")
		if r.Code != 2 || !strings.Contains(r.Stderr, want) || r.Stdout != "" {
			t.Errorf("%q: çıkış %d, standart hata %q; çıkış 2 ve %q bekleniyordu", value, r.Code, r.Stderr, want)
		}
		if _, err := os.Stat(filepath.Join(dir, "calisti.txt")); err == nil {
			t.Fatalf("%q: program başladı", value)
		}
	}
}

// suggestedLimit iletinin önerdiği yazımlardır (TR_MEMORY_LIMIT=<bayt>); tırnaklı değer eşleşmez.
var suggestedLimit = regexp.MustCompile(`TR_MEMORY_LIMIT=([0-9]+)`)

// Kural (19.12a): birimli değerin iletisi bayt yazımını önerir; önerilen her yazım kopyalanınca
// program başlar ve tavan odur.
func TestMemoryLimitWithAUnitSuggestsBytes(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "program.tr", limitProgram)
	osLimit := tr(t, dir, "program.tr").Stdout
	for value, want := range map[string][]string{
		"6GiB":    {"6442450944"},
		"1,5 gib": {"1610612736"},
		"6GB":     {"6000000000", "6442450944"},
	} {
		r := trEnv(t, dir, []string{"TR_MEMORY_LIMIT=" + value}, "program.tr")
		var got []string
		for _, m := range suggestedLimit.FindAllStringSubmatch(r.Stderr, -1) {
			got = append(got, m[1])
		}
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%q: öneriler %v, %v bekleniyordu (%s)", value, got, want, r.Stderr)
			continue
		}
		for _, s := range got {
			n, _ := strconv.ParseInt(s, 10, 64)
			r := trEnv(t, dir, []string{"TR_MEMORY_LIMIT=" + s}, "program.tr")
			if lim, ok := machineLimit(t, r.Stdout); r.Code != 0 || !ok || lim != expectedLimit(t, osLimit, n) {
				t.Errorf("öneri TR_MEMORY_LIMIT=%s: çıkış %d, çıktı %q; sınır %d bekleniyordu (%s)", s, r.Code, r.Stdout, expectedLimit(t, osLimit, n), r.Stderr)
			}
		}
	}
}

// Kural (19.12a): Go çalışma zamanının değişkenleri dilin ayarı değildir: ortamda olanlar süreç başında
// tek satırla söylenir. GOGC ya da GOMEMLIMIT varsa satır tavanı önerir (H8893), yalnız öbürleri varsa
// önermez (H8895); program yine çalışır.
func TestRuntimeVariablesWarnOnce(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "program.tr", "TR_print(\"tamam\")\n")
	values := map[string]string{"GOGC": "100", "GOMEMLIMIT": "8GiB", "GODEBUG": "gctrace=0", "GOMAXPROCS": "2",
		"GORACE": "halt_on_error=1", "GOTRACEBACK": "single"}
	order := []string{"GOGC", "GOMEMLIMIT", "GODEBUG", "GOMAXPROCS", "GORACE", "GOTRACEBACK"}
	cases := [][]string{{"GODEBUG", "GOMAXPROCS"}, order}
	for _, name := range order {
		cases = append(cases, []string{name})
	}
	const suggestion = "TR_MEMORY_LIMIT=6442450944"
	for _, names := range cases {
		var env []string
		memory := false
		for _, n := range names {
			env = append(env, n+"="+values[n])
			memory = memory || n == "GOGC" || n == "GOMEMLIMIT"
		}
		r := trEnv(t, dir, env, "program.tr")
		lines := strings.Split(strings.TrimRight(r.Stderr, "\n"), "\n")
		line := lines[0]
		word := "ayarı"
		if len(names) > 1 {
			word = "ayarları"
		}
		switch {
		case r.Code != 0 || strings.TrimSpace(r.Stdout) != "tamam":
			t.Errorf("%v: çıkış %d, çıktı %q (%s)", names, r.Code, r.Stdout, r.Stderr)
		case len(lines) != 1 || !strings.HasPrefix(line, "⚠️ "+strings.Join(names, ", ")+": "):
			t.Errorf("%v: tek uyarı satırı bekleniyordu: %q", names, r.Stderr)
		case !strings.Contains(line, "çalışma zamanının "+word):
			t.Errorf("%v: %q sözcüğü yok: %q", names, word, line)
		case memory != strings.Contains(line, suggestion):
			t.Errorf("%v: tavan önerisi %v bekleniyordu: %q", names, memory, line)
		}
	}
}

// Kural (19.12a, 21.6): dış program ortamı alır; trlang olan çocuk aynı tavanı kendisi için kurar.
func TestChildProcessTakesTheSameMemoryLimit(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "program.tr", limitProgram)
	osLimit := tr(t, dir, "program.tr").Stdout
	writeTR(t, dir, "child.tr", "TR_write_file(\"cocuk.txt\", TR_String(TR_machine().TR_memory_limit_bytes))\n")
	writeTR(t, dir, "parent.tr", callOf()+"\n"+limitProgram)
	const memCap = 6442450944
	r := trEnv(t, dir, []string{"TR_MEMORY_LIMIT=" + strconv.Itoa(memCap)}, "parent.tr")
	child, err := os.ReadFile(filepath.Join(dir, "cocuk.txt"))
	if r.Code != 0 || err != nil {
		t.Fatalf("çıkış %d, çocuğun dosyası %v (%s)", r.Code, err, r.Stderr)
	}
	want := expectedLimit(t, osLimit, memCap)
	parentLimit, ok1 := machineLimit(t, r.Stdout)
	childLimit, ok2 := machineLimit(t, string(child))
	if !ok1 || !ok2 || parentLimit != want || childLimit != want {
		t.Errorf("başlatan %q, çocuk %q; ikisi de %d bekleniyordu", r.Stdout, child, want)
	}
}

// collectorWarning H8894'ün başıdır (sınama süreci Türkçedir).
const collectorWarning = "⚠️ Çöp toplayıcının işlemci payı son 10 saniyede "

// runLong trlang'ı ortama env eklenmiş olarak, testkit.RunBinary'nin iki dakikasından uzun bir sınırla
// çalıştırır; çalışma zamanının değişkenleri ortamdan çıkarılır.
func runLong(t *testing.T, dir string, limit time.Duration, env []string, args ...string) (stdout, stderr string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, trBinary, args...)
	cmd.Dir = dir
	cmd.Env = append(testkit.WithoutRuntimeVariables(os.Environ()), env...)
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("trlang %v: %v\n%s", args, err, errOut.String())
	}
	return out.String(), errOut.String()
}

// Kural (19.12a, "Süreç tavana yaklaştıkça çöp toplayıcı sıklaşır"): canlı öbeğin altında kalan tavanla
// 14 saniye ayıran program toplayıcının payı uyarısını bir kez alır ve sonuna kadar çalışır; uyarı
// tavanı kopyalanınca çalışan biçimde yazar.
func TestTightMemoryLimitWarnsAboutTheCollector(t *testing.T) {
	if testing.Short() {
		t.Skip("uzun: program 14 sn çalışır")
	}
	t.Parallel()
	dir := t.TempDir()
	writeTR(t, dir, "program.tr", `TR_var tut = TR_zeros([5000000])
TR_var baş = TR_monotonic_ns()
TR_for i TR_in 0..1000000000 {
    TR_if TR_monotonic_ns() - baş > 14000000000 {
        TR_break
    }
    TR_var g = TR_zeros([100000])
}
TR_print(tut.TR_shape)
`)
	const memCap = "8000000"
	stdout, stderr := runLong(t, dir, time.Minute, []string{"TR_MEMORY_LIMIT=" + memCap}, "program.tr")
	if strings.TrimSpace(stdout) != "[5000000]" {
		t.Errorf("program sonuna kadar çalışmadı: %q", stdout)
	}
	var lines []string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.HasPrefix(line, collectorWarning) {
			lines = append(lines, line)
		}
	}
	if len(lines) != 1 || !strings.Contains(lines[0], "(TR_MEMORY_LIMIT="+memCap+";") {
		t.Errorf("tek toplayıcı uyarısı bekleniyordu, tavanıyla:\n%s", stderr)
	}
}

// gcTraceStart gctrace satırının toplama anıdır: "gc 12 @121.004s".
var gcTraceStart = regexp.MustCompile(`(?m)^gc \d+ @([0-9.]+)s`)

// Kural (19.12a; tasarımın C "Uyarı"sı): bekleyen süreçte Go iki dakikada bir toplama zorlar; işlemcinin az
// kullanıldığı pencerede bu toplamanın payı büyüse de bellek tavandan uzaktır, uyarı çıkmaz. Ön koşul:
// gctrace'e göre 120. saniyeden sonra bir toplama olmuştur.
func TestIdleProgramWithAMemoryLimitDoesNotWarn(t *testing.T) {
	if testing.Short() {
		t.Skip("uzun: program 130 sn bekler")
	}
	t.Parallel()
	dir := t.TempDir()
	writeTR(t, dir, "program.tr", `TR_var liste = []
TR_for i TR_in 0..1000000 {
    liste.TR_append([i])
}
TR_sleep(130)
TR_print("bitti")
`)
	stdout, stderr := runLong(t, dir, 4*time.Minute,
		[]string{"TR_MEMORY_LIMIT=4294967296", "GODEBUG=gctrace=1"}, "program.tr")
	if strings.TrimSpace(stdout) != "bitti" {
		t.Errorf("program bitmedi: %q", stdout)
	}
	forced := false
	for _, m := range gcTraceStart.FindAllStringSubmatch(stderr, -1) {
		if at, err := strconv.ParseFloat(m[1], 64); err == nil && at >= 120 {
			forced = true
		}
	}
	if !forced {
		t.Fatalf("ön koşul: 120. saniyeden sonra toplama yok, test bekleyen süreci sınamadı:\n%s", stderr)
	}
	if strings.Contains(stderr, collectorWarning) {
		t.Errorf("bekleyen süreçte toplayıcı uyarısı çıktı:\n%s", stderr)
	}
}
