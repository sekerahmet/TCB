package assistant_test

import (
	"strings"
	"testing"
)

// Tek tık kurulumunun A dilimi: katalog (katalog.json), onu okuyan modül (katalog.tr) ve modelsiz komut (kur.tr).
// Davranışı trlang sınar (katalog_test.tr); bu test yalnız dosyaların README'de anıldığını ve testin modülü
// aktardığını korur (TestChainFilesAreListed'ın kalıbı; var olan testi değiştirmemek için ayrı dosya).

const (
	setupCatalog = "tools/assistant/katalog.json"
	setupModule  = "tools/assistant/katalog.tr"
	setupTest    = "tools/assistant/katalog_test.tr"
	setupCommand = "tools/assistant/kur.tr"
	setupGoTest  = "behavior_setup_test.go"
)

func TestSetupFilesAreListed(t *testing.T) {
	readme := readFile(t, readmePath)
	for _, f := range []string{setupCatalog, setupModule, setupTest, setupCommand} {
		name := f[strings.LastIndex(f, "/")+1:]
		if !strings.Contains(readme, "`"+name+"`") {
			t.Errorf("%s does not name %s", readmePath, name)
		}
	}
	if !strings.Contains(readme, "- `"+setupGoTest+"`: ") {
		t.Errorf("%s does not list %s", readmePath, setupGoTest)
	}
	for _, f := range []string{setupTest, setupCommand} {
		if !strings.Contains(readFile(t, f), `TR_import "katalog"`) {
			t.Errorf("%s does not import the catalog module", f)
		}
	}
}
