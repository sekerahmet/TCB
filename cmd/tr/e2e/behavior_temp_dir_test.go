package e2e

import (
	"path/filepath"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// tempEnv işletim sisteminin geçici klasörünü boş bir klasöre çeviren ortam değişkenleridir.
func tempEnv(t *testing.T) (string, []string) {
	t.Helper()
	temp := t.TempDir()
	return temp, []string{"TMP=" + temp, "TEMP=" + temp, "TMPDIR=" + temp}
}

func tempIsEmpty(t *testing.T, temp, ne string) {
	t.Helper()
	inputs, err := reporoot.ReadDir(temp)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 0 {
		var names []string
		for _, g := range inputs {
			names = append(names, g.Name())
		}
		t.Errorf("%s: geçici klasörde bir şey kalmamalı: %v", ne, names)
	}
}

// Geçici klasör kullanan dil tanımı testleri (18, 19) koşudan sonra geçici klasörde ve
// depoda bir şey bırakmaz (18.14: tr test de program bitince geçici klasörleri siler).
func TestTestCommandLeavesNoTempDir(t *testing.T) {
	for _, file := range []string{"tests/surec_ve_kayit_test.tr", "tests/dosya_ve_yol_test.tr"} {
		temp, env := tempEnv(t)
		s := testkit.RunBinary(t, trBinary, reporoot.Root(), "", env, "test", filepath.FromSlash(file))
		testkit.Wait(t, s, 0, []string{"0 kaldı"}, nil)
		tempIsEmpty(t, temp, file)
	}
	if old, _ := reporoot.GlobAll(reporoot.Path("tests/gecici_*")); len(old) != 0 {
		t.Errorf("testler depoda geçici klasör bırakmamalı: %v", old)
	}
}

// Program nasıl biterse bitsin (başarı, çalışma hatası, komut satırı hatasıyla çıkış)
// geçici_klasör()'ün klasörleri içlerindeki açık satır okuyucusuyla birlikte silinir.
func TestTempDirRemovedWhenProgramEnds(t *testing.T) {
	dir := t.TempDir()
	shared := "TR_var k = TR_temp_dir()\nTR_write_file(TR_path(k, \"a.txt\"), \"1\\n2\")\nTR_var o = TR_read_lines(TR_path(k, \"a.txt\"))\no.TR_next()\nTR_print(k.TR_exists)\n"
	states := []struct {
		ad, last, argument string
		code               int
	}{
		{"başarı", "", "", 0},
		{"çalışma hatası", "TR_print(1 / 0)\n", "", 1},
		{"komut satırı çıkışı", "TR_command_line({TR_step: 1})\n", "--bilinmeyen", 2},
	}
	for _, d := range states {
		temp, env := tempEnv(t)
		testkit.WriteFile(t, dir, "p.tr", shared+d.last)
		args := []string{"p.tr"}
		if d.argument != "" {
			args = append(args, d.argument)
		}
		s := testkit.RunBinary(t, trBinary, dir, "", env, args...)
		testkit.Wait(t, s, d.code, []string{"doğru"}, nil)
		tempIsEmpty(t, temp, d.ad)
	}
}
