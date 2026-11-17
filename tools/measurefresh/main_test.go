package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureReadme sınama deposunun ölçüler README'sidir; %s depo dışı sürüm tablosunun satırlarıdır.
const fixtureReadme = "# Ölçüler\n\n## Yol kümeleri\n\n| ad | yollar |\n| :-- | :-- |\n" +
	"| engine | `internal/compute/`, `internal/lang/`, `internal/stdlib/`, `internal/detmath/`, `internal/sysmem/`, `cmd/trlang/`, `go.mod`, `go.sum` |\n" +
	"| broken | `nowhere/` |\n\n" +
	"## Makineler\n\n| kod | başlık | aygıt |\n| :-- | :-- | :-- |\n| L4GPU | Colab L4, GPU | GPU |\n| LAPTOP | Dizüstü i7-13700H | işlemci |\n\n" +
	"## Oturumlar\n\n| kod | oturum |\n| :-- | :-- |\n| H1 | H1 |\n| T1 | T1 |\n| TASINDI | taşınan satır |\n\n" +
	"## Güncel depo dışı sürümler\n\n| ad | makine | sürüm |\n| :-- | :-- | :-- |\n%s"

// fixture sınama için kurulan küçük bir git deposudur: motorun kümesi, iki program ve kitaplık.
type fixture struct {
	t   *testing.T
	dir string
}

func newFixture(t *testing.T, versions string) *fixture {
	t.Helper()
	fx := &fixture{t, t.TempDir()}
	files := map[string]string{
		"go.mod":                "module example.com/x\n\ngo 1.27.0\n\ntoolchain go1.27.4\n",
		"go.sum":                "",
		"internal/compute/a.go": "package compute\n",
		"internal/lang/a.go":    "package lang\n",
		"internal/stdlib/a.go":  "package stdlib\n",
		"internal/detmath/a.go": "package detmath\n",
		"internal/sysmem/a.go":  "package sysmem\n",
		"cmd/trlang/a.go":       "package main\n",
		"lib/layers.tr":         "# TR 4\nTR_import \"base\"\n",
		"lib/base.tr":           "# TR 4\n",
		"lib/other.tr":          "# TR 4\n",
		"examples/a.tr":         "# TR 4\nTR_import \"tr/layers\"\n  TR_import \"util/geo\" TR_as g\n# TR_import \"yorum\"\n",
		"examples/util/geo.tr":  "# TR 4\n",
		"examples/b.tr":         "# TR 4\n",
		"docs/other.md":         "başka belge\n",
		readmePath:              fmt.Sprintf(fixtureReadme, versions),
	}
	for _, m := range measureFiles {
		files[m.path] = "# " + m.prefix + "\n"
	}
	for rel, text := range files {
		fx.write(rel, text)
	}
	return fx
}

func (fx *fixture) write(rel, text string) {
	fx.t.Helper()
	p := filepath.Join(fx.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		fx.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		fx.t.Fatal(err)
	}
}

func (fx *fixture) git(args ...string) string {
	fx.t.Helper()
	base := []string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}
	cmd := exec.Command("git", append(base, args...)...)
	cmd.Dir = fx.dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		fx.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commit ağacın bütününü commit'ler ve karmasını verir; ilk çağrı depoyu kurar.
func (fx *fixture) commit(msg string) string {
	fx.t.Helper()
	if _, err := os.Stat(filepath.Join(fx.dir, ".git")); err != nil {
		fx.git("init", "-q")
	}
	fx.git("add", "-A")
	fx.git("commit", "-q", "-m", msg)
	return fx.git("rev-parse", "HEAD")
}

// publish HEAD'i origin/main yapar.
func (fx *fixture) publish() { fx.git("update-ref", "refs/remotes/origin/main", "HEAD") }

// section bir ölçü dosyasına tek bölüm ve satırlarını yazar.
func (fx *fixture) section(file, title string, rows ...[]string) {
	fx.t.Helper()
	var b strings.Builder
	b.WriteString("# Sınama\n\nGiriş paragrafı.\n\n## " + title + "\n\n" + rowHeader + "\n|" + strings.Repeat(" :-- |", cellCount) + "\n")
	for _, r := range rows {
		b.WriteString(rowLine(r) + "\n")
	}
	fx.write(folderRel+"/"+file, b.String())
}

func (fx *fixture) run(args ...string) (string, int) {
	fx.t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, fx.dir, &out, &errOut)
	if errOut.Len() > 0 {
		fx.t.Logf("stderr: %s", errOut.String())
	}
	return out.String(), code
}

// validCells biçimi doğru bir satırdır (L4 GPU, H1); changes hücreleri değiştirir.
func validCells(changes map[int]string) []string {
	c := []string{
		"TG-2026-11-17-L4GPU-H1-1", "2026-11-17 14:47 +0300", "H1", "GPU, NVIDIA L4",
		strings.Repeat("ab", 20), "engine, examples/a.tr", "llama.cpp b12124; Go go1.27.4",
		"32 adım", "qwen.gguf 0123456789ab", "4 koşu, ortanca, aralık %3", "10 token/sn",
		"H1 devri, https://example.com/kayit", "—",
	}
	for i, v := range changes {
		c[i] = v
	}
	return c
}

func rowLine(c []string) string { return "| " + strings.Join(c, " | ") + " |" }

// lines komutun çıktısının ilk satırdan (kıyas satırı) sonraki satırlarıdır.
func lines(t *testing.T, out string) []string {
	t.Helper()
	all := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if !strings.HasPrefix(all[0], "origin/main ") || !strings.HasSuffix(all[0], " ile kıyaslandı") {
		t.Fatalf("ilk satır kıyas satırı değil: %q", all[0])
	}
	return all[1:]
}

func expectLines(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("çıktı:\n%s\nbeklenen:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestRunArguments(t *testing.T) {
	fx := newFixture(t, "")
	fx.section("text-generation.md", "Colab L4, GPU", validCells(map[int]string{cCommit: strings.Repeat("0", 40)}))
	fx.commit("taban")
	fx.publish()
	for _, args := range [][]string{{"--refs"}, {"--bilinmeyen"}, {"--refs", "a", "--refs", "b"}} {
		if _, code := fx.run(args...); code != 2 {
			t.Errorf("%v: çıkış %d, beklenen 2 (kullanım)", args, code)
		}
	}
	out, code := fx.run("TG-2026-01-01-L4GPU-H1-1")
	if code != 1 {
		t.Errorf("bulunamayan kimlik: çıkış %d, beklenen 1", code)
	}
	expectLines(t, lines(t, out), "TG-2026-01-01-L4GPU-H1-1: klasörde yok")
	refs := filepath.Join(fx.dir, "notlar.md")
	fx.write("notlar.md", "TG-2026-11-17-L4GPU-H1-1 güncel mi\nbkz. TG-2026-01-01-L4GPU-H1-1\n")
	out, code = fx.run("--refs", refs)
	if code != 1 {
		t.Errorf("kırık gönderme: çıkış %d, beklenen 1", code)
	}
	want := refs + ":2: TG-2026-01-01-L4GPU-H1-1 klasörde yok"
	if !strings.HasPrefix(out, want+"\n") {
		t.Errorf("kırık gönderme satırı yok:\n%s", out)
	}
	expectLines(t, lines(t, strings.TrimPrefix(out, want+"\n")),
		"TG-2026-11-17-L4GPU-H1-1: eski: commit bulunamadı ("+strings.Repeat("0", 40)+"), 2026-11-17, Colab L4, GPU")
}
