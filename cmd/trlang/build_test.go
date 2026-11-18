package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: 'trlang build' paketinin JSON anahtarları dilden bağımsız İngilizce tanımlayıcılardır (dil
// tanımı 17.3).
func TestPackageKeys(t *testing.T) {
	testkit.FormatJSONKeys(t, pkg{})
}

// Kural: paketin imzası dilden bağımsız İngilizce tanımlayıcı TR_PACKAGE, sürümü 4'tür (dil tanımı
// 17.3). Eski biçimler okunmaz (geriye uyumluluk yok): sürüm 1 (TR_EXE_1) ve sürüm 3 (TR_PAKET)
// imzalı ikili paketsiz sayılır, yorumlayıcının kendisidir.
func TestPackageSignature(t *testing.T) {
	coded, err := encodePackage(&pkg{Main: packageFile{Code: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	signature := "TR_PACKAGE"
	if !strings.HasSuffix(string(coded), signature) {
		t.Fatalf("paket %q imzasıyla bitmeli: %q", signature, coded[len(coded)-12:])
	}
	if v := binary.LittleEndian.Uint32(coded[len(coded)-len(signature)-4:]); v != 4 {
		t.Errorf("paket sürümü %d, beklenen 4", v)
	}
	body := []byte(`{"main":{"code":"x"}}`)
	v3 := make([]byte, 20)
	binary.LittleEndian.PutUint64(v3[0:8], uint64(len(body)))
	binary.LittleEndian.PutUint32(v3[8:12], 3)
	copy(v3[12:], "TR_PAKET")
	for name, exe := range map[string][]byte{
		"sürüm 1": []byte("x\n\x02\x00\x00\x00\x00\x00\x00\x00TR_EXE_1"),
		"sürüm 3": append(body, v3...),
	} {
		if _, templateSize, found, err := extractPackage(exe); found || err != nil || templateSize != len(exe) {
			t.Errorf("%s paketi okunmamalı: bulundu=%v boy=%d hata=%v", name, found, templateSize, err)
		}
	}
}

// Kural: gömülü paketteki modül adı hangi biçimde olursa olsun ("..", "ust/..", ".", boş)
// modül, geçici klasörün modül alt klasörüne yazılır; klasörün kökünün ya da dışının yolu
// olamaz (SECURITY.md: yol geçişi ayıklanır). Neden: filepath.Base ".." verir ve Join onu
// alt klasörden çıkarıp kökü hedef yapar; kök yoksa dosya olarak yazılır, varsa ham işletim
// sistemi hatası döner.
func TestWriteEmbeddedModulesKeepsDotNamesInsideTheModuleFolder(t *testing.T) {
	for _, name := range []string{"..", "ust/..", ".", "", "a/b/.."} {
		pk := &pkg{
			Main:    packageFile{Name: "ana.tr", Code: "TR_var a = 1\n"},
			Modules: []packageFile{{Name: name, Code: "TR_var x = 1\n"}},
		}
		for _, dir := range []string{t.TempDir(), filepath.Join(t.TempDir(), "paket")} {
			paths, err := writeEmbeddedModules(pk, dir)
			if err != nil {
				t.Errorf("ad %q: beklenmeyen hata: %v", name, err)
				continue
			}
			for _, y := range paths {
				if rel, err := filepath.Rel(dir, y); err != nil || rel == "." || strings.HasPrefix(rel, "..") || !strings.HasPrefix(rel, "0"+string(filepath.Separator)) {
					t.Errorf("ad %q: modül modül alt klasörünün dışına yazıldı: %q (rel=%q)", name, y, rel)
				}
			}
		}
	}
}

// Kural (11.3, 17.3): iki yazımla (dizin bağlantısı) ulaşılan aynı modül pakete bir kez girer, iki
// aktarım da aynı sırayı bulur; yönlendirilmiş program `trlang run` gibi durumu paylaşır ("1 2").
// Neden: toplayıcı modülü mutlak yolla anahtarlıyordu, pakete iki kopya giriyordu.
func TestBuildEmbedsAModuleReachedTwoWaysOnce(t *testing.T) {
	root := t.TempDir()
	testkit.WritePath(t, filepath.Join(root, "lib", "sayac.tr"), source.Header+"\nTR_var sayi = 0\n\nTR_func artir() -> TR_Int {\n    sayi = sayi + 1\n    TR_return sayi\n}\n")
	if err := os.Symlink(filepath.Join(root, "lib"), filepath.Join(root, "bag")); err != nil {
		t.Skipf("dizin bağlantısı kurulamadı: %v", err)
	}
	code := source.Header + "\nTR_import \"lib/sayac\" TR_as a\nTR_import \"bag/sayac\" TR_as b\nTR_print(a.artir())\nTR_print(b.artir())\n"
	srcPath := testkit.WritePath(t, filepath.Join(root, "ana.tr"), code)
	pk, err := preparePackage(srcPath, code)
	if err != nil {
		t.Fatal(err)
	}
	if len(pk.Modules) != 1 || len(pk.Main.Imports) != 2 || pk.Main.Imports[0] != 0 || pk.Main.Imports[1] != 0 {
		t.Fatalf("pakette bir modül, iki aktarım aynı sırada olmalı: %d modül, sıralar %v", len(pk.Modules), pk.Main.Imports)
	}
	paths, err := writeEmbeddedModules(pk, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	redirected, err := redirectImports(pk.Main, paths)
	if err != nil {
		t.Fatal(err)
	}
	s := source.Prepare(redirected, source.Options{TypeCheck: true})
	if s.Stage != "" {
		t.Fatalf("yönlendirilmiş program: %s %v", s.Stage, s.Errors)
	}
	var out bytes.Buffer
	m := interp.New()
	m.Out, m.Err = &out, &out
	m.SourceFile, m.SourceCode = embeddedSourceName, s.Text
	if _, err := m.Run(s.Program); err != nil || out.String() != "1\n2\n" {
		t.Fatalf("paket aynı modülü paylaşmalı: çıktı %q, hata %v", out.String(), err)
	}
}
