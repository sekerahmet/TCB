package stdlib

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/stdlib/logging"
)

// Kod listesinin (internal/lang/codes/definitions.json, tr.json) gerçek davranışla aynı olduğunu sınayan
// testler: örnekler denetlenir ve çalışır, tanımdaki adlı argümanlar yerleşiklerin ve
// metotların gerçekten kabul ettikleridir, üye tanımlarındaki tip listeleri üyeyi gerçekten
// destekleyen tiplerdir.

// geciciKlasoreGec çalışma klasörünü test bitene kadar geçici bir klasör yapar (örneklerin
// yazdığı dosyalar orada kalır). Bu paketin testleri paralel çalışmaz.
func geciciKlasoreGec(t *testing.T) {
	t.Helper()
	// Depo kökü çalışma klasöründen yukarı aranarak bulunur: klasör değişmeden önce bulunmalı
	// (test tek başına koşunca kökü ilk kez bu test arar).
	reporoot.Root()
	eski, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(eski) })
	// Örneklerin açtığı günlük ve satır okuyucusu dosyaları kapansın (Windows'ta açık dosya
	// silinemez), geçici_klasör()'ün klasörleri silinsin.
	t.Cleanup(Kapat)
}

func depoKoku(*testing.T) string { return reporoot.Root() }

func ornekProgrami(t *testing.T, k codes.Code) (*ast.Program, string) {
	t.Helper()
	g, _ := codes.Active().Entry(k)
	p := parser.New(lexer.New(g.Example))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Errorf("%s (%s) örneği ayrıştırılamadı: %v\n  %s", k, g.Name, p.Errors(), g.Example)
		return nil, g.Example
	}
	return prog, g.Example
}

// ornekCalistir programı depo kökünde duran bir program gibi (göreli yollar depo köküne
// göre de çözülür), boş girdiyle ve çıktısı atılarak çalıştırır.
func ornekCalistir(t *testing.T, prog *ast.Program) error {
	t.Helper()
	m := interp.New()
	m.Out = io.Discard
	m.Err = io.Discard
	m.Stdin = strings.NewReader("Ada\n")
	m.SourceFile = filepath.Join(depoKoku(t), "ornek.tr")
	sonuc := make(chan error, 1)
	go func() {
		_, err := m.Run(prog)
		m.WaitAsync()
		sonuc <- err
	}()
	select {
	case err := <-sonuc:
		return err
	case <-time.After(20 * time.Second):
		t.Fatal("örnek 20 saniyede bitmedi")
		return nil
	}
}

// Her örnek `tr kontrol`'den (ayrıştırma ve tip denetimi) geçer ve hatasız çalışır
// (ağ gerektirenler yalnızca denetlenir). Örnekler geçici bir çalışma klasöründe çalışır;
// yazdıkları dosyalar orada kalır.
func TestOrneklerCalisir(t *testing.T) {
	geciciKlasoreGec(t)
	for _, k := range codes.Codes() {
		g, _ := codes.Active().Entry(k)
		if g.Example == "" {
			continue
		}
		prog, ornek := ornekProgrami(t, k)
		if prog == nil {
			continue
		}
		if hatalar := checker.New().CheckProgram(prog); len(hatalar) > 0 {
			t.Errorf("%s (%s) örneği tip denetiminden geçmedi: %v\n  %s", k, g.Name, hatalar, ornek)
			continue
		}
		if err := ornekCalistir(t, prog); err != nil {
			t.Errorf("%s (%s) örneği çalışmadı: %v\n  %s", k, g.Name, err, ornek)
		}
	}
}

// cagrilar programdaki çağrılardan uygun olanları verir.
func cagrilar(dugum any, uygun func(*ast.CallExpression) bool) []*ast.CallExpression {
	var out []*ast.CallExpression
	gorulen := map[uintptr]bool{}
	var gez func(v reflect.Value)
	gez = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() || gorulen[v.Pointer()] {
				return
			}
			gorulen[v.Pointer()] = true
			if c, ok := v.Interface().(*ast.CallExpression); ok && uygun(c) {
				out = append(out, c)
			}
			gez(v.Elem())
		case reflect.Interface:
			if !v.IsNil() {
				gez(v.Elem())
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					gez(v.Field(i))
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				gez(v.Index(i))
			}
		}
	}
	gez(reflect.ValueOf(dugum))
	return out
}

// bilinmeyenAdli denemede kullanılan, hiçbir adlı argümana yakın olmayan addır.
const bilinmeyenAdli = "zzq_bilinmeyen"

// gecerliAdlar bilinmeyen adlı argüman hatasından geçerli adlar listesini çıkarır
// ("(geçerli adlar: a, b)" ya da "(adlı argüman almaz)"); hata bu biçimde değilse ok yanlış.
func gecerliAdlar(err error) ([]string, bool) {
	if err == nil {
		return nil, false
	}
	s := err.Error()
	if !strings.Contains(s, "'"+bilinmeyenAdli+"' adında bir parametre almaz") {
		return nil, false
	}
	if strings.Contains(s, "(adlı argüman almaz)") {
		return []string{}, true
	}
	i := strings.Index(s, "(geçerli adlar: ")
	if i < 0 {
		return nil, false
	}
	s = s[i+len("(geçerli adlar: "):]
	j := strings.Index(s, ")")
	if j < 0 {
		return nil, false
	}
	adlar := strings.Split(s[:j], ", ")
	sort.Strings(adlar)
	return adlar, true
}

func adliAdlari(im codes.Signature) []string {
	out := []string{}
	for _, a := range im.Named {
		out = append(out, codes.Name(a.Code))
	}
	sort.Strings(out)
	return out
}

// Tanımdaki adlı argümanlar, yerleşiğin ya da metodun gerçekten kabul ettikleridir: her
// fonksiyonun ve metodun örneğindeki çağrısına bilinmeyen bir adlı argüman eklenir;
// çağrı, geçerli adları sayan (ya da adlı argüman almadığını söyleyen) hatayla durmalı ve
// bu adlar tanımdakilerle aynı olmalıdır.
func TestAdliArgumanlarTanimlaAyni(t *testing.T) {
	geciciKlasoreGec(t)
	denenen := 0
	for _, k := range codes.Codes() {
		tn, _ := codes.DefinitionOf(k)
		if tn.Kind != codes.KindFunction && tn.Kind != codes.KindMethod {
			continue
		}
		prog, ornek := ornekProgrami(t, k)
		if prog == nil {
			continue
		}
		ad := codes.Name(k)
		cs := cagrilar(prog, func(c *ast.CallExpression) bool {
			switch f := c.Function.(type) {
			case *ast.Identifier:
				return tn.Kind == codes.KindFunction && f.Value == ad
			case *ast.MemberExpression:
				return tn.Kind == codes.KindMethod && f.Property == ad
			}
			return false
		})
		if len(cs) == 0 {
			t.Errorf("%s (%s): örnek bu kodu çağırmıyor; adlı argümanlar sınanamaz\n  %s", k, ad, ornek)
			continue
		}
		for _, c := range cs {
			c.NamedArgs = append(c.NamedArgs, ast.NamedArg{Name: bilinmeyenAdli, Value: &ast.IntegerLiteral{Value: 1}})
		}
		err := ornekCalistir(t, prog)
		alinan, ok := gecerliAdlar(err)
		if !ok {
			t.Errorf("%s (%s): bilinmeyen adlı argüman geçerli adları sayan bir hata vermedi: %v\n  %s", k, ad, err, ornek)
			continue
		}
		denenen++
		var beklenenler [][]string
		if tn.Kind == codes.KindFunction {
			beklenenler = append(beklenenler, adliAdlari(tn.Signature))
		}
		for _, u := range tn.Members {
			beklenenler = append(beklenenler, adliAdlari(u.Signature))
		}
		if !slices.ContainsFunc(beklenenler, func(b []string) bool { return slices.Equal(b, alinan) }) {
			t.Errorf("%s (%s): yerleşik %v adlı argümanlarını kabul ediyor, tanımda %v", k, ad, alinan, beklenenler)
		}
	}
	if denenen < 70 {
		t.Errorf("yalnızca %d kod denendi", denenen)
	}
}

// Üye tanımlarındaki tip listeleri, üyeyi gerçekten destekleyen tiplerle aynıdır: her
// tipten bir örnek değer üzerinde her üye okunur (metotlar çağrılmaz, bağlı değer olarak
// döner); okuma hatasızsa (sözlükte: yok dönmezse) tip üyeyi destekler.
func TestUyeTipleriTanimlaAyni(t *testing.T) {
	m := interp.New()
	m.Out, m.Err = io.Discard, io.Discard
	m.SourceFile = filepath.Join(depoKoku(t), "ornek.tr")
	ornekler := []struct {
		tip codes.Code
		kur string
	}{
		{codes.A200, `5`}, {codes.A201, `2.5`}, {codes.A202, `"abc"`}, {codes.A203, `doğru`},
		{codes.A205, `[1, 2]`}, {codes.A206, `{}`}, {codes.A207, `1..3`}, {codes.A208, `(x) => x`},
		{codes.A209, `yazdır`}, {codes.A212, `tensör([[1.0, 2.0]], türev_izle: doğru)`},
		{codes.A214, `AdamW([tensör([1.0], türev_izle: doğru)])`}, {codes.A214, `SGD([tensör([1.0], türev_izle: doğru)])`},
		{codes.A231, `veri_kümesi("tests/data/numbers")`},
		{codes.A232, `veri_kümesi("tests/data/numbers").örnekleyici(toplu: 1, bağlam: 2)`},
		{codes.A233, `veri_kümesi("tests/data/numbers").sıralı(toplu: 1, bağlam: 2)`},
		{codes.A234, `parçalayıcı_yükle("tests/data/parcalayici.json")`},
		{codes.A250, `yol("tests/yerlesikler_test.tr")`},
		{codes.A257, `dosya_satırları("tests/data/parcalayici.json")`},
		{codes.A255, `ilerleme(3)`},
		{codes.A256, `günlük("` + filepath.ToSlash(filepath.Join(t.TempDir(), "u.jsonl")) + `")`},
		{codes.A258, `kontrol_noktası("` + filepath.ToSlash(t.TempDir()) + `", {w: tensör([1.0], türev_izle: doğru)})`},
	}
	t.Cleanup(logging.CloseAll)
	var kur []string
	for i, o := range ornekler {
		kur = append(kur, "değişken u_"+string(rune('a'+i))+" = "+o.kur)
	}
	if _, err := calistirAyni(t, m, strings.Join(kur, "\n")); err != nil {
		t.Fatal(err)
	}
	for _, k := range codes.Codes() {
		tn, _ := codes.DefinitionOf(k)
		if tn.Kind != codes.KindMethod && tn.Kind != codes.KindProperty {
			continue
		}
		var gercek []codes.Code
		for i, o := range ornekler {
			v, err := calistirAyni(t, m, "u_"+string(rune('a'+i))+"."+codes.Name(k))
			if err == nil && !(o.tip == codes.A206 && v == interp.NULL) {
				gercek = append(gercek, o.tip)
			}
		}
		slices.Sort(gercek)
		gercek = slices.Compact(gercek) // aynı tipten birden çok örnek (AdamW, SGD: İyileştirici)
		tanim := tn.Types()
		slices.Sort(tanim)
		if !slices.Equal(gercek, tanim) {
			t.Errorf("%s (%s): üyeyi destekleyen tipler %v, tanımda %v", k, codes.Name(k), codes.Names(gercek...), codes.Names(tanim...))
		}
	}
}
