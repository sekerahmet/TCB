package assistant_test

import (
	"regexp"
	"strings"
	"testing"
)

// Zincir (asistan.tr) dizüstünde kişisel yollarla koşar; depoya yol ve aygıt adı girmez (S1 zincir yönergesi, B.1).
// Davranışı tarayıcı değil trlang sınar: modelsiz parçalar zincir_test.tr'de.

const (
	chainProgram = "tools/assistant/asistan.tr"
	chainModule  = "tools/assistant/zincir.tr"
	chainTest    = "tools/assistant/zincir_test.tr"
	chainGoTest  = "behavior_chain_test.go"
)

// Programın varsayılan model yolu ya da aygıtı yoktur: verilmezse hata verir, başka bir makinenin yolunu sessizce açmaz.
func TestChainHasNoDefaultModelOrDevice(t *testing.T) {
	program := readFile(t, chainProgram)
	for _, option := range []string{"kip", "girdi", "çıktı", "dil_modeli", "whisper", "vad", "ses_modeli", "aygıt", "sahte_mikrofon"} {
		if !regexp.MustCompile(`"` + option + `": "",`).MatchString(program) {
			t.Errorf("%s: option %q must default to the empty text", chainProgram, option)
		}
	}
	if regexp.MustCompile(`(?i)[a-z]:[/\\]|/users/|/home/`).MatchString(program) {
		t.Errorf("%s holds a local path", chainProgram)
	}
}

// Zincirin dosyaları README'de anılır; test dosyası modülü aktarır.
func TestChainFilesAreListed(t *testing.T) {
	readme := readFile(t, readmePath)
	for _, f := range []string{chainProgram, chainModule, chainTest} {
		name := f[strings.LastIndex(f, "/")+1:]
		if !strings.Contains(readme, "`"+name+"`") {
			t.Errorf("%s does not name %s", readmePath, name)
		}
	}
	if !strings.Contains(readme, "- `"+chainGoTest+"`: ") {
		t.Errorf("%s does not list %s", readmePath, chainGoTest)
	}
	if !strings.Contains(readFile(t, chainTest), `TR_import "zincir"`) {
		t.Errorf("%s does not import the chain module", chainTest)
	}
	if !strings.Contains(readFile(t, chainProgram), `TR_import "zincir"`) {
		t.Errorf("%s does not import the chain module", chainProgram)
	}
}
