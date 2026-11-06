package process_test

import (
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// helperEnv dolu olan test ikilisi test koşmaz, helperMain'in dış programı olur: testler sh ve cmd
// varsaymaz, her işletim sisteminde aynı programı çalıştırır (21.1).
const helperEnv = "TR_PROCESS_HELPER"

func TestMain(m *testing.M) {
	if os.Getenv(helperEnv) != "" {
		helperMain(os.Args[1:])
		return
	}
	os.Exit(m.Run())
}

// helperMain ilk argümanın söylediğini yapar.
func helperMain(args []string) {
	switch args[0] {
	case "args": // argümanları JSON olarak yazar
		_ = json.NewEncoder(os.Stdout).Encode(args[1:])
	case "bytes": // bütün baytları, sonra CR LF yazar
		b := make([]byte, 256)
		for i := range b {
			b[i] = byte(i)
		}
		_, _ = os.Stdout.Write(append(b, '\r', '\n'))
	case "cat": // girdiyi çıktıya kopyalar
		_, _ = io.Copy(os.Stdout, os.Stdin)
	case "sleep": // verilen saniye kadar uyur
		n, _ := strconv.ParseFloat(args[1], 64)
		time.Sleep(time.Duration(n * float64(time.Second)))
	default:
		helperExtra(args)
	}
}

// helperExtra öbür test dosyalarının programlarıdır.
func helperExtra(args []string) { panic("bilinmeyen yardımcı: " + args[0]) }

// helperProgram test ikilisini programın adı olarak TR metnine yazar ve onu dış program yapar.
func helperProgram(t *testing.T) string {
	t.Helper()
	t.Setenv(helperEnv, "1")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return "TR_Path(" + strconv.Quote(exe) + ")"
}

// Çıktı bayt bayt gelir: satır sonu ve geçersiz UTF-8 değişmez (21.1, 21.7).
func TestRunProcessOutputIsBytes(t *testing.T) {
	got, _, err := commandLine(t, `TR_var b = TR_run_process(`+helperProgram(t)+`, ["bytes"])
[b.TR_length, b[0], b[255], b[256], b[257]]`)
	if want := "[258, 0, 255, 13, 10]"; err != nil || got != want {
		t.Errorf("%s (%v), beklenen %s", got, err, want)
	}
	_, _, err = commandLine(t, `TR_String(TR_run_process(`+helperProgram(t)+`, ["bytes"]))`)
	if err == nil {
		t.Error("UTF-8 olmayan çıktı TR_String'de hata vermeli (21.7)")
	}
}

// Her öğe tek argüman olarak, değiştirilmeden gider; kabuk karakterleri sıradandır (21.1, 21.8).
func TestRunProcessArgumentsAreUnchanged(t *testing.T) {
	args := []string{"a b", "|", ">", "*", "$HOME", "~", "%PATH%", `"tırnak"`, `ters\`, `\"`, "", "-x", "çğü", "a\nb", "^&"}
	list := make([]string, len(args))
	for i, a := range args {
		list[i] = strconv.Quote(a)
	}
	got, _, err := commandLine(t, `TR_String(TR_run_process(`+helperProgram(t)+`, ["args", `+strings.Join(list, ", ")+`]))`)
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	if err := json.Unmarshal([]byte(got), &seen); err != nil {
		t.Fatalf("%s: %v", got, err)
	}
	if strings.Join(seen, "\x00") != strings.Join(args, "\x00") {
		t.Errorf("gelen %q, verilen %q", seen, args)
	}
}

// TR_input: programın girdisidir; verilmezse girdi boş aygıttır, okuyan program hemen biter (21.1).
func TestRunProcessInput(t *testing.T) {
	prog := helperProgram(t)
	got, _, err := commandLine(t, `TR_run_process(`+prog+`, ["cat"], TR_input: TR_Bytes([1, 0, 255])).TR_length`)
	if err != nil || got != "3" {
		t.Errorf("girdi: %s (%v)", got, err)
	}
	got, _, err = commandLine(t, `TR_run_process(`+prog+`, ["cat"]).TR_length`)
	if err != nil || got != "0" {
		t.Errorf("boş aygıt: %s (%v)", got, err)
	}
	got, _, err = commandLine(t, `TR_run_process(`+prog+`, ["cat"], TR_input: TR_Bytes([])).TR_length`)
	if err != nil || got != "0" {
		t.Errorf("boş girdi: %s (%v)", got, err)
	}
	if _, _, err := commandLine(t, `TR_run_process(`+prog+`, ["cat"], TR_input: "metin")`); !messages.Has(err, messages.H7493) {
		t.Errorf("metin girdi: %v", err)
	}
}

// Listenin öğeleri çalışma anında da denetlenir: tipi bilinmeyen listede yol H7490, başka tip H7491,
// boş bayt H7492. Program başlamaz (21.1).
func TestRunProcessArgumentListErrors(t *testing.T) {
	prog := helperProgram(t)
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{`TR_var y = TR_Path("a.txt")
TR_run_process(` + prog + `, ["-i", y])`, messages.H7490},
		{`TR_var l: TR_List[TR_Any] = ["a", 1]
TR_run_process(` + prog + `, l)`, messages.H7491},
		{`TR_run_process(` + prog + `, ["a\u{0}b"])`, messages.H7492},
	} {
		if _, _, err := commandLine(t, d.code); !messages.Has(err, d.want) {
			t.Errorf("%s: %s bekleniyordu: %v", d.code, d.want, err)
		}
	}
}

// Önerilen TR_String(yol) yazımı çalışır: yol argümanı programa metin olarak gider.
func TestRunProcessPathSuggestionRuns(t *testing.T) {
	got, _, err := commandLine(t, `TR_String(TR_run_process(`+helperProgram(t)+`, ["args", TR_String(TR_Path("a.txt"))]))`)
	if err != nil || !strings.Contains(got, "a.txt") {
		t.Errorf("%s (%v)", got, err)
	}
}

// Yanlış çağrılar program başlamadan hatadır.
func TestRunProcessCallErrors(t *testing.T) {
	for _, code := range []string{`TR_run_process("a")`, `TR_run_process(1, [])`, `TR_run_process("a", "b")`, `TR_run_process("a", [], TR_dim: 1)`} {
		if _, _, err := commandLine(t, code); err == nil {
			t.Errorf("%s: hata bekleniyordu", code)
		}
	}
}

// Bekleme kilidi bırakır: program uyurken öbür görevler ilerler (21.1, 10.3).
func TestRunProcessReleasesTheLock(t *testing.T) {
	got, _, err := commandLine(t, `TR_var n = 0
TR_var son = -1
TR_var g = TR_async {
    TR_run_process(`+helperProgram(t)+`, ["sleep", "0.4"])
    son = n
}
TR_for i TR_in 0..10 {
    TR_sleep(0.01)
    n += 1
}
g.TR_wait()
son`)
	if err != nil || got != "10" {
		t.Errorf("%s (%v), program beklerken öbür görev ilerlemeliydi", got, err)
	}
}
