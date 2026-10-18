package motor

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

	"tr_dili/internal/depo"
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/denetci"
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/sozcuk"
	"tr_dili/internal/dil/yurut"
)

// Kod listesinin (internal/dil/kod/tanim.json, tr.json) gerçek davranışla aynı olduğunu sınayan
// testler: örnekler denetlenir ve çalışır, tanımdaki adlı argümanlar yerleşiklerin ve
// metotların gerçekten kabul ettikleridir, üye tanımlarındaki tip listeleri üyeyi gerçekten
// destekleyen tiplerdir.

// agGerekenler ağ ya da yerel dil modeli sunucusu gerektiren örneklerdir; yalnızca
// denetlenir, çalıştırılmaz.
var agGerekenler = map[kod.Kod]bool{}

// geciciKlasoreGec çalışma klasörünü test bitene kadar geçici bir klasör yapar (örneklerin
// yazdığı dosyalar orada kalır). Bu paketin testleri paralel çalışmaz.
func geciciKlasoreGec(t *testing.T) {
	t.Helper()
	eski, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(eski) })
}

func depoKoku(*testing.T) string { return depo.Kok() }

func ornekProgrami(t *testing.T, k kod.Kod) (*agac.Program, string) {
	t.Helper()
	g, _ := kod.Etkin().Girdisi(k)
	p := ayristir.New(sozcuk.New(g.Ornek))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Errorf("%s (%s) örneği ayrıştırılamadı: %v\n  %s", k, g.Ad, p.Errors(), g.Ornek)
		return nil, g.Ornek
	}
	return prog, g.Ornek
}

// ornekCalistir programı depo kökünde duran bir program gibi (göreli yollar depo köküne
// göre de çözülür), boş girdiyle ve çıktısı atılarak çalıştırır.
func ornekCalistir(t *testing.T, prog *agac.Program) error {
	t.Helper()
	m := yurut.New()
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
	for _, k := range kod.Kodlar() {
		g, _ := kod.Etkin().Girdisi(k)
		if g.Ornek == "" {
			continue
		}
		prog, ornek := ornekProgrami(t, k)
		if prog == nil {
			continue
		}
		if hatalar := denetci.New().CheckProgram(prog); len(hatalar) > 0 {
			t.Errorf("%s (%s) örneği tip denetiminden geçmedi: %v\n  %s", k, g.Ad, hatalar, ornek)
			continue
		}
		if agGerekenler[k] {
			continue
		}
		if err := ornekCalistir(t, prog); err != nil {
			t.Errorf("%s (%s) örneği çalışmadı: %v\n  %s", k, g.Ad, err, ornek)
		}
	}
}

// cagrilar programdaki çağrılardan uygun olanları verir.
func cagrilar(dugum any, uygun func(*agac.CallExpression) bool) []*agac.CallExpression {
	var out []*agac.CallExpression
	gorulen := map[uintptr]bool{}
	var gez func(v reflect.Value)
	gez = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() || gorulen[v.Pointer()] {
				return
			}
			gorulen[v.Pointer()] = true
			if c, ok := v.Interface().(*agac.CallExpression); ok && uygun(c) {
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

func adliAdlari(im kod.Imza) []string {
	out := []string{}
	for _, a := range im.Adli {
		out = append(out, kod.Ad(a.Kod))
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
	for _, k := range kod.Kodlar() {
		tn, _ := kod.Tanimi(k)
		if (tn.Tur != kod.TurFonksiyon && tn.Tur != kod.TurMetot) || agGerekenler[k] {
			continue
		}
		prog, ornek := ornekProgrami(t, k)
		if prog == nil {
			continue
		}
		ad := kod.Ad(k)
		cs := cagrilar(prog, func(c *agac.CallExpression) bool {
			switch f := c.Function.(type) {
			case *agac.Identifier:
				return tn.Tur == kod.TurFonksiyon && f.Value == ad
			case *agac.MemberExpression:
				return tn.Tur == kod.TurMetot && f.Property == ad
			}
			return false
		})
		if len(cs) == 0 {
			t.Errorf("%s (%s): örnek bu kodu çağırmıyor; adlı argümanlar sınanamaz\n  %s", k, ad, ornek)
			continue
		}
		for _, c := range cs {
			c.NamedArgs = append(c.NamedArgs, agac.NamedArg{Name: bilinmeyenAdli, Value: &agac.IntegerLiteral{Value: 1}})
		}
		err := ornekCalistir(t, prog)
		alinan, ok := gecerliAdlar(err)
		if !ok {
			t.Errorf("%s (%s): bilinmeyen adlı argüman geçerli adları sayan bir hata vermedi: %v\n  %s", k, ad, err, ornek)
			continue
		}
		denenen++
		var beklenenler [][]string
		if tn.Tur == kod.TurFonksiyon {
			beklenenler = append(beklenenler, adliAdlari(tn.Imza))
		}
		for _, u := range tn.Uyeler {
			beklenenler = append(beklenenler, adliAdlari(u.Imza))
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
	m := yurut.New()
	m.Out = io.Discard
	m.SourceFile = filepath.Join(depoKoku(t), "ornek.tr")
	ornekler := []struct {
		tip kod.Kod
		kur string
	}{
		{kod.A200, `5`}, {kod.A201, `2.5`}, {kod.A202, `"abc"`}, {kod.A203, `doğru`},
		{kod.A205, `[1, 2]`}, {kod.A206, `{}`}, {kod.A207, `1..3`}, {kod.A208, `(x) => x`},
		{kod.A209, `yazdır`}, {kod.A212, `tensör([[1.0, 2.0]], türev_izle: doğru)`},
		{kod.A214, `AdamW([tensör([1.0], türev_izle: doğru)])`}, {kod.A214, `SGD([tensör([1.0], türev_izle: doğru)])`},
		{kod.A231, `veri_kümesi("testler/veri/sayilar")`},
		{kod.A232, `veri_kümesi("testler/veri/sayilar").örnekleyici(toplu: 1, bağlam: 2)`},
		{kod.A233, `veri_kümesi("testler/veri/sayilar").sıralı(toplu: 1, bağlam: 2)`},
		{kod.A234, `parçalayıcı_yükle("testler/veri/parcalayici.json")`},
	}
	var kur []string
	for i, o := range ornekler {
		kur = append(kur, "değişken u_"+string(rune('a'+i))+" = "+o.kur)
	}
	if _, err := calistirAyni(t, m, strings.Join(kur, "\n")); err != nil {
		t.Fatal(err)
	}
	for _, k := range kod.Kodlar() {
		tn, _ := kod.Tanimi(k)
		if tn.Tur != kod.TurMetot && tn.Tur != kod.TurOzellik {
			continue
		}
		var gercek []kod.Kod
		for i, o := range ornekler {
			v, err := calistirAyni(t, m, "u_"+string(rune('a'+i))+"."+kod.Ad(k))
			if err == nil && !(o.tip == kod.A206 && v == yurut.NULL) {
				gercek = append(gercek, o.tip)
			}
		}
		slices.Sort(gercek)
		gercek = slices.Compact(gercek) // aynı tipten birden çok örnek (AdamW, SGD: İyileştirici)
		tanim := tn.Tipler()
		slices.Sort(tanim)
		if !slices.Equal(gercek, tanim) {
			t.Errorf("%s (%s): üyeyi destekleyen tipler %v, tanımda %v", k, kod.Ad(k), kod.Adlar(gercek...), kod.Adlar(tanim...))
		}
	}
}
