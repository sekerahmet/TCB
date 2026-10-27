package interp

import (
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// Komut satırı ulaşılabilirliği (1.4): argüman verilen bir programda programın en dışından
// ulaşılan bir komut_satırı çağrısı yoksa program çalışmaz. İnceleme kaynak metin üzerindedir ve
// ad tabanlıdır; kesin bilinemeyen her yerde çağrı VAR sayılır (yanlışlıkla "yok" demek, çalışan
// bir programı reddetmek olurdu):
//
//   - Adlandırılmış fonksiyon ve yapı, kendi adıyla anıldığı (çağrıldığı ya da değer olarak
//     geçirildiği) birim ulaşılabilir olunca ulaşılabilir olur; üye çağrısı (m.f(), x.metot())
//     o adı taşıyan her fonksiyonu ve yapıyı ulaşılabilir kılar.
//   - Fonksiyon değeriyle çağrı (bir değişken, parametre ya da ifade çağrılıyor) çağrı var sayılır.
//   - Aktarılan modülün en dışı aktarma ile ulaşılabilir olur; çözülemeyen modül çağrı var
//     sayılır. Dilin kitaplık modülleri komut satırını okumaz.
//   - Test blokları programın normal çalışmasında atlanır.

// reachFile incelenen bir dosyadır.
type reachFile struct {
	path    string
	program *ast.Program
}

// reachScan bir incelemenin durumudur.
type reachScan struct {
	vm      *VM
	main    string
	files   map[string]*reachFile // mutlak yol → dosya
	funcs   map[string][]*ast.FunctionLiteral
	structs map[string][]*ast.StructStatement
	methods map[string][]*ast.StructStatement // üye adı → o adlı yöntemi olan yapılar
	// assigned, yeni değer atanan adlardır: adlı bir fonksiyon sonradan başka bir değerle
	// değiştirilebiliyorsa çağrısı fonksiyon değeriyle çağrı sayılır.
	assigned map[string]bool
	// bound, parametre ya da değişken olarak bağlanan adlardır: böyle bir adın çağrısı bir
	// fonksiyon değeriyle çağrıdır (aynı adlı bir fonksiyon ya da yerleşik olsa bile).
	bound   map[string]bool
	reached map[any]bool
	queue   []any
	found   bool
}

// ReadsCommandLine, programın en dışından ulaşılabilen bir komut_satırı çağrısı olup olmadığını
// söyler; kesin bilinemiyorsa true döner (1.4). prog ana programdır, v.SourceFile yeri.
func (v *VM) ReadsCommandLine(prog *ast.Program) bool {
	s := &reachScan{
		vm: v, main: v.SourceFile,
		files:    map[string]*reachFile{},
		funcs:    map[string][]*ast.FunctionLiteral{},
		structs:  map[string][]*ast.StructStatement{},
		methods:  map[string][]*ast.StructStatement{},
		assigned: map[string]bool{},
		bound:    map[string]bool{},
		reached:  map[any]bool{},
	}
	s.reach(s.addFile(absPath(v.SourceFile), v.SourceFile, prog))
	for len(s.queue) > 0 && !s.found {
		unit := s.queue[0]
		s.queue = s.queue[1:]
		s.scan(unit)
	}
	return s.found
}

// addFile dosyayı ve bildirimlerini kaydeder; path dosyanın yolu (modül yollarının çözümünde
// aktaranın klasörü), key tekrar eklenmesini önleyen mutlak yoludur.
func (s *reachScan) addFile(key, path string, prog *ast.Program) *reachFile {
	f := &reachFile{path: path, program: prog}
	s.files[key] = f
	ast.Walk(prog, func(n any) bool {
		switch x := n.(type) {
		case *ast.FunctionLiteral:
			if x.Name != "" {
				s.funcs[x.Name] = append(s.funcs[x.Name], x)
			}
			for _, p := range x.Parameters {
				s.bound[p.Name] = true
			}
		case *ast.StructStatement:
			s.structs[x.Name] = append(s.structs[x.Name], x)
			for name := range x.Methods {
				s.methods[name] = append(s.methods[name], x)
			}
		case *ast.AssignStatement:
			if id, ok := x.Target.(*ast.Identifier); ok {
				s.assigned[id.Value] = true
			}
		case *ast.VarStatement:
			s.bound[x.Name.Value] = true
		case *ast.MultiVarStatement:
			for _, id := range x.Names {
				s.bound[id.Value] = true
			}
		case *ast.ForInStatement:
			s.bound[x.KeyVar], s.bound[x.ValueVar] = true, true
		case *ast.TryCatchStatement:
			s.bound[x.ErrorVar] = true
		case *ast.ListComprehension:
			s.bound[x.VarName] = true
		}
		return true
	})
	return f
}

// reach birimi (dosya, fonksiyon ya da yapı) bir kez incelenmek üzere kuyruğa koyar.
func (s *reachScan) reach(unit any) {
	if !s.reached[unit] {
		s.reached[unit] = true
		s.queue = append(s.queue, unit)
	}
}

// scan bir birimin kendi gövdesini gezer; başka bir birimin bildirimine girmez.
func (s *reachScan) scan(unit any) {
	var root any = unit
	if f, ok := unit.(*reachFile); ok {
		root = f.program
	}
	ast.Walk(root, func(n any) bool {
		if s.found {
			return false
		}
		return s.visit(unit, root, n)
	})
}

// visit gezinin bir düğümünü inceler; false dönerse alt düğümlerine inilmez.
func (s *reachScan) visit(unit, root, n any) bool {
	switch x := n.(type) {
	case *ast.FunctionLiteral:
		if n == root || x.Name == "" {
			return true // adsız fonksiyon satır içidir
		}
		st, isStruct := root.(*ast.StructStatement)
		return isStruct && slices.ContainsFunc(slices.Collect(maps.Values(st.Methods)), func(m *ast.FunctionLiteral) bool { return m == x })
	case *ast.StructStatement:
		return n == root
	case *ast.TestStatement:
		return false
	case *ast.ImportStatement:
		s.importModule(unit, x)
	case *ast.CallExpression:
		s.call(x)
	case *ast.Identifier:
		s.name(x.Value)
	case *ast.MemberExpression:
		s.member(x.Property)
	}
	return true
}

// importModule 'aktar' ile gelen modülün en dışını ulaşılabilir kılar.
func (s *reachScan) importModule(unit any, im *ast.ImportStatement) {
	path := im.Path
	if !strings.HasSuffix(path, ".tr") {
		path += ".tr"
	}
	if _, library := LibraryModule(path); library {
		return
	}
	importer := s.main
	if f, ok := unit.(*reachFile); ok {
		importer = f.path
	}
	resolved, _, ok := ResolveModulePath(path, FileDirs(importer, s.main))
	if !ok {
		s.found = true // modül bulunamıyor: denetim ayrıca bildirir, burada çağrı var sayılır
		return
	}
	abs := absPath(resolved)
	if f, seen := s.files[abs]; seen {
		s.reach(f)
		return
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		s.found = true
		return
	}
	r := source.Prepare(string(data), source.Options{Module: true})
	if r.Stage != "" || r.Program == nil {
		s.found = true
		return
	}
	s.reach(s.addFile(abs, resolved, r.Program))
}

// call bir çağrının neyi çağırdığını sınıflar.
func (s *reachScan) call(c *ast.CallExpression) {
	switch f := c.Function.(type) {
	case *ast.Identifier:
		if s.isCommandLine(f.Value) {
			s.found = true
		} else if !s.known(f.Value) {
			s.found = true // fonksiyon değeriyle çağrı (değişken, parametre)
		}
	case *ast.MemberExpression, *ast.FunctionLiteral:
		// üye çağrısı ve satır içi adsız fonksiyon: Walk onlara ayrıca iner
	default:
		s.found = true // başka bir ifadenin değeri çağrılıyor
	}
}

// known adın adlı bir fonksiyon, yapı ya da yerleşik olduğunu söyler. Sonradan atanan bir
// fonksiyon adı bilinmeyen sayılır.
func (s *reachScan) known(name string) bool {
	if s.bound[name] {
		return false
	}
	if len(s.funcs[name]) > 0 || len(s.structs[name]) > 0 {
		return !s.assigned[name]
	}
	_, isBuiltin := s.vm.GlobalEnv.Get(name)
	return isBuiltin
}

// name bir adın anılmasını işler: komut_satırı, adlı fonksiyon ya da yapı.
func (s *reachScan) name(name string) {
	if s.isCommandLine(name) {
		s.found = true
		return
	}
	for _, fn := range s.funcs[name] {
		s.reach(fn)
	}
	for _, st := range s.structs[name] {
		s.reach(st)
	}
}

// member bir üyenin anılmasını işler: o adı taşıyan fonksiyonlar ve yapılar ulaşılabilir olur.
func (s *reachScan) member(name string) {
	s.name(name)
	for _, st := range s.methods[name] {
		s.reach(st)
	}
}

// isCommandLine adın komut_satırı yerleşiği olduğunu söyler: adın global kapsamda bağlandığı
// yerleşik kayıtlı komut_satırı yerleşiğidir; yerleşik kayıtlı değilse (yorumlayıcı tek başına)
// yazım dilindeki adı ya da kimliği.
func (s *reachScan) isCommandLine(name string) bool {
	if want := s.vm.Builtin(codes.FnCommandLine); want != nil {
		if got, ok := s.vm.GlobalEnv.Get(name); ok && got == Value(want) {
			return true
		}
	}
	if slices.Contains(codes.Spellings(codes.FnCommandLine), name) {
		return true
	}
	k, err := codes.Parse(name)
	return err == nil && k == codes.FnCommandLine
}
