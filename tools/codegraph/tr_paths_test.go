package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

func buildTRPathFixture(t *testing.T, files map[string]string) *Graph {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b := newBuilder("example.com/trpaths")
	extractTR(dir, b, nil)
	extractDataFiles(dir, b)
	linkTRPaths(dir, b)
	return b.graph()
}

func rp(parts ...string) string { return strings.Join(parts, "/") }

func hasEdgeToFile(g *Graph, fromID, targetRel, kind string) bool {
	for _, e := range g.Edges {
		if e.From != fromID || e.Kind != kind {
			continue
		}
		for _, n := range g.Nodes {
			if n.ID == e.To && n.File == targetRel {
				return true
			}
		}
	}
	return false
}

func testIDByName(t *testing.T, g *Graph, file, sub string) string {
	t.Helper()
	for _, n := range g.Nodes {
		if n.File == file && n.Kind == kindTRTest && (strings.Contains(n.Name, sub) || strings.Contains(n.Detail, sub)) {
			return n.ID
		}
	}
	t.Fatalf("no TR test matching %q in %s", sub, file)
	return ""
}

func testsForText(t *testing.T, ix *index, target string) string {
	t.Helper()
	n := ix.byID["file:"+target]
	if n == nil {
		t.Fatalf("missing file node for %s", target)
	}
	var buf bytes.Buffer
	ix.printTestsFor(&buf, n, defaultLimit)
	return buf.String()
}

// Kural: 18.16 tekil dosya okuyucuları, TR_Dataset (dosya ve klasör) ve _test.tr yorumlarındaki yollar
// doğru hedef düğümlere bağlanır; 13.1a sırası önce çalışma köküne, yoksa modül klasörüne bakar.
func TestTRSingleAndDatasetReadersAndCites(t *testing.T) {
	src := "# TR 4\n" +
		"# Üst yorum: fx_doc/KURALLAR.md ve YEREL.md\n" +
		"TR_const _KÖK = \"fx_root\"\n" +
		"TR_func yardımcı(ek: TR_String, son: TR_String = \"varsayilan.json\") {\n" +
		"    TR_read_file(\"\\(_KÖK)/\\(ek)\")\n" +
		"    TR_load_tensors(\"fx_local/\\(son)\")\n" +
		"}\n" +
		"TR_test \"tekil ve küme okumaları\" {\n" +
		"    yardımcı(\"a.txt\")\n" +
		"    TR_var p = TR_Path(\"fx_local\", \"var.bin\")\n" +
		"    TR_var _ = p.TR_exists\n" +
		"    TR_load_tokenizer(\"fx_local/tok.json\")\n" +
		"    TR_copy(\"fx_local/kaynak.csv\", \"fx_local/hedef_yazici.csv\")\n" +
		"    TR_file_response(\"fx_web\", \"/index.html\")\n" +
		"    TR_for ad TR_in [\"d1.wav\", \"d2.wav\"] { TR_read_wav(\"fx_local/\\(ad)\") }\n" +
		"    TR_var _ = TR_Dataset(\"fx_ds_one/tek.npy\")\n" +
		"    TR_var _ = TR_Dataset(\"fx_ds_dir\", TR_cache: \"fx_local/onbellek.npy\")\n" +
		"}\n"
	g := buildTRPathFixture(t, map[string]string{
		"tests/okuma_test.tr":             src,
		"fx_doc/KURALLAR.md":              "# kural\n",
		"tests/YEREL.md":                  "# yerel\n",
		"fx_root/a.txt":                   "kok",
		"tests/fx_root/a.txt":             "modul-degil",
		"tests/fx_local/varsayilan.json":  "{}",
		"tests/fx_local/var.bin":          "x",
		"tests/fx_local/tok.json":         "{}",
		"tests/fx_local/kaynak.csv":       "a,b\n",
		"tests/fx_local/hedef_yazici.csv": "yok\n",
		"tests/fx_local/onbellek.npy":     "yok",
		"fx_web/index.html":               "<p>x</p>",
		"tests/fx_local/d1.wav":           "w1",
		"tests/fx_local/d2.wav":           "w2",
		"tests/fx_ds_one/tek.npy":         "n",
		"tests/fx_ds_dir/p0.npy":          "n0",
		"tests/fx_ds_dir/meta.json":       "{}",
		"tests/fx_ds_dir/notlar.txt":      "okunmaz",
	})
	tid := testIDByName(t, g, "tests/okuma_test.tr", "tekil ve küme")
	want := []string{
		"fx_root/a.txt", "tests/fx_local/varsayilan.json", "tests/fx_local/var.bin",
		"tests/fx_local/tok.json", "tests/fx_local/kaynak.csv", "fx_web/index.html",
		"tests/fx_local/d1.wav", "tests/fx_local/d2.wav", "tests/fx_ds_one/tek.npy",
		"tests/fx_ds_dir/p0.npy", "tests/fx_ds_dir/meta.json",
	}
	for _, f := range want {
		if !hasEdgeToFile(g, tid, f, edgeTests) {
			t.Errorf("missing edgeTests from %s to %s", tid, f)
		}
	}
	for _, no := range []string{"tests/fx_root/a.txt", "tests/fx_local/hedef_yazici.csv", "tests/fx_local/onbellek.npy", "tests/fx_ds_dir/notlar.txt"} {
		if hasEdgeToFile(g, tid, no, edgeTests) {
			t.Errorf("unexpected edgeTests from %s to %s", tid, no)
		}
	}
	for _, cited := range []string{"fx_doc/KURALLAR.md", "tests/YEREL.md"} {
		if !hasEdgeToFile(g, tid, cited, edgeCites) {
			t.Errorf("missing edgeCites from %s to %s", tid, cited)
		}
	}
}

// Kural (514 N1, 518 N1, TCB 22:28): hazır model ve sohbet ailesi yükleyicileri config.json içindeki model_type
// ile tokenizer_config.json kapısına bakar; kapı geçmezse ağırlık ve aile dosyalarına yanlış bağ kurulmaz.
func TestTRModelAndChatFamilyFolderLoaders(t *testing.T) {
	files := map[string]string{
		"tests/yukleyici_test.tr": "# TR 4\n" +
			"TR_test \"model mimarileri\" {\n" +
			"    TR_pretrained_model(\"fx_m_llama\")\n" +
			"    TR_pretrained_model(\"fx_m_qwen3\")\n" +
			"    TR_pretrained_model(\"fx_m_gemma3\")\n" +
			"    TR_pretrained_model(TR_folder: \"fx_m_gemma3_text\")\n" +
			"}\n" +
			"TR_test \"kapısız aile bağlanmaz\" {\n" +
			"    TR_pretrained_model(\"fx_m_no_tokcfg\")\n" +
			"    TR_folder_chat_family(0, TR_Path(\"fx_m_no_tokcfg\"), {})\n" +
			"}\n" +
			"TR_test \"doğrudan aile ve ses yükleyicileri\" {\n" +
			"    TR_folder_chat_family(0, TR_Path(\"fx_fam_ok\"), {})\n" +
			"    TR_gguf_chat_family(0, \"fx_gguf/m.gguf\", \"llama\")\n" +
			"    TR_pretrained_whisper(\"fx_w_ok\")\n" +
			"    TR_pretrained_whisper(\"fx_w_bad\")\n" +
			"}\n",
		"tests/fx_gguf/m.gguf":                        "g",
		"tests/fx_fam_ok/tokenizer_config.json":       "{}",
		"tests/fx_fam_ok/tokenizer.json":              "{}",
		"tests/fx_fam_ok/generation_config.json":      "{}",
		"tests/fx_w_ok/config.json":                   `{"model_type":"whisper"}`,
		"tests/fx_w_ok/model.safetensors":             "w",
		"tests/fx_w_ok/tokenizer.json":                "{}",
		"tests/fx_w_ok/generation_config.json":        "{}",
		"tests/fx_w_bad/config.json":                  `{"model_type":"llama"}`,
		"tests/fx_w_bad/model.safetensors":            "w",
		"tests/fx_m_no_tokcfg/config.json":            `{"model_type":"llama"}`,
		"tests/fx_m_no_tokcfg/model.safetensors":      "w",
		"tests/fx_m_no_tokcfg/tokenizer.json":         "{}",
		"tests/fx_m_no_tokcfg/generation_config.json": "{}",
	}
	five := []string{"config.json", "model.safetensors", "tokenizer_config.json", "tokenizer.json", "generation_config.json"}
	for _, mt := range []string{"llama", "qwen3", "gemma3", "gemma3_text"} {
		dir := "tests/fx_m_" + mt
		files[dir+"/config.json"] = `{"model_type":"` + mt + `"}`
		for _, f := range five[1:] {
			files[dir+"/"+f] = "x"
		}
	}
	g := buildTRPathFixture(t, files)
	tArch := testIDByName(t, g, "tests/yukleyici_test.tr", "model mimarileri")
	for _, mt := range []string{"llama", "qwen3", "gemma3", "gemma3_text"} {
		for _, f := range five {
			if !hasEdgeToFile(g, tArch, "tests/fx_m_"+mt+"/"+f, edgeTests) {
				t.Errorf("%s missing %s", mt, f)
			}
		}
	}
	tNoCfg := testIDByName(t, g, "tests/yukleyici_test.tr", "kapısız aile")
	for _, okFile := range []string{"config.json", "model.safetensors"} {
		if !hasEdgeToFile(g, tNoCfg, "tests/fx_m_no_tokcfg/"+okFile, edgeTests) {
			t.Errorf("missing %s on fx_m_no_tokcfg", okFile)
		}
	}
	for _, skip := range []string{"tokenizer.json", "generation_config.json"} {
		if hasEdgeToFile(g, tNoCfg, "tests/fx_m_no_tokcfg/"+skip, edgeTests) {
			t.Errorf("518 N1: linked %s without tokenizer_config.json", skip)
		}
	}
	tFam := testIDByName(t, g, "tests/yukleyici_test.tr", "doğrudan aile")
	for _, want := range []string{
		"tests/fx_fam_ok/tokenizer_config.json", "tests/fx_fam_ok/tokenizer.json", "tests/fx_fam_ok/generation_config.json",
		"tests/fx_gguf/m.gguf", "tests/fx_w_ok/config.json", "tests/fx_w_ok/model.safetensors",
		"tests/fx_w_ok/tokenizer.json", "tests/fx_w_ok/generation_config.json", "tests/fx_w_bad/config.json",
	} {
		if !hasEdgeToFile(g, tFam, want, edgeTests) {
			t.Errorf("missing %s", want)
		}
	}
	if hasEdgeToFile(g, tFam, "tests/fx_w_bad/model.safetensors", edgeTests) {
		t.Errorf("whisper wrong model_type linked model.safetensors")
	}
}

// Kural (514 N1, 518 N2, 521 N1, TCB 22:28): tr_paths.go yükleyici tablosu lib/vits.tr, lib/whisper.tr,
// lib/pretrained.tr ve lib/model_family.tr dosyalarının bütünüyle iki yönlü eşleşir (satır aralığı yok).
func TestTRLoaderTableMatchesLibrarySources(t *testing.T) {
	if missing, extra := diffStringSets([]string{"a", "b"}, []string{"b", "c"}); len(missing) != 1 || len(extra) != 1 {
		t.Fatalf("diffStringSets broken: %v %v", missing, extra)
	}
	vitsFiles, vitsTypes := scanDosyaLoader(t, reporoot.Path("lib", "vits.tr"))
	vitsSpec := trModelLoaders["vits"]
	assertSameSet(t, "vits files", append(slices.Clone(vitsSpec.always), vitsSpec.modelFiles...), vitsFiles)
	assertSameSet(t, "vits model_type", vitsSpec.modelTypes, vitsTypes)

	whFiles, whTypes := scanDosyaLoader(t, reporoot.Path("lib", "whisper.tr"))
	whSpec := trModelLoaders["whisper"]
	assertSameSet(t, "whisper files", append(slices.Clone(whSpec.always), whSpec.modelFiles...), whFiles)
	assertSameSet(t, "whisper model_type", whSpec.modelTypes, whTypes)

	preFiles, preTypes := scanPretrainedLoader(t, reporoot.Path("lib", "pretrained.tr"))
	preSpec := trModelLoaders["pretrained"]
	assertSameSet(t, "pretrained files", append(slices.Clone(preSpec.always), preSpec.modelFiles...), preFiles)
	assertSameSet(t, "pretrained model_type", preSpec.modelTypes, preTypes)

	famFiles, _ := scanPretrainedLoader(t, reporoot.Path("lib", "model_family.tr"))
	assertSameSet(t, "model_family files", append([]string{trChatGuard}, trChatFiles...), famFiles)
}

func parseWholeLibTR(t *testing.T, fullPath string) *ast.Program {
	t.Helper()
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatal(err)
	}
	sb, err := source.Split(string(data))
	if err != nil || !sb.Coded {
		t.Fatalf("cannot split %s: %v", fullPath, err)
	}
	prog := parseTRSource(sb, true)
	if prog == nil {
		t.Fatalf("cannot parse %s", fullPath)
	}
	return prog
}

func scanDosyaLoader(t *testing.T, fullPath string) ([]string, []string) {
	t.Helper()
	prog := parseWholeLibTR(t, fullPath)
	consts := map[string]string{}
	for _, st := range prog.Statements {
		if v, ok := st.(*ast.VarStatement); ok && v.IsConst {
			if lit, ok := v.Value.(*ast.StringLiteral); ok {
				consts[v.Name.Value] = lit.Value
			}
		}
	}
	var files, types []string
	ast.Walk(prog, func(n any) bool {
		switch n := n.(type) {
		case *ast.CallExpression:
			fn, ok := n.Function.(*ast.Identifier)
			if !ok || fn.Value != "_dosya" || len(n.Arguments) != 2 {
				return true
			}
			switch a := n.Arguments[1].(type) {
			case *ast.Identifier:
				if consts[a.Value] != "" {
					files = append(files, consts[a.Value])
				}
			case *ast.StringLiteral:
				files = append(files, a.Value)
			}
		case *ast.InfixExpression:
			if (n.Operator == "!=" || n.Operator == "==") && isIdentNamed(n.Left, "mimari") {
				if lit, ok := n.Right.(*ast.StringLiteral); ok {
					types = append(types, lit.Value)
				}
			}
		}
		return true
	})
	return files, types
}

func scanPretrainedLoader(t *testing.T, fullPath string) ([]string, []string) {
	t.Helper()
	prog := parseWholeLibTR(t, fullPath)
	lang := codes.Active()
	archConst := lang.Name(codes.ConstSUPPORTEDARCHITECTURES)
	typePath := lang.Name(codes.TypePath)
	var files, types []string
	for _, st := range prog.Statements {
		v, ok := st.(*ast.VarStatement)
		if !ok || !v.IsConst || v.Name.Value != archConst {
			continue
		}
		if ll, ok := v.Value.(*ast.ListLiteral); ok {
			for _, el := range ll.Elements {
				if lit, ok := el.(*ast.StringLiteral); ok {
					types = append(types, lit.Value)
				}
			}
		}
	}
	ast.Walk(prog, func(n any) bool {
		switch n := n.(type) {
		case *ast.CallExpression:
			fn, ok := n.Function.(*ast.Identifier)
			if ok && fn.Value == typePath && len(n.Arguments) == 2 && isIdentNamed(n.Arguments[0], "k") {
				if lit, ok := n.Arguments[1].(*ast.StringLiteral); ok {
					files = append(files, lit.Value)
				}
			}
		case *ast.InfixExpression:
			if n.Operator == "==" && isIdentNamed(n.Left, "mimari") {
				if lit, ok := n.Right.(*ast.StringLiteral); ok {
					types = append(types, lit.Value)
				}
			}
		}
		return true
	})
	return files, types
}

func isIdentNamed(e ast.Expression, name string) bool {
	id, ok := e.(*ast.Identifier)
	return ok && id.Value == name
}

func diffStringSets(inTable, inLib []string) (extraInTable, missingFromTable []string) {
	tSet, lSet := map[string]bool{}, map[string]bool{}
	for _, s := range inTable {
		tSet[s] = true
	}
	for _, s := range inLib {
		lSet[s] = true
	}
	for s := range tSet {
		if !lSet[s] {
			extraInTable = append(extraInTable, s)
		}
	}
	for s := range lSet {
		if !tSet[s] {
			missingFromTable = append(missingFromTable, s)
		}
	}
	slices.Sort(extraInTable)
	slices.Sort(missingFromTable)
	return extraInTable, missingFromTable
}

func assertSameSet(t *testing.T, label string, inTable, inLib []string) {
	t.Helper()
	extra, missing := diffStringSets(inTable, inLib)
	if len(extra) > 0 || len(missing) > 0 {
		t.Errorf("%s drift: extra in tr_paths.go=%v, missing from tr_paths.go=%v", label, extra, missing)
	}
}

// Kural (Ş2, N2, 514 N2, 18.16, 560 Ş1b): 9. adım, çağrılmayan yardımcı, döngü, modül sınırı,
// başlıksız/view/bozuk dosyalar, TR_list_dir, 18.16 yazıcıları ve tekil okuyucuya verilen klasörler bağ kurmaz.
func TestTRHelperLimitsSkippedFilesAnd1816Negatives(t *testing.T) {
	chain := "# TR 4\n" +
		"TR_import \"./diger\"\n" +
		"TR_func f9() { TR_read_file(\"fx_neg/dokuz.txt\") }\n" +
		"TR_func f8() { f9(); TR_read_file(\"fx_neg/sekiz.txt\") }\n" +
		"TR_func f7() { f8() }\n" +
		"TR_func f6() { f7() }\n" +
		"TR_func f5() { f6() }\n" +
		"TR_func f4() { f5() }\n" +
		"TR_func f3() { f4() }\n" +
		"TR_func f2() { f3() }\n" +
		"TR_func f1() { f2() }\n" +
		"TR_func cagrilmayan() { TR_read_file(\"fx_neg/kimse.txt\") }\n" +
		"TR_func donA() { donB() }\n" +
		"TR_func donB() { donA() }\n" +
		"TR_func yazicilar(x: TR_String) {\n" +
		"    TR_var k = TR_temp_dir()\n" +
		"    TR_read_file(\"\\(k)/sabit.txt\")\n" +
		"    TR_read_file(\"\\(x)\")\n" +
		"    TR_read_file(\"\\(x)/sabit.txt\")\n" +
		"    TR_list_dir(\"fx_neg\")\n" +
		"    TR_list_dir(\"fx_neg\", \"*.json\")\n" +
		"    TR_write_file(\"fx_neg/yaz.txt\", \"x\")\n" +
		"    TR_save_tensors(\"fx_neg/ag.safetensors\", {})\n" +
		"    TR_var _ = TR_Checkpoint(\"fx_neg\", {})\n" +
		"    TR_var _ = TR_Logger(\"fx_neg/gunluk.jsonl\")\n" +
		"    TR_make_dir(\"fx_neg\")\n" +
		"    TR_remove(\"fx_neg/sil.txt\")\n" +
		"    TR_move(\"fx_neg/tasi1.txt\", \"fx_neg/tasi2.txt\")\n" +
		"    TR_read_bytes(\"fx_neg\")\n" +
		"    TR_load_tokenizer(\"fx_neg\")\n" +
		"}\n" +
		"TR_test \"sinirlar\" {\n" +
		"    f1()\n" +
		"    donA()\n" +
		"    diger_oku()\n" +
		"    yazicilar(\"bilinmez\")\n" +
		"}\n"
	g := buildTRPathFixture(t, map[string]string{
		"tests/sinir_test.tr":         chain,
		"tests/diger.tr":              "# TR 4\nTR_func diger_oku() { TR_read_file(\"fx_neg/modul_disi.txt\") }\n",
		"tests/basliksiz_test.tr":     "TR_test \"x\" { TR_read_file(\"fx_neg/basliksiz.txt\") }\n",
		"tests/gorunum_test.tr":       "# TR 4 view=tr\ntest \"x\" { dosya_oku(\"fx_neg/gorunum.txt\") }\n",
		"tests/bozuk_test.tr":         "# TR 4\nTR_test \"x\" { TR_read_file( }\n",
		"tests/fx_neg/sekiz.txt":      "8",
		"tests/fx_neg/dokuz.txt":      "9",
		"tests/fx_neg/kimse.txt":      "0",
		"tests/fx_neg/modul_disi.txt": "m",
		"tests/fx_neg/sabit.txt":      "s",
		"tests/fx_neg/yaz.txt":        "w",
		"tests/fx_neg/ag.safetensors": "t",
		"tests/fx_neg/gunluk.jsonl":   "l",
		"tests/fx_neg/sil.txt":        "r",
		"tests/fx_neg/tasi1.txt":      "m1",
		"tests/fx_neg/tasi2.txt":      "m2",
		"tests/fx_neg/tokenizer.json": "{}",
		"tests/fx_neg/basliksiz.txt":  "b",
		"tests/fx_neg/gorunum.txt":    "v",
	})
	tid := testIDByName(t, g, "tests/sinir_test.tr", "sinirlar")
	if !hasEdgeToFile(g, tid, "tests/fx_neg/sekiz.txt", edgeTests) {
		t.Errorf("8-step helper read was not linked")
	}
	assertNoTestEdgesTo(t, g, "tests/fx_neg/",
		"dokuz.txt", "kimse.txt", "modul_disi.txt", "sabit.txt", "yaz.txt",
		"ag.safetensors", "gunluk.jsonl", "sil.txt", "tasi1.txt", "tasi2.txt",
		"tokenizer.json", "basliksiz.txt", "gorunum.txt",
	)
}

func assertNoTestEdgesTo(t *testing.T, g *Graph, prefix string, names ...string) {
	t.Helper()
	for _, name := range names {
		target := prefix + name
		for _, e := range g.Edges {
			if e.Kind != edgeTests {
				continue
			}
			for _, n := range g.Nodes {
				if n.ID == e.To && n.File == target {
					t.Errorf("unexpected edgeTests from %s to %s", e.From, target)
				}
			}
		}
	}
}

// Kural: gerçek depo grafiğinde Madde 2 olumlu/olumsuz beklentileri (llama3_chat, hf_tiny, vits_tiny) tutar.
func TestTRPathsInRepositoryGraph(t *testing.T) {
	_, _, g := repoGraph(t)
	ix := newIndex(g)

	for _, name := range []string{"cases.json", "tokenizer.json"} {
		out := testsForText(t, ix, rp("tests", "data", "llama3_chat", name))
		if !strings.Contains(out, "run: go run ./cmd/trlang test tests/llama3_chat_test.tr") || !strings.Contains(out, "<- tests (") {
			t.Errorf("llama3_chat/%s missing run line or direct tests:\n%s", name, out)
		}
	}
	tokCfgOut := testsForText(t, ix, rp("tests", "data", "llama3_chat", "tokenizer_config.json"))
	if !strings.Contains(tokCfgOut, "no tests read it by name") {
		t.Errorf("unreferenced llama3_chat/tokenizer_config.json gained a reader:\n%s", tokCfgOut)
	}

	k6 := testIDByName(t, g, rp("lib", "pretrained_test.tr"), "(K6)")
	for _, f := range []string{"config.json", "model.safetensors"} {
		if !hasEdgeToFile(g, k6, rp("tests", "data", "hf_tiny", f), edgeTests) {
			t.Errorf("pretrained_test K6 missing hf_tiny/%s", f)
		}
	}

	vitsFile := rp("tests", "vits_model_test.tr")
	tSessiz := testIDByName(t, g, vitsFile, "gürültüsüz Hugging Face")
	tGurultulu := testIDByName(t, g, vitsFile, "gürültülü Hugging Face")
	izSessiz := rp("tests", "data", "vits_tiny", "reference", "iz_sessiz_f32.safetensors")
	izGurultulu := rp("tests", "data", "vits_tiny", "reference", "iz_gurultulu_f32.safetensors")
	if !hasEdgeToFile(g, tSessiz, izSessiz, edgeTests) || hasEdgeToFile(g, tSessiz, izGurultulu, edgeTests) {
		t.Errorf("vits noiseless test wrong reference edges")
	}
	if !hasEdgeToFile(g, tGurultulu, izGurultulu, edgeTests) || hasEdgeToFile(g, tGurultulu, izSessiz, edgeTests) {
		t.Errorf("vits noisy test wrong reference edges")
	}

	tHata := testIDByName(t, g, vitsFile, "klasör hataları adıyla")
	if !hasEdgeToFile(g, tHata, rp("tests", "data", "hf_tiny", "config.json"), edgeTests) {
		t.Errorf("vits folder error test should read hf_tiny/config.json")
	}
	if hasEdgeToFile(g, tHata, rp("tests", "data", "hf_tiny", "model.safetensors"), edgeTests) {
		t.Errorf("Ş1: vits H7552 error test must not read hf_tiny/model.safetensors")
	}
}
