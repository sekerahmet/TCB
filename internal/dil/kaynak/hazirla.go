package kaynak

import (
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/denetci"
)

// Hazırlığın kaldığı aşamalar (Sonuc.Asama). Modül hataları bu adlarla bildirilir.
const (
	AsamaSozdizimi = "sözdizimi"
	AsamaTip       = "tip denetimi"
)

// Secenekler bir kaynağın hangi aşamalardan geçeceğidir.
type Secenekler struct {
	// Modul kaynağı bir modül olarak ayrıştırır ('aktar' ile alınan dosya: üst düzeyde 'sonuç'
	// hatadır).
	Modul bool
	// TipDenetimi tip denetimini ve tanımsız ad denetimini yapar; uyarılar da bununla gelir.
	TipDenetimi bool
	// Denetleyici verilmişse tip denetimi onunla yapılır (etkileşimli kabuk satırlar arasında
	// aynı denetleyiciyi kullanır; o zaman Sonuc.Hatalar ve Uyarilar birikmiş listelerdir).
	Denetleyici *denetci.Checker
	// GenelKapsam, kaynağın bir aktaranın genel kapsamının altında çalıştığını söyler (11.1):
	// GenelAdlar o kapsamın adlarıdır, GenelDinamik adların tam bilinmediğini söyler.
	GenelKapsam  bool
	GenelAdlar   []string
	GenelDinamik bool
}

// Sonuc hazırlığın sonucudur. Hata varsa Asama hatanın aşamasıdır ve Program o aşamaya kadar
// kurulabilen ağaçtır (sözdizimi hatasında eksik olabilir).
type Sonuc struct {
	Bolumler
	// Metin hata çerçevelerinin, hata ayıklayıcının ve yorumlayıcının kaynak metnidir: Türkçe
	// yazılmış dosyada gövde, kodlu dosyada gövdenin Türkçe gösterimi. Hata mesajlarındaki
	// satır ve sütunlar bu metne göredir (17.5).
	Metin    string
	Program  *agac.Program
	Asama    string
	Hatalar  []string
	Uyarilar []string
}

// Hazirla bir kaynak metnini (dosyanın tamamı, BOM dahil) programa çevirir: başlık çözümü,
// sözcük çözümleme, ayrıştırma ve istenmişse tip denetimi.
func Hazirla(metin string, s Secenekler) *Sonuc {
	b, err := Ayir(metin)
	r := &Sonuc{Bolumler: b, Metin: b.Govde}
	if b.Kodlu {
		r.Metin = gosterim(b)
	}
	if err != nil {
		r.Program = &agac.Program{}
		r.Asama, r.Hatalar = AsamaSozdizimi, []string{err.Error()}
		return r
	}
	p := ayristir.New(b.Sozcukler())
	if s.Modul {
		r.Program = p.ParseModule()
	} else {
		r.Program = p.ParseProgram()
	}
	if errs := p.Errors(); len(errs) > 0 {
		r.Asama, r.Hatalar = AsamaSozdizimi, errs
		return r
	}
	if !s.TipDenetimi {
		return r
	}
	chk := s.Denetleyici
	if chk == nil {
		chk = denetci.New()
	}
	if s.GenelKapsam {
		chk.GenelKapsam(s.GenelAdlar, s.GenelDinamik)
	}
	errs := chk.CheckProgram(r.Program)
	r.Uyarilar = chk.Warnings()
	if len(errs) > 0 {
		r.Asama, r.Hatalar = AsamaTip, errs
	}
	return r
}
