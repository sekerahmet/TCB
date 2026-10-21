package main

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// TR kaynak düğümleri: TR'nin kendi ayrıştırıcısıyla (internal/lang/parser) okunan .tr dosyaları.
const (
	kindTRFunc   = "tr_func"
	kindTRMethod = "tr_method"
	kindTRStruct = "tr_struct"
	kindTRField  = "tr_field"
	kindTRConst  = "tr_const"
	kindTRVar    = "tr_var"
	kindTRTest   = "tr_test"
)

// trRoots .tr dosyalarının aranacağı klasörlerdir (modül köküne göre).
var trRoots = []string{"lib", "tools", "examples"}

// trImport bir `aktar` ifadesidir; file çözülemezse "".
type trImport struct {
	file, alias string
}

// trFile ayrıştırılmış bir .tr dosyasıdır.
type trFile struct {
	rel, base string
	res       *trResult
	lines     []string          // gösterim metninin satırları (satır numaraları bununla uyumlu)
	defs      map[string]string // üst düzey ad -> düğüm kimliği (işlev, yapı, sabit, değişken)
	methods   map[string][]string
	imports   []trImport
	visible   map[string]string // ad -> kimlik: kendi tanımları ve takma adsız aktarılan modüllerinki
	aliases   map[string]*trFile
}

// trExtractor .tr dosyalarını düğüm ve kenara çevirir.
type trExtractor struct {
	root  string
	b     *builder
	lang  *codes.Language
	files map[string]*trFile
	order []string
}

func trFileID(rel string) string { return "tr:file:" + rel }

// extractTR lib/, tools/ ve examples/ altındaki .tr dosyalarını okur (kodlu biçimliler dahil)
// ve işlev, yapı, metot, alan, sabit/değişken ve test düğümleriyle çağrı, kullanım,
// ad kodu, aktarma ve test kenarlarını kurar. Ayrıştırılamayan dosya uyarı olur.
func extractTR(root string, b *builder) {
	lang, err := codes.LoadLanguage(codes.SourceLanguage)
	if err != nil {
		b.warn("tr sources skipped: " + err.Error())
		return
	}
	x := &trExtractor{root: root, b: b, lang: lang, files: map[string]*trFile{}}
	x.discover()
	for _, rel := range x.order {
		x.nodes(x.files[rel])
	}
	for _, rel := range x.order {
		x.resolveImports(x.files[rel])
	}
	for _, rel := range x.order {
		x.edges(x.files[rel])
	}
}

// discover .tr dosyalarını bulur ve ayrıştırır (yol sırasıyla).
func (x *trExtractor) discover() {
	var rels []string
	for _, top := range trRoots {
		dir := filepath.Join(x.root, top)
		_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(d.Name(), ".tr") {
				r, _ := filepath.Rel(x.root, p)
				rels = append(rels, filepath.ToSlash(r))
			}
			return nil
		})
	}
	sort.Strings(rels)
	for _, rel := range rels {
		if f := x.parse(rel); f != nil {
			x.files[rel] = f
			x.order = append(x.order, rel)
		}
	}
}

// parse bir dosyayı program olarak, olmazsa modül olarak ayrıştırır.
func (x *trExtractor) parse(rel string) *trFile {
	data, err := os.ReadFile(filepath.Join(x.root, filepath.FromSlash(rel)))
	if err != nil {
		return nil
	}
	res := readTurkish(string(data), false)
	if len(res.Errors) > 0 {
		res = readTurkish(string(data), true)
	}
	base := strings.TrimSuffix(path.Base(rel), ".tr")
	detail := "tr"
	if res.Coded {
		detail = "tr, coded"
	}
	aliases := []string{base}
	if rest, ok := strings.CutPrefix(rel, "lib/"); ok {
		aliases = append(aliases, "tr/"+strings.TrimSuffix(rest, ".tr"))
	}
	x.b.addNode(Node{ID: trFileID(rel), Kind: kindFile, Name: rel, File: rel, Line: 1, Detail: detail, Aliases: aliases})
	if len(res.Errors) > 0 {
		x.b.warn("tr syntax error, definitions skipped: " + rel + ": " + firstLine(res.Errors[0]))
		return nil
	}
	return &trFile{rel: rel, base: base, res: res, lines: strings.Split(res.Text, "\n"),
		defs: map[string]string{}, methods: map[string][]string{}, aliases: map[string]*trFile{}}
}

// nodes bir dosyanın üst düzey bildirimlerini düğüm yapar.
func (x *trExtractor) nodes(f *trFile) {
	for _, s := range f.res.Program.Statements {
		switch s := s.(type) {
		case *ast.ExpressionStatement:
			if fl, ok := s.Expression.(*ast.FunctionLiteral); ok && fl.Name != "" {
				id := "tr:func:" + f.rel + ":" + fl.Name
				x.def(f, fl.Name, id, kindTRFunc, f.base+"."+fl.Name, fl.Line(), signature(fl.Token.Literal, fl), trFileID(f.rel))
			}
		case *ast.VarStatement:
			x.valueNode(f, s.Name.Value, s.IsConst, s.Line())
		case *ast.MultiVarStatement:
			for _, n := range s.Names {
				x.valueNode(f, n.Value, s.IsConst, s.Line())
			}
		case *ast.StructStatement:
			x.structNodes(f, s)
		case *ast.TestStatement:
			id := "tr:test:" + f.rel + ":" + strconv.Itoa(s.Line())
			x.def(f, "", id, kindTRTest, f.base+": "+s.Name, s.Line(), s.Token.Literal+" "+strconv.Quote(s.Name), trFileID(f.rel))
		case *ast.ImportStatement:
			f.imports = append(f.imports, trImport{file: x.importTarget(f, s.Path), alias: s.Alias})
		}
	}
}

// def bir TR düğümünü ekler ve dosya/üst düğümden `contains` kenarı çeker.
func (x *trExtractor) def(f *trFile, name, id, kind, display string, line int, detail, parent string) {
	x.b.addNode(Node{ID: id, Kind: kind, Name: display, File: f.rel, Line: line, End: x.blockEnd(f, line), Detail: detail})
	x.b.addEdge(parent, id, edgeContains, extracted, f.rel, line)
	if name != "" && parent == trFileID(f.rel) {
		f.defs[name] = id
	}
}

func (x *trExtractor) valueNode(f *trFile, name string, isConst bool, line int) {
	kind, word := kindTRVar, "var"
	if isConst {
		kind, word = kindTRConst, "const"
	}
	x.def(f, name, "tr:"+word+":"+f.rel+":"+name, kind, f.base+"."+name, line, word+" "+name, trFileID(f.rel))
}

func (x *trExtractor) structNodes(f *trFile, s *ast.StructStatement) {
	sid := "tr:struct:" + f.rel + ":" + s.Name
	x.def(f, s.Name, sid, kindTRStruct, f.base+"."+s.Name, s.Line(), s.Token.Literal+" "+s.Name, trFileID(f.rel))
	for _, fld := range s.Fields {
		line := fieldLine(f, fld.Name, s.Line())
		x.def(f, "", "tr:field:"+f.rel+":"+s.Name+"."+fld.Name, kindTRField, f.base+"."+s.Name+"."+fld.Name, line, fld.Name, sid)
	}
	names := make([]string, 0, len(s.Methods))
	for n := range s.Methods {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fl := s.Methods[n]
		mid := "tr:method:" + f.rel + ":" + s.Name + "." + n
		x.def(f, "", mid, kindTRMethod, f.base+"."+s.Name+"."+n, fl.Line(), signature(fl.Token.Literal, fl), sid)
		f.methods[n] = append(f.methods[n], mid)
	}
}

// signature "<anahtar sözcük> ad(parametreler)" biçiminde bir işlev özetidir; anahtar sözcük
// kaynaktaki sözcüğün kendisidir.
func signature(word string, fl *ast.FunctionLiteral) string {
	params := make([]string, len(fl.Parameters))
	for i, p := range fl.Parameters {
		params[i] = p.Name
		if p.DefaultValue != nil {
			params[i] += " = …"
		}
	}
	return word + " " + fl.Name + "(" + strings.Join(params, ", ") + ")"
}

// fieldLine yapı alanının satırıdır: yapının başlığından sonra adla başlayan ilk satır.
func fieldLine(f *trFile, name string, from int) int {
	for i := from; i < len(f.lines) && i < from+400; i++ {
		if t := strings.TrimSpace(f.lines[i]); strings.HasPrefix(t, name) && (len(t) == len(name) || !isIdentByte(t[len(name)])) {
			return i + 1
		}
	}
	return from
}

func isIdentByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

// blockEnd bildirimin başladığı satırdan, ilk `{`yi kapatan `}` satırına kadar olan son satırdır
// (metin ve yorumların içindeki parantezler sayılmaz); `{` yoksa başlangıç satırı.
func (x *trExtractor) blockEnd(f *trFile, start int) int {
	depth, opened := 0, false
	for i := start - 1; i < len(f.lines); i++ {
		line := f.lines[i]
		inStr := false
		for j := 0; j < len(line); j++ {
			c := line[j]
			switch {
			case inStr:
				if c == '\\' {
					j++
				} else if c == '"' {
					inStr = false
				}
			case c == '"':
				inStr = true
			case c == '/' && j+1 < len(line) && line[j+1] == '/':
				j = len(line)
			case c == '{':
				depth++
				opened = true
			case c == '}':
				depth--
			}
		}
		if opened && depth <= 0 {
			return i + 1
		}
		if !opened && i > start && i-start > 3 {
			break
		}
	}
	return start
}
