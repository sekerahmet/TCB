package e2e

import (
	"cmp"
	"slices"
	"testing"
)

// Kural (6.1, 7.1, 2.13): tipsiz parametrenin ve alanın tipi varsayılandan, dönüş tipi TR_return'lerin
// ortak tipinden çıkınca 'tr check --json' uygulanabilir düzeltme verir; hepsi uygulanınca dosya
// hatasız geçer. Çağrılardan tahmin edilen tip (H1110) uygulanabilir değildir.
func TestCheckJSONTypedSignatureFixesMakeTheFileClean(t *testing.T) {
	const program = "# TR 4\nTR_struct S {\n    n = 0 # ERROR H1109\n}\nTR_func f(a: TR_Int, b = 2) { # ERROR H1109 H1112\n    TR_print(b)\n    TR_return a * 2\n}\nTR_print(f(1), S().n)\n"
	dir := t.TempDir()
	writeTR(t, dir, "gate.tr", program)
	records, _ := checkRecords(t, checkJSON(t, dir, "en", "gate.tr").Stdout)
	var fixes []jsonFixRecord
	for _, rec := range records {
		if rec.Code != "H1109" && rec.Code != "H1112" {
			t.Errorf("yalnız H1109 ve H1112 bekleniyordu: %+v", rec)
		}
		for _, f := range rec.Fixes {
			if !f.Applicable {
				t.Errorf("%s: düzeltme uygulanabilir olmalı: %+v", rec.Code, f)
			}
			fixes = append(fixes, f)
		}
	}
	if len(fixes) != 3 {
		t.Fatalf("üç düzeltme bekleniyordu (n, b, dönüş), %d: %+v", len(fixes), records)
	}
	// Sondan başa: aynı satırdaki iki düzeltmenin konumları kaymasın.
	slices.SortFunc(fixes, func(a, b jsonFixRecord) int {
		return cmp.Or(cmp.Compare(b.Start.Line, a.Start.Line), cmp.Compare(b.Start.Column, a.Start.Column))
	})
	source := program
	for _, f := range fixes {
		source = applyFix(t, source, f)
	}
	writeTR(t, dir, "gate_fixed.tr", source)
	if r := checkJSON(t, dir, "en", "gate_fixed.tr"); r.Code != 0 || r.Stdout != "" {
		t.Errorf("düzeltilmiş dosya hatasız geçmeli: çıkış %d\n%s\n%s", r.Code, r.Stdout, source)
	}

	writeTR(t, dir, "guess.tr", "# TR 4\nTR_func g(c) { # ERROR H1110\n    TR_print(c)\n}\ng(1)\n")
	guessCheck(t, dir)
}

// Kural (6.1, 7.1; kitaplık istisnası): gömülü kitaplığın 4c'yi bekleyen tipsiz yerleri tanı vermez;
// istisnalı modülleri aktaran kullanıcı programı 'tr check'te hatasız ve uyarısız geçer.
func TestCheckLibraryImportsGiveNoGateDiagnostics(t *testing.T) {
	const program = "# TR 4\nTR_import \"tr/layers\"\nTR_import \"tr/optimizers\"\nTR_import \"tr/pretrained\"\nTR_import \"tr/training\"\nTR_print(1)\n"
	dir := t.TempDir()
	writeTR(t, dir, "uses_library.tr", program)
	if r := checkJSON(t, dir, "en", "uses_library.tr"); r.Code != 0 || r.Stdout != "" {
		t.Errorf("kitaplığı aktaran program tanısız geçmeli: çıkış %d\n%s\n%s", r.Code, r.Stdout, r.Stderr)
	}
}

func guessCheck(t *testing.T, dir string) {
	t.Helper()
	guess, _ := checkRecords(t, checkJSON(t, dir, "en", "guess.tr").Stdout)
	if len(guess) != 1 || guess[0].Code != "H1110" || len(guess[0].Fixes) != 1 || guess[0].Fixes[0].Applicable {
		t.Errorf("çağrılardan tahmin uygulanamaz bir düzeltme vermeli: %+v", guess)
	}
}
