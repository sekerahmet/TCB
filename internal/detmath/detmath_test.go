package detmath

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

const (
	pkgPath    = "github.com/sekerahmet/TCB/internal/detmath"
	stdlibTree = "github.com/sekerahmet/TCB/internal/stdlib/..."
	langTree   = "github.com/sekerahmet/TCB/internal/lang/..."
)

// fused birleşik komuttur: amd64'te VFMADD231SD ve benzerleri, arm64'te FMADDD ve benzerleri.
var fused = regexp.MustCompile(`\s(VFN?M(ADD|SUB)\d+[A-Z]*S[DS]|FN?M(ADD|SUB)[DS])\s`)

// Kural (13.1): bu paketin, internal/stdlib ve internal/lang'in hiçbir fonksiyonunda birleşik komut
// yoktur. Örneklemsizdir: çapraz derlemenin komutları sayılır, makine istemez. Bir çarpımın
// float64() dönüşümü unutulursa düşer.
func TestNoFusedInstructions(t *testing.T) {
	for _, target := range [][]string{
		{"GOOS=linux", "GOARCH=amd64", "GOAMD64=v3"},
		{"GOOS=linux", "GOARCH=arm64"},
	} {
		cmd := exec.Command("go", "build", "-gcflags="+pkgPath+"=-S", "-gcflags="+stdlibTree+"=-S",
			"-gcflags="+langTree+"=-S", "-o", os.DevNull, stdlibTree, langTree)
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		cmd.Env = append(cmd.Env, target...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v\n%s", target, err, out)
		}
		listing := string(out)
		for _, symbol := range []string{"detmath.Exp STEXT", "interp.NewInt STEXT", "ml.gelu64 STEXT", "logging.(*Progress).measureRate STEXT"} {
			if !strings.Contains(listing, symbol) {
				t.Fatalf("%v: derleme çıktısında %q yok", target, symbol)
			}
		}
		count := 0
		function := ""
		for line := range strings.SplitSeq(listing, "\n") {
			if name, _, ok := strings.Cut(line, " STEXT"); ok {
				function = name
			}
			if fused.MatchString(line) {
				count++
				t.Logf("%s: %s", function, strings.TrimSpace(line))
			}
		}
		if count != 0 {
			t.Errorf("%v: %d birleşik komut var", target, count)
		}
	}
}

// machineFree math paketinden sonucu makineden bağımsız olan adlardır. Liste bilerek dar: yalnız
// kullanılan ve Go kaynağından tek tek bakılmış adlar. Yeni ad gerekçesiyle eklenir.
var machineFree = map[string]bool{
	// Bit işlemi, yuvarlama yok.
	"Abs": true, "Copysign": true, "Signbit": true, "Float32bits": true, "Float32frombits": true,
	"Inf": true, "NaN": true, "IsInf": true, "IsNaN": true,
	// Sonuç tam sayıya kesin yuvarlanır, ara hesap yok.
	"Ceil": true, "Floor": true, "Round": true, "RoundToEven": true, "Trunc": true,
	// Girdilerden birini seçer. arm64'ün komutu NaN'ın yükünü başka verebilir; sayı NaN'ları tek değerdir.
	"Max": true, "Min": true,
	// IEEE 754 doğru yuvarlanmış sonucu şart koşar.
	"Sqrt": true,
	// Sonuç tam: Mod'un çıkarması yuvarlamaz, Ldexp yalnız üsse yazar (alt normalde tek yuvarlama).
	"Mod": true, "Ldexp": true,
	// Sabitler.
	"MaxInt": true, "MaxInt64": true, "MinInt64": true, "Pi": true, "Sqrt2": true,
}

// Kural (13.1, 4.3): sayı yerleşikleri bu paketin fonksiyonlarını çağırır; internal/stdlib ve
// internal/lang math paketinden yalnız machineFree'deki adları kullanır, listede olmayan ad düşer.
// Fonksiyon değeri olarak verilmesi de sayılır. Tensör yolu (internal/compute) 15'indir.
func TestCallersUseThisPackage(t *testing.T) {
	for _, f := range functions {
		if machineFree[f.name] {
			t.Errorf("%s hem bu pakette hem listede", f.name)
		}
	}
	count := 0
	for _, dir := range []string{"../stdlib", "../lang"} {
		err := reporoot.WalkDir(dir, func(file string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			if entry.IsDir() || !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
				return nil
			}
			count++
			for _, use := range machineBoundUses(t, file) {
				t.Errorf("%s: math.%s izin listesinde yok: sonucu makineye bağlıysa bu pakete eklenir", use.at, use.name)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if count < 100 {
		t.Fatalf("yalnız %d Go dosyası bulundu", count)
	}
}

// Listede olmayan ad, başka adla ve noktayla içe aktarma ve fonksiyon değeri düşer.
func TestUnlistedMathNameFails(t *testing.T) {
	for _, c := range []struct {
		name, code string
		want       []string
	}{
		{"çağrı", `import "math"
var a, b = math.Exp2(1), math.Hypot(3, 4)`, []string{"Exp2", "Hypot"}},
		{"bu paketin adı", `import "math"
var a = math.Exp(1)`, []string{"Exp"}},
		{"fonksiyon değeri", `import "math"
var f = math.Cbrt`, []string{"Cbrt"}},
		{"başka ad", `import m "math"
var a = m.Erf(1)`, []string{"Erf"}},
		{"nokta", `import . "math"
var a = Gamma(1)`, []string{"*"}},
		{"izinli", `import "math"
var a = math.Sqrt(math.Abs(-1))`, nil},
		{"içe aktarmıyor", `var math struct{ Exp int }
var a = math.Exp`, nil},
	} {
		file := filepath.Join(t.TempDir(), "x.go")
		if err := os.WriteFile(file, []byte("package x\n"+c.code+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, use := range machineBoundUses(t, file) {
			got = append(got, use.name)
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, beklenen %v", c.name, got, c.want)
		}
	}
}

// machineBoundUses dosyanın machineFree dışında kullandığı math adlarıdır.
func machineBoundUses(t *testing.T, file string) []mathUse {
	t.Helper()
	var out []mathUse
	for _, use := range mathUses(t, file) {
		if !machineFree[use.name] {
			out = append(out, use)
		}
	}
	return out
}

type mathUse struct {
	at   token.Position
	name string
}

// mathUses dosyanın math paketinden kullandığı adları verir; içe aktarma başka adla da olabilir.
// Noktayla içe aktarmada adlar ayırt edilemez, "*" adıyla bir kullanım sayılır.
func mathUses(t *testing.T, file string) []mathUse {
	t.Helper()
	fset := token.NewFileSet()
	tree, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	local := ""
	var uses []mathUse
	for _, spec := range tree.Imports {
		if spec.Path.Value == `"math"` {
			local = "math"
			if spec.Name != nil {
				local = spec.Name.Name
			}
			if local == "." {
				uses = append(uses, mathUse{fset.Position(spec.Pos()), "*"})
			}
		}
	}
	ast.Inspect(tree, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == local {
				uses = append(uses, mathUse{fset.Position(sel.Pos()), sel.Sel.Name})
			}
		}
		return true
	})
	return uses
}

// functions paketin bütün fonksiyonlarıdır; ikili olanlar iki girdi alır.
var functions = []struct {
	name string
	one  func(float64) float64
	go1  func(float64) float64
	two  func(float64, float64) float64
	go2  func(float64, float64) float64
	exp  bool // Go'nun Exp'ini kullanır: Exp'in assembly'si saf koddan ayrılır
}{
	{name: "Exp", one: Exp, go1: math.Exp, exp: true},
	{name: "Log", one: Log, go1: math.Log},
	{name: "Log2", one: Log2, go1: math.Log2},
	{name: "Log10", one: Log10, go1: math.Log10},
	{name: "Log1p", one: Log1p, go1: math.Log1p},
	{name: "Expm1", one: Expm1, go1: math.Expm1},
	{name: "Sin", one: Sin, go1: math.Sin},
	{name: "Cos", one: Cos, go1: math.Cos},
	{name: "Tan", one: Tan, go1: math.Tan},
	{name: "Asin", one: Asin, go1: math.Asin},
	{name: "Acos", one: Acos, go1: math.Acos},
	{name: "Atan", one: Atan, go1: math.Atan},
	{name: "Atan2", two: Atan2, go2: math.Atan2},
	{name: "Sinh", one: Sinh, go1: math.Sinh, exp: true},
	{name: "Cosh", one: Cosh, go1: math.Cosh, exp: true},
	{name: "Tanh", one: Tanh, go1: math.Tanh, exp: true},
	{name: "Pow", two: Pow, go2: math.Pow, exp: true},
	{name: "Erfc", one: Erfc, go1: math.Erfc, exp: true},
}

// inputs sabit bir girdi akışıdır: bit örüntüsünden gelen her değer (NaN, ±∞, çok küçük ve çok
// büyük sayılar dahil), küçük aralıklar ve 1'e yakın değerler.
func inputs(n int) []float64 {
	out := []float64{0, math.Copysign(0, -1), 1, -1, 0.5, 2, math.Inf(1), math.Inf(-1), math.NaN(), math.Pi, 1e300, 1e-300, 5e-324}
	state := uint64(0x9E3779B97F4A7C15)
	next := func() uint64 { // splitmix64
		state += 0x9E3779B97F4A7C15
		z := state
		z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
		z = (z ^ (z >> 27)) * 0x94D049BB133111EB
		return z ^ (z >> 31)
	}
	for len(out) < n {
		u := next()
		unit := float64(u>>11) / (1 << 53)
		switch len(out) % 4 {
		case 0:
			out = append(out, math.Float64frombits(u))
		case 1:
			out = append(out, float64(unit*40)-20)
		case 2:
			out = append(out, float64(unit*2)-1)
		default:
			out = append(out, 1+float64(unit-0.5)/1024)
		}
	}
	return out
}

// digest fonksiyonun sonuçlarının özetidir. NaN'ların biti makineden makineye değişir, değerleri
// değişmez: hepsi tek bir NaN sayılır.
func digest(one func(float64) float64, two func(float64, float64) float64, in []float64) string {
	h := sha256.New()
	var b [8]byte
	for i, x := range in {
		var y float64
		if one != nil {
			y = one(x)
		} else {
			y = two(x, in[(i*7+3)%len(in)])
		}
		if math.IsNaN(y) {
			y = math.NaN()
		}
		binary.LittleEndian.PutUint64(b[:], math.Float64bits(y))
		h.Write(b[:])
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// golden 40 000 girdinin özetidir; değerler GOAMD64=v1 ile Windows amd64'te üretildi.
var golden = map[string]string{
	"Exp": "f641fe39852291e6", "Log": "27c6e5a7397edfdb", "Log2": "55a2487e452168b1", "Log10": "c94c9488d25b14f8",
	"Log1p": "1938389c34a5eac9", "Expm1": "ca1b6797cf8ae5de", "Sin": "dbe47c8833b22682", "Cos": "341da0e7205fb081",
	"Tan": "a2ac88036a16f8c5", "Asin": "a450747102e994e0", "Acos": "dda2757f78fc7332", "Atan": "9376b4167c06b5d4",
	"Atan2": "84e4445a13efc9a8", "Sinh": "461a444c3e267624", "Cosh": "92de9bb703591726", "Tanh": "6000706fd7b2e663",
	"Pow": "71ee09b3ba815476", "Erfc": "3158adba00a4ddba",
}

// Kural (13.1): aynı girdi aynı biti verir. Özet sabittir, derlemeden ve makineden bağımsızdır.
func TestGoldenBits(t *testing.T) {
	in := inputs(40000)
	for _, f := range functions {
		if got := digest(f.one, f.two, in); got != golden[f.name] {
			t.Errorf("%s: özet %q, beklenen %q", f.name, got, golden[f.name])
		}
	}
}

// Kural (13.1): GOAMD64=v3 derlemesi v1 ile aynı biti verir. Komut sayımının değil, eksik dönüşümün
// bitte görünüp görünmediğinin sınamasıdır; Exp'in çalışma anındaki yolunu bu ayırmaz, assembly
// yolu olan her fonksiyon zaten saf koddadır.
func TestGoldenBitsOnV3(t *testing.T) {
	if os.Getenv("GOAMD64") != "" {
		t.Skip("alt koşu")
	}
	cmd := exec.Command("go", "test", "-count=1", "-run", "^TestGoldenBits$", ".")
	cmd.Env = append(os.Environ(), "GOAMD64=v3")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}

// Kopya sadıktır: Go'nun kodu birleşmeden derlendiğinde (GOAMD64=v1, amd64) sonuç aynı bittir.
// Exp'in amd64 assembly'si saf koddan ayrıldığı için Exp ve onu kullananlar sınanmaz. Log'un
// assembly'si alt normal sayıları normalleştirmez (Log(5e-324) −709 verir, doğrusu −744): Log ve
// Log10 normal sayılarda sınanır.
func TestSameBitsAsGoWithoutFusion(t *testing.T) {
	if !goUnfused {
		t.Skip("yalnız birleşmesiz derlemede")
	}
	all := inputs(40000)
	normal := make([]float64, 0, len(all))
	for _, x := range all {
		if !(x > 0 && x < 0x1p-1022) {
			normal = append(normal, x)
		}
	}
	for _, f := range functions {
		if f.exp {
			continue
		}
		in := all
		if f.name == "Log" || f.name == "Log10" {
			in = normal
		}
		if got, want := digest(f.one, f.two, in), digest(f.go1, f.go2, in); got != want {
			t.Errorf("%s: özet %s, Go'nun %s", f.name, got, want)
		}
	}
}

// Sabit vektörler: Go'nun FMA'lı yoluyla ayrıldığı girdiler de burada (TR_exp(1.0) aynıdır).
func TestKnownValues(t *testing.T) {
	for _, c := range []struct {
		name string
		got  float64
		want uint64
	}{
		{"Exp(1)", Exp(1), 0x4005bf0a8b145769},
		{"Log(10)", Log(10), 0x40026bb1bbb55516},
		{"Log(5e-324)", Log(5e-324), 0xc0874385446d71c3},
		{"Pow(2, 0.5)", Pow(2, 0.5), 0x3ff6a09e667f3bcd},
		{"Pow(2, 10)", Pow(2, 10), 0x4090000000000000},
		{"Log2(8)", Log2(8), 0x4008000000000000},
		{"Sin(0)", Sin(0), 0},
		{"Cos(0)", Cos(0), 0x3ff0000000000000},
		{"Erfc(0)", Erfc(0), 0x3ff0000000000000},
	} {
		if got := math.Float64bits(c.got); got != c.want {
			t.Errorf("%s: %#x, beklenen %#x", c.name, got, c.want)
		}
	}
}

func benchOne(b *testing.B, f func(float64) float64, x float64) {
	var s float64
	for b.Loop() {
		s += f(x)
	}
	sink = s
}

var sink float64

// Bedel: her fonksiyonun tek çağrısı; go/ altı Go'nun math'ı, det/ altı bu paket.
func BenchmarkCall(b *testing.B) {
	for _, f := range functions {
		x := 0.7
		if f.one != nil {
			b.Run("go/"+f.name, func(b *testing.B) { benchOne(b, f.go1, x) })
			b.Run("det/"+f.name, func(b *testing.B) { benchOne(b, f.one, x) })
			continue
		}
		b.Run("go/"+f.name, func(b *testing.B) { benchOne(b, func(v float64) float64 { return f.go2(v, 1.3) }, x) })
		b.Run("det/"+f.name, func(b *testing.B) { benchOne(b, func(v float64) float64 { return f.two(v, 1.3) }, x) })
	}
}
