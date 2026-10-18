package tensor

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"
	"unicode/utf8"

	ct "tr_dili/pkg/tensor"
	"tr_dili/pkg/vm"
)

// TR tensör dosyası biçimi (sürüm 1). Tüm tam sayılar küçük sonlu (little-endian):
//
//	sihirli    8 bayt   "TRTENSOR"
//	sürüm      uint16   1
//	girdi      uint32   girdi sayısı
//	her girdi:
//	  ad_uzun  uint32   adın bayt uzunluğu (en fazla 65535)
//	  ad       UTF-8
//	  tür      uint8    veri türü kodu: 1 = float32 (ileride başka türler eklenecek)
//	  boyut    uint8    eksen sayısı (en fazla 32)
//	  şekil    uint64 × boyut
//	  veri     eleman sayısı × tür boyu bayt (satır öncelikli)
//	sağlama    uint32   önceki tüm baytların CRC-32 (IEEE) değeri
//
// Yüklemede başlık, sürüm, tür, boyutlar, dosya uzunluğu ve sağlama toplamı
// denetlenir; bozuk ya da eksik dosya açık bir hatayla reddedilir.

const (
	dosyaSihirli = "TRTENSOR"
	dosyaSurumu  = 1
	turFloat32   = 1
	enFazlaAdUz  = 65535
	enFazlaBoyut = 32
)

type kayitGirdisi struct {
	ad    string
	sekil []int
	veri  []float32
}

// dosyaHatasi işletim sistemi hatalarını Türkçe açıklar.
func dosyaHatasi(fn, yol string, err error) error {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("%s: dosya bulunamadı: %q", fn, yol)
	case errors.Is(err, os.ErrPermission):
		return fmt.Errorf("%s: %q için erişim izni yok", fn, yol)
	}
	return fmt.Errorf("%s: %q kullanılamadı: %v", fn, yol, err)
}

func yolArg(fn string, v vm.Value) (string, error) {
	s, ok := v.(*vm.StringVal)
	if !ok {
		return "", fmt.Errorf("%s: dosya yolu metin olmalıdır, %s verildi", fn, tipAdi(v))
	}
	if s.Val == "" {
		return "", fmt.Errorf("%s: dosya yolu boş", fn)
	}
	return s.Val, nil
}

// tensorKaydet adları tensör olan bir sözlüğü dosyaya yazar: tensör_kaydet(yol, sözlük).
func tensorKaydet(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	const fn = "tensör_kaydet"
	if len(args) != 2 {
		return nil, fmt.Errorf("%s(yol, sözlük) 2 parametre bekler, %d verildi", fn, len(args))
	}
	if err := adliDenetle(fn+"()", named); err != nil {
		return nil, err
	}
	yol, err := yolArg(fn, args[0])
	if err != nil {
		return nil, err
	}
	sozluk, ok := args[1].(*vm.DictVal)
	if !ok {
		return nil, fmt.Errorf("%s: ikinci parametre adları tensör olan bir sözlük olmalıdır ({\"ağırlık\": w}), %s verildi", fn, tipAdi(args[1]))
	}
	// Veri yorumlayıcı kilidi altında kopyalanır; yazma kilit bırakılarak yapılır.
	girdiler := make([]kayitGirdisi, 0, len(sozluk.Order))
	for _, k := range sozluk.Order {
		if _, metin := sozluk.KeyValue(k).(*vm.StringVal); !metin {
			return nil, fmt.Errorf("%s: sözlük anahtarları metin olmalıdır, %s verildi", fn, sozluk.DisplayKey(k))
		}
		if len(k) > enFazlaAdUz {
			return nil, fmt.Errorf("%s: bir ad çok uzun (%d bayt; en fazla %d)", fn, len(k), enFazlaAdUz)
		}
		d, ok := sozluk.Pairs[k].(*Deger)
		if !ok {
			return nil, fmt.Errorf("%s: %q değeri çok boyutlu bir tensör olmalıdır, %s verildi", fn, k, tipAdi(sozluk.Pairs[k]))
		}
		sh := d.t.Shape()
		if len(sh) > enFazlaBoyut {
			return nil, fmt.Errorf("%s: %q en fazla %d eksenli olabilir, %s verildi", fn, k, enFazlaBoyut, sekilMetni(sh))
		}
		girdiler = append(girdiler, kayitGirdisi{ad: k, sekil: sh, veri: d.t.Data()})
	}
	var yazHata error
	m.Blocking(func() { yazHata = dosyayaYaz(yol, girdiler) })
	if yazHata != nil {
		return nil, dosyaHatasi(fn, yol, yazHata)
	}
	return vm.NULL, nil
}

func dosyayaYaz(yol string, girdiler []kayitGirdisi) (err error) {
	f, err := os.OpenFile(yol, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if kapat := f.Close(); err == nil {
			err = kapat
		}
	}()
	bw := bufio.NewWriterSize(f, 1<<16)
	crc := crc32.NewIEEE()
	w := io.MultiWriter(bw, crc)
	var b [8]byte
	yaz := func(p []byte) {
		if err == nil {
			_, err = w.Write(p)
		}
	}
	u16 := func(x uint16) { binary.LittleEndian.PutUint16(b[:2], x); yaz(b[:2]) }
	u32 := func(x uint32) { binary.LittleEndian.PutUint32(b[:4], x); yaz(b[:4]) }
	u64 := func(x uint64) { binary.LittleEndian.PutUint64(b[:8], x); yaz(b[:8]) }

	yaz([]byte(dosyaSihirli))
	u16(dosyaSurumu)
	u32(uint32(len(girdiler)))
	tampon := make([]byte, 0, 1<<16)
	for _, g := range girdiler {
		u32(uint32(len(g.ad)))
		yaz([]byte(g.ad))
		yaz([]byte{turFloat32, byte(len(g.sekil))})
		for _, d := range g.sekil {
			u64(uint64(d))
		}
		for i := 0; i < len(g.veri); {
			tampon = tampon[:0]
			for ; i < len(g.veri) && len(tampon) < cap(tampon); i++ {
				tampon = binary.LittleEndian.AppendUint32(tampon, math.Float32bits(g.veri[i]))
			}
			yaz(tampon)
		}
	}
	if err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(b[:4], crc.Sum32())
	if _, err = bw.Write(b[:4]); err != nil {
		return err
	}
	return bw.Flush()
}

// tensorYukle tensör_kaydet ile yazılmış dosyayı sözlük olarak okur.
func tensorYukle(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	const fn = "tensör_yükle"
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(yol) 1 parametre bekler, %d verildi", fn, len(args))
	}
	if err := adliDenetle(fn+"()", named, "türev_izle", "turev_izle"); err != nil {
		return nil, err
	}
	yol, err := yolArg(fn, args[0])
	if err != nil {
		return nil, err
	}
	izle := turevIzle(named, nil, false)
	var girdiler []kayitGirdisi
	var okuHata, osHata error
	m.Blocking(func() { girdiler, okuHata, osHata = dosyadanOku(yol) })
	if osHata != nil {
		return nil, dosyaHatasi(fn, yol, osHata)
	}
	if okuHata != nil {
		return nil, fmt.Errorf("%s: %q: %v", fn, yol, okuHata)
	}
	sonuc := vm.NewDictVal()
	for _, g := range girdiler {
		t := ct.New(g.veri, g.sekil...)
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
	kalan := bilgi.Size()
	crc := crc32.NewIEEE()
	br := bufio.NewReaderSize(f, 1<<16)
	r := io.TeeReader(br, crc)
	var b [8]byte
	var okumaHatasi error
	oku := func(n int) []byte {
		if okumaHatasi != nil {
			return b[:n]
		}
		if _, err := io.ReadFull(r, b[:n]); err != nil {
			okumaHatasi = err
		}
		kalan -= int64(n)
		return b[:n]
	}
	eksik := func(ne string) error {
		if okumaHatasi != nil && !errors.Is(okumaHatasi, io.EOF) && !errors.Is(okumaHatasi, io.ErrUnexpectedEOF) {
			return fmt.Errorf("%s okunamadı: %v", ne, okumaHatasi)
		}
		return fmt.Errorf("dosya bozuk ya da eksik: %s okunurken dosya bitti", ne)
	}

	if string(oku(8)) != dosyaSihirli || okumaHatasi != nil {
		return nil, fmt.Errorf("bir TR tensör dosyası değil (başlık %q bulunamadı; dosya tensör_kaydet ile yazılmalı)", dosyaSihirli), nil
	}
	surum := binary.LittleEndian.Uint16(oku(2))
	if okumaHatasi != nil {
		return nil, eksik("sürüm"), nil
	}
	if surum == 0 || surum > dosyaSurumu {
		return nil, fmt.Errorf("dosya biçimi sürümü %d; bu TR sürümü en fazla %d okuyabilir", surum, dosyaSurumu), nil
	}
	sayi := binary.LittleEndian.Uint32(oku(4))
	if okumaHatasi != nil {
		return nil, eksik("girdi sayısı"), nil
	}
	// Her girdi en az 4+1+1 bayt tutar; bozuk sayının dev ayırmaya yol açmasını önler.
	if int64(sayi)*6 > kalan {
		return nil, fmt.Errorf("dosya bozuk: %d girdi bildiriliyor ama dosyada yalnızca %d bayt kaldı", sayi, kalan), nil
	}
	girdiler := make([]kayitGirdisi, 0, sayi)
	adlar := map[string]bool{}
	for gi := 0; gi < int(sayi); gi++ {
		sira := fmt.Sprintf("%d. girdi", gi+1)
		adUz := binary.LittleEndian.Uint32(oku(4))
		if okumaHatasi != nil {
			return nil, eksik(sira + " adı"), nil
		}
		if adUz > enFazlaAdUz || int64(adUz) > kalan {
			return nil, fmt.Errorf("dosya bozuk: %s adının uzunluğu %d geçersiz", sira, adUz), nil
		}
		adB := make([]byte, adUz)
		if _, err := io.ReadFull(r, adB); err != nil {
			okumaHatasi = err
			return nil, eksik(sira + " adı"), nil
		}
		kalan -= int64(adUz)
		ad := string(adB)
		if !utf8.ValidString(ad) {
			return nil, fmt.Errorf("dosya bozuk: %s adı geçerli UTF-8 değil", sira), nil
		}
		if adlar[ad] {
			return nil, fmt.Errorf("dosya bozuk: %q adı iki kez geçiyor", ad), nil
		}
		adlar[ad] = true
		sira = fmt.Sprintf("%d. girdi (%q)", gi+1, ad)
		tb := oku(2)
		if okumaHatasi != nil {
			return nil, eksik(sira + " türü"), nil
		}
		tur, boyut := tb[0], int(tb[1])
		if tur != turFloat32 {
			return nil, fmt.Errorf("%s: desteklenmeyen veri türü kodu %d (bu TR sürümü yalnızca float32 = %d okuyabilir)", sira, tur, turFloat32), nil
		}
		if boyut > enFazlaBoyut {
			return nil, fmt.Errorf("dosya bozuk: %s %d eksenli bildiriliyor (en fazla %d)", sira, boyut, enFazlaBoyut), nil
		}
		sekil := make([]int, boyut)
		n := int64(1)
		for i := range sekil {
			d := binary.LittleEndian.Uint64(oku(8))
			if okumaHatasi != nil {
				return nil, eksik(sira + " şekli"), nil
			}
			if d > uint64(vm.MaxCollectionSize) || (d > 0 && n > int64(vm.MaxCollectionSize)/int64(d)) {
				return nil, fmt.Errorf("dosya bozuk ya da tensör çok büyük: %s ekseni %d (en fazla %d eleman)", sira, d, vm.MaxCollectionSize), nil
			}
			sekil[i] = int(d)
			n *= int64(d)
		}
		if n*4 > kalan-4 {
			return nil, fmt.Errorf("dosya bozuk ya da eksik: %s %s şeklinde %d bayt veri bekliyor, dosyada %d bayt kaldı", sira, sekilMetni(sekil), n*4, max(kalan-4, 0)), nil
		}
		veri := make([]float32, n)
		tampon := make([]byte, 1<<16)
		for i := 0; i < len(veri); {
			parca := min(len(tampon)/4, len(veri)-i)
			if _, err := io.ReadFull(r, tampon[:parca*4]); err != nil {
				okumaHatasi = err
				return nil, eksik(sira + " verisi"), nil
			}
			for k := 0; k < parca; k++ {
				veri[i+k] = math.Float32frombits(binary.LittleEndian.Uint32(tampon[k*4:]))
			}
			i += parca
			kalan -= int64(parca * 4)
		}
		girdiler = append(girdiler, kayitGirdisi{ad: ad, sekil: sekil, veri: veri})
	}
	beklenen := crc.Sum32()
	if _, err := io.ReadFull(br, b[:4]); err != nil {
		return nil, fmt.Errorf("dosya bozuk ya da eksik: sağlama toplamı okunurken dosya bitti"), nil
	}
	if binary.LittleEndian.Uint32(b[:4]) != beklenen {
		return nil, fmt.Errorf("dosya bozuk: sağlama toplamı tutmuyor (dosya değiştirilmiş ya da yarım yazılmış olabilir)"), nil
	}
	if _, err := br.ReadByte(); err != io.EOF {
		return nil, fmt.Errorf("dosya bozuk: sağlama toplamından sonra fazladan veri var"), nil
	}
	return girdiler, nil, nil
}
