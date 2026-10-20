package main

import (
	"path"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// importTarget bir `aktar` yolunu dosyaya çevirir: "tr/ad" → lib/ad.tr, öbürleri içe aktaran
// dosyanın klasörüne göre (".tr" uzantısı isteğe bağlıdır). Bulunamazsa "".
func (x *trExtractor) importTarget(f *trFile, p string) string {
	cands := []string{path.Join(path.Dir(f.rel), p)}
	if rest, ok := strings.CutPrefix(p, "tr/"); ok {
		cands = append([]string{path.Join("lib", rest)}, cands...)
	}
	for _, c := range cands {
		for _, name := range []string{c, c + ".tr"} {
			if strings.HasSuffix(name, ".tr") && x.hasFile(name) {
				return name
			}
		}
	}
	return ""
}

func (x *trExtractor) hasFile(rel string) bool {
	_, ok := x.b.nodes[trFileID(rel)]
	return ok
}

// resolveImports bir dosyada görünen adları kurar: kendi tanımları, takma adsız aktarılan
// modüllerin (dolaylı olanlar dahil) tanımları ve `olarak` ile verilen takma adlar.
func (x *trExtractor) resolveImports(f *trFile) {
	f.visible = map[string]string{}
	for name, id := range f.defs {
		f.visible[name] = id
	}
	seen := map[string]bool{f.rel: true}
	var merge func(g *trFile)
	merge = func(g *trFile) {
		for _, imp := range g.imports {
			target := x.files[imp.file]
			if target == nil || seen[imp.file] {
				continue
			}
			if imp.alias != "" {
				if g == f {
					f.aliases[imp.alias] = target
				}
				continue
			}
			seen[imp.file] = true
			for name, id := range target.defs {
				if _, taken := f.visible[name]; !taken {
					f.visible[name] = id
				}
			}
			merge(target)
		}
	}
	merge(f)
	for _, imp := range f.imports {
		if imp.file != "" {
			x.b.addEdge(trFileID(f.rel), trFileID(imp.file), edgeImports, extracted, f.rel, 0)
		}
	}
}

// owner bir kenarın çıkış düğümüdür.
type trOwner struct {
	id   string
	kind string
}

// edges dosyadaki her bildirimin gövdesini tarar; bildirim dışı üst düzey ifadeler dosya
// düğümünden çıkar.
func (x *trExtractor) edges(f *trFile) {
	file := trOwner{trFileID(f.rel), kindFile}
	for _, s := range f.res.Program.Statements {
		switch s := s.(type) {
		case *ast.ExpressionStatement:
			if fl, ok := s.Expression.(*ast.FunctionLiteral); ok && fl.Name != "" {
				x.scan(f, trOwner{"tr:func:" + f.rel + ":" + fl.Name, kindTRFunc}, fl)
				continue
			}
			x.scan(f, file, s)
		case *ast.StructStatement:
			x.scanStruct(f, s)
		case *ast.TestStatement:
			x.scan(f, trOwner{"tr:test:" + f.rel + ":" + strconv.Itoa(s.Line()), kindTRTest}, s.Body)
		case *ast.VarStatement:
			if s.Value != nil {
				kind := "var"
				if s.IsConst {
					kind = "const"
				}
				x.scan(f, trOwner{"tr:" + kind + ":" + f.rel + ":" + s.Name.Value, kindTRVar}, s.Value)
			}
		case *ast.ImportStatement:
		default:
			x.scan(f, file, s)
		}
	}
}

func (x *trExtractor) scanStruct(f *trFile, s *ast.StructStatement) {
	for _, fld := range s.Fields {
		if fld.DefaultValue != nil {
			x.scan(f, trOwner{"tr:field:" + f.rel + ":" + s.Name + "." + fld.Name, kindTRField}, fld.DefaultValue)
		}
	}
	for n, fl := range s.Methods {
		x.scan(f, trOwner{"tr:method:" + f.rel + ":" + s.Name + "." + n, kindTRMethod}, fl)
	}
}

// nameCode bir ad kodunun düğümüne gidecek kimlik (yerleşik: G, T, V, Y... aileleri; anahtar
// sözcükler, ileti ve komut kodları sayılmaz).
func nameCode(k codes.Code) (string, bool) {
	if k == codes.None || !k.IsBuiltin() {
		return "", false
	}
	return "name:" + k.String(), true
}

// link bir kenar ekler; test sahibinden çıkan çağrı aynı zamanda `tests` kenarıdır.
func (x *trExtractor) link(o trOwner, to, kind, conf, file string, line int) {
	x.b.addEdge(o.id, to, kind, conf, file, line)
	if o.kind == kindTRTest && kind == edgeCalls {
		x.b.addEdge(o.id, to, edgeTests, conf, file, line)
	}
}
