package interp

import (
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Kural (13.11; 4a2'nin ön şartı, (a) statik): denetleyici çekirdek metodun argüman tipini tanımın
// "params"ından okur ve uymayan argümanı çalışma anının koduyla (H2423) bildirir. Bu yüzden her
// çekirdek metodun (Metin, Liste, Sözlük, Aralık) tek tipli her konumlu parametresi, motorun o
// argümanda denetlediği tiple (signature.typesOf) aynıdır. Motorun denetlemediği ya da başka tip
// denetlediği alıcılar checker.ParamsNotVerified'dadır: denetleyici orada argüman tipine bakmaz;
// uyuşan alıcı listeden çıkar.
func TestParamsMatchEngineKinds(t *testing.T) {
	checked := 0
	for k, cm := range coreMethods {
		def, _ := codes.DefinitionOf(k)
		for typ, m := range []*builtinMethod{cm.text, cm.listVal, cm.dictVal, cm.rangeVal} {
			sig, ok := memberSignatureOf(def, typeCodes[typ])
			if m == nil || !ok {
				continue
			}
			agree, typed := true, false
			for i, p := range sig.Params {
				want, single := checker.ParamType(p)
				if !single {
					continue
				}
				typed = true
				var got codes.Code
				if i < len(m.sig.typesOf) && m.sig.typesOf[i] != nil {
					got = m.sig.typesOf[i].typ
				}
				if got != want {
					agree = false
					t.Logf("%s (%s) %d. parametre: tanım %s, motor %s", k, typeCodes[typ], i+1, want, got)
				}
			}
			listed := slices.Contains(checker.ParamsNotVerified[k], typeCodes[typ])
			switch {
			case !typed:
			case agree && listed:
				t.Errorf("%s (%s): parametre tipleri motorla uyuştu; checker.ParamsNotVerified'dan çıkarın", k, typeCodes[typ])
			case agree:
				checked++
			case !listed:
				t.Errorf("%s (%s): parametre tipleri motorla uyuşmuyor; düzeltin ya da checker.ParamsNotVerified'a yazın", k, typeCodes[typ])
			}
		}
	}
	t.Logf("%d metot-alıcının parametre tipleri motorla uyuştu", checked)
	if checked < 5 {
		t.Errorf("yalnız %d metot-alıcı sınandı", checked)
	}
}
