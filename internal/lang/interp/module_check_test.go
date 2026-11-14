package interp

import (
	"errors"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// checkImports ana programı ve yanındaki modülleri (yol → içerik) yazar, CheckImportedModules'ü
// çağırır.
func checkImports(t *testing.T, main string, files map[string]string) error {
	t.Helper()
	dir := t.TempDir()
	mainPath := testkit.WriteFile(t, dir, "ana.tr", main)
	for name, content := range files {
		testkit.WritePath(t, filepath.Join(dir, filepath.FromSlash(name)), content)
	}
	return CheckImportedModules(mainPath, mustParseProgram(t, main))
}

// Çalışmadan önce aktarılan bütün modüller (özyinelemeli) aynı denetimden geçer (1.3): hatalı
// modül aktar satırıyla ve kendi hatalarıyla bildirilir; çalışma anı aktarması ikinci bir
// denetim yapmaz.
func TestCheckImportedModules(t *testing.T) {
	good := "# TR 4\nTR_func f() -> TR_Int {\n    TR_return 1\n}\n"
	bad := "# TR 4\nTR_func f() -> TR_Int {\n    TR_return \"a\"\n}\n"
	if err := checkImports(t, "# TR 4\nTR_import \"iyi\"\n", map[string]string{"iyi.tr": good}); err != nil {
		t.Errorf("hatasız modül geçmeli: %v", err)
	}
	err := checkImports(t, "# TR 4\nTR_import \"bozuk\" TR_as b\n", map[string]string{"bozuk.tr": bad})
	var moduleErr *ModuleError
	if !errors.As(err, &moduleErr) || moduleErr.Kind != source.StageType || len(moduleErr.Errors) == 0 || moduleErr.ImportLine != 2 {
		t.Errorf("modülün tip hatası aktar satırıyla bildirilmeli: %v", err)
	}
	// Modülün aktardığı modül de denetlenir; çevrimsel aktarma sonsuza gitmez.
	err = checkImports(t, "# TR 4\nTR_import \"a\"\n", map[string]string{
		"a.tr": "# TR 4\nTR_import \"b\"\n",
		"b.tr": "# TR 4\nTR_import \"a\"\nTR_import \"c\"\n",
		"c.tr": bad,
	})
	if !errors.As(err, &moduleErr) || moduleErr.ModulePath != "c" {
		t.Errorf("iç içe aktarılan modülün hatası bildirilmeli: %v", err)
	}
	if err := checkImports(t, "# TR 4\nTR_import \"yok_modul\"\n", nil); err == nil || errors.As(err, &moduleErr) {
		t.Errorf("bulunamayan modül bir okuma hatasıdır: %v", err)
	}
	// Olmayan bir kitaplık modülü çalışmadan önce bulunur; varolanın denetimi
	// TestEmbeddedModulesAreChecked'te.
	withLibrary(t, fstest.MapFS{"layers.tr": {Data: []byte("# TR 4\nTR_var K = 1\n")}})
	if err := checkImports(t, "# TR 4\nTR_import \"tr/layers\"\n", nil); err != nil {
		t.Errorf("hatasız kitaplık modülü geçmeli: %v", err)
	}
	if err := checkImports(t, "# TR 4\nTR_import \"tr/olmayan\"\n", nil); err == nil {
		t.Error("kitaplıkta olmayan modül çalışmadan önce bulunmalı")
	}
}

// Kural (1.2, 1.3): dilin gömülü kitaplığının aktarılan modülleri de, onların aktardıkları ve
// kitaplıkta göreli aktarılanlar dahil, çalışmadan önce aynı denetimden geçer; hata "tr/..." adıyla
// bildirilir. Kitaplığın metni değişmediği için modül süreç başına bir kez denetlenir; çalışma anı
// aktarması da, çağıran önceden denetlemese, denetlenmiş ağacı kullanır.
func TestEmbeddedModulesAreChecked(t *testing.T) {
	good := "# TR 4\nTR_func f() -> TR_Int {\n    TR_return 1\n}\n"
	bad := "# TR 4\nTR_func f() -> TR_Int {\n    TR_return \"a\"\n}\n"
	withLibrary(t, fstest.MapFS{
		"bozuk.tr":     {Data: []byte(bad)},
		"iyi.tr":       {Data: []byte(good)},
		"aktaran.tr":   {Data: []byte("# TR 4\nTR_import \"tr/bozuk\"\n")},
		"goreli.tr":    {Data: []byte("# TR 4\nTR_import \"bozuk\"\n")},
		"alt/a.tr":     {Data: []byte("# TR 4\nTR_import \"b\"\n")},
		"alt/b.tr":     {Data: []byte(bad)},
		"cevrimsel.tr": {Data: []byte("# TR 4\nTR_import \"tr/cevrimsel\"\nTR_import \"tr/iyi\"\n")},
	})
	var moduleErr *ModuleError
	for _, c := range []struct{ main, module, source string }{
		{"TR_import \"tr/bozuk\"", "tr/bozuk", "tr/bozuk.tr"},
		{"TR_import \"tr/aktaran\" TR_as a", "tr/bozuk", "tr/bozuk.tr"},
		{"TR_import \"tr/goreli\"", "bozuk", "tr/bozuk.tr"},
		{"TR_import \"tr/alt/a\"", "b", "tr/alt/b.tr"},
	} {
		err := checkImports(t, "# TR 4\n"+c.main+"\n", nil)
		if !errors.As(err, &moduleErr) || moduleErr.Kind != source.StageType || len(moduleErr.Errors) == 0 ||
			moduleErr.ModulePath != c.module || moduleErr.Source.Name != c.source || !moduleErr.Source.Embedded {
			t.Errorf("%s: kitaplık modülünün tip hatası %q (%s) olarak bildirilmeli: %v", c.main, c.module, c.source, err)
		}
	}
	if err := checkImports(t, "# TR 4\nTR_import \"tr/cevrimsel\"\n", nil); err != nil {
		t.Errorf("kendini aktaran hatasız kitaplık modülü geçmeli: %v", err)
	}
	first := checkedLibraryModule("iyi.tr", good)
	if again := checkedLibraryModule("iyi.tr", good); again != first || first.Stage != "" {
		t.Errorf("aynı metinli kitaplık modülü bir kez denetlenmeli: %p, %p (%q)", first, again, first.Stage)
	}
	if changed := checkedLibraryModule("iyi.tr", bad); changed == first || changed.Stage != source.StageType {
		t.Errorf("metni değişen kitaplık modülü yeniden denetlenmeli: %q", changed.Stage)
	}
	// Denetleyicinin çağrıya yazdığı boş listenin öğe tipi (ast.CallExpression.ListElem) kitaplıkta
	// da yazılır: tipli boş tensör listesinin toplamı kitaplıkta da 15.18'in hatasıdır.
	sum := "# TR 4\nTR_func f() -> TR_Tensor {\n    TR_var l: TR_List[TR_Tensor] = []\n    TR_return TR_sum(l)\n}\n"
	var elem ast.ListElem
	ast.Walk(checkedLibraryModule("toplam.tr", sum).Program, func(n any) bool {
		if c, ok := n.(*ast.CallExpression); ok && c.ListElem != ast.ListElemUnknown {
			elem = c.ListElem
		}
		return true
	})
	if elem != ast.ListElemTensor {
		t.Errorf("kitaplık modülünün ağacında TR_sum'ın öğe tipi TR_Tensor olmalı: %v", elem)
	}
	root := t.TempDir()
	runInDir(t, root)
	_, err := runWithModules(t, root, map[string]string{"ana.tr": "TR_import \"tr/bozuk\"\nTR_print(1)\n"})
	if !errors.As(err, &moduleErr) || moduleErr.Kind != source.StageType {
		t.Errorf("çalışma anında aktarılan kitaplık modülü de denetlenmiş ağaçla çalışmalı: %v", err)
	}
}
