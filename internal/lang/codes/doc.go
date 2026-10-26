// Package codes binds every user-visible name in TR (keywords, type names, builtin
// functions, methods and properties, named arguments) to a fixed code. Go code uses codes,
// never names; the names themselves live in a language file (tr.json for Turkish, embedded
// in the binary). Renaming a name changes only its line in the language file; codes never
// change. Other languages (en.json, pt.json ...) give their own names to the same codes. A
// concept written with international notation or a proper name (sin, exp, AdamW, json) has a
// common name: the same in every language, written once as "common_name" in definitions.json and
// never in a language file; Load gives it to every language.
//
// # Identities
//
// A code's identity is TR_<class>_<English name>, given once when the code is created and never
// changed afterwards, even if a name (the English one included) changes: TR_kw_if, TR_fn_sum,
// TR_type_Tensor, TR_arg_lr, TR_mem_step. The class says what the code is (Class, Classes):
//
//	kw    keywords (reserved in the lexer)       ctx   contextual words (olarak, test, kurucu, o)
//	val   value names (inf, nan; never coded)    type  type names (tip(), type annotations)
//	fn    builtin functions                      const builtin constants (pi)
//	arg   named arguments                        mem   methods and properties (x.name)
//	cmd   commands of tr                         sub   subcommands of 'tr dizin'
//	opt   command-line options                   repl  shell commands
//	dbg   debugger commands                      env   environment variables (TR_env_LANG for TR_LANG)
//
// When two different concepts would get the same identity, or identities that differ only in
// letter case, the owner's name is added to the identity (TR_arg_G_muon). Message codes
// (H2410, package messages) are not identities; they stay numbers. Inside the binary a code is
// a small number (its place in the language files) that never leaves the binary.
//
// # Namespaces
//
// Within one language a name must be unique only in its own namespace (see Namespace, from the
// class): global names (keywords, contextual words, value names and builtins), type names,
// members and named arguments. So TR_fn_abs mutlak (absolute value, a builtin) and
// TR_mem_absolute mutlak (absolute path, a Yol method) are separate codes. Member names are unique
// regardless of type: the compiler turns x.name into a code at compile time without looking at
// the type (for the mapping rule at the definition site see docs/spec/17_kodlar.md, member
// names).
//
// # Language file fields
//
//	"TR_fn_print": { "ad": "yazdır", "eski": ["yaz"], "aile": "giriş ve çıkış" }
//	"TR_arg_seed": { "ad": "tohum", "yabancı": ["seed"], "aile": "adlı argüman: rastgelelik" }
//
// ad is the code's name in this language. eski holds removed names: they are not accepted
// and only make the error message suggest ad; when a name is changed or removed, the old name
// is added here. yabancı holds names familiar from other languages or libraries (lr, seed,
// axis): they were never part of the language, only give hints like eski, and are specific to
// each language file (each file records its own readers' habits). Every operation has exactly
// one name: a synonym or a second spelling is not accepted. aile is a short description for
// people. Entries are one per line, in the order of Classes and by identity inside a class.
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
// (codes_generated.go): codes.FnPrint, codes.KwIf, codes.MemShape. A constant's name is the code
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
