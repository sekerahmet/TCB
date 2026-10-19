package checkpoint

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// Noktanın yazılması (bkz. paket belgesi). Bütün yazma yorumlayıcı kilidi tutularak yapılır:
// tensörler tek tek konağa indirilip yazılır (bellekte modelin ikinci bir kopyası
// toplanmaz) ve kayıt sürerken başka bir görev modeli değiştiremez. Eğitim döngüsü kayıt
// boyunca zaten bekler.

// yenidenAdlandir klasör ve dosyaları yerine koyar; testler ağ sürücülerindeki (FUSE)
// yeniden adlandırma hatasını benzetmek için değiştirir.
var yenidenAdlandir = os.Rename

// toplamaHatasi kaydedilecek değerlerin toplanamamasıdır (ör. düz değerli bir sözlüğe tensör
// konmuş): yazma hatası değildir, yeniden denemekle geçmez; hemen bildirilir.
type toplamaHatasi struct{ error }

// kaydet noktayı kn.adim adımıyla yazar ve kullanıcıya gösterilecek yolunu verir.
func (kn *KontrolNoktasi) kaydet(m *interp.VM) (string, error) {
	fn := codes.Name(codes.T702)
	durum, plan, err := kn.topla(m)
	if err != nil {
		return "", toplamaHatasi{fmt.Errorf("%s: %v", fn, err)}
	}
	ad := noktaAdi(kn.adim)
	gorunen := kn.gorunen + "/" + ad
	var yazHata error
	if _, err = bind.Kurtar(func() bool {
		yazHata = kn.noktayiYaz(ad, durum, plan)
		return true
	}, func(r any) error { return fmt.Errorf("%v", r) }); err == nil {
		err = yazHata
	}
	if err != nil {
		var ekler []string
		var le *os.LinkError
		if errors.As(err, &le) {
			ekler = append(ekler, "ağ ve bulut sürücülerinde (ör. bağlanmış Drive) yeniden adlandırma başarısız olabilir: noktaları yerel diske kaydedip koşu klasörünü ara ara oraya kopyalayın")
		}
		return "", turkish.FileError(fn, gorunen, "kaydedilemedi", err, ekler...)
	}
	kn.sonGecerli = kn.adim
	if err := interp.WriteAtomic(filepath.Join(kn.klasor, sonDosyasi), func(w io.Writer) error {
		return json.NewEncoder(w).Encode(sonKaydi{Surum: bicimSurumu, Adim: kn.adim, Klasor: ad})
	}); err != nil {
		m.WriteOutput(m.Err, fmt.Sprintf("Uyarı: %s: %s güncellenemedi (nokta kaydedildi; yükleme klasörlere bakar): %v\n", fn, sonDosyasi, turkish.FileErrorReason(sonDosyasi, err)))
	}
	if err := kn.eskileriSil(ad); err != nil {
		m.WriteOutput(m.Err, fmt.Sprintf("Uyarı: %s: eski noktalar silinemedi: %v\n", fn, turkish.FileErrorReason(kn.gorunen, err)))
	}
	return gorunen, nil
}

// topla durum kaydını ve tensör planını yorumlayıcı kilidi altında hazırlar.
func (kn *KontrolNoktasi) topla(m *interp.VM) ([]byte, []tensorPlani, error) {
	d := durumKaydi{Surum: bicimSurumu, Adim: kn.adim, Zaman: saat().Format(time.RFC3339), TR: trSurumu(),
		Program: filepath.Base(m.SourceFile), Isci: runtime.GOMAXPROCS(0)}
	if kn.surec != nil {
		d.Ayarlar = ayarlarYazimi(m, kn.surec.Ayarlar())
	}
	var plan []tensorPlani
	for _, n := range kn.nesneler {
		k, err := n.kaydi(m)
		if err != nil {
			return nil, nil, err
		}
		d.Nesneler = append(d.Nesneler, k)
		plan = append(plan, n.tensorleri()...)
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return b, plan, nil
}

// trSurumu çalışan TR'nin sürümüdür (bilgi için; geliştirme derlemesinde "(devel)").
func trSurumu() string {
	if b, ok := debug.ReadBuildInfo(); ok {
		return b.Main.Version
	}
	return ""
}

// noktayiYaz noktayı geçici klasöre yazar, diske işler ve yerine koyar.
func (kn *KontrolNoktasi) noktayiYaz(ad string, durum []byte, plan []tensorPlani) (err error) {
	if b, err := os.Stat(kn.klasor); err == nil && !b.IsDir() {
		return turkish.Reason(klasorAdi + " bir dosya; noktaların klasörü olmalı")
	}
	if err := os.MkdirAll(kn.klasor, 0o755); err != nil {
		return err
	}
	gecici := filepath.Join(kn.klasor, ad+geciciEki+strconv.Itoa(os.Getpid()))
	if err := os.RemoveAll(gecici); err != nil {
		return err
	}
	if err := os.Mkdir(gecici, 0o755); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(gecici)
		}
	}()
	ozet := ozetKaydi{Surum: bicimSurumu, Adim: kn.adim, Dosyalar: map[string]dosyaOzeti{}}
	if ozet.Dosyalar[tensorDosyasi], err = dosyaYaz(filepath.Join(gecici, tensorDosyasi), func(w io.Writer) error { return tensorleriYaz(w, plan) }); err != nil {
		return err
	}
	if ozet.Dosyalar[durumDosyasi], err = dosyaYaz(filepath.Join(gecici, durumDosyasi), func(w io.Writer) error {
		_, err := w.Write(durum)
		return err
	}); err != nil {
		return err
	}
	ob, _ := json.MarshalIndent(ozet, "", "  ")
	if _, err = dosyaYaz(filepath.Join(gecici, ozetDosyasi), func(w io.Writer) error {
		_, err := w.Write(ob)
		return err
	}); err != nil {
		return err
	}
	dizinSenkron(gecici)
	if err = yerineKoy(gecici, filepath.Join(kn.klasor, ad)); err != nil {
		return err
	}
	dizinSenkron(kn.klasor)
	return nil
}

// tensorleriYaz planı tensör dosyası biçiminde yazar; her tensör yazılırken konağa iner.
func tensorleriYaz(w io.Writer, plan []tensorPlani) error {
	y := ml.YeniTensorYazici(w, len(plan))
	for _, p := range plan {
		if err := y.Yaz(p.ad, p.t.Shape(), p.t.DType(), p.t.Data()); err != nil {
			return err
		}
	}
	return y.Bitir()
}

// dosyaYaz yeni bir dosya yazar, diske işler ve boyunu ve SHA-256 özetini verir.
func dosyaYaz(yol string, yaz func(io.Writer) error) (dosyaOzeti, error) {
	f, err := os.OpenFile(yol, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return dosyaOzeti{}, err
	}
	h := sha256.New()
	s := &sayac{}
	if err := yaz(io.MultiWriter(f, h, s)); err != nil {
		f.Close()
		return dosyaOzeti{}, err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return dosyaOzeti{}, err
	}
	if err := f.Close(); err != nil {
		return dosyaOzeti{}, err
	}
	return dosyaOzeti{Bayt: s.n, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

// sayac yazılan bayt sayısını sayar.
type sayac struct{ n int64 }

func (s *sayac) Write(p []byte) (int, error) { s.n += int64(len(p)); return len(p), nil }

// yerineKoy geçici klasörü hedefin yerine koyar. Aynı adımlı bir nokta varsa (ör. eğitim
// sonunda kaydet(), son aralıklı kayıtla aynı adımda) önce kenara alınır, yenisi yerine
// konunca silinir; arada kesilirse kenardaki yükleme sırasında geri konur.
func yerineKoy(gecici, hedef string) error {
	if _, err := os.Lstat(hedef); err != nil {
		return yenidenAdlandir(gecici, hedef)
	}
	eski := hedef + eskiEki + strconv.Itoa(os.Getpid())
	if err := os.RemoveAll(eski); err != nil {
		return err
	}
	if err := yenidenAdlandir(hedef, eski); err != nil {
		return err
	}
	if err := yenidenAdlandir(gecici, hedef); err != nil {
		_ = yenidenAdlandir(eski, hedef)
		return err
	}
	return os.RemoveAll(eski)
}

// dizinSenkron klasörün girdilerini diske işler (yeniden adlandırma elektrik kesintisinde
// kaybolmasın); desteklenmeyen sistemlerde (Windows) bir şey yapmaz.
func dizinSenkron(yol string) {
	if f, err := os.Open(yol); err == nil {
		_ = f.Sync()
		f.Close()
	}
}

// eskileriSil en yeni tut noktayı (ve yeni yazılanı) bırakıp eskileri siler.
func (kn *KontrolNoktasi) eskileriSil(yeni string) error {
	noktalar, err := noktalariListele(kn.klasor)
	if err != nil {
		return err
	}
	var hatalar []error
	for i, n := range noktalar {
		if i < kn.tut || n.ad == yeni {
			continue
		}
		hatalar = append(hatalar, os.RemoveAll(filepath.Join(kn.klasor, n.ad)))
	}
	return errors.Join(hatalar...)
}

// noktaGirdisi klasördeki bir noktadır.
type noktaGirdisi struct {
	ad   string
	adim int64
}

// noktalariListele klasördeki nokta klasörlerini en yeniden eskiye verir (geçici ve kenara
// alınmış klasörler dışında).
func noktalariListele(klasor string) ([]noktaGirdisi, error) {
	girdiler, err := os.ReadDir(klasor)
	if err != nil {
		return nil, err
	}
	var out []noktaGirdisi
	for _, g := range girdiler {
		if adim, ok := noktaAdimi(g.Name()); ok && g.IsDir() {
			out = append(out, noktaGirdisi{ad: g.Name(), adim: adim})
		}
	}
	slices.SortFunc(out, func(a, b noktaGirdisi) int { return cmp.Compare(b.adim, a.adim) })
	return out, nil
}

// okuSinirli dosyayı en çok sinir bayt olarak okur; daha büyükse hata verir.
func okuSinirli(yol string, sinir int64) ([]byte, error) {
	f, err := os.Open(yol)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var b bytes.Buffer
	n, err := io.Copy(&b, io.LimitReader(f, sinir+1))
	if err != nil {
		return nil, err
	}
	if n > sinir {
		return nil, fmt.Errorf("%s çok büyük (en çok %d bayt)", filepath.Base(yol), sinir)
	}
	return b.Bytes(), nil
}
