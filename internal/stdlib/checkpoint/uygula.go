package checkpoint

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// uygula doğrulanmış ve uyumu denetlenmiş noktayı nesnelere yükler. Sıra önemlidir:
// okuyucular önce (kendi denetimleri hatayla dönerse parametrelere dokunulmamış olur),
// parametreler iyileştiricilerden önce (16 bitlik parametrelerin ana kopyaları parametrenin
// yüklemeden sonraki sürümüne bağlanır).
func (kn *KontrolNoktasi) uygula(m *interp.VM, fn string, o *okunanNokta) error {
	for _, n := range kn.nesneler {
		if n.tur == turDurumlu {
			if err := n.durumlu.DurumuYukle(m, o.nesneler[n.ad].deger.(*interp.DictVal)); err != nil {
				return fmt.Errorf("%s: %q: %v", fn, n.ad, err)
			}
		}
	}
	durumlar, err := kn.tensorleriYukle(o)
	if err != nil {
		return fmt.Errorf("%s: %q yüklenemedi: %v", fn, kn.gorunen+"/"+o.ad, err)
	}
	for _, n := range kn.nesneler {
		switch n.tur {
		case turIyilestirici:
			if err := n.op.DurumuYukle(fmt.Sprintf("%s: %q", fn, n.ad), durumlar[n.ad]); err != nil {
				return err
			}
		case turDuz:
			if err := yerindeYenile(n.ad, n.deger, o.nesneler[n.ad].deger); err != nil {
				return fmt.Errorf("%s: %v", fn, err)
			}
		case turGunluk:
			kn.gunlugeDevamYaz(m, fn, n, o)
		}
	}
	return nil
}

// tensorleriYukle tensör dosyasını ikinci kez, girdi girdi okur: model parametrelerine
// yerinde yazar (parametrenin cihazına taşıyarak), iyileştirici durumlarını nesneye göre
// toplar. Çekirdek panikleri (cihaz belleği) hataya çevrilir.
func (kn *KontrolNoktasi) tensorleriYukle(o *okunanNokta) (map[string]map[string]*tensor.Tensor, error) {
	modeller := map[string]map[string]*ml.Deger{}
	durumlar := map[string]map[string]*tensor.Tensor{}
	for _, n := range kn.nesneler {
		switch n.tur {
		case turModel:
			p := make(map[string]*ml.Deger, len(n.adlar))
			for i, ad := range n.adlar {
				p[ad] = n.params[i]
			}
			modeller[n.ad] = p
		case turIyilestirici:
			durumlar[n.ad] = map[string]*tensor.Tensor{}
		}
	}
	var gezHata error
	_, err := bind.Kurtar(func() bool {
		gezHata = o.tensorleriGez(func(nesneAdi, tensorAdi string, g ml.TensorGirdisi) error {
			kaynak := tensor.NewOn(tensor.CPU, g.Veri, g.Sekil...)
			if p, ok := modeller[nesneAdi][tensorAdi]; ok {
				ml.TensoreYaz(p.Tensor(), kaynak)
			} else if d, ok := durumlar[nesneAdi]; ok {
				d[tensorAdi] = kaynak
			}
			return nil
		})
		return true
	}, func(r any) error { return fmt.Errorf("%v", r) })
	if err == nil {
		err = gezHata
	}
	return durumlar, err
}

// gunlugeDevamYaz günlüğe devam kaydını yazar; günlük kayıttakinden kısaysa (silinmiş ya da
// kesilmiş) uyarır.
func (kn *KontrolNoktasi) gunlugeDevamYaz(m *interp.VM, fn string, n *nesne, o *okunanNokta) {
	if u, err := n.gunluk.Uzunluk(); err == nil && u < o.nesneler[n.ad].kayit.Uzunluk {
		m.CiktiYaz(m.Err, fmt.Sprintf("Uyarı: %s: %q günlüğü kayıttakinden kısa (%d bayt, kayıtta %d); günlük silinmiş ya da kesilmiş olabilir\n",
			fn, n.ad, u, o.nesneler[n.ad].kayit.Uzunluk))
	}
	if err := n.gunluk.DevamYaz(o.adim); err != nil {
		m.CiktiYaz(m.Err, fmt.Sprintf("Uyarı: %s: %q günlüğüne devam kaydı yazılamadı: %v\n", fn, n.ad, err))
	}
}

// ayarlariKarsilastir komut satırı ayarları kayıttakinden farklıysa not düşer (adım sayısını
// ya da öğrenme oranını bilerek değiştirmek meşrudur, en sık durum budur; uyarı değildir).
func (kn *KontrolNoktasi) ayarlariKarsilastir(m *interp.VM, d *durumKaydi) {
	if kn.surec == nil || len(d.Ayarlar) == 0 {
		return
	}
	simdi := ayarlarYazimi(m, kn.surec.Ayarlar())
	if simdi == nil || bytes.Equal(simdi, d.Ayarlar) {
		return
	}
	eskiV, err1 := duzOku(d.Ayarlar)
	yeniV, err2 := duzOku(simdi)
	eski, ok1 := eskiV.(*interp.DictVal)
	yeni, ok2 := yeniV.(*interp.DictVal)
	if err1 != nil || err2 != nil || !ok1 || !ok2 {
		return
	}
	var farklar []string
	for _, k := range yeni.Order {
		e, var_ := eski.Pairs[k]
		if !var_ || e.Inspect() != yeni.Pairs[k].Inspect() {
			eskiMetin := codes.Ad(codes.A042)
			if var_ {
				eskiMetin = e.Inspect()
			}
			farklar = append(farklar, fmt.Sprintf("%s: %s → %s", k, eskiMetin, yeni.Pairs[k].Inspect()))
		}
	}
	if len(farklar) == 0 {
		return
	}
	slices.Sort(farklar)
	m.CiktiYaz(m.Err, fmt.Sprintf("Not: %s: komut satırı ayarları kayıttakinden farklı (%s); bilerek değiştirdiyseniz sorun yok\n",
		codes.Ad(codes.T702), strings.Join(farklar, ", ")))
}
