package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Depodaki ölçüler klasörü biçimine uyar: README'nin dört tablosu, beş dosya, her satırın alanları.
func TestMeasurementDocsAreWellFormed(t *testing.T) {
	f, err := Load(reporoot.Root())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range Check(f) {
		t.Error(p)
	}
	if len(f.Machines) == 0 || !f.Sessions["TASINDI"] {
		t.Errorf("README'nin makine ya da oturum tablosu okunamadı: %d makine, %d oturum", len(f.Machines), len(f.Sessions))
	}
	engine := "internal/compute/ internal/lang/ internal/stdlib/ internal/detmath/ internal/sysmem/ cmd/trlang/ go.mod go.sum"
	if got := strings.Join(f.Sets["engine"], " "); got != engine {
		t.Errorf("engine kümesi %q, beklenen %q", got, engine)
	}
	for _, m := range measureFiles {
		if f.Lines[m.path] == nil {
			t.Errorf("%s okunmadı", m.path)
		}
	}
}

// Her biçim yanlışı kendi iletisiyle bulunur; doğru satır (Kaynak'ında https:// adresiyle) temiz geçer.
func TestCheckFindsEachProblem(t *testing.T) {
	moved := map[int]string{cID: "TG-2026-11-10-L4GPU-TASINDI-1", cDate: "2026-11-10 (saat yazılmamış)", cSession: "TASINDI",
		cModel: "qwen.gguf sha256 yazılmamış (taşındı)", cPaths: "engine, — (program bilinmiyor, taşındı)",
		cSource: "taşındı: docs/measurements.md@c061399a:20; günlüğün saat dilimi +0300"}
	movedWith := func(k int, v string) []string {
		c := validCells(moved)
		c[k] = v
		return c
	}
	one := func(k int, v string) [][]string { return [][]string{validCells(map[int]string{k: v})} }
	older := validCells(map[int]string{cID: "TG-2026-11-16-L4GPU-H1-1", cDate: "2026-11-16 09:00 +0300"})
	cases := []struct {
		name  string
		rows  [][]string
		extra string // tablodan sonra bölüme eklenen satır
		want  string
	}{
		{"boş hücre", one(cSettings, ""), "", "boş hücre"},
		{"tek tire", one(cRuns, "—"), "", "boş hücre"},
		{"kimlik biçimi", one(cID, "TG-2026-11-17-L4GPU-H1"), "", "kimlik <kısaltma>"},
		{"kısaltma", one(cID, "STT-2026-11-17-L4GPU-H1-1"), "", "kısaltması STT"},
		{"kimliğin günü", one(cDate, "2026-11-16 10:00 +0300"), "", "kimliğin günü"},
		{"tarih biçimi", one(cDate, "2026-11-17 14:47"), "", "Tarih \"YYYY"},
		{"makine", one(cID, "TG-2026-11-17-LAPTOP-H1-1"), "", "kimliğin makinesi"},
		{"oturum tabloda yok", [][]string{validCells(map[int]string{cID: "TG-2026-11-17-L4GPU-X9-1", cSession: "X9"})}, "", "oturum X9"},
		{"oturum uyuşmuyor", one(cSession, "T1"), "", "Oturum \"T1\""},
		{"aygıt biçimi", one(cDevice, "NVIDIA L4"), "", "Aygıt \"işlemci"},
		{"aygıt türü", one(cDevice, "işlemci, Intel Xeon"), "", "makinenin aygıtı"},
		{"kısa commit", one(cCommit, "abababab"), "", "Commit tam karma"},
		{"programsız TR satırı", one(cPaths, "engine"), "", "en az bir .tr programı"},
		{"engine yok", one(cPaths, "examples/a.tr"), "", "engine olmalı"},
		{"bilinmeyen küme", one(cPaths, "engine, gpu, examples/a.tr"), "", "bilinmeyen küme"},
		{"program yok", one(cPaths, "engine, examples/zz.tr"), "", "program ağaçta yok"},
		{"kümenin yolu yok", one(cPaths, "engine, broken, examples/a.tr"), "", "broken kümesinin yolu ağaçta yok"},
		{"depo dışı biçimi", one(cOutOfRepo, "llama.cpp"), "", "Depo dışı \"<ad>"},
		{"motordan bağımsız", [][]string{validCells(map[int]string{cPaths: engineFree, cOutOfRepo: "— (yok)"})}, "", "Depo dışı'yı doldurur"},
		{"model biçimi", one(cModel, "qwen.gguf"), "", "Model \"<dosya adı>"},
		{"taşınan kaynak", [][]string{movedWith(cSource, "eski tablo")}, "", "taşındı: <dosya>@<commit>:<satır>"},
		{"taşınan Colab", [][]string{movedWith(cSource, "taşındı: docs/measurements.md@c061399a:20")}, "", "saat dilimi"},
		{"yerini alan yok", one(cReplacedBy, "TG-2026-11-18-L4GPU-H1-1"), "", "klasörde yok"},
		{"yerini alan eski", [][]string{validCells(map[int]string{cReplacedBy: older[cID]}), older}, "", "yeni değil"},
		{"yerini alan aynı saatte", [][]string{validCells(map[int]string{cReplacedBy: "TG-2026-11-17-L4GPU-H1-2"}),
			validCells(map[int]string{cID: "TG-2026-11-17-L4GPU-H1-2"})}, "", "yeni değil"},
		{"sıra", [][]string{older, validCells(nil)}, "", "en yeni satır üstte"},
		{"kimlik iki kez", [][]string{validCells(nil), validCells(nil)}, "", "daha önce"},
		{"hücre sayısı", nil, "| a | b |", "hücre olmalı"},
		{"bölümde metin", [][]string{validCells(nil)}, "Not: bu satır tablo değil.", "tablodan başka metin"},
		{"sürücü harfi ve ters bölü", one(cSource, `kayıt D:\veri\x`), "", "yerel yol"},
		{"sürücü harfi ve bölü", one(cSource, "kayıt `D:/veri/x`"), "", "yerel yol"},
		{"Git Bash yolu", one(cSource, "kayıt /d/veri/x"), "", "yerel yol"},
		{"USERPROFILE", one(cSource, `%USERPROFILE%\x`), "", "yerel yol"},
		{"LOCALAPPDATA", one(cSource, `%LOCALAPPDATA%\x`), "", "yerel yol"},
		{"Users ters bölü", one(cSource, `kayıt Users\ad`), "", "yerel yol"},
		{"Users bölü", one(cSource, "kayıt Users/ad"), "", "yerel yol"},
		{"claude klasörü", one(cSource, "kayıt ~/.claude/x"), "", "yerel yol"},
		{"Colab klasörü", one(cSource, "kayıt /content/drive/x"), "", "yerel yol"},
		{"ev klasörü", one(cSource, "kayıt /home/ad/x"), "", "yerel yol"},
		{"scratchpad", one(cSource, "kayıt Scratchpad'de"), "", "yerel yol"},
		{"kişinin makinesi", one(cDevice, "GPU, NVIDIA L4 (Ali'nin makinesi)"), "", "yerel yol ya da kişi"},
		{"kişinin dizüstüsü", one(cSource, "Oya'nın dizüstüsü"), "", "yerel yol ya da kişi"},
		{"kişinin bilgisayarı", one(cSource, "Umut'un bilgisayarı"), "", "yerel yol ya da kişi"},
	}
	clean := newFixture(t, "")
	// Git Bash deseninin olumsuz örnekleri: depo yolunun içindeki /s/ (önünde harf ya da nokta)
	// yerel yol değildir (DENETİM 842 N3).
	repoPath := validCells(map[int]string{cID: "TG-2026-11-17-L4GPU-H1-2", cSource: "H1 devri, tools/x/testdata/s/x ve ./s/x"})
	clean.section("text-generation.md", "Colab L4, GPU", validCells(nil), repoPath, validCells(moved))
	if f, err := Load(clean.dir); err != nil {
		t.Fatal(err)
	} else if ps := Check(f); len(ps) > 0 {
		t.Fatalf("doğru satırlarda yanlış bulundu: %v", ps)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newFixture(t, "")
			fx.section("text-generation.md", "Colab L4, GPU", tc.rows...)
			if tc.extra != "" {
				p := filepath.Join(fx.dir, filepath.FromSlash(folderRel), "text-generation.md")
				data, _ := os.ReadFile(p)
				fx.write(folderRel+"/text-generation.md", string(data)+tc.extra+"\n")
			}
			expectProblem(t, fx.dir, tc.want)
		})
	}
	t.Run("bilinmeyen bölüm", func(t *testing.T) {
		fx := newFixture(t, "")
		fx.section("text-generation.md", "Colab A1", validCells(nil))
		expectProblem(t, fx.dir, "makine tablosunda yok")
	})
	t.Run("beklenmeyen dosya", func(t *testing.T) {
		fx := newFixture(t, "")
		fx.write(folderRel+"/notes.md", "# not\n")
		expectProblem(t, fx.dir, "beklenmeyen dosya")
	})
	t.Run("eksik dosya", func(t *testing.T) {
		fx := newFixture(t, "")
		os.Remove(filepath.Join(fx.dir, filepath.FromSlash(folderRel), "assistant.md"))
		expectProblem(t, fx.dir, "dosya yok")
	})
}

func expectProblem(t *testing.T, dir, want string) {
	t.Helper()
	f, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	ps := Check(f)
	for _, p := range ps {
		if strings.Contains(p.Msg, want) {
			return
		}
	}
	t.Errorf("%q bulunmadı; bulunanlar: %v", want, ps)
}

// Depodaki her .md'nin kimlik sözü klasördeki bir satıra gider; bilinen olumlu: klasörde olmayan kimlik.
func TestMeasurementReferencesExist(t *testing.T) {
	root := reporoot.Root()
	f, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := References(f, "giriş\nbkz. TG-2026-01-01-LAPTOP-H1-1 ve STT-2026-01-01-L4GPU-T1-2"); len(got) != 2 || got[0].Line != 2 {
		t.Fatalf("klasörde olmayan iki kimlik bulunmadı: %v", got)
	}
	files := 0
	err = reporoot.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && p != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		files++
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		for _, b := range References(f, string(data)) {
			t.Errorf("%s:%d: %s klasörde yok", filepath.ToSlash(rel), b.Line, b.Msg)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files < 50 {
		t.Fatalf("yalnız %d .md dosyası okundu; depo yapısı değişmiş olabilir", files)
	}
}
