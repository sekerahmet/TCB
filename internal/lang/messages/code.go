// Package messages, kullanıcıya giden her iletiyi (hata, uyarı, bilgi) değişmez bir koda bağlar:
// H7003 → "{fn}: {path|q} bir klasör; CSV dosyasının yolunu verin". Adlar için internal/lang/codes
// ne ise iletiler için bu pakettir: metinler dil dosyalarında (tr.json), dilden bağımsız tanımlar
// (parametre adları) definitions.json'dadır. Go kodu ileti metni yazmaz, kodu yazar
// (messages.New(messages.H7003, fn, path)); metin etkin dilde, hata yazılırken kurulur.
//
// Layer: language core (internal/lang); dil çekirdeğinin en alt paketidir, yalnızca standart
// kitaplığı içe aktarır (codes ve turkish de onu kullanabilir).
package messages

//go:generate go run ../../../tools/gen/msgconsts

import (
	"fmt"
	"strconv"
)

// Code is the language-independent, unchanging identity of a message: H followed by four
// digits (H0000-H9999). The thousands digit is the area (H0 syntax, H1 type check,
// H2 interpreter, H3 tensors and CPU, H4 GPU, H5 data and tokenizer, H6 stdlib/ml, H7 the rest
// of the standard library, H8 tools, H9 internal errors); numbers are never reused.
type Code uint16

// LanguageEnvVar selects the active writing language (empty: tr); codes.LanguageEnvVar is
// the same variable. It is the source-language name of the name code K400.
const LanguageEnvVar = "TR_DIL"

// LanguageEnvVars, LanguageEnvVar'ın bütün gömülü yazım dillerindeki adlarıdır, kaynak dil önce:
// öbür ortam değişkenleri gibi her dilin adıyla okunur (codes.EnvValue, dil tanımı 17.8). Bu paket
// dil listelerinden önce okunduğu için adlar burada yazılıdır; codes'un
// TestLanguageEnvVarsAreK400 testi listeyi K400'ün adlarıyla aynı tutar.
var LanguageEnvVars = []string{LanguageEnvVar, "TR_LANG"}

// String writes the code as in the language file (H7003).
func (c Code) String() string { return fmt.Sprintf("H%04d", uint16(c)) }

// Parse converts text such as "H7003" to a code.
func Parse(s string) (Code, error) {
	if len(s) != 5 || s[0] != 'H' {
		return 0, fmt.Errorf("message code %q is invalid: H and four digits (e.g. H7003)", s)
	}
	n, err := strconv.Atoi(s[1:])
	if err != nil || n < 0 {
		return 0, fmt.Errorf("message code %q is invalid: H and four digits (e.g. H7003)", s)
	}
	return Code(n), nil
}
