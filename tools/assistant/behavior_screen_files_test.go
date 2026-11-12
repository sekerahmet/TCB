package assistant_test

import (
	"strings"
	"testing"
)

// Ekranın TR testleri iki dosyadır, ortak yardımcıları bir modüldedir: üçü README'de anılır, iki test de ekranı ve
// yardımcıları aktarır, yardımcı modül test bloğu taşımaz.
const (
	screenTurnTest = "tools/assistant/ekran_tur_test.tr"
	screenHelpers  = "tools/assistant/screen_helpers.tr"
)

func TestScreenTestFilesAreListed(t *testing.T) {
	readme := readFile(t, readmePath)
	for _, f := range []string{screenTest, screenTurnTest, screenHelpers} {
		name := f[strings.LastIndex(f, "/")+1:]
		if !strings.Contains(readme, "`"+name+"`") {
			t.Errorf("%s does not name %s", readmePath, name)
		}
	}
	for _, f := range []string{screenTest, screenTurnTest} {
		src := readFile(t, f)
		for _, imp := range []string{`TR_import "ekran"`, `TR_import "screen_helpers"`} {
			if !strings.Contains(src, imp) {
				t.Errorf("%s does not hold %s", f, imp)
			}
		}
	}
	if strings.Contains(readFile(t, screenHelpers), "TR_test ") {
		t.Errorf("%s holds a test block; tests go to %s or %s", screenHelpers, screenTest, screenTurnTest)
	}
}

// Ekranın eşleme kodu (tek tık G, dil tanımı 28.2b): modül ve testi README'de anılır; test modülü aktarır, ekran da.
const (
	pairingModule = "tools/assistant/esleme.tr"
	pairingTest   = "tools/assistant/esleme_test.tr"
)

func TestPairingFilesAreListed(t *testing.T) {
	readme := readFile(t, readmePath)
	for _, f := range []string{pairingModule, pairingTest} {
		if name := f[strings.LastIndex(f, "/")+1:]; !strings.Contains(readme, "`"+name+"`") {
			t.Errorf("%s does not name %s", readmePath, name)
		}
	}
	for _, f := range []string{pairingTest, screenModule} {
		if !strings.Contains(readFile(t, f), `TR_import "esleme"`) {
			t.Errorf("%s does not import the pairing module", f)
		}
	}
}
