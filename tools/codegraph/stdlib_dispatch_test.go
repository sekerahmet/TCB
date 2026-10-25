package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/devtools/goload"
)

var stdlibFiles = map[string]string{
	"internal/lang/messages/messages.go": "package messages\n\nfunc Text(c int, args ...any) string { return \"\" }\n",
	"kinds/kinds.go": `package kinds

import (
	"encoding/json"
	"fmt"
	"sort"

	"example.com/audit/internal/lang/messages"
)

type Code uint16

func (c Code) String() string { return "" }

type Fail struct{}

func (*Fail) Error() string { return "" }

type Inner struct{ Count int }

type Snap struct {
	Start  string ` + "`json:\"başlangıç\"`" + `
	Hidden string ` + "`json:\"-\"`" + `
	Rows   []Inner
	local  int
}

type byLen []string

func (b byLen) Len() int           { return len(b) }
func (b byLen) Less(i, j int) bool { return b[i] < b[j] }
func (b byLen) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

func code() Code { return 0 }

func Describe(k Code, list []Code, f *Fail) string {
	_ = messages.Text(1, list[0])
	_ = fmt.Errorf("%w", f)
	return fmt.Sprintf("%s %v", k, code())
}

func Save(s *Snap) ([]byte, error) { return json.MarshalIndent(s, "", " ") }

func Order(names []string) { sort.Sort(byLen(names)) }

func Plain(k Code) string { return fmt.Sprint(uint16(k)) }
`,
}

// Kural: bir değer standart kitaplığa arayüz tipli parametreyle verildiğinde, kitaplığın onun üzerinde çağırdığı metotlar
// ve okuduğu alanlar who-uses'ta INFERRED kenardır; kenarın türü kullanımın rolüdür: biçimleme (fmt, log, testing, ileti
// paketi: String, Error, Format, GoString) `formats`, JSON (dışa açık ve `json:"-"` olmayan alanlar, iç içe yapılarınki
// dahil; MarshalJSON ...) `encodes`, boş olmayan arayüz (sort.Interface) `stdlib_calls`. Argümanın tipi tanımlayıcıdan,
// dilim öğesinden, gösterici argümandan, dönüşümden ve çağrı sonucundan çözülür; temel tipe dönüştürülmüş değer
// (`uint16(k)`) metodunu taşımaz (eksikler listesi kritik 3: codes.Code.String fmt ile, profiler.Snapshot.Start JSON ile).
func TestStandardLibraryDispatchIsAUseWithItsRole(t *testing.T) {
	g, err := buildGraph(auditModule(t, stdlibFiles))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind == edgeFormats || e.Kind == edgeEncodes || e.Kind == edgeStdlibCalls {
			if e.Confidence != inferred {
				t.Errorf("%+v is not INFERRED", e)
			}
			got[fmt.Sprintf("%s %s %s", strings.TrimPrefix(e.From, "go:func:example.com/audit/kinds."), e.Kind,
				e.To[strings.LastIndex(e.To, ".")+1:])] = true
		}
	}
	want := []string{"Describe formats String", "Describe formats Error", "Save encodes Start", "Save encodes Rows",
		"Save encodes Count", "Order stdlib_calls Len", "Order stdlib_calls Less", "Order stdlib_calls Swap"}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing %q (got %v)", w, got)
		}
	}
	for k := range got {
		if !slices.Contains(want, k) {
			t.Errorf("unexpected %q: Hidden is json:\"-\", local is not exported, Plain formats a uint16", k)
		}
	}
}

// Kural (sınıf): depoda standart kitaplığın biçimleyicisine ya da encoding/json'a verilen her modül değeri için kenar
// vardır: tipinin String/Error/Format/GoString metoduna `formats`, JSON'da dışa açık alanlarına `encodes`. Beklenen
// küme çıkarıcıdan bağımsız, tam go/types ile (Info.Types: her argümanın tipi) kurulur; çıkarıcı tip bilgisi
// yüklemeden çözer, çözemediği argüman biçimi burada eksik çıkar.
func TestEveryStandardLibraryDispatchHasAnEdge(t *testing.T) {
	root, _, g := repoGraph(t)
	m, err := goload.Load(root, goload.Options{Types: true, Selections: true})
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, e := range g.Edges {
		have[e.From+" "+e.Kind+" "+e.To] = true
	}
	fields := fieldsByDecl(g)
	var misses []string
	checked := 0
	expect := func(owners []string, kind, to, where string) {
		if to == "" {
			return // yerel tipin alanı: grafikte düğümü yok (alan düğümleri paket düzeyi yapılarındır)
		}
		checked++
		for _, o := range owners {
			if have[o+" "+kind+" "+to] {
				return
			}
		}
		misses = append(misses, fmt.Sprintf("%s: %v %s %s", where, owners, kind, to))
	}
	for _, u := range m.Units {
		for _, f := range u.Files {
			file := m.Rel(m.Fset.Position(f.Pos()).Filename)
			for _, d := range f.Decls {
				owners := declOwners(m, u, file, d)
				ast.Inspect(d, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					kind := dispatchRole(m.Path, u, call)
					if kind == "" {
						return true
					}
					sig := u.Info.Uses[nameIdent(call.Fun)].Type().(*types.Signature)
					for i, arg := range call.Args {
						pt := paramType(sig, i, call.Ellipsis.IsValid())
						if pt == nil {
							continue
						}
						if it, ok := pt.Underlying().(*types.Interface); !ok || !it.Empty() {
							continue
						}
						named, ok := types.Unalias(derefType(u.Info.TypeOf(arg))).(*types.Named)
						if !ok || named.Obj().Pkg() == nil || !strings.HasPrefix(named.Obj().Pkg().Path(), m.Path) {
							continue
						}
						where := fmt.Sprintf("%s:%d", file, m.Fset.Position(arg.Pos()).Line)
						names := formatMethods
						if kind == edgeEncodes {
							names = encodeMethods
							if st, ok := named.Underlying().(*types.Struct); ok {
								for i := 0; i < st.NumFields(); i++ {
									fl := st.Field(i)
									if fl.Exported() && !strings.HasPrefix(reflect.StructTag(st.Tag(i)).Get("json"), "-") {
										p := m.Fset.Position(fl.Pos())
										expect(owners, kind, fields[fmt.Sprintf("%s:%d:%s", m.Rel(p.Filename), p.Line, fl.Name())], where)
									}
								}
							}
						}
						for _, fn := range namedMethods(named, names) {
							expect(owners, kind, typesID(m.Path, fn.Origin()), where)
						}
					}
					return true
				})
			}
		}
	}
	if len(misses) > 0 {
		t.Errorf("%d of %d standard library dispatches have no edge, first %d:\n  %s", len(misses), checked,
			min(len(misses), 30), strings.Join(misses[:min(len(misses), 30)], "\n  "))
	}
	if checked < 200 {
		t.Errorf("only %d dispatches checked; the repository formats and encodes more", checked)
	}
}

// dispatchRole çağrının biçimleme ya da JSON çağrısı olup olmadığıdır (testin kendi tablosu; argümanın parametresi
// boş arayüz olmalı).
func dispatchRole(mod string, u *goload.Unit, call *ast.CallExpr) string {
	fn, ok := u.Info.Uses[nameIdent(call.Fun)].(*types.Func)
	if !ok || fn.Pkg() == nil {
		return ""
	}
	switch p := fn.Pkg().Path(); {
	case p == "fmt", p == "log", p == mod+"/internal/lang/messages", p == "testing" && (strings.HasPrefix(fn.Name(), "Log") ||
		strings.HasPrefix(fn.Name(), "Error") || strings.HasPrefix(fn.Name(), "Fatal") || strings.HasPrefix(fn.Name(), "Skip")):
		return edgeFormats
	case p == "encoding/json" && slices.Contains([]string{"Marshal", "MarshalIndent", "Unmarshal", "Encode", "Decode"}, fn.Name()):
		return edgeEncodes
	}
	return ""
}
