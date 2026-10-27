package interp

import (
	"errors"
	"path/filepath"
	"testing"
	"testing/fstest"

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
	// Dilin kitaplığı derlemeyle gelir, içeriği denetlenmez; olmayan bir kitaplık modülü bulunur.
	withLibrary(t, fstest.MapFS{"katman.tr": {Data: []byte("# TR 4\nTR_var K = 1\n")}})
	if err := checkImports(t, "# TR 4\nTR_import \"tr/katman\"\n", nil); err != nil {
		t.Errorf("kitaplıktaki modül çalışmadan önce denetlenmez, yalnız varlığı bakılır: %v", err)
	}
	if err := checkImports(t, "# TR 4\nTR_import \"tr/olmayan\"\n", nil); err == nil {
		t.Error("kitaplıkta olmayan modül çalışmadan önce bulunmalı")
	}
}
