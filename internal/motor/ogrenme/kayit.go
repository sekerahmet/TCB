package ogrenme

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"math"
	"os"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/tensor"
	"tr_dili/internal/motor/ortak/baglama"
)

// TR tensör dosyası biçimi (sürüm 2). Tüm tam sayılar küçük sonlu (little-endian):
//
//	sihirli    8 bayt   "TRTENSOR"
//	sürüm      uint16   2
//	girdi      uint32   girdi sayısı
//	her girdi:
//	  ad_uzun  uint32   adın bayt uzunluğu (en fazla 65535)
//	  ad       UTF-8
//	  tür      uint8    veri türü kodu: 1 = float32, 2 = bf16, 3 = fp16
//	  boyut    uint8    eksen sayısı (en fazla 32)
//	  şekil    uint64 × boyut
//	  veri     eleman sayısı × tür boyu bayt (satır öncelikli; float32 4 bayt,
//	           bf16/fp16 2 baytlık bit deseni)
//	sağlama    uint32   önceki tüm baytların CRC-32 (IEEE) değeri
//
// Sürüm 1 aynı düzendedir ama yalnızca tür kodu 1'i (float32) içerir; artık okunmaz.
// Yüklemede (kayit_oku.go) başlık, sürüm, tür, boyutlar, eleman sınırı, dosya uzunluğu ve
// sağlama toplamı denetlenir; bozuk ya da eksik dosya açık bir hatayla reddedilir.

const (
	dosyaSihirli = "TRTENSOR"
	dosyaSurumu  = 2
	turFloat32   = 1
	turBF16      = 2
	turFP16      = 3
	enFazlaAdUz  = 65535
	enFazlaBoyut = 32
)

// turKodu çekirdek türünün dosyadaki kodu.
func turKodu(dt tensor.DType) byte {
	switch dt {
	case tensor.BFloat16:
		return turBF16
	case tensor.Float16:
		return turFP16
	}
	return turFloat32
}

// koddanTur dosyadaki tür kodunu çekirdek türüne çevirir.
func koddanTur(kod byte) (tensor.DType, bool) {
	switch kod {
	case turFloat32:
		return tensor.Float32, true
	case turBF16:
		return tensor.BFloat16, true
	case turFP16:
		return tensor.Float16, true
	}
	return 0, false
}

type kayitGirdisi struct {
	ad    string
	sekil []int
	tur   tensor.DType
	veri  []float32 // 16 bitlik türlerde de tensörün (kayıpsız açılmış) değerleri
}

func yolArg(fn string, v yurut.Value) (string, error) {
	yol, ok, err := yurut.YolArgumani(fn, v)
	if !ok {
		return "", fmt.Errorf("%s: dosya yolu metin olmalıdır, %s verildi", fn, tipAdi(v))
	}
	if err != nil {
		return "", err
	}
	if yol == "" {
		return "", fmt.Errorf("%s: dosya yolu boş", fn)
	}
	return yol, nil
}

// tensorKaydet adları tensör olan bir sözlüğü dosyaya yazar: tensör_kaydet(yol, sözlük).
func tensorKaydet(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := kod.Ad(kod.T700)
	if len(args) != 2 {
		return nil, fmt.Errorf("%s(yol, sözlük) 2 parametre bekler, %d verildi", fn, len(args))
	}
	if err := baglama.AdliDenetle(fn+"()", named); err != nil {
		return nil, err
	}
	yol, err := yolArg(fn, args[0])
	if err != nil {
		return nil, err
	}
	sozluk, ok := args[1].(*yurut.DictVal)
	if !ok {
		return nil, fmt.Errorf("%s: ikinci parametre adları tensör olan bir sözlük olmalıdır ({\"w\": w}), %s verildi", fn, tipAdi(args[1]))
	}
	// Veri yorumlayıcı kilidi altında kopyalanır; yazma kilit bırakılarak yapılır.
	girdiler := make([]kayitGirdisi, 0, len(sozluk.Order))
	for _, k := range sozluk.Order {
		if _, metin := sozluk.KeyValue(k).(*yurut.StringVal); !metin {
			return nil, fmt.Errorf("%s: sözlük anahtarları metin olmalıdır, %s verildi", fn, sozluk.DisplayKey(k))
		}
		if len(k) > enFazlaAdUz {
			return nil, fmt.Errorf("%s: bir ad çok uzun (%d bayt; en fazla %d)", fn, len(k), enFazlaAdUz)
		}
		d, ok := sozluk.Pairs[k].(*Deger)
		if !ok {
			return nil, fmt.Errorf("%s: %q değeri bir tensör olmalıdır, %s verildi", fn, k, tipAdi(sozluk.Pairs[k]))
		}
		sh := d.t.Shape()
		if len(sh) > enFazlaBoyut {
			return nil, fmt.Errorf("%s: %q en fazla %d eksenli olabilir, %s verildi", fn, k, enFazlaBoyut, sekilMetni(sh))
		}
		girdiler = append(girdiler, kayitGirdisi{ad: k, sekil: sh, tur: d.t.DType(), veri: d.t.Data()})
	}
	// Göreli yol TR'nin tek yol kuralıyla çözülür (bkz. vm.(*VM).YolCoz).
	cozulen := baglama.YolCoz(m, yol)
	var yazHata error
	m.Blocking(func() { yazHata = dosyayaYaz(cozulen, girdiler) })
	if yazHata != nil {
		return nil, turkce.DosyaHatasi(fn, yol, "yazılamadı", yazHata)
	}
	return yurut.NULL, nil
}

// dosyayaYaz girdileri yol'a atomik olarak yazar (bkz. yurut.AtomikYaz): yazma yarıda kalırsa
// önceki kontrol noktası bozulmaz.
func dosyayaYaz(yol string, girdiler []kayitGirdisi) error {
	return yurut.AtomikYaz(yol, func(f io.Writer) error { return girdileriYaz(f, girdiler) })
}

func girdileriYaz(f io.Writer, girdiler []kayitGirdisi) error {
	y := YeniTensorYazici(f, len(girdiler))
	for _, g := range girdiler {
		if err := y.Yaz(g.ad, g.sekil, g.tur, g.veri); err != nil {
			return err
		}
	}
	return y.Bitir()
}

// TensorYazici tensör dosyasını (biçim yukarıda) girdi girdi yazar: kontrol noktası büyük
// modelleri bütün veriyi bellekte toplamadan, tensör tensör konağa indirip bununla yazar.
// Girdi sayısı baştan bilinir; Bitir sağlama toplamını yazar.
type TensorYazici struct {
	bw     *bufio.Writer
	crc    hash.Hash32
	w      io.Writer
	kalan  int
	b      [8]byte
	tampon []byte
	hata   error
}

// YeniTensorYazici f'ye girdiSayisi girdilik bir tensör dosyası yazmaya başlar (başlık).
func YeniTensorYazici(f io.Writer, girdiSayisi int) *TensorYazici {
	y := &TensorYazici{bw: bufio.NewWriterSize(f, 1<<16), crc: crc32.NewIEEE(), kalan: girdiSayisi, tampon: make([]byte, 0, 1<<16)}
	y.w = io.MultiWriter(y.bw, y.crc)
	y.yaz([]byte(dosyaSihirli))
	y.u16(dosyaSurumu)
	y.u32(uint32(girdiSayisi))
	return y
}

func (y *TensorYazici) yaz(p []byte) {
	if y.hata == nil {
		_, y.hata = y.w.Write(p)
	}
}
func (y *TensorYazici) u16(x uint16) { binary.LittleEndian.PutUint16(y.b[:2], x); y.yaz(y.b[:2]) }
func (y *TensorYazici) u32(x uint32) { binary.LittleEndian.PutUint32(y.b[:4], x); y.yaz(y.b[:4]) }
func (y *TensorYazici) u64(x uint64) { binary.LittleEndian.PutUint64(y.b[:8], x); y.yaz(y.b[:8]) }

// Yaz bir girdiyi yazar: veri satır öncelikli değerlerdir (16 bitlik türlerde o türden
// açılmış değerler; geri çevirme kayıpsızdır).
func (y *TensorYazici) Yaz(ad string, sekil []int, tur tensor.DType, veri []float32) error {
	switch {
	case y.hata != nil:
		return y.hata
	case y.kalan <= 0:
		return fmt.Errorf("tensör dosyasına bildirilenden fazla girdi yazılıyor (%q)", ad)
	case len(ad) > enFazlaAdUz:
		return fmt.Errorf("%q adı çok uzun (%d bayt; en fazla %d)", ad[:64], len(ad), enFazlaAdUz)
	case len(sekil) > enFazlaBoyut:
		return fmt.Errorf("%q en fazla %d eksenli olabilir, %s verildi", ad, enFazlaBoyut, sekilMetni(sekil))
	}
	y.kalan--
	y.u32(uint32(len(ad)))
	y.yaz([]byte(ad))
	y.yaz([]byte{turKodu(tur), byte(len(sekil))})
	for _, d := range sekil {
		y.u64(uint64(d))
	}
	on6 := tur == tensor.BFloat16 || tur == tensor.Float16
	for i := 0; i < len(veri); {
		t := y.tampon[:0]
		for ; i < len(veri) && len(t) < cap(t); i++ {
			if on6 {
				t = binary.LittleEndian.AppendUint16(t, tensor.Bits16(tur, veri[i]))
			} else {
				t = binary.LittleEndian.AppendUint32(t, math.Float32bits(veri[i]))
			}
		}
		y.yaz(t)
	}
	return y.hata
}

// Bitir sağlama toplamını yazar ve tamponu boşaltır; bütün girdiler yazılmış olmalıdır.
func (y *TensorYazici) Bitir() error {
	if y.hata != nil {
		return y.hata
	}
	if y.kalan != 0 {
		return fmt.Errorf("tensör dosyasına %d girdi eksik yazıldı", y.kalan)
	}
	binary.LittleEndian.PutUint32(y.b[:4], y.crc.Sum32())
	if _, err := y.bw.Write(y.b[:4]); err != nil {
		return err
	}
	return y.bw.Flush()
}

// tensorYukle tensör_kaydet ile yazılmış dosyayı sözlük olarak okur.
func tensorYukle(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := kod.Ad(kod.T701)
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(yol) 1 parametre bekler, %d verildi", fn, len(args))
	}
	if err := baglama.AdliDenetle(fn+"()", named, adTurevIzle); err != nil {
		return nil, err
	}
	yol, err := yolArg(fn, args[0])
	if err != nil {
		return nil, err
	}
	izle := turevIzle(named, nil, false)
	cozulen, bakilanlar := baglama.YolCozBakilan(m, yol)
	var girdiler []kayitGirdisi
	var okuHata, osHata error
	m.Blocking(func() { girdiler, okuHata, osHata = dosyadanOku(cozulen) })
	if osHata != nil {
		return nil, yurut.OkumaHatasi(fn, yol, "yüklenemedi", osHata, bakilanlar)
	}
	if okuHata != nil {
		return nil, fmt.Errorf("%s: %q: %v", fn, yol, okuHata)
	}
	sonuc := yurut.NewDictVal()
	for _, g := range girdiler {
		var t *tensor.Tensor
		if g.tur == tensor.Float32 {
			t = tensor.New(g.veri, g.sekil...)
		} else {
			t = tensor.NewOf(g.tur, g.veri, g.sekil...)
		}
		t.SetRequiresGrad(izle)
		sonuc.Set(g.ad, Yeni(t))
	}
	return sonuc, nil
}

// dosyadanOku dosyayı çözer. İlk hata dosya biçimi hatası, ikincisi işletim sistemi
// hatasıdır.
func dosyadanOku(yol string) ([]kayitGirdisi, error, error) {
	f, err := os.Open(yol)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	bilgi, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if bilgi.IsDir() {
		return nil, fmt.Errorf("bir klasör, tensör dosyası değil"), nil
	}
	girdiler, err := kayitCoz(f, bilgi.Size())
	return girdiler, err, nil
}
