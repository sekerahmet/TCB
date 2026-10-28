package codes

//go:generate go run ../../../tools/gen/codeconsts
//go:generate go run ../../../tools/gen/codesdoc -root ../../..
//go:generate go run ../../../tools/gen/grammar -root ../../..
//go:generate go run ../../../tools/gen/snippets -root ../../..

import (
	"fmt"
	"maps"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Code is a name's place in the code list inside the binary. Files, messages and tools write
// a code by its identity (TR_fn_sum, String); the number is only an index and an order (the
// language files' order) and never leaves the binary. Go code writes codes with the generated
// named constants (codes.FnSum).
type Code uint16

// None is the zero value returned for a name that has no code.
const None Code = 0

// IdentityPrefix starts every identity: TR_<class>_<English name> (TR_kw_if, TR_fn_sum,
// TR_env_LANG). Names starting with it are reserved (IsReservedName).
const IdentityPrefix = "TR_"

// Class is the class part of a code's identity (TR_fn_sum → fn). It is written in the
// identity once and never changes; the code's namespace and kind follow from it.
type Class string

const (
	ClassKeyword     Class = "kw"    // keywords, reserved in the lexer
	ClassContextual  Class = "ctx"   // contextual words (olarak, test, kurucu, o ...)
	ClassValueName   Class = "val"   // value names, bound like constants (TR_inf, TR_nan; 3.2)
	ClassType        Class = "type"  // type names
	ClassFunction    Class = "fn"    // builtin functions
	ClassConstant    Class = "const" // builtin constants (pi)
	ClassNamedArg    Class = "arg"   // named arguments
	ClassMember      Class = "mem"   // methods and properties
	ClassCommand     Class = "cmd"   // commands of tr
	ClassSubcommand  Class = "sub"   // subcommands of 'tr dizin'
	ClassOption      Class = "opt"   // command-line options
	ClassShell       Class = "repl"  // shell commands
	ClassDebugger    Class = "dbg"   // terminal debugger commands
	ClassEnvironment Class = "env"   // environment variables (the identity drops the TR_ of the name)
)

// Classes lists the classes in the order the language files list their codes (inside a class,
// codes are sorted by identity), with the namespace each class's names live in.
var Classes = []struct {
	Class     Class
	Namespace Namespace
}{
	{ClassKeyword, NamespaceGlobal},
	{ClassContextual, NamespaceGlobal},
	{ClassValueName, NamespaceGlobal},
	{ClassType, NamespaceType},
	{ClassFunction, NamespaceGlobal},
	{ClassConstant, NamespaceGlobal},
	{ClassNamedArg, NamespaceParam},
	{ClassMember, NamespaceMember},
	{ClassCommand, NamespaceCommand},
	{ClassSubcommand, NamespaceSubcommand},
	{ClassOption, NamespaceOption},
	{ClassShell, NamespaceCommand},
	{ClassDebugger, NamespaceDebugger},
	{ClassEnvironment, NamespaceEnvironment},
}

// tables kod listesinden (codes_generated.go'daki identities) bir kez kurulur: kodun sınıfı ve ad
// alanı, kimlikten koda eşleme. Paket değişkenidir (init değil): dil listeleri de paket
// değişkeniyle kurulur ve Parse'a başvurur; Go başlatma sırasını başvurulardan çıkarır.
var tables = buildTables()

type codeTables struct {
	class      []Class
	namespace  []Namespace
	byIdentity map[string]Code
}

func buildTables() codeTables {
	ns := map[Class]Namespace{}
	for _, c := range Classes {
		ns[c.Class] = c.Namespace
	}
	t := codeTables{class: make([]Class, len(identities)), namespace: make([]Namespace, len(identities)),
		byIdentity: make(map[string]Code, len(identities))}
	for k, id := range identities {
		if id == "" {
			continue
		}
		c := IdentityClass(id)
		t.class[k], t.namespace[k] = c, ns[c]
		t.byIdentity[id] = Code(k)
	}
	return t
}

// IdentityClass is the class part of an identity (TR_fn_sum → fn); empty when s does not have
// the identity form.
func IdentityClass(s string) Class {
	rest, ok := strings.CutPrefix(s, IdentityPrefix)
	if !ok {
		return ""
	}
	c, name, ok := strings.Cut(rest, "_")
	if !ok || name == "" {
		return ""
	}
	return Class(c)
}

// String writes the code's identity (TR_kw_if, TR_mem_step).
func (k Code) String() string {
	if int(k) <= 0 || int(k) >= len(identities) {
		return fmt.Sprintf("Kod(%d)", uint16(k))
	}
	return identities[k]
}

// Parse returns the code of an identity (TR_fn_sum).
func Parse(s string) (Code, error) {
	if k, ok := tables.byIdentity[s]; ok {
		return k, nil
	}
	return None, messages.Internalf("invalid code %q: not an identity in the code list (e.g. TR_fn_print)", s)
}

// IsReservedName reports whether s starts with IdentityPrefix: such names are reserved for the
// identities of the language and its library (files are stored with identities; see
// docs/spec/02_sozcuk_yapisi.md 2.3a) and cannot be user names, whether or not the identity
// exists yet.
func IsReservedName(s string) bool { return strings.HasPrefix(s, IdentityPrefix) }

// Class is the class part of the code's identity; empty for an invalid code.
func (k Code) Class() Class {
	if int(k) >= len(tables.class) {
		return ""
	}
	return tables.class[k]
}

// Namespace is the scope in which a name must be unique.
type Namespace uint8

const (
	NamespaceNone        Namespace = iota // a code with no namespace (an invalid code)
	NamespaceGlobal                       // keywords, contextual words, value names, builtins
	NamespaceType                         // type names
	NamespaceMember                       // methods and properties
	NamespaceParam                        // named arguments
	NamespaceCommand                      // commands of tr and the shell
	NamespaceOption                       // command-line options, written --name: 'kontrol' there is not 'tr kontrol'
	NamespaceDebugger                     // terminal debugger commands: its 'izle' is not 'tr izle'
	NamespaceSubcommand                   // subcommands of 'tr dizin': 'tr tanım' is not a command
	NamespaceEnvironment                  // environment variables: TR_DIL, TR_GPU_ESZAMANLI ...
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
	case NamespaceSubcommand:
		return "index subcommands"
	case NamespaceEnvironment:
		return "environment variables"
	}
	return "no namespace"
}

// Namespace is the code's namespace, from its class.
func (k Code) Namespace() Namespace {
	if int(k) >= len(tables.namespace) {
		return NamespaceNone
	}
	return tables.namespace[k]
}

// IsIndexSubcommand reports whether the code is a subcommand of 'tr dizin' (TR_cmd_index). They
// live in their own namespace, so a program or shell script cannot call them as top-level
// commands ('tr tanım' is an error, 'tr dizin tanım' works).
func (k Code) IsIndexSubcommand() bool { return k.Class() == ClassSubcommand }

// IsKeyword reports whether the code is a keyword reserved in the lexer.
func (k Code) IsKeyword() bool { return k.Class() == ClassKeyword }

// IsBuiltin reports whether the code is a builtin registered in the global scope (a function,
// a constant or a value name of the core: TR_inf, TR_nan, 3.2; a library code is not a builtin).
func (k Code) IsBuiltin() bool {
	c := k.Class()
	return (c == ClassFunction || c == ClassConstant || c == ClassValueName) && !k.IsLibrary()
}

// IsLibrary reports whether the code is a public name of a standard library module (tr/…;
// definitions.json "module"): it comes with the module's import, not with the core.
func (k Code) IsLibrary() bool {
	_, ok := libraryModules[k]
	return ok
}

// SetLibraryModulesForTest marks the given codes as library codes of the given modules until the
// returned function is called (tests check the library mechanism with made-up library codes). It
// is not safe for parallel tests.
func SetLibraryModulesForTest(modules map[Code]string) (restore func()) {
	old := libraryModules
	libraryModules = maps.Clone(old)
	maps.Copy(libraryModules, modules)
	return func() { libraryModules = old }
}
