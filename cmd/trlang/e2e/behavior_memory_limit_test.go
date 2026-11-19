package e2e

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

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
