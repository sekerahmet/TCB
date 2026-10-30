package codeindex

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// copyFixture deneme projesini geçici bir klasöre kopyalar (önbellek yazılabilsin).
func copyFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	testkit.CopyDir(t, fixtureDir, dir)
	return dir
}

func mustJSON(t *testing.T, idx *Index) string {
	t.Helper()
	data, err := EncodeIndex(idx)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCacheHitSkipsParsing(t *testing.T) {
	dir := copyFixture(t)
	first, st1, err := BuildCached(dir, Options{})
	if err != nil || !st1.Full || st1.Parsed != st1.Files {
		t.Fatalf("first build: %+v %v", st1, err)
	}
	second, st2, err := BuildCached(dir, Options{})
	if err != nil || !st2.FromCache || st2.Parsed != 0 || st2.Full {
		t.Fatalf("second build must come from the cache: %+v %v", st2, err)
	}
	if mustJSON(t, first) != mustJSON(t, second) {
		t.Error("the cached index differs from the built one")
	}
	if _, err := os.Stat(filepath.Join(dir, CacheDir, indexName)); err != nil {
		t.Errorf("the cache is on disk: %v", err)
	}
}

func TestCacheNotUsedForOtherVersionOrOptions(t *testing.T) {
	dir := copyFixture(t)
	if _, _, err := BuildCached(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	// Başka seçenekle (Dirs) kurulmuş önbellek kullanılmaz.
	_, st, err := BuildCached(dir, Options{Dirs: []string{"lib"}})
	if err != nil || !st.Full {
		t.Errorf("another Dirs must rebuild: %+v %v", st, err)
	}
	// Sürümü değişmiş manifest kullanılmaz.
	manPath := filepath.Join(dir, CacheDir, manifestName)
	data, _ := os.ReadFile(manPath)
	if err := os.WriteFile(manPath, []byte(strings.Replace(string(data), fmt.Sprintf(`"index_version":%d`, IndexVersion), `"index_version":999`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, st, err := BuildCached(dir, Options{Dirs: []string{"lib"}}); err != nil || !st.Full {
		t.Errorf("another IndexVersion must rebuild: %+v %v", st, err)
	}
	// Bozuk önbellek sessizce yok sayılır.
	if err := os.WriteFile(manPath, []byte("{bozuk"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, st, err := BuildCached(dir, Options{Dirs: []string{"lib"}}); err != nil || !st.Full {
		t.Errorf("a corrupt cache is ignored: %+v %v", st, err)
	}
}

func TestIncrementalReparsesOnlyChangedAndImporters(t *testing.T) {
	dir := copyFixture(t)
	if _, _, err := BuildCached(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	// ana.tr'yi kimse aktarmıyor: yalnızca kendisi yeniden ayrıştırılır.
	p := filepath.Join(dir, "examples", "ana.tr")
	data, _ := os.ReadFile(p)
	if err := os.WriteFile(p, append(data, []byte("TR_print(1)\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	_, st, err := BuildCached(dir, Options{})
	if err != nil || st.Full || st.FromCache || st.Parsed != 1 {
		t.Errorf("a leaf edit parses one file: %+v %v", st, err)
	}
	// geo.tr'yi ana.tr ve geo_test.tr aktarıyor: üçü yeniden çözülür.
	g := filepath.Join(dir, "lib", "geo.tr")
	data, _ = os.ReadFile(g)
	if err := os.WriteFile(g, append(data, []byte("\nTR_func yeni_ad() -> TR_Int {\n    TR_return 1\n}\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	_, st, err = BuildCached(dir, Options{})
	if err != nil || st.Full || st.Parsed != 3 {
		t.Errorf("an edit of a module re-resolves its importers: %+v %v", st, err)
	}
}

func TestStatusReportsChanges(t *testing.T) {
	dir := copyFixture(t)
	if st, _ := Status(dir, Options{}); st.Exists {
		t.Error("no cache yet")
	}
	if _, _, err := BuildCached(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lib", "geo.tr"), trText(filepath.Join(dir, "lib", "geo.tr"), "TR_const X = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "yeni.tr"), trText(filepath.Join(dir, "yeni.tr"), "TR_const Y = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "examples", "ana.tr")); err != nil {
		t.Fatal(err)
	}
	st, err := Status(dir, Options{})
	if err != nil || !st.Exists {
		t.Fatalf("%+v %v", st, err)
	}
	if strings.Join(st.Changed, ",") != "lib/geo.tr" || strings.Join(st.Added, ",") != "yeni.tr" || strings.Join(st.Removed, ",") != "examples/ana.tr" {
		t.Errorf("status: %+v", st)
	}
}

// genProject rastgele ama belirli bir TR projesi yazar: modüller birbirini aktarır (takma adlı ve
// takma adsız), yapılar, metotlar, testler, gömülü kitaplık aktarması.
func genProject(t *testing.T, dir string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		writeModule(t, dir, i, n, nil)
	}
}

func writeModule(t *testing.T, dir string, i, n int, extra []string) {
	t.Helper()
	var b strings.Builder
	for k := 0; k < i && k < 3; k++ {
		fmt.Fprintf(&b, "TR_import \"m%d\"\n", i-1-k)
	}
	if i%4 == 1 && i > 0 {
		fmt.Fprintf(&b, "TR_import \"m%d\" TR_as a\n", i-1)
	}
	if i%5 == 2 {
		b.WriteString("TR_import \"tr/layers\"\n")
	}
	fmt.Fprintf(&b, "TR_const C%d = %d\n", i, i)
	fmt.Fprintf(&b, "TR_struct S%d {\n    x = 1\n    TR_func TR_forward() {\n        TR_return TR_self.x\n    }\n}\n", i)
	for j := 0; j < 3; j++ {
		fmt.Fprintf(&b, "TR_func f%d_%d(p) {\n", i, j)
		if i > 0 {
			fmt.Fprintf(&b, "    TR_var v = f%d_%d(p)\n", i-1, j)
		}
		if i%4 == 1 && i > 0 {
			fmt.Fprintf(&b, "    TR_var w = a.f%d_0(p)\n", i-1)
		}
		if i%5 == 2 {
			b.WriteString("    TR_var g = Katman(1)\n")
		}
		b.WriteString("    TR_print(p.TR_forward())\n    TR_return p\n}\n")
	}
	for _, line := range extra {
		b.WriteString(line + "\n")
	}
	if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("m%d.tr", i)), trText(filepath.Join(dir, fmt.Sprintf("m%d.tr", i)), b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Artımlı güncelleme sıfırdan kurulanla aynı baytları verir: rastgele düzenleme dizisi (çağrı
// ekleme, ad değiştirme, aktarma ekleme/çıkarma, dosyayı bozup düzeltme, dosya ekleme/silme).
func TestIncrementalEqualsFull(t *testing.T) {
	dir := t.TempDir()
	const n = 12
	genProject(t, dir, n)
	lib := fstest.MapFS{"layers.tr": {Data: []byte("TR_struct Katman {\n    n: TR_Int = 1\n}\n")}}
	opts := Options{Library: lib}
	rng := rand.New(rand.NewSource(7))
	extras := make([][]string, n)
	broken := map[int]bool{}
	steps := 45
	if testing.Short() {
		steps = 15
	}
	if _, _, err := BuildCached(dir, opts); err != nil {
		t.Fatal(err)
	}
	for step := 0; step < steps; step++ {
		i := rng.Intn(n)
		switch rng.Intn(6) {
		case 0: // gövdeye çağrı ekle
			extras[i] = append(extras[i], fmt.Sprintf("TR_func e%d_%d() {\n    TR_return f%d_0(1)\n}", i, step, rng.Intn(i+1)))
		case 1: // sabiti kullanan bir işlev
			extras[i] = append(extras[i], fmt.Sprintf("TR_func k%d_%d() {\n    TR_return C%d\n}", i, step, rng.Intn(n)))
		case 2: // yorum/boşluk değişikliği (içerik değişir, anlam değişmez)
			extras[i] = append(extras[i], fmt.Sprintf("# not %d", step))
		case 3: // dosyayı boz ya da düzelt
			if broken[i] {
				delete(broken, i)
			} else {
				broken[i] = true
			}
		case 4: // ad değiştir: ilk işlevin adı değişsin (aktaranlar çözülemez olur)
			extras[i] = append(extras[i], fmt.Sprintf("TR_func f%d_0(p) {\n    TR_return p\n}", i)) // yineleme: son tanım kazanır
		case 5: // yazma
			extras[i] = append(extras[i], fmt.Sprintf("C%d = %d", i, step))
		}
		writeModule(t, dir, i, n, extras[i])
		if broken[i] {
			p := filepath.Join(dir, fmt.Sprintf("m%d.tr", i))
			data, _ := os.ReadFile(p)
			if err := os.WriteFile(p, append(data, []byte("TR_func {\n")...), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		inc, _, err := BuildCached(dir, opts)
		if err != nil {
			t.Fatal(err)
		}
		full, err := Build(dir, opts)
		if err != nil {
			t.Fatal(err)
		}
		if a, b := mustJSON(t, inc), mustJSON(t, full); a != b {
			t.Fatalf("step %d (module m%d): the incremental index differs from a fresh build\n%s", step, i, firstDiff(a, b))
		}
	}
	// Dosya eklemek ve silmek baştan kurulumdur ve yine aynıdır.
	if err := os.WriteFile(filepath.Join(dir, "yeni.tr"), trText(filepath.Join(dir, "yeni.tr"), "TR_import \"m3\"\nTR_func y() -> TR_Int {\n    TR_return f3_0(1)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inc, st, err := BuildCached(dir, opts)
	full, _ := Build(dir, opts)
	if err != nil || !st.Full || mustJSON(t, inc) != mustJSON(t, full) {
		t.Errorf("an added file rebuilds: %+v %v", st, err)
	}
	if err := os.Remove(filepath.Join(dir, "m5.tr")); err != nil {
		t.Fatal(err)
	}
	inc, st, err = BuildCached(dir, opts)
	full, _ = Build(dir, opts)
	if err != nil || !st.Full || mustJSON(t, inc) != mustJSON(t, full) {
		t.Errorf("a removed file rebuilds: %+v %v", st, err)
	}
}

// firstDiff iki metnin ilk ayrıldığı yerin çevresini gösterir.
func firstDiff(a, b string) string {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	lo := max(n-200, 0)
	cut := func(s string) string { return s[lo:min(n+200, len(s))] }
	return "incremental: " + cut(a) + "\n---\nfresh: " + cut(b)
}

// benchProject sentetik, ama gerçekçi bir proje yazar: n modül, her biri üç işlev, bir yapı, bir metot. Her modül
// önceki üçünü aktarır (zincir: i. modül öncekilerin hepsini görür) ve yapay zekâ kitaplığının kalıbını taşır: her
// yapıda aynı adlı metot (`ileri`) ve alan (`ağırlık`), işlevlerde alıcısı bilinmeyen `p.ileri()` ve `p.ağırlık`. Adlar
// kod biçiminde (`C100`, 2.3a) ya da `TR_` önekli olmaz: öyle bir ad dosyayı Skipped'a düşürür ve ölçülen iş küçülür.
func benchProject(tb testing.TB, n int) string {
	tb.Helper()
	dir := tb.TempDir()
	for i := 0; i < n; i++ {
		var b strings.Builder
		for k := 0; k < i && k < 3; k++ {
			fmt.Fprintf(&b, "TR_import \"m%d\"\n", i-1-k)
		}
		fmt.Fprintf(&b, "TR_const c_%d = %d\nTR_struct S_%d {\n    x = 1\n    TR_weight = 1\n    TR_func TR_forward() {\n        TR_return TR_self.x\n    }\n}\n", i, i, i)
		for j := 0; j < 3; j++ {
			fmt.Fprintf(&b, "TR_func f%d_%d(p) {\n", i, j)
			if i > 0 {
				fmt.Fprintf(&b, "    TR_var v = f%d_%d(p)\n", i-1, j)
			}
			b.WriteString("    TR_print(p.TR_forward())\n    TR_print(p.TR_weight)\n    TR_return p\n}\n")
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("m%d.tr", i)), trText(filepath.Join(dir, fmt.Sprintf("m%d.tr", i)), b.String()), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
	return dir
}

// BenchmarkBuild sıfırdan kurulum, önbellekten okuma ve tek dosya değişikliğinin yenilemesini ölçer
// (`go test -bench Build ./internal/devtools/codeindex`).
func BenchmarkBuild(b *testing.B) {
	dir := benchProject(b, 200)
	opts := Options{}
	b.Run("full", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := Build(dir, opts); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("cache_hit", func(b *testing.B) {
		if _, _, err := BuildCached(dir, opts); err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, _, err := BuildCached(dir, opts); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("one_leaf_edit", func(b *testing.B) {
		p := filepath.Join(dir, "m199.tr")
		orig, _ := os.ReadFile(p)
		defer os.WriteFile(p, orig, 0o644)
		for i := 0; i < b.N; i++ {
			if err := os.WriteFile(p, append(append([]byte(nil), orig...), []byte(fmt.Sprintf("# %d\n", i))...), 0o644); err != nil {
				b.Fatal(err)
			}
			if _, _, err := BuildCached(dir, opts); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Kural: kurulum modül sayısıyla doğrusal büyür (sıfırdan kurulum, önbellekten okuma, bir
// modülün düzenlenmesinden sonra yenileme). Mutlak süre sınanmaz: bütün takım paralel koşarken
// 62 ms'lik kurulum 5,8 sn sürdü ve eski 5 sn'lik tavanı aştı (testkit.ScalesLinearly). Önbelleğin
// işini yaptığı (isabette ayrıştırma yok, düzenlemede yalnızca değişen ve içe aktaranlar) sayılarak
// sınanır: TestCacheHitSkipsParsing, TestIncrementalReparsesOnlyChangedAndImporters. Ölçülen kurulum hatasız ve tam
// olmalıdır: birim okunan dosyadır, atlanan dosya hatadır (buildFiles; ScalesLinearly iki boyun birimlerini karşılaştırır).
// Kod biçimli adlar 400 modülün 300'ünü Skipped'a düşürüyor, iki boyda aynı iş ölçülüyordu.
// Proje zincir aktarımlıdır (her modül öncekileri dolaylı görür): görünen adları dosya başına kopyalamak ve üye adaylarını
// bütün kapanışta aramak karesel büyüyordu, CI'da oran 9-10; ad sorulunca aranır (lookupVisible, methodCandidates).
func TestBuildScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("zaman ölçümü")
	}
	testkit.ScalesLinearly(t, "sıfırdan kurulum", 100, func(n int) func() (int, error) {
		dir := benchProject(t, n)
		return func() (int, error) { return buildFiles(dir, false) }
	})
	testkit.ScalesLinearly(t, "önbellekten okuma", 100, func(n int) func() (int, error) {
		dir := benchProject(t, n)
		buildFiles(dir, true)
		return func() (int, error) { return buildFiles(dir, true) }
	})
	testkit.ScalesLinearly(t, "bir modülün düzenlenmesinden sonra yenileme", 100, func(n int) func() (int, error) {
		dir := benchProject(t, n)
		buildFiles(dir, true)
		p := filepath.Join(dir, "m0.tr")
		orig, _ := os.ReadFile(p)
		edits := 0
		return func() (int, error) {
			edits++
			os.WriteFile(p, append(append([]byte(nil), orig...), fmt.Sprintf("# düzenleme %d", edits)...), 0o644)
			return buildFiles(dir, true)
		}
	})
}

// buildFiles dizini kurar (cached: önbellekle) ve okunan dosya sayısını verir; kurulum hatası ya da atlanan dosya hatadır.
func buildFiles(dir string, cached bool) (int, error) {
	idx, err := buildIndex(dir, cached)
	if err != nil {
		return 0, err
	}
	return len(idx.Files), nil
}

// buildIndex dizini kurar (cached: önbellekle); atlanan dosya hatadır.
func buildIndex(dir string, cached bool) (*Index, error) {
	var idx *Index
	var err error
	if cached {
		idx, _, err = BuildCached(dir, Options{})
	} else {
		idx, err = Build(dir, Options{})
	}
	if err == nil && len(idx.Skipped) > 0 {
		err = fmt.Errorf("%d files skipped (%v)", len(idx.Skipped), idx.Skipped[:min(len(idx.Skipped), 2)])
	}
	return idx, err
}

// Kural: dizinin boyu modül sayısıyla doğrusal büyür; aynı adlı üyeler (her katmanda `ileri` metodu ve `ağırlık` alanı)
// karesel büyütmez. Zamandan bağımsız, sayıyla: 100 ve 400 modülde dizinin başvuru ve kenar sayısının oranı ≤ 4,5.
// Alıcısı bilinmeyen `p.ileri()` ve `p.ağırlık` kullanım yeri başına bir kez adıyla saklanır (MemberGuesses), hangi
// üyelere "olası" diye bağlandığı sorguda çözülür (eksikler listesi: n=400'de 252 188 başvurunun 240 570'i ad-yalnızdı,
// ×15,9; DENETİM 2026-10-26).
func TestIndexSizeGrowsLinearlyWithSameNamedMembers(t *testing.T) {
	small, err := buildIndex(benchProject(t, 100), false)
	if err != nil {
		t.Fatal(err)
	}
	large, err := buildIndex(benchProject(t, 400), false)
	if err != nil || len(large.Files) != 4*len(small.Files) {
		t.Fatalf("the measured builds are incomplete: %v, %d and %d files", err, len(small.Files), len(large.Files))
	}
	size := func(idx *Index) int { return len(idx.Occurrences) + len(idx.Edges) + len(idx.MemberGuesses) }
	t.Logf("occurrences %d -> %d, edges %d -> %d, member guesses %d -> %d", len(small.Occurrences), len(large.Occurrences),
		len(small.Edges), len(large.Edges), len(small.MemberGuesses), len(large.MemberGuesses))
	if ratio := float64(size(large)) / float64(size(small)); ratio > 4.5 {
		t.Errorf("400 modules hold %.1f× the index of 100 (%d / %d occurrences, %d / %d edges, %d / %d member guesses); linear is 4",
			ratio, len(large.Occurrences), len(small.Occurrences), len(large.Edges), len(small.Edges), len(large.MemberGuesses), len(small.MemberGuesses))
	}
}

// Kural: WriteFileAtomic dosyayı bütün olarak yerine koyar (eski içerik tam değişir), geçici dosya bırakmaz ve klasör yoksa
// hata verir; dizin önbelleği ile codegraph'ın graph.json'ı aynı yazıcıyı kullanır.
func TestWriteFileAtomicReplacesWholeAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.json")
	for _, content := range []string{"{\"a\": 1}", "{}"} {
		if err := WriteFileAtomic(path, []byte(content)); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(path); string(got) != content {
			t.Errorf("content = %q, want %q", got, content)
		}
	}
	if entries, _ := reporoot.ReadDir(dir); len(entries) != 1 {
		t.Errorf("temporary file left behind: %v", entries)
	}
	if err := WriteFileAtomic(filepath.Join(dir, "none", "y.json"), []byte("x")); err == nil {
		t.Error("a missing directory is an error")
	}
}
