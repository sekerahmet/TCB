package main

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Ortam değişkeni adları (K4xx ad kodları) iki yerde koda bağlanır: hesap çekirdeği ad kodlarını göremediği
// için değişkeni kaynak dildeki adıyla düz metin okur ya da koyar (`os.Getenv("TR_GPU_ESZAMANLI")`,
// `t.Setenv(...)`, alt süreç ortamındaki `"TR_DIL=en"` → `uses_env`), iletiler de adı şablonda taşır (H4009:
// "... TR_GPU_SYNC=1 ile çalıştırın" → `uses_name`, her dilin şablonu kendi adını yazar). Ad, hangi dilde
// yazılmış olursa olsun aynı koda varır. Değişkeni koyan test (kendisi ya da test dosyasındaki bir yardımcısı)
// o ad kodunu sınar: `tests` kenarı (addTestEdges; değer dili 3. tur: `tests-for K400` TR_DIL'i koyan e2e
// testlerini göstermiyordu).

// envCodePrefix ortam değişkeni ad kodlarının ön ekidir (codes.NamespaceEnvironment: K4xx).
const envCodePrefix = "K4"

// envCodes ad kodu dosyalarındaki (her yazım dili) K4xx adlarını koda eşler: `TR_GPU_SYNC` → K402.
func envCodes(root string) map[string]string {
	files, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(codesDir), "*.json"))
	sort.Strings(files)
	out := map[string]string{}
	for _, f := range files {
		if filepath.Base(f) == "definitions.json" {
			continue
		}
		var names map[string]struct {
			Ad string `json:"ad"`
		}
		if !readJSON(f, &names) {
			continue
		}
		for code, n := range names {
			if strings.HasPrefix(code, envCodePrefix) && n.Ad != "" {
				out[n.Ad] = code
			}
		}
	}
	return out
}

// envFuncs ortam değişkenini okuyan ya da yazan işlevlerdir (os ve testing paketlerinde).
var envFuncs = map[string]bool{"Getenv": true, "LookupEnv": true, "Setenv": true, "Unsetenv": true}

// envSet bir işlevin bir ortam değişkenini koyduğu yerdir (Setenv ya da "AD=..." dizesi).
type envSet struct {
	code, file string
	line       int
}

// envSetters ortam değişkenini koyan işlevlerdir (envFuncs'un alt kümesi).
var envSetters = map[string]bool{"Setenv": true, "Unsetenv": true}

// envUse `os.Getenv("AD")` gibi bir çağrının ilk bağımsız değişkeni bir ortam değişkeni adıysa
// owner'dan o ad koduna `uses_env` kenarı çeker. Ad dize sabiti ya da dize sabitine çözülen bir addır.
// Setenv ve Unsetenv değişkeni koyar (envSets).
func (x *goExtractor) envUse(fc *fileCtx, owner string, call *ast.CallExpr) {
	id := nameIdent(call.Fun)
	if len(x.env) == 0 || len(call.Args) == 0 || id == nil || !envFuncs[id.Name] {
		return
	}
	fn, ok := fc.u.Info.Uses[id].(*types.Func)
	if !ok || fn.Pkg() == nil || (fn.Pkg().Path() != "os" && fn.Pkg().Path() != "testing") {
		return
	}
	if code := x.env[constString(fc, call.Args[0])]; code != "" {
		x.b.addEdge(owner, "name:"+code, edgeUsesEnv, extracted, fc.rel, x.line(call.Pos()))
		if envSetters[id.Name] {
			x.idx.envSets[owner] = append(x.idx.envSets[owner], envSet{code, fc.rel, x.line(call.Pos())})
		}
	}
}

// envStrings `"AD=değer"` biçimli ortam dizelerini (alt sürecin `cmd.Env`'i, `[]string{"TR_DIL=en"}`) ad koduna
// bağlar: düz metin, sabit (`Ad + "=xx"`) ya da başı sabit olan toplama (`"TR_DIL=" + dil`). Owner'dan `uses_env`
// kenarı çıkar ve owner değişkeni koymuş sayılır. Bir dize ifadesi bütün olarak okunur, parçalarına inilmez (aynı
// ad bir kez sayılır).
func (x *goExtractor) envStrings(fc *fileCtx, owner string, root ast.Node) {
	if len(x.env) == 0 {
		return
	}
	ast.Inspect(root, func(n ast.Node) bool {
		e, ok := n.(ast.Expr)
		if !ok {
			return true
		}
		text, str, _ := envText(fc, e)
		if !str {
			return true
		}
		name, _, found := strings.Cut(text, "=")
		if code := x.env[name]; found && code != "" {
			x.b.addEdge(owner, "name:"+code, edgeUsesEnv, extracted, fc.rel, x.line(e.Pos()))
			x.idx.envSets[owner] = append(x.idx.envSets[owner], envSet{code, fc.rel, x.line(e.Pos())})
		}
		return false
	})
}

// envText bir dize ifadesinin metnidir: str ifade bir dize metni, dize sabiti adı ya da böyle başlayan bir toplamaysa
// true, full metin ifadenin bütün değeriyse (sabit) true; toplamada yalnızca sabit baş kısım bilinir. Grafik tür
// değerlerini (types.Info.Types) yüklemez: değer constString gibi metinden ve sabit adlarından hesaplanır.
func envText(fc *fileCtx, e ast.Expr) (text string, str, full bool) {
	switch e := e.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return constString(fc, e), true, true
		}
	case *ast.Ident:
		if c, ok := fc.u.Info.Uses[e].(*types.Const); ok && c.Val().Kind() == constant.String {
			return constant.StringVal(c.Val()), true, true
		}
	case *ast.ParenExpr:
		return envText(fc, e.X)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false, false
		}
		x, xs, xf := envText(fc, e.X)
		if !xs {
			return "", false, false
		}
		if xf {
			if y, ys, yf := envText(fc, e.Y); ys && yf {
				return x + y, true, true
			}
		}
		return x, true, false
	}
	return "", false, false
}

// constString ifadenin dize sabiti değerini verir (dize sabiti değilse "").
func constString(fc *fileCtx, e ast.Expr) string {
	if lit, ok := e.(*ast.BasicLit); ok {
		s, _ := strconv.Unquote(lit.Value)
		return s
	}
	if id := nameIdent(e); id != nil {
		if c, ok := fc.u.Info.Uses[id].(*types.Const); ok && c.Val().Kind() == constant.String {
			return constant.StringVal(c.Val())
		}
	}
	return ""
}

// envMessages her yazım dilinin ileti şablonlarında geçen ortam değişkeni adlarını ileti düğümünden ad koduna
// `uses_name` kenarı yapar. Kenarın yeri ilk bulunduğu dil dosyasındaki anahtar satırıdır (tr önce).
func envMessages(root string, b *builder) {
	env := envCodes(root)
	if len(env) == 0 {
		return
	}
	dir := filepath.Join(root, filepath.FromSlash(messagesDir))
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	sort.Slice(files, func(i, j int) bool { return langRank(files[i]) < langRank(files[j]) })
	for _, f := range files {
		if base := filepath.Base(f); base == "definitions.json" || base == "retired.json" {
			continue
		}
		var texts map[string]string
		if !readJSON(f, &texts) {
			continue
		}
		lines := jsonKeyLines(f)
		rel := messagesDir + "/" + filepath.Base(f)
		for code, text := range texts {
			for _, word := range strings.FieldsFunc(text, notNameChar) {
				if k := env[word]; k != "" {
					b.addEdge("msg:"+code, "name:"+k, edgeUsesName, extracted, rel, lines[code])
				}
			}
		}
	}
}

// langRank dil dosyalarını tr, sonra alfabetik sıraya koyar (sıra kenarın yerini belirler).
func langRank(file string) string {
	if filepath.Base(file) == "tr.json" {
		return "\x00"
	}
	return file
}

// notNameChar ortam değişkeni adında geçmeyen karakterdir.
func notNameChar(r rune) bool {
	return !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
}
