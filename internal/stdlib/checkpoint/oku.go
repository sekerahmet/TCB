package checkpoint

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// Noktanın yüklenmesi (bkz. paket belgesi). Sıra: (1) en yeni nokta klasörü doğrulanır
// (ozet.json, dosyaların boyu ve SHA-256 özeti, durum.json, tensör dosyasının biçimi ve
// sağlaması); (2) her nesnenin kayda uyumu hiçbir şey yüklenmeden denetlenir; (3) okuyucular,
// parametreler (tensör dosyası ikinci kez, girdi girdi okunarak; bellekte modelin ikinci bir
// kopyası toplanmaz), iyileştirici durumları, düz değerler yüklenir, günlüklere devam kaydı
// yazılır.

// okunanNokta doğrulanmış bir noktadır.
type okunanNokta struct {
	ad, klasor string
	adim       int64
	ozet       *ozetKaydi
	durum      *durumKaydi
	nesneler   map[string]*yuklenecek
}

// yukle klasörde nokta varsa en yenisini yükler; yoksa bir şey yapmaz (yeni koşu).
func (kn *KontrolNoktasi) yukle(m *interp.VM, fn string) error {
	if _, err := os.Stat(kn.klasor); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	artiklariTemizle(kn.klasor)
	noktalar, err := noktalariListele(kn.klasor)
	if err != nil {
		return turkish.FileError(fn, kn.gorunen, "okunamadı", err)
	}
	if len(noktalar) == 0 {
		return nil
	}
	o, err := noktaDogrula(kn.klasor, noktalar[0])
	if err != nil {
		return kn.bozukNoktaHatasi(fn, noktalar, err)
	}
	if err := kn.uyumDenetle(fn, o); err != nil {
		return err
	}
	if err := kn.uygula(m, fn, o); err != nil {
		return err
	}
	kn.kaldigi, kn.adim, kn.sonGecerli = o.adim, o.adim, o.adim
	kn.ayarlariKarsilastir(m, o.durum)
	m.WriteOutput(m.Err, fmt.Sprintf("Kaldığı yerden devam: adım %d (%s/%s)\n", o.adim, kn.gorunen, o.ad))
	return nil
}

// bozukNoktaHatasi en yeni nokta bozuksa açık hatadır; bir önceki geçerli nokta varsa onu
// önerir (bozuk nokta kendiliğinden atlanmaz: kullanıcı neyi kaybettiğini bilerek seçer).
func (kn *KontrolNoktasi) bozukNoktaHatasi(fn string, noktalar []noktaGirdisi, neden error) error {
	bozuk := kn.gorunen + "/" + noktalar[0].ad
	oneri := fmt.Sprintf("Başka geçerli nokta yok; baştan başlamak için %q klasörünü silin ya da başka bir koşu klasörü verin", kn.gorunen)
	for _, n := range noktalar[1:] {
		if _, err := noktaDogrula(kn.klasor, n); err == nil {
			oneri = fmt.Sprintf("Bir önceki geçerli nokta: %s (adım %d); ondan sürdürmek için bozuk klasörü silin ya da koşu klasörünün dışına taşıyın", n.ad, n.adim)
			break
		}
	}
	return fmt.Errorf("%s: %q bozuk: %v. %s", fn, bozuk, neden, oneri)
}

// artiklariTemizle yarıda kalmış kayıtların klasörlerini siler: geçici klasörler atılır;
// kenara alınmış (aynı adımlı yenisi yerine konamamış) bir nokta yerine geri konur.
func artiklariTemizle(klasor string) {
	girdiler, err := os.ReadDir(klasor)
	if err != nil {
		return
	}
	for _, g := range girdiler {
		ad := g.Name()
		if !strings.HasPrefix(ad, noktaOneki) {
			continue
		}
		if i := strings.Index(ad, geciciEki); i > 0 {
			_ = os.RemoveAll(filepath.Join(klasor, ad))
		} else if i := strings.Index(ad, eskiEki); i > 0 {
			asil := filepath.Join(klasor, ad[:i])
			if _, err := os.Lstat(asil); errors.Is(err, fs.ErrNotExist) {
				_ = yenidenAdlandir(filepath.Join(klasor, ad), asil)
			} else {
				_ = os.RemoveAll(filepath.Join(klasor, ad))
			}
		}
	}
}

// noktaDogrula bir noktanın dosyalarını doğrular ve kayıtlarını okur (hiçbir şey yüklemez).
func noktaDogrula(klasor string, n noktaGirdisi) (*okunanNokta, error) {
	o := &okunanNokta{ad: n.ad, klasor: filepath.Join(klasor, n.ad), adim: n.adim}
	ob, err := okuSinirli(filepath.Join(o.klasor, ozetDosyasi), enFazlaKucukJSON)
	if err != nil {
		return nil, okumaNedeni(ozetDosyasi, err)
	}
	if o.ozet, err = ozetCoz(ob, n.adim); err != nil {
		return nil, err
	}
	db, err := okuSinirli(filepath.Join(o.klasor, durumDosyasi), enFazlaDurumJSON)
	if err != nil {
		return nil, okumaNedeni(durumDosyasi, err)
	}
	if err := ozetTutar(durumDosyasi, o.ozet.Dosyalar[durumDosyasi], int64(len(db)), sha256Metni(db)); err != nil {
		return nil, err
	}
	if o.durum, err = durumCoz(db, n.adim); err != nil {
		return nil, err
	}
	if o.durum.Surum != o.ozet.Surum {
		return nil, fmt.Errorf("%s biçim sürümü %d, %s %d diyor", durumDosyasi, o.durum.Surum, ozetDosyasi, o.ozet.Surum)
	}
	o.nesneler = map[string]*yuklenecek{}
	for _, k := range o.durum.Nesneler {
		o.nesneler[k.Ad] = &yuklenecek{kayit: k, sekiller: map[string][]int{}, turler: map[string]tensor.DType{}}
	}
	err = o.tensorleriGez(func(nesneAdi, tensorAdi string, g ml.TensorGirdisi) error {
		y := o.nesneler[nesneAdi]
		y.sekiller[tensorAdi], y.turler[tensorAdi] = g.Sekil, g.Tur
		y.sira = append(y.sira, tensorAdi)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return o, nil
}

// tensorleriGez tensör dosyasını girdi girdi okur; her girdinin adı "<nesne>/<tensör>"
// olmalı ve nesne tensör tutan bir türden olmalıdır. Dosyanın boyu ve SHA-256 özeti
// ozet.json'la, sağlaması kendi biçimiyle denetlenir (gez yalnızca bunlar tutarsa güvenilir).
func (o *okunanNokta) tensorleriGez(gez func(nesneAdi, tensorAdi string, g ml.TensorGirdisi) error) error {
	yol := filepath.Join(o.klasor, tensorDosyasi)
	f, err := os.Open(yol)
	if err != nil {
		return okumaNedeni(tensorDosyasi, err)
	}
	defer f.Close()
	bilgi, err := f.Stat()
	if err != nil {
		return okumaNedeni(tensorDosyasi, err)
	}
	beklenen := o.ozet.Dosyalar[tensorDosyasi]
	if bilgi.Size() != beklenen.Bayt {
		return fmt.Errorf("%s %d bayt, %s %d bayt diyor (dosya yarım kopyalanmış ya da değişmiş)", tensorDosyasi, bilgi.Size(), ozetDosyasi, beklenen.Bayt)
	}
	h := sha256.New()
	err = ml.TensorDosyasiGez(io.TeeReader(f, h), bilgi.Size(), func(g ml.TensorGirdisi) error {
		nesneAdi, tensorAdi, ok := strings.Cut(g.Ad, "/")
		y := o.nesneler[nesneAdi]
		if !ok || y == nil || (y.kayit.Tur != turModel.String() && y.kayit.Tur != turIyilestirici.String()) {
			return fmt.Errorf("%q girdisi kayıttaki bir modele ya da iyileştiriciye ait değil", g.Ad)
		}
		return gez(nesneAdi, tensorAdi, g)
	})
	if err != nil {
		return fmt.Errorf("%s: %v", tensorDosyasi, err)
	}
	return ozetTutar(tensorDosyasi, beklenen, bilgi.Size(), hashMetni(h))
}

func sha256Metni(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func hashMetni(h hash.Hash) string { return hex.EncodeToString(h.Sum(nil)) }

// ozetTutar dosyanın boyunu ve özetini ozet.json'daki kayıtla karşılaştırır.
func ozetTutar(ad string, beklenen dosyaOzeti, bayt int64, ozet string) error {
	if bayt != beklenen.Bayt || ozet != beklenen.SHA256 {
		return fmt.Errorf("%s özeti tutmuyor (dosya değişmiş, bozulmuş ya da yarım kopyalanmış)", ad)
	}
	return nil
}

// okumaNedeni bir dosyanın okunamama nedenini Türkçe söyler.
func okumaNedeni(ad string, err error) error {
	return fmt.Errorf("%s okunamadı: %s", ad, turkish.FileErrorReason(ad, err))
}

// uyumDenetle şimdiki nesnelerin kayıttakilerle aynı olduğunu ve her birinin kaydına
// uyduğunu, hiçbir şey yüklemeden denetler.
func (kn *KontrolNoktasi) uyumDenetle(fn string, o *okunanNokta) error {
	for _, n := range kn.nesneler {
		y, ok := o.nesneler[n.ad]
		if !ok {
			return fmt.Errorf("%s: %q kayıtta yok (kayıttaki nesneler: %s); aynı nesnelerle sürdürün ya da yeni bir koşu klasörü verin", fn, n.ad, o.nesneAdlari())
		}
		if err := n.uyumDenetle(fn, y, o.durum.Surum); err != nil {
			return err
		}
	}
	if len(o.nesneler) != len(kn.nesneler) {
		for _, k := range o.durum.Nesneler {
			if !kn.nesneVar(k.Ad) {
				return fmt.Errorf("%s: kayıttaki %q (%s) verilmedi; aynı nesnelerle sürdürün ya da yeni bir koşu klasörü verin", fn, k.Ad, k.Tur)
			}
		}
	}
	return nil
}

func (kn *KontrolNoktasi) nesneVar(ad string) bool {
	for _, n := range kn.nesneler {
		if n.ad == ad {
			return true
		}
	}
	return false
}

func (o *okunanNokta) nesneAdlari() string {
	adlar := make([]string, len(o.durum.Nesneler))
	for i, k := range o.durum.Nesneler {
		adlar[i] = k.Ad
	}
	return strings.Join(adlar, ", ")
}
