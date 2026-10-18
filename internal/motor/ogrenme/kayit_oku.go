package ogrenme

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"math"
	"unicode/utf8"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/tensor"
)

// Tensör dosyasının (biçim: kayit.go) çözücüsü. Dosya güvenilmeyen girdidir (indirilen
// kontrol noktası): her uzunluk alanı, ayırmadan önce dosyada kalan bayt sayısıyla ve
// eleman sınırıyla sınanır; sağlama toplamı en sonda denetlenir.

// kayitOkuyucu tensör dosyasını sırayla okur.
type kayitOkuyucu struct {
	br    *bufio.Reader
	r     io.Reader // br'den okur, okuduğunu sağlama toplamına da yazar
	crc   hash.Hash32
	kalan int64 // dosyada henüz okunmamış bayt sayısı
	b     [8]byte
	hata  error // ilk okuma hatası (sonraki okumalar yapılmaz)
}

// oku n baytı (en fazla 8) okur; hata olursa k.hata dolar.
func (k *kayitOkuyucu) oku(n int) []byte {
	if k.hata != nil {
		return k.b[:n]
	}
	if _, err := io.ReadFull(k.r, k.b[:n]); err != nil {
		k.hata = err
	}
	k.kalan -= int64(n)
	return k.b[:n]
}

// tam p'yi tümüyle doldurur; dosya biterse ya da okunamazsa yanlış döner.
func (k *kayitOkuyucu) tam(p []byte) bool {
	if k.hata != nil {
		return false
	}
	if _, err := io.ReadFull(k.r, p); err != nil {
		k.hata = err
		return false
	}
	k.kalan -= int64(len(p))
	return true
}

// eksik ne okunurken oluşan okuma hatasını anlatır.
func (k *kayitOkuyucu) eksik(ne string) error {
	if k.hata != nil && !errors.Is(k.hata, io.EOF) && !errors.Is(k.hata, io.ErrUnexpectedEOF) {
		return fmt.Errorf("%s okunamadı: %v", ne, k.hata)
	}
	return fmt.Errorf("dosya bozuk ya da eksik: %s okunurken dosya bitti", ne)
}

// kayitCoz boyut bayt uzunluğundaki bir tensör dosyasının içeriğini f'den çözer.
func kayitCoz(f io.Reader, boyut int64) ([]kayitGirdisi, error) {
	var girdiler []kayitGirdisi
	err := kayitGez(f, boyut, func(g kayitGirdisi) error {
		girdiler = append(girdiler, g)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return girdiler, nil
}

// kayitGez boyut bayt uzunluğundaki bir tensör dosyasını f'den sırayla çözer ve her girdiyi
// her'e verir; her hata dönerse durur. Sağlama toplamı en sonda denetlenir: her'in gördüğü
// veriler kayitGez hatasız dönene kadar güvenilir sayılmamalıdır.
func kayitGez(f io.Reader, boyut int64, her func(kayitGirdisi) error) error {
	k := &kayitOkuyucu{br: bufio.NewReaderSize(f, 1<<16), crc: crc32.NewIEEE(), kalan: boyut}
	k.r = io.TeeReader(k.br, k.crc)
	sayi, err := k.baslik()
	if err != nil {
		return err
	}
	adlar := map[string]bool{}
	for gi := 0; gi < int(sayi); gi++ {
		g, err := k.girdi(gi, adlar)
		if err != nil {
			return err
		}
		if err := her(g); err != nil {
			return err
		}
	}
	return k.saglama()
}

// TensorGirdisi tensör dosyasındaki bir girdidir: ad, şekil, veri türü ve (16 bitlik
// türlerde kayıpsız açılmış) değerler.
type TensorGirdisi struct {
	Ad    string
	Sekil []int
	Tur   tensor.DType
	Veri  []float32
}

// TensorDosyasiGez tensör_kaydet biçimindeki bir dosyayı (boyut bayt) r'den sırayla çözer
// ve her girdiyi her'e verir; büyük dosyalar bütün veri bellekte toplanmadan okunur. Bütün
// sınırlar ve denetimler tensör_yükle'ninkilerdir; sağlama toplamı en sonda denetlenir, bu
// yüzden her'in gördüğü veriler dönüş hatasız olana kadar güvenilir sayılmamalıdır (kontrol
// noktası dosyayı önce yalnızca denetler, sonra yükler). Hata dosya biçimi hatasıdır.
func TensorDosyasiGez(r io.Reader, boyut int64, her func(TensorGirdisi) error) error {
	return kayitGez(r, boyut, func(g kayitGirdisi) error {
		return her(TensorGirdisi{Ad: g.ad, Sekil: g.sekil, Tur: g.tur, Veri: g.veri})
	})
}

// baslik sihirli baytları ve sürümü denetler, girdi sayısını verir.
func (k *kayitOkuyucu) baslik() (uint32, error) {
	if string(k.oku(8)) != dosyaSihirli || k.hata != nil {
		return 0, fmt.Errorf("bir TR tensör dosyası değil (başlık %q bulunamadı; dosya %s ile yazılmalı)", dosyaSihirli, kod.Ad(kod.T700))
	}
	surum := binary.LittleEndian.Uint16(k.oku(2))
	if k.hata != nil {
		return 0, k.eksik("sürüm")
	}
	if surum != dosyaSurumu {
		return 0, fmt.Errorf("dosya biçimi sürümü %d; bu TR sürümü yalnızca sürüm %d okur (dosyayı %s ile yeniden yazın)", surum, dosyaSurumu, kod.Ad(kod.T700))
	}
	sayi := binary.LittleEndian.Uint32(k.oku(4))
	if k.hata != nil {
		return 0, k.eksik("girdi sayısı")
	}
	// Her girdi en az 4+1+1 bayt tutar; bozuk sayının dev ayırmaya yol açmasını önler.
	if int64(sayi)*6 > k.kalan {
		return 0, fmt.Errorf("dosya bozuk: %d girdi bildiriliyor ama dosyada yalnızca %d bayt kaldı", sayi, k.kalan)
	}
	return sayi, nil
}

// girdi gi. girdiyi (ad, tür, şekil, veri) okur; adlar aynı adın iki kez geçmesini yakalar.
func (k *kayitOkuyucu) girdi(gi int, adlar map[string]bool) (kayitGirdisi, error) {
	sira := fmt.Sprintf("%d. girdi", gi+1)
	adUz := binary.LittleEndian.Uint32(k.oku(4))
	if k.hata != nil {
		return kayitGirdisi{}, k.eksik(sira + " adı")
	}
	if adUz > enFazlaAdUz || int64(adUz) > k.kalan {
		return kayitGirdisi{}, fmt.Errorf("dosya bozuk: %s adının uzunluğu %d geçersiz", sira, adUz)
	}
	adB := make([]byte, adUz)
	if !k.tam(adB) {
		return kayitGirdisi{}, k.eksik(sira + " adı")
	}
	ad := string(adB)
	if !utf8.ValidString(ad) {
		return kayitGirdisi{}, fmt.Errorf("dosya bozuk: %s adı geçerli UTF-8 değil", sira)
	}
	if adlar[ad] {
		return kayitGirdisi{}, fmt.Errorf("dosya bozuk: %q adı iki kez geçiyor", ad)
	}
	adlar[ad] = true
	sira = fmt.Sprintf("%d. girdi (%q)", gi+1, ad)
	tb := k.oku(2)
	if k.hata != nil {
		return kayitGirdisi{}, k.eksik(sira + " türü")
	}
	kod, boyut := tb[0], int(tb[1])
	tur, bilinen := koddanTur(kod)
	if !bilinen {
		return kayitGirdisi{}, fmt.Errorf("%s: desteklenmeyen veri türü kodu %d (bu TR sürümü float32 = %d, bf16 = %d, fp16 = %d okuyabilir)", sira, kod, turFloat32, turBF16, turFP16)
	}
	if boyut > enFazlaBoyut {
		return kayitGirdisi{}, fmt.Errorf("dosya bozuk: %s %d eksenli bildiriliyor (en fazla %d)", sira, boyut, enFazlaBoyut)
	}
	sekil, n, err := k.sekil(sira, boyut)
	if err != nil {
		return kayitGirdisi{}, err
	}
	veri, err := k.veri(sira, tur, sekil, n)
	if err != nil {
		return kayitGirdisi{}, err
	}
	return kayitGirdisi{ad: ad, sekil: sekil, tur: tur, veri: veri}, nil
}

// sekil boyut eksenli şekli ve eleman sayısını okur. Eleman sınırı sıfır uzunluklu eksenler
// 1 sayılarak uygulanır: sıfır eksenli bir tensör veri tutmaz, ama öbür eksenleri
// işlemlerin çıktı şeklini belirler (ör. [4, 0] @ [0, N] → [4, N]). Sınır yalnızca gerçek
// eleman sayısına uygulansaydı yüz baytlık bir dosya dev eksenler bildirip ilk işlemde
// belleği tüketebilirdi (güvenlik denetimi 2026-10-18).
func (k *kayitOkuyucu) sekil(sira string, boyut int) ([]int, int64, error) {
	sekil := make([]int, boyut)
	n, sinir := int64(1), int64(1)
	for i := range sekil {
		d := binary.LittleEndian.Uint64(k.oku(8))
		if k.hata != nil {
			return nil, 0, k.eksik(sira + " şekli")
		}
		if d > uint64(yurut.MaxCollectionSize) || (d > 0 && sinir > int64(yurut.MaxCollectionSize)/int64(d)) {
			return nil, 0, fmt.Errorf("dosya bozuk ya da tensör çok büyük: %s ekseni %d (en fazla %d eleman; sıfır uzunluklu eksenler 1 sayılır)", sira, d, yurut.MaxCollectionSize)
		}
		sekil[i] = int(d)
		n *= int64(d)
		sinir *= max(int64(d), 1)
	}
	return sekil, n, nil
}

// veri n elemanlık veriyi okur ve float32'ye açar.
func (k *kayitOkuyucu) veri(sira string, tur tensor.DType, sekil []int, n int64) ([]float32, error) {
	boy := int64(tur.Bits() / 8) // eleman başına bayt
	if n*boy > k.kalan-4 {
		return nil, fmt.Errorf("dosya bozuk ya da eksik: %s %s şeklinde %d bayt veri bekliyor, dosyada %d bayt kaldı", sira, sekilMetni(sekil), n*boy, max(k.kalan-4, 0))
	}
	veri := make([]float32, n)
	tampon := make([]byte, 1<<16)
	eb := int(boy)
	for i := 0; i < len(veri); {
		parca := min(len(tampon)/eb, len(veri)-i)
		if !k.tam(tampon[:parca*eb]) {
			return nil, k.eksik(sira + " verisi")
		}
		for j := 0; j < parca; j++ {
			if eb == 2 {
				veri[i+j] = tensor.FromBits16(tur, binary.LittleEndian.Uint16(tampon[j*2:]))
			} else {
				veri[i+j] = math.Float32frombits(binary.LittleEndian.Uint32(tampon[j*4:]))
			}
		}
		i += parca
	}
	return veri, nil
}

// saglama dosyanın sonundaki sağlama toplamını denetler; sonrasında veri kalmamalıdır.
func (k *kayitOkuyucu) saglama() error {
	beklenen := k.crc.Sum32()
	if _, err := io.ReadFull(k.br, k.b[:4]); err != nil {
		return fmt.Errorf("dosya bozuk ya da eksik: sağlama toplamı okunurken dosya bitti")
	}
	if binary.LittleEndian.Uint32(k.b[:4]) != beklenen {
		return fmt.Errorf("dosya bozuk: sağlama toplamı tutmuyor (dosya değiştirilmiş ya da yarım yazılmış olabilir)")
	}
	if _, err := k.br.ReadByte(); err != io.EOF {
		return fmt.Errorf("dosya bozuk: sağlama toplamından sonra fazladan veri var")
	}
	return nil
}
