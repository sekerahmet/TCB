package interp

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Kural (17, kitaplık kodu): kitaplığın tip kodu genel kapsamda yerleşik bir tip değeri olmaz; tipi
// kitaplığın kendi yapısıdır. Çekirdek tipleri genel kapsamda kalır.
func TestLibraryTypeIsNotAGlobalTypeValue(t *testing.T) {
	t.Cleanup(codes.SetLibraryModulesForTest(map[codes.Code]string{codes.TypeOptimizer: "tr/deneme"}))
	v := New()
	if _, ok := v.GlobalEnv.Get(codes.Name(codes.TypeOptimizer)); ok {
		t.Errorf("kitaplık tipi %q genel kapsamda olmamalı", codes.Name(codes.TypeOptimizer))
	}
	if _, ok := v.GlobalEnv.Get(codes.Name(codes.TypeInt)); !ok {
		t.Errorf("çekirdek tipi %q genel kapsamda olmalı", codes.Name(codes.TypeInt))
	}
}
