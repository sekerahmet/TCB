package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// --- FONKSİYONLAR ---

// fonkKodu bir fonksiyonun derlenmiş hâlidir; o fonksiyondan üretilen tüm değerler
// (her kapanış) paylaşır.
type fonkKodu struct {
	bilgi *kapsamBilgisi
	govde *blokKodu
	// param[i] i. parametrenin yuvası; varsayilan[i] varsayılan değer kodu (yoksa nil).
	// Varsayılanlar çağrının kendi ortamında, önceki parametreler bağlandıktan sonra
	// değerlendirilir.
	param      []int
	varsayilan []ifadeKodu
	kendi      int // metotlarda 'kendi'nin yuvası, değilse -1
	// basit: parametre yuvaları birbirinden ve 'kendi'den farklı; argümanlar doğrudan
	// yuvalara yazılabilir (bağlama sırası sonucu değiştirmez).
	basit     bool
	erteleVar bool
	// havuzlu: gövde ortamı yakalayamaz (iç fonksiyon, ertele, eşzamanlı, yapı yok); çağrı
	// bitince ortam temizlenip sonraki çağrılarda yeniden kullanılabilir. serbest bu
	// ortamların yığınıdır (yalnızca yorumlayıcı kilidi tutulurken erişilir).
	havuzlu bool
	serbest []*Environment
	// kaynak fonksiyonun tanımlandığı dosyadır (nil: ana program); çağıranınkinden farklıysa
	// çağrı o dosyanın bağlamında yapılır (kaynaktaCagir).
	kaynak *Kaynak
}

// havuzSiniri bir fonksiyon için saklanan boş ortam sayısının üst sınırıdır.
const havuzSiniri = 64

// ortamAl fonksiyonun bir çağrısı için ortam verir (varsa havuzdan).
func (fk *fonkKodu) ortamAl(outer *Environment) *Environment {
	if n := len(fk.serbest); n > 0 {
		e := fk.serbest[n-1]
		fk.serbest = fk.serbest[:n-1]
		if len(e.vals) == len(fk.bilgi.adlar) {
			e.outer = outer
			return e
		}
	}
	return yeniOrtam(outer, fk.bilgi)
}

// birak biten bir çağrının ortamını havuza geri koyar.
func (fk *fonkKodu) birak(e *Environment) {
	if len(fk.serbest) >= havuzSiniri {
		return
	}
	clear(e.vals)
	e.outer = nil
	e.ek = nil
	fk.serbest = append(fk.serbest, e)
}

// fonksiyon bir fonksiyon gövdesini, tanımlandığı k kapsamının içinde derler.
func (c *derleyici) fonksiyon(k *derleyiciKapsam, params []ast.Param, body *ast.BlockStatement, metot bool) *fonkKodu {
	alt := altKapsam(k)
	fk := &fonkKodu{
		param:      make([]int, len(params)),
		varsayilan: make([]ifadeKodu, len(params)),
		kendi:      -1,
		basit:      true,
		kaynak:     c.kaynak,
	}
	if metot {
		fk.kendi = alt.bilgi.yuva(kendiAdi)
	}
	gorulen := map[int]bool{}
	if fk.kendi >= 0 {
		gorulen[fk.kendi] = true
	}
	parametreSatiri := 0
	if body != nil {
		parametreSatiri = body.Line()
	}
	for i, p := range params {
		fk.param[i] = alt.bilgi.yuva(p.Name)
		alt.golgeKaydet(p.Name, parametreSatiri, "bir parametreye")
		if gorulen[fk.param[i]] {
			fk.basit = false
		}
		gorulen[fk.param[i]] = true
	}
	// Varsayılanlar fonksiyonun kendi kapsamında derlenir ve çağrının ortamında, soldan
	// sağa hesaplanır: önceki parametreleri görür; parametrenin kendisi ve sonrakiler henüz
	// yoktur (dıştaki aynı adlı değişkene çözülmez).
	for i, p := range params {
		if p.DefaultValue == nil {
			continue
		}
		henuzYok := make(map[string]bool, len(params)-i)
		for _, s := range params[i:] {
			henuzYok[s.Name] = true
		}
		vk := &derleyiciKapsam{bilgi: alt.bilgi, ust: alt.ust, henuzYok: henuzYok}
		fk.varsayilan[i] = c.ifade(vk, p.DefaultValue)
	}
	var stmts []ast.Statement
	if body != nil {
		stmts = body.Statements
	}
	alt.onTanimla(stmts)
	fk.govde = c.deyimler(alt, stmts)
	fk.bilgi = alt.bilgi
	fk.erteleVar = govdedeErteleVar(stmts)
	fk.havuzlu = true
	for _, st := range stmts {
		if deyimYakalar(st) {
			fk.havuzlu = false
			break
		}
	}
	return fk
}

// fonkKoduHazirla derleyici dışında oluşturulmuş bir fonksiyon değerini (kod alanı boş)
// adla çözümlemeyle derler.
func (v *VM) fonkKoduHazirla(fn *FuncVal) *fonkKodu {
	fk := v.yeniDerleyici().fonksiyon(sinirKapsami(), fn.Parameters, fn.Body, fn.BoundSelf != nil)
	fn.kod = fk
	return fk
}
