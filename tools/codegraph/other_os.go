package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/goload"
)

// Bu dosya grafiğin göremediği yapılandırmayı söyler ve oradan okunabileni okur. Tip denetimi tek yapılandırmada yapılır
// (goload: bu makinenin GOOS/GOARCH'ı); yapı kısıtı yüzünden derlenmeyen dosyalar (cuda_linux.go, file_error_unix.go,
// race_on.go ...) tiplerle okunamaz. Bütün yapılandırmaları ayrı ayrı denetleyip birleştirmek (staticcheck -matrix gibi)
// her `build`'i katlar ve aynı adlı iki bildirimi (Windows ve Linux sürümü) tek kimlikte çakıştırır. Onun yerine bu
// dosyalar tipsiz ayrıştırılır: dosya düğümü ("not built for windows/amd64"), yalnızca orada olan işlevlerin düğümü ve
// ileti ile ad kodu kullanımları (`messages.H2154`, `codes.T720`: paket seçicisi sözdiziminden çözülür) INFERRED
// kenarla girer. Çağrılar ve tür kullanımları girmez; sorgu, cevabı boş kalan pakette bunu not eder (bağımsız
// denetim Y4).

// otherConfigDetail derlenmeyen dosya düğümünün açıklamasıdır; sorgu notu bu önekle tanır.
const otherConfigDetail = "not built for "

// extractOtherConfigs derlenmeyen modül dosyalarını tipsiz okur ve grafiğe hangi yapılandırmanın görüldüğünü yazar.
func extractOtherConfigs(m *module, b *builder) {
	if len(m.Ignored) == 0 {
		return
	}
	b.warn(fmt.Sprintf("types come from %s only; %d Go files are not built for it and are read without types (message and name codes, declarations): %s",
		m.Config, len(m.Ignored), strings.Join(m.Ignored, ", ")))
	fset := token.NewFileSet()
	for _, rel := range m.Ignored {
		f, err := parser.ParseFile(fset, filepath.Join(m.Root, filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
		if err != nil {
			continue // yapılandırması olmayan bozuk dosya: go list zaten bildirir
		}
		pkg := m.Path
		if dir := path.Dir(rel); dir != "." {
			pkg += "/" + dir
		}
		b.addNode(Node{ID: fileID(rel), Kind: kindFile, Name: rel, File: rel, Line: 1, Detail: otherConfigDetail + m.Config})
		b.addEdge(pkgID(pkg), fileID(rel), edgeContains, extracted, rel, 0)
		imports := packageImports(f)
		for _, decl := range f.Decls {
			owner := fileID(rel)
			if d, ok := decl.(*ast.FuncDecl); ok {
				owner = otherConfigFunc(b, fset, pkg, rel, d, m.Config)
			}
			locals := declaredNames(decl)
			ast.Inspect(decl, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.SelectorExpr:
					codeUse(b, m.Path, imports, n, owner, rel, fset.Position(n.Pos()).Line)
				case *ast.CallExpr:
					if to := callTarget(b, m.Path, pkg, imports, locals, n); to != "" && to != owner {
						b.addEdge(owner, to, edgeCalls, extracted, rel, fset.Position(n.Pos()).Line)
					}
				}
				return true
			})
		}
	}
}

// otherConfigFunc derlenmeyen dosyadaki işlevin kimliğidir. Aynı ad bu yapılandırmada da tanımlıysa (Windows ve Linux
// sürümü) o düğüm kalır, kullanım ona bağlanır; yalnızca burada tanımlıysa düğümü eklenir.
func otherConfigFunc(b *builder, fset *token.FileSet, pkg, rel string, d *ast.FuncDecl, config string) string {
	recv, kind := "", kindFunc
	if d.Recv != nil && len(d.Recv.List) == 1 {
		recv, kind = goload.RecvExprName(d.Recv.List[0].Type), kindMethod
	}
	id := memberID(pkg, recv, d.Name.Name)
	if d.Name.Name == "init" || d.Name.Name == "_" {
		id += "#" + path.Base(rel) + ":" + strconv.Itoa(fset.Position(d.Pos()).Line)
	}
	qual := path.Base(pkg)
	if recv != "" {
		qual += "." + recv
	}
	b.addNode(Node{ID: id, Kind: kind, Name: qual + "." + d.Name.Name, File: rel, Line: fset.Position(d.Pos()).Line,
		End: fset.Position(d.End()).Line, Doc: docText(d.Doc), Detail: otherConfigDetail + config})
	b.addEdge(fileID(rel), id, edgeContains, extracted, rel, fset.Position(d.Pos()).Line) // öbür sürümün yeri de görünür
	return id
}

// packageImports dosyanın içe aktardığı paketlerin yerel adlarıdır (ad -> yol); adsız içe aktarmada yolun son öğesi.
func packageImports(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, is := range f.Imports {
		p, err := strconv.Unquote(is.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(p)
		if is.Name != nil {
			name = is.Name.Name
		}
		out[name] = p
	}
	return out
}

// codeUse `messages.H2154` ya da `codes.T720` seçicisini ileti ya da ad kodu kullanımı olarak bağlar.
func codeUse(b *builder, mod string, imports map[string]string, sel *ast.SelectorExpr, owner, rel string, line int) {
	x, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}
	switch imports[x.Name] {
	case mod + "/" + messagesDir:
		if messageConst.MatchString(sel.Sel.Name) {
			b.addEdge(owner, "msg:"+sel.Sel.Name, edgeUsesMessage, inferred, rel, line)
		}
	case mod + "/" + codesDir:
		if nameConst.MatchString(sel.Sel.Name) {
			b.addEdge(owner, "name:"+sel.Sel.Name, edgeUsesName, inferred, rel, line)
		}
	}
}

// callTarget derlenmeyen dosyadaki düz bir işlev çağrısının hedefidir: aynı paketin `F(...)`'i ya da bir modül paketinin
// `pkg.F(...)`'i, hedef düğüm varsa. Ad paket kapsamında çözülür (derleyicinin kuralı); işlevin kendisi aynı adı
// tanımlıyorsa (parametre, yerel) gölgelenmiş sayılır ve bağlanmaz. Yöntem çağrıları tip ister, bağlanmaz. Böylece
// başka işletim sisteminin testi (mmap_unix_test.go: TestMmapTruncatedDoesNotCrash → OpenShard) zincire girer.
func callTarget(b *builder, mod, pkg string, imports map[string]string, locals map[string]bool, c *ast.CallExpr) string {
	id := ""
	switch fn := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		if !locals[fn.Name] {
			id = memberID(pkg, "", fn.Name)
		}
	case *ast.SelectorExpr:
		if x, ok := fn.X.(*ast.Ident); ok && !locals[x.Name] {
			if p := imports[x.Name]; p == mod || strings.HasPrefix(p, mod+"/") {
				id = memberID(p, "", fn.Sel.Name)
			}
		}
	}
	if b.nodes[id] == nil {
		return ""
	}
	return id
}

// declaredNames bir bildirimin içinde tanımlanan adlardır (parametre, sonuç, alıcı, :=, var, range değişkeni).
func declaredNames(decl ast.Decl) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(decl, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Field:
			for _, id := range n.Names {
				out[id.Name] = true
			}
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE {
				for _, l := range n.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						out[id.Name] = true
					}
				}
			}
		case *ast.ValueSpec:
			for _, id := range n.Names {
				out[id.Name] = true
			}
		case *ast.RangeStmt:
			for _, e := range []ast.Expr{n.Key, n.Value} {
				if id, ok := e.(*ast.Ident); ok && n.Tok == token.DEFINE {
					out[id.Name] = true
				}
			}
		}
		return true
	})
	if f, ok := decl.(*ast.FuncDecl); ok {
		delete(out, f.Name.Name) // işlevin kendi adı alan listesinde geçmez; özyineleme kendine çağrıdır
	}
	return out
}
