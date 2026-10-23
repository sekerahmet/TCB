package compute_test

import (
	"go/ast"
	"go/build"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/devtools/goload"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// buildConfigs desteklenen işletim sistemi ve mimari çiftleridir (tam denetimin ARM adımıyla
// aynı küme: amd64 ve arm64, üç işletim sistemi).
var buildConfigs = []struct{ goos, goarch string }{
	{"linux", "amd64"}, {"linux", "arm64"},
	{"windows", "amd64"}, {"windows", "arm64"},
	{"darwin", "amd64"}, {"darwin", "arm64"},
}

// tagSets ek derleme etiketi kümeleridir: etiketsiz, yarış dedektörlü ve GPU'suz derleme.
var tagSets = [][]string{nil, {"race"}, {"gpusuz"}}

// parsed her dosyayı bir kez ayrıştırır (yapılandırmalar aynı dosyaları paylaşır).
var parsed = map[string]*ast.File{}

func parseOnce(path string) (*ast.File, error) {
	if f, ok := parsed[path]; ok {
		return f, nil
	}
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err == nil {
		parsed[path] = f
	}
	return f, err
}

// declNames bir paketin seçilen dosyalarındaki üst düzey adları sayar; ikinci tanım derleme
// hatasıdır (örneğin iki dosyanın yapı kısıtları örtüşürse). stubs gövdesiz işlev sayısıdır.
func declNames(t *testing.T, dir string, files []string) (stubs int) {
	t.Helper()
	names := map[string]string{}
	add := func(key, file string) {
		if prev, dup := names[key]; dup {
			t.Errorf("%s: %q hem %s hem %s içinde tanımlı", dir, key, prev, file)
		}
		names[key] = file
	}
	for _, f := range files {
		file, err := parseOnce(filepath.Join(dir, f))
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				key := d.Name.Name
				if d.Recv != nil && len(d.Recv.List) == 1 {
					key = goload.RecvExprName(d.Recv.List[0].Type) + "." + key
				}
				if d.Name.Name != "init" && d.Name.Name != "_" {
					add(key, f)
				}
				if d.Body == nil {
					stubs++
				}
			case *ast.GenDecl:
				for _, s := range d.Specs {
					switch s := s.(type) {
					case *ast.TypeSpec:
						add(s.Name.Name, f)
					case *ast.ValueSpec:
						for _, n := range s.Names {
							if n.Name != "_" {
								add(n.Name, f)
							}
						}
					}
				}
			}
		}
	}
	return stubs
}

// Her işletim sistemi, mimari ve etiket kümesinde her hesap paketinin dosya seçimi tutarlıdır:
// en az bir dosya kalır, cgo yoktur, bir ad iki dosyada tanımlanmaz ve gövdesiz (assembly)
// işlev ancak assembly dosyası seçildiğinde bulunur. Gerçek derleme: TestCompileEveryConfiguration.
func TestFileSelectionIsConsistentInEveryConfiguration(t *testing.T) {
	for _, dir := range computeDirs(t) {
		for _, cfg := range buildConfigs {
			for _, tags := range tagSets {
				ctx := build.Default
				ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled, ctx.BuildTags = cfg.goos, cfg.goarch, true, tags
				label := cfg.goos + "/" + cfg.goarch + " " + strings.Join(tags, ",")
				p, err := ctx.ImportDir(dir, 0)
				if err != nil {
					t.Errorf("%s (%s): %v", dir, label, err)
					continue
				}
				if len(p.CgoFiles) != 0 {
					t.Errorf("%s (%s): cgo dosyası var %v; dağıtım cgo'suz kalmalı", dir, label, p.CgoFiles)
				}
				stubs := declNames(t, dir, p.GoFiles)
				if stubs > 0 && len(p.SFiles) == 0 {
					t.Errorf("%s (%s): gövdesiz işlev var ama assembly dosyası seçilmedi: bağlama hatası olur", dir, label)
				}
				if stubs == 0 && len(p.SFiles) > 0 {
					t.Errorf("%s (%s): assembly dosyası %v seçildi ama bildirimi yok", dir, label, p.SFiles)
				}
			}
		}
	}
}

// goBuildTags bir kaynak dosyanın başlığındaki //go:build satırının kullandığı etiketleri verir.
func goBuildTags(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			break
		}
		if !constraint.IsGoBuild(line) {
			continue
		}
		expr, err := constraint.Parse(line)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			return nil
		}
		var tags []string
		expr.Eval(func(tag string) bool { tags = append(tags, tag); return false })
		return tags
	}
	return nil
}

// allowedBuildTags derleme etiketi olarak kullanılabilecek adlardır: işletim sistemi, mimari,
// "unix", yarış dedektörü ve GPU'suz derleme. İşlemci yeteneği (avx2, GOAMD64 düzeyi) etiket
// olamaz: yetenek çalışma anında seçilir ve tek program dosyası her işlemcide çalışır.
var allowedBuildTags = func() map[string]bool {
	m := map[string]bool{"unix": true, "race": true, "gpusuz": true, "ignore": true}
	for _, w := range strings.Fields("linux windows darwin freebsd openbsd netbsd solaris plan9 js wasip1 aix dragonfly illumos ios android " +
		"amd64 arm64 386 arm riscv64 ppc64le s390x wasm mips mipsle mips64 mips64le loong64 ppc64") {
		m[w] = true
	}
	return m
}()

// Hız derleme ayarına bağlanmaz: hiçbir kaynak ya da assembly dosyası işlemci yeteneği etiketi
// taşımaz (yetenek çalışma anında seçilir; bkz. tensor/cpu_features_amd64.go).
func TestBuildTagsNeverSelectCPUFeatures(t *testing.T) {
	seen := 0
	for _, dir := range computeDirs(t) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			n := e.Name()
			if !strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, ".s") {
				continue
			}
			seen++
			for _, tag := range goBuildTags(t, filepath.Join(dir, n)) {
				if !allowedBuildTags[tag] {
					t.Errorf("%s: derleme etiketi %q izinli değil (izinliler: işletim sistemi, mimari, unix, race, gpusuz); işlemci yeteneği çalışma anında seçilir",
						filepath.Join(dir, n), tag)
				}
			}
		}
	}
	if seen < 100 {
		t.Fatalf("yalnızca %d kaynak dosyası tarandı", seen)
	}
}

// Her desteklenen yapılandırma gerçekten derlenir ve go vet'ten geçer (test dosyaları dahil);
// assembly çekirdeğin Go karşılığı olmayan mimaride eksik ad derleme hatası verir. Yavaştır
// (altı yapılandırma ve GPU'suz derleme): -short ile atlanır; tam denetim ARM ve GPU'suz
// adımlarını ayrıca koşar.
func TestCompileEveryConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("derleme matrisi uzun test; -short ile atlanır")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go komutu bulunamadı")
	}
	type run struct{ goos, goarch, tags string }
	var runs []run
	for _, c := range buildConfigs {
		runs = append(runs, run{c.goos, c.goarch, ""})
	}
	runs = append(runs, run{"linux", "amd64", "gpusuz"})
	for _, r := range runs {
		args := []string{"vet"}
		if r.tags != "" {
			args = append(args, "-tags", r.tags)
		}
		args = append(args, "./internal/compute/...")
		cmd := exec.Command(goBin, args...)
		cmd.Dir = reporoot.Root()
		cmd.Env = append(os.Environ(), "GOOS="+r.goos, "GOARCH="+r.goarch, "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("%s/%s %s: go vet başarısız: %v\n%s", r.goos, r.goarch, r.tags, err, out)
		}
	}
}
