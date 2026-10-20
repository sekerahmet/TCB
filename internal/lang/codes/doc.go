// Package codes binds every user-visible name in TR (keywords, type names, builtin
// functions, methods and properties, named arguments) to a fixed code. Go code uses codes,
// never names; the names themselves live in a language file (tr.json for Turkish, embedded
// in the binary). Renaming a name changes only its line in the language file; codes never
// change. Other languages (en.json, pt.json ...) give their own names to the same codes.
//
// # Code scheme
//
// A code is a letter and a three-digit number (A001, G100, Y400). The letter is the family;
// the hundreds digit is the group inside the family. Gaps are left between groups for later
// additions.
//
//	A  language core
//	   A0xx keywords (reserved in the lexer)
//	        A00x conditions, A01x loops, A02x functions, A03x declarations, A04x values and
//	        logic, A05x structures, A06x error handling, A07x modules, A08x concurrency,
//	        A09x debugging
//	   A1xx contextual words (meaningful only in one place: olarak, test)
//	   A2xx type names (result of tip(), type annotations); A23x-A24x engine types
//	G  general builtins
//	   G1xx input, output and files; G2xx values, types and conversion; G3xx lists;
//	   G4xx math and statistics; G5xx time and concurrency; G6xx data formats
//	   (JSON, CSV); G7xx testing
//	T  tensors and AI
//	   T1xx tensor creation and views; T2xx activations and elementwise math;
//	   T3xx autograd; T4xx optimizers; T5xx neural network layers, losses, model
//	   parameters; T6xx language models (sampling); T7xx save, load, data types;
//	   T9xx named arguments
//	Y  members: methods and properties (x.name)
//	   Y0xx shared by several types; Y1xx Metin; Y2xx Liste; Y3xx Sözlük; Y4xx Tensör;
//	   Y5xx optimizer; Y6xx dataset, reader, tokenizer;
//	   (Y7xx database, Y8xx web server, Y9xx GUI: removed)
//	V  data and tokenizer
//	   V1xx datasets; V2xx tokenizer; V9xx named arguments
//	W  web, database, GUI (removed; the codes are never reused)
//	   W1xx web client and server; W2xx database; W3xx GUI;
//	   W9xx named arguments
//	H  error messages (second stage; empty for now)
//
// # Namespaces
//
// Within one language a name must be unique only in its own namespace (see Namespace):
// global names (keywords, contextual words and G/T/V/W builtins), type names, members (Y)
// and named arguments (x9xx). So T121 birleştir (a tensor function) and Y203 birleştir (a
// list method) are separate codes. Member names are unique regardless of type: the compiler
// turns x.name into a code at compile time without looking at the type (for the mapping rule
// at the definition site see docs/spec/17_kodlar.md, member names).
//
// # Language file fields
//
//	"G100": { "ad": "yazdır", "eski": ["yaz"], "aile": "giriş ve çıkış" }
//	"T903": { "ad": "tohum", "yabancı": ["seed"], "aile": "adlı argüman: rastgelelik" }
//
// ad is the code's name in this language. eski holds removed names: they are not accepted
// and only make the error message suggest ad; when a name is changed or removed, the old name
// is added here. yabancı holds names familiar from other languages or libraries (lr, seed,
// axis): they were never part of the language, only give hints like eski, and are specific to
// each language file (each file records its own readers' habits). Every operation has exactly
// one name: a synonym or a second spelling is not accepted. aile is a short description for
// people. Entries are sorted by code, one per line.
//
// # ASCII form
//
// The form of a name with its Turkish letters folded to ASCII (yazdır → yazdir) is not stored
// in the language file; it is derived from the name and is NOT ACCEPTED: every operation has
// one spelling. A single switch, the AcceptASCII constant (off), controls this rule;
// Spellings returns the accepted spellings of a name. The ASCII form exists only for error
// messages: Suggest and CanonicalNameIn suggest the Turkish name in place of a name (or an old
// name) written in ASCII in a namespace.
//
// # Codes in Go
//
// Go code writes codes with named constants generated from the language file
// (codes_generated.go): codes.G100, codes.A001, codes.Y400. A constant's name is the code
// itself, not the user-visible name; renaming a name never changes the constant. Writing a
// code that is not in the dictionary (or has moved) breaks the build. The constants work in
// switch statements. When a code is added, removed or moved in the language file, regenerate:
//
//	go generate ./internal/lang/codes   (or: go run ./tools/gen/codeconsts)
//
// The same command refreshes the other files generated from the code list: docs/codes.md
// (what each code does, its signature and an example) and the editor's highlighting lists
// (editors/vscode/syntaxes/tr.tmLanguage.json).
//
// Layer: language core (internal/lang); imports only internal/lang packages.
package codes
