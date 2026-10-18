package denetci

import (
	"fmt"
	"sort"

	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/kod"
)

// Tanımsız ad denetimi (docs/dil/05_deyimler.md §5.14): bir ad, okunduğu ya da atandığı yerde
// görünen hiçbir kapsamda tanımlı değilse ve bir yerleşik değilse program çalışmadan önce
// hata verir; çalışma anındaki tanımsız ad hatasıyla aynı mesaj ve ipucu kullanılır
// ("'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?").
//
// Dil dinamiktir; denetim yanlış alarm vermemek için yalnızca kesin durumları yakalar:
//   - Bir kapsamda tanımlanan ad, o kapsamın her yerinden görünür sayılır (tanım sırası
//     aranmaz): fonksiyon gövdesi, kendisinden sonra tanımlanan bir genel adı çağrıldığı
//     anda görebilir.
//   - Takma adsız 'aktar' adlarını çalışma anında belli olur; böyle bir aktarımın bulunduğu
//     kapsamda ve altında tanımsız ad aranmaz.
//   - Üye adları (x.ad), adlı argümanlar, yapı alanları denetlenmez; sondaki bloğun 'o'su,
//     yakala'nın 'hata'sı, döngü ve üreteç değişkenleri, parametreler ve yapı metotlarındaki
//     'kendi' kendi kapsamlarında tanımlıdır.

// adKapsami tanımsız ad denetiminin sözcüksel kapsamıdır.
type adKapsami struct {
	ust     *adKapsami
	adlar   map[string]bool
	dinamik bool // takma adsız 'aktar': adlar çalışma anında belli olur
}

func yeniAdKapsami(ust *adKapsami) *adKapsami {
	return &adKapsami{ust: ust, adlar: map[string]bool{}}
}

func (k *adKapsami) tanimla(ad string) {
	if ad == "" {
		k.dinamik = true
		return
	}
	k.adlar[ad] = true
}

// bilinir ad bu kapsamdan görünüyor mu (ya da adları belirsiz bir kapsamın altında mı)?
func (k *adKapsami) bilinir(ad string) bool {
	for c := k; c != nil; c = c.ust {
		if c.adlar[ad] || c.dinamik {
			return true
		}
	}
	return yerlesikAdlari()[ad]
}

// gorunenler öneri için bu kapsamdan görünen adlardır (yerleşikler dahil).
func (k *adKapsami) gorunenler() []string {
	var out []string
	for c := k; c != nil; c = c.ust {
		for a := range c.adlar {
			out = append(out, a)
		}
	}
	for a := range yerlesikAdlari() {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

// adDenetleyici bir programın tanımsız ad denetimidir; bulunan hatalar Checker'a eklenir.
type adDenetleyici struct {
	c        *Checker
	bildirim map[string]bool // aynı ad için tek hata (ilk geçtiği yer)
}

// GenelKapsam modül denetimi içindir: modül, aktaran programın genel kapsamının altında
// çalışır (11.1) ve oradaki adları görür. adlar o genel kapsamın adlarıdır; dinamik, adların
// tam bilinmediğini söyler (ör. genel kapsamda takma adsız 'aktar' var): o zaman modülde
// tanımsız ad aranmaz. CheckProgram'dan önce çağrılır.
func (c *Checker) GenelKapsam(adlar []string, dinamik bool) {
	ust := yeniAdKapsami(nil)
	for _, a := range adlar {
		ust.tanimla(a)
	}
	ust.dinamik = ust.dinamik || dinamik
	c.genelAdlar = yeniAdKapsami(ust)
}

// UstDuzeyAdlari programın genel kapsamına doğrudan tanımladığı adları verir; dinamik,
// takma adsız bir 'aktar' yüzünden adların tam bilinmediğini söyler. Programın aktardığı
// modüller denetlenirken GenelKapsam'a verilir.
func UstDuzeyAdlari(prog *agac.Program) (adlar []string, dinamik bool) {
	k := yeniAdKapsami(nil)
	(&adDenetleyici{}).tanimlar(prog.Statements, k)
	for a := range k.adlar {
		adlar = append(adlar, a)
	}
	sort.Strings(adlar)
	return adlar, k.dinamik
}

// adlariDenetle programı tanımsız adlar için denetler. Genel kapsam Checker'da kalıcıdır:
// etkileşimli kabukta önceki satırların tanımları sonraki satırlarda görünür.
func (c *Checker) adlariDenetle(prog *agac.Program) {
	if c.genelAdlar == nil {
		c.genelAdlar = yeniAdKapsami(nil)
	}
	d := &adDenetleyici{c: c, bildirim: map[string]bool{}}
	d.tanimlar(prog.Statements, c.genelAdlar)
	for _, st := range prog.Statements {
		d.deyim(st, c.genelAdlar)
	}
}

// tanimlar deyimlerin, çalıştıkları kapsama doğrudan tanımladığı adları kapsama ekler
// (çalışma anının kapsam kuralıyla aynı: değişken, sabit, yapı, takma adlı aktar ve
// ifadelerin içindeki adlı fonksiyonlar; iç bloklar ve fonksiyon gövdeleri kendi kapsamıdır).
func (d *adDenetleyici) tanimlar(stmts []agac.Statement, k *adKapsami) {
	for _, st := range stmts {
		switch s := st.(type) {
		case *agac.VarStatement:
			k.tanimla(s.Name.Value)
			d.ifadeTanimlari(s.Value, k)
		case *agac.MultiVarStatement:
			for _, n := range s.Names {
				k.tanimla(n.Value)
			}
			for _, v := range s.Values {
				d.ifadeTanimlari(v, k)
			}
		case *agac.StructStatement:
			k.tanimla(s.Name)
		case *agac.ImportStatement:
			k.tanimla(s.Alias)
		case *agac.ExpressionStatement:
			d.ifadeTanimlari(s.Expression, k)
		case *agac.AssignStatement:
			d.ifadeTanimlari(s.Target, k)
			d.ifadeTanimlari(s.Value, k)
		case *agac.MultiAssignStatement:
			for _, e := range s.Targets {
				d.ifadeTanimlari(e, k)
			}
			for _, e := range s.Values {
				d.ifadeTanimlari(e, k)
			}
		case *agac.ReturnStatement:
			d.ifadeTanimlari(s.ReturnValue, k)
		case *agac.ThrowStatement:
			d.ifadeTanimlari(s.Value, k)
		case *agac.ForInStatement:
			d.ifadeTanimlari(s.Iterable, k)
		case *agac.WhileStatement:
			d.ifadeTanimlari(s.Condition, k)
		case *agac.DeferStatement:
			d.ifadeTanimlari(s.Call, k)
		case *agac.AsyncStatement:
			d.ifadeTanimlari(s.Call, k)
		case *agac.BreakpointStatement:
			d.ifadeTanimlari(s.Condition, k)
		}
	}
}

// ifadeTanimlari ifadenin, değerlendirildiği kapsama tanımladığı adları (fonksiyon f() { }
// biçimindeki adlı fonksiyonlar) ekler; fonksiyon gövdelerine ve bloklara inmez.
func (d *adDenetleyici) ifadeTanimlari(e agac.Expression, k *adKapsami) {
	agacGez(e, func(x agac.Expression) bool {
		if f, ok := x.(*agac.FunctionLiteral); ok {
			if f.Name != "" {
				k.tanimla(f.Name)
			}
			return false
		}
		_, blok := x.(*agac.IfExpression)
		_, eslesme := x.(*agac.MatchExpression)
		return !blok && !eslesme
	})
}

// agacGez ifade ağacını ön sırayla gezer; ziyaret false dönerse düğümün çocuklarına inmez.
// Blok içeren düğümlerin (eğer, eşle, fonksiyon) yalnızca ifade çocukları gezilir.
func agacGez(e agac.Expression, ziyaret func(agac.Expression) bool) {
	if e == nil || !ziyaret(e) {
		return
	}
	switch x := e.(type) {
	case *agac.InterpolatedString:
		for _, p := range x.Parts {
			agacGez(p, ziyaret)
		}
	case *agac.ListLiteral:
		for _, el := range x.Elements {
			agacGez(el, ziyaret)
		}
	case *agac.DictLiteral:
		for _, p := range x.Pairs {
			agacGez(p.Key, ziyaret)
			agacGez(p.Value, ziyaret)
		}
	case *agac.RangeExpression:
		agacGez(x.Start, ziyaret)
		agacGez(x.End, ziyaret)
	case *agac.PrefixExpression:
		agacGez(x.Right, ziyaret)
	case *agac.InfixExpression:
		agacGez(x.Left, ziyaret)
		agacGez(x.Right, ziyaret)
	case *agac.CallExpression:
		agacGez(x.Function, ziyaret)
		for _, a := range x.Arguments {
			agacGez(a, ziyaret)
		}
		for _, na := range x.NamedArgs {
			agacGez(na.Value, ziyaret)
		}
	case *agac.IndexExpression:
		agacGez(x.Left, ziyaret)
		agacGez(x.Index, ziyaret)
	case *agac.MemberExpression:
		agacGez(x.Object, ziyaret)
	case *agac.ListComprehension:
		agacGez(x.Iterable, ziyaret)
	}
}

// blok deyimleri yeni bir alt kapsamda denetler; once alt kapsama önceden tanımlanacak
// adları (döngü değişkeni, parametreler ...) ekler.
func (d *adDenetleyici) blok(b *agac.BlockStatement, ust *adKapsami, once ...string) {
	if b == nil {
		return
	}
	k := yeniAdKapsami(ust)
	for _, a := range once {
		k.tanimla(a)
	}
	d.tanimlar(b.Statements, k)
	for _, st := range b.Statements {
		d.deyim(st, k)
	}
}

func (d *adDenetleyici) deyim(st agac.Statement, k *adKapsami) {
	switch s := st.(type) {
	case *agac.VarStatement:
		d.ifade(s.Value, k)
	case *agac.MultiVarStatement:
		for _, v := range s.Values {
			d.ifade(v, k)
		}
	case *agac.AssignStatement:
		d.hedef(s.Target, k)
		d.ifade(s.Value, k)
	case *agac.MultiAssignStatement:
		for _, t := range s.Targets {
			d.hedef(t, k)
		}
		for _, v := range s.Values {
			d.ifade(v, k)
		}
	case *agac.ExpressionStatement:
		d.ifade(s.Expression, k)
	case *agac.ReturnStatement:
		d.ifade(s.ReturnValue, k)
	case *agac.ThrowStatement:
		d.ifade(s.Value, k)
	case *agac.BlockStatement:
		d.blok(s, k)
	case *agac.ForInStatement:
		d.ifade(s.Iterable, k)
		var once []string
		if s.KeyVar != "" {
			once = append(once, s.KeyVar)
		}
		d.blok(s.Body, k, append(once, s.ValueVar)...)
	case *agac.WhileStatement:
		d.ifade(s.Condition, k)
		d.blok(s.Body, k)
	case *agac.TryCatchStatement:
		d.blok(s.TryBody, k)
		d.blok(s.CatchBody, k, s.ErrorVar)
	case *agac.StructStatement:
		for _, f := range s.Fields {
			d.ifade(f.DefaultValue, k)
		}
		adlar := make([]string, 0, len(s.Methods))
		for ad := range s.Methods {
			adlar = append(adlar, ad)
		}
		sort.Strings(adlar) // hatalar kararlı sırada
		for _, ad := range adlar {
			d.fonksiyon(s.Methods[ad], k, kod.Ad(kod.A051))
		}
	case *agac.DeferStatement:
		d.blok(s.Body, k)
		d.ifade(s.Call, k)
	case *agac.AsyncStatement:
		d.blok(s.Body, k)
		d.ifade(s.Call, k)
	case *agac.BreakpointStatement:
		d.ifade(s.Condition, k)
	case *agac.TestStatement:
		d.blok(s.Body, k)
	}
}

// fonksiyon gövdeyi parametrelerin (ve metotta 'kendi'nin) tanımlı olduğu kapsamda denetler.
func (d *adDenetleyici) fonksiyon(f *agac.FunctionLiteral, k *adKapsami, ek ...string) {
	if f == nil {
		return
	}
	adlar := append([]string{}, ek...)
	for _, p := range f.Parameters {
		adlar = append(adlar, p.Name)
	}
	pk := yeniAdKapsami(k)
	for _, a := range adlar {
		pk.tanimla(a)
	}
	for _, p := range f.Parameters {
		d.ifade(p.DefaultValue, pk)
	}
	d.blok(f.Body, pk)
}

// hedef atamanın sol tarafını denetler: ad tanımlı olmalı (çalışma anında tanımsız ada
// atama hatadır, bkz. 5.4); indeks ve alan hedeflerinde nesne okunur.
func (d *adDenetleyici) hedef(e agac.Expression, k *adKapsami) {
	d.ifade(e, k)
}

func (d *adDenetleyici) ifade(e agac.Expression, k *adKapsami) {
	agacGez(e, func(x agac.Expression) bool {
		switch y := x.(type) {
		case *agac.Identifier:
			d.ad(y, k)
		case *agac.FunctionLiteral:
			d.fonksiyon(y, k)
			return false
		case *agac.IfExpression:
			d.ifade(y.Condition, k)
			d.blok(y.Consequence, k)
			for _, elif := range y.ElseIfs {
				d.ifade(elif.Condition, k)
				d.blok(elif.Consequence, k)
			}
			d.blok(y.Alternative, k)
			return false
		case *agac.MatchExpression:
			d.ifade(y.Subject, k)
			for _, mc := range y.Cases {
				for _, p := range mc.Patterns {
					d.ifade(p, k)
				}
				d.blok(mc.Body, k)
				d.ifade(mc.ValueExpr, k)
			}
			return false
		case *agac.ListComprehension:
			d.ifade(y.Iterable, k)
			uk := yeniAdKapsami(k)
			uk.tanimla(y.VarName)
			d.ifade(y.Output, uk)
			d.ifade(y.Condition, uk)
			return false
		}
		return true
	})
}

// ad, okunan ya da atanan bir adın görünür olduğunu denetler.
func (d *adDenetleyici) ad(id *agac.Identifier, k *adKapsami) {
	if k.bilinir(id.Value) || d.bildirim[id.Value] {
		return
	}
	d.bildirim[id.Value] = true
	d.c.adHatalari = append(d.c.adHatalari, fmt.Sprintf("Satır %d, Sütun %d: Tanımsız ad — %s",
		id.Line(), id.Token.Column, TanimsizAdMetni(id.Value, k.gorunenler())))
}

// satiraGoreSirala hata metinlerini baştaki "Satır N" numarasına göre (eşitlikte ilk
// sırayı koruyarak) sıralar.
func satiraGoreSirala(hatalar []string) {
	satir := func(h string) int {
		var n int
		if _, err := fmt.Sscanf(h, "Satır %d", &n); err != nil {
			return 1 << 30
		}
		return n
	}
	sort.SliceStable(hatalar, func(i, j int) bool { return satir(hatalar[i]) < satir(hatalar[j]) })
}
