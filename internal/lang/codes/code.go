package codes

//go:generate go run ../../../tools/gen/codeconsts
//go:generate go run ../../../tools/gen/codesdoc -root ../../..
//go:generate go run ../../../tools/gen/grammar -root ../../..

import (
	"fmt"
)

// Code is the language-independent, unchanging identity of a name. Its value is
// family*1000 + number: A001 = A+1, Y400 = Y+400.
type Code uint16

// Family bases. Go code writes codes with the generated named constants (G100, A001), not
// with these; the family + number form is used only inside this package, in the generated
// file. The numeric values exist only inside the binary; the language file stores codes as
// text (G100).
const (
	A Code = 1000 * (iota + 1) // dilin çekirdeği: anahtar kelimeler, tip adları
	G                          // genel yerleşikler
	H                          // hata mesajları (ikinci aşama)
	K                          // komutlar: tr alt komutları, seçenekler, kabuk ve hata ayıklayıcı
	T                          // tensör ve yapay zekâ
	V                          // veri ve parçalayıcı
	W                          // web, veritabanı, arayüz (kaldırıldı)
	Y                          // yöntemler: metot ve özellikler
)

// familyLetters alfabe sırasındadır; böylece kodların sayısal sırası dil dosyasındaki
// (harf, sayı) sırasıyla aynıdır.
const familyLetters = "AGHKTVWY"

// None is the zero value returned for a name that has no code.
const None Code = 0

// Letter is the code's family letter ('A', 'G' ...); 0 for an invalid code.
func (k Code) Letter() byte {
	i := int(k)/1000 - 1
	if i < 0 || i >= len(familyLetters) {
		return 0
	}
	return familyLetters[i]
}

// Number is the code's number inside its family (0-999).
func (k Code) Number() int { return int(k) % 1000 }

// Group is the code's hundreds digit (A013 → 0, T402 → 4).
func (k Code) Group() int { return k.Number() / 100 }

// String writes the code as in the language file (A001, Y400).
func (k Code) String() string {
	if k.Letter() == 0 {
		return fmt.Sprintf("Kod(%d)", uint16(k))
	}
	return fmt.Sprintf("%c%03d", k.Letter(), k.Number())
}

// Parse converts text such as "A001" to a code.
func Parse(s string) (Code, error) {
	if len(s) != 4 {
		return None, internalf("invalid code %q: want one letter and three digits (e.g. G100)", s)
	}
	family := -1
	for i := 0; i < len(familyLetters); i++ {
		if s[0] == familyLetters[i] {
			family = i
		}
	}
	if family < 0 {
		return None, internalf("invalid code %q: family letter must be one of %s", s, familyLetters)
	}
	n := 0
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return None, internalf("invalid code %q: three digits must follow the letter", s)
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return None, internalf("invalid code %q: 000 is not used", s)
	}
	return Code(1000*(family+1) + n), nil
}

// IsCodeForm reports whether s has the form of a code: an upper-case letter and three digits
// (G100, A001, K001). Names of this form are reserved (files are stored with codes; see
// docs/spec/02_sozcuk_yapisi.md 2.3a) and cannot be user names. The family letter may be one
// not used yet (for future families).
func IsCodeForm(s string) bool {
	if len(s) != 4 || s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	for i := 1; i < 4; i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Namespace is the scope in which a name must be unique.
type Namespace uint8

const (
	NamespaceNone     Namespace = iota // a code with no namespace (H: error messages)
	NamespaceGlobal                    // keywords, contextual words, G/T/V/W builtins
	NamespaceType                      // type names (A2xx)
	NamespaceMember                    // methods and properties (Y)
	NamespaceParam                     // named arguments (the x9xx groups in G/T/V/W)
	NamespaceCommand                   // commands of tr and the shell (K0xx, K2xx)
	NamespaceOption                    // command-line options, written --name (K1xx): 'kontrol' there is not 'tr kontrol'
	NamespaceDebugger                  // terminal debugger commands (K3xx): its 'izle' is not 'tr izle'
	namespaceCount
)

func (a Namespace) String() string {
	switch a {
	case NamespaceGlobal:
		return "global names"
	case NamespaceType:
		return "type names"
	case NamespaceMember:
		return "members"
	case NamespaceParam:
		return "named arguments"
	case NamespaceCommand:
		return "commands"
	case NamespaceOption:
		return "command-line options"
	case NamespaceDebugger:
		return "debugger commands"
	}
	return "no namespace"
}

// Namespace is the code's namespace.
func (k Code) Namespace() Namespace {
	switch k.Letter() {
	case 'A':
		if k.Group() >= 2 {
			return NamespaceType
		}
		return NamespaceGlobal
	case 'G', 'T', 'V', 'W':
		if k.Group() == 9 {
			return NamespaceParam
		}
		return NamespaceGlobal
	case 'Y':
		return NamespaceMember
	case 'K':
		switch k.Group() {
		case 1:
			return NamespaceOption
		case 3:
			return NamespaceDebugger
		}
		return NamespaceCommand
	}
	return NamespaceNone
}

// IsKeyword reports whether the code is a keyword reserved in the lexer (A0xx).
func (k Code) IsKeyword() bool { return k.Letter() == 'A' && k.Group() == 0 }

// IsBuiltin reports whether the code is a builtin function registered in the global scope
// (G/T/V/W, except x9xx).
func (k Code) IsBuiltin() bool {
	switch k.Letter() {
	case 'G', 'T', 'V', 'W':
		return k.Group() != 9
	}
	return false
}
