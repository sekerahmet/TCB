package tensor

import (
	"math"
	"time"
)

// ============================================================================
// Profil sayaçları: her zaman açık, çok ucuz tensör işlemi sayaçları
// ============================================================================
//
// Canlı profil (tr izle, tr profil) tensör işlemlerinin türüne, yönüne (ileri / geri
// yayılım) ve cihazına göre çağrı sayısını, süresini ve ayırdığı baytı bu sayaçlardan
// okur. Sayaçlar her zaman açıktır; açma/kapama yoktur. Bir işlemin maliyeti:
//
//   - Çağrı sayısı ve bayt kesindir: her çağrıda bir toplama.
//   - Süre örneklenerek ölçülür: her kayıt, ölçülecek çağrıları olasılığı p olan bağımsız
//     (geometrik aralıklı) seçimle belirler ve ölçülen süreyi 1/p ile ağırlıklandırır
//     (Horvitz-Thompson tahmini: p yalnızca geçmiş ölçümlere bağlı olduğundan yansızdır).
//     Uzun süren işlemlerde (son ölçüm uzunIslemTik'ten uzunsa) p = 1: her çağrı ölçülür;
//     kısa işlemlerde p = 1/ornekAraligi. Böylece birkaç yüz nanosaniyelik küçük tensör
//     işlemlerinde ölçülmeyen çağrının yükü bir toplama, bir gösterici yazması ve bir
//     azaltmadır (ölçüm: sayac_hiz_test.go ve docs/mimari.md). GPU işlemlerinin her çağrısı
//     ölçülür (olcumBitir).
//   - Saat amd64'te işlemcinin değişmez zaman damgası sayacıdır (okuması ~7 ns, çözünürlüğü
//     nanosaniyenin altında); yoksa Go'nun monoton saati. Tıklar görüntü alınırken duvar
//     saatiyle süreye çevrilir (sayac_saat*.go).
//
// Eşzamanlılık: sayaçlar kilitsizdir. Yarış denetimsiz derlemede düz okuma ve yazma
// kullanılır (sayac_duz.go): aynı anda iki goroutine'den çağrılan işlemlerde nadiren bir
// artırım kaybolabilir; TR görevleri yorumlayıcı kilidiyle sıraya girdiğinden pratikte
// olmaz. Kelime boyundaki değerler bozulmaz. Yarış denetimli derlemede (-race) aynı yollar
// atomik işlemlerle çalışır (sayac_yaris.go); davranış aynıdır.
//
// GPU: cihaz işleri eşzamansız olduğundan konakta ölçülen süre GPU işlemlerinde yalnızca
// başlatma süresidir (eşzamanlamada, ör. konağa okumada bekleme görünür). Çekirdeklerin
// cihazdaki gerçek süresini GPU arka ucunun kendi profili ölçer (internal/hesap/gpu,
// profil_arayuz.go); canlı profil ikisini yan yana gösterir.

// Islem profil sayaçlarının işlem türüdür. Sayaçlar işlem türü, yön (ileri / geri yayılım)
// ve cihaz türüne göre ayrı tutulur.
type Islem uint8

// İşlem türleri. İkili işlemler BinOp, tekli işlemler UnOp sırasıyla aynı sıradadır (IslemToplama +
// Islem(op), IslemUstel + Islem(op); sayac_test.go denetler).
const (
	IslemDiger Islem = iota
	// İkili işlemler (BinOp sırası).
	IslemToplama
	IslemCikarma
	IslemCarpma
	IslemBolme
	IslemAtan2
	IslemKucuk
	IslemBuyuk
	IslemKucukEsit
	IslemBuyukEsit
	// Tekli işlemler (UnOp sırası).
	IslemUstel
	IslemLog
	IslemKarekok
	IslemUs
	IslemTanh
	IslemRelu
	IslemGelu
	IslemSigmoid
	IslemSkalerToplama
	IslemSkalerCarpma
	IslemSkalerdenCikarma
	IslemSkalerBolme
	IslemSinus
	IslemKosinus
	IslemSilu
	IslemTanjant
	IslemArksinus
	IslemArkkosinus
	IslemArktanjant
	IslemSinh
	IslemCosh
	IslemLog1p
	IslemLog2
	IslemLog10
	IslemUstelEksiBir
	IslemTaban
	IslemTavan
	IslemIsaret
	IslemMutlak
	// Diğer işlemler.
	IslemMatrisCarpimi
	IslemToplam
	IslemOrtalama
	IslemEnBuyuk
	IslemEnBuyukIndeks
	IslemSoftmax
	IslemLogSoftmax
	IslemGomme
	IslemCaprazEntropi
	IslemKatmanNormu
	IslemRMSNormu
	IslemDikkat
	IslemSinirlama
	IslemKosulluSecim
	IslemBirikimliToplam
	IslemEnBuyukK
	IslemSiralamaIndeksi
	IslemBirlestirme
	IslemGorunum
	IslemKopya
	IslemTurDonusumu
	IslemCihazaTasima
	IslemKonagaOkuma
	IslemOlusturma
	IslemGradyanSifirlama
	IslemSGD
	IslemAdamW
	IslemGradyanKirpma
	IslemOzel
	// IslemSayisi işlem türlerinin sayısıdır (dizi boyu).
	IslemSayisi
)

// islemKodlari dosya biçimlerinde (profil.json) kullanılan, yazım dilinden bağımsız
// değişmez anahtarlardır; islemAdlariTR insana gösterilen açıklamalardır.
var islemKodlari = [IslemSayisi]string{
	"diger", "toplama", "cikarma", "carpma", "bolme", "atan2", "kucuk", "buyuk", "kucuk_esit",
	"buyuk_esit", "ustel", "log", "karekok", "us", "tanh", "relu", "gelu", "sigmoid",
	"skaler_toplama", "skaler_carpma", "skalerden_cikarma", "skaler_bolme", "sinus", "kosinus",
	"silu", "tanjant", "arksinus", "arkkosinus", "arktanjant", "sinh", "cosh", "log1p", "log2",
	"log10", "ustel_eksi_bir", "taban", "tavan", "isaret", "mutlak", "matris_carpimi", "toplam",
	"ortalama", "en_buyuk", "en_buyuk_indeks", "softmax", "log_softmax", "gomme", "capraz_entropi",
	"katman_normu", "rms_normu", "dikkat", "sinirlama", "kosullu_secim", "birikimli_toplam",
	"en_buyuk_k", "siralama_indeksi", "birlestirme", "gorunum", "kopya", "tur_donusumu",
	"cihaza_tasima", "konaga_okuma", "olusturma", "gradyan_sifirlama", "sgd", "adamw",
	"gradyan_kirpma", "ozel",
}

var islemAdlariTR = [IslemSayisi]string{
	"diğer", "toplama", "çıkarma", "çarpma", "bölme", "atan2", "küçüktür", "büyüktür", "küçük eşit",
	"büyük eşit", "üstel", "logaritma", "karekök", "üs alma", "tanh", "relu", "gelu", "sigmoid",
	"skaler toplama", "skaler çarpma", "skalerden çıkarma", "skaler bölme", "sinüs", "kosinüs",
	"silu", "tanjant", "arksinüs", "arkkosinüs", "arktanjant", "sinh", "cosh", "log1p", "log2",
	"log10", "üstel eksi bir", "taban", "tavan", "işaret", "mutlak değer", "matris çarpımı", "toplam",
	"ortalama", "en büyük", "en büyüğün indeksi", "softmax", "log softmax", "gömme", "çapraz entropi",
	"katman normu", "rms normu", "dikkat", "sınırlama", "koşullu seçim", "birikimli toplam",
	"en büyük k", "sıralama indeksi", "birleştirme", "görünüm", "kopya", "tür dönüşümü",
	"cihaza taşıma", "konağa okuma", "tensör oluşturma", "gradyan sıfırlama", "SGD adımı",
	"AdamW adımı", "gradyan kırpma", "özel işlem",
}

// Kod işlemin dosya biçimlerindeki değişmez anahtarıdır (ör. "matris_carpimi").
func (i Islem) Kod() string {
	if i < IslemSayisi {
		return islemKodlari[i]
	}
	return islemKodlari[IslemDiger]
}

// String işlemin Türkçe açıklamasıdır (ör. "matris çarpımı").
func (i Islem) String() string {
	if i < IslemSayisi {
		return islemAdlariTR[i]
	}
	return islemAdlariTR[IslemDiger]
}

// uzunIslemTik, son ölçümü bundan uzun süren işlemin her çağrısının ölçüldüğü sınırdır (saat
// tıkı; 3 GHz'lik sayaçta ~10 µs, monoton saatte 30 µs). Bu uzunluktaki bir işlemde iki saat
// okuması (~15 ns) önemsizdir. Sınır yalnızca verimi etkiler, tahmin her durumda yansızdır.
const uzunIslemTik = 30000

// ornekAraligi kısa işlemlerde ortalama kaç çağrıda bir sürenin ölçüldüğüdür (1/p).
const ornekAraligi = 32

// sayacKaydi bir (cihaz, yön, işlem) üçlüsünün sayaçlarıdır.
type sayacKaydi struct {
	cagri int64 // çağrı sayısı (kesin)
	sure  int64 // ağırlıklandırılmış saat tıkı (tahmini toplam süre)
	olcum int64 // süresi ölçülen çağrı sayısı
	bayt  int64 // ayrılan bayt (çıktılar ve gradyanlar)
	geri  int32 // sonraki ölçüme kalan çağrı (≤ 0: bu çağrı ölçülür)
	agr   int32 // geçerli ağırlık 1/p (0: henüz ölçülmedi, p = 1)
	rnd   int32 // aralık seçen sözde rastgele durum
	no    int32 // kaydın kimliği (kayitNo: cihaz, yön, işlem); init'te yazılır, değişmez
}

// Sayaç tablosu: [cihaz (0 cpu, 1 diğer)][yön (0 ileri, 1 geri)][işlem]. İşlem ekseni 256
// uzunluktadır: Islem (uint8) ile indekslemede sınır denetimi gerekmez.
var sayaclar [2][2][256]sayacKaydi

func init() {
	for c := range sayaclar {
		for y := range sayaclar[c] {
			for i := range sayaclar[c][y] {
				sayaclar[c][y][i].no = kayitNo(c, y, Islem(i))
			}
		}
	}
}

// etkinNo o anda çalışan işlemin kaydının numarasıdır (kayitNo; 0: tensör işlemi yok). Bayt
// sayacı ayırmayı ona yazar; track düğümü onunla etiketler (geri yayılım düğümün işlem
// türüyle sayılır; sayaçsız işlemlerin düğümü "diğer" olur); örnekleyici (tr izle) zamanı
// tensör işlemi / TR kodu diye ona bakarak ayırır. Gösterici değil numaradır: küresel bir
// göstericiye her yazma, çöp toplayıcı işaretlerken yazma engelinden (write barrier) geçerdi;
// çok iş parçacıklı, çok ayırmalı küçük işlemlerde bu, sayaçların yükünü üçe katlıyordu.
var etkinNo int32

// kayitNo (cihaz, yön, işlem) kaydının numarasıdır (0 "yok" anlamına gelir: +1).
func kayitNo(c, yon int, is Islem) int32 { return int32(c<<9|yon<<8|int(is)) + 1 }

// cihazNo sayaç tablosundaki cihaz indeksidir.
func cihazNo(be Backend) int {
	if _, ok := be.(cpuBackend); ok {
		return 0
	}
	return 1
}

// sayacOlcumu bir işlemin başladığı anın bilgisidir; işlem bitince bitir ile kapatılır:
//
//	sy := sayacBasla(be, IslemMatrisCarpimi).baslat()
//	... hesap ...
//	r := track(out, ...) // düğüm etkin işlemin türüyle etiketlenir
//	sy.bitir()
//	return r
//
// Ertelemeli kapatma (defer) kullanılmaz: küçük işlemlerde payı ölçülür büyüklükteydi.
// İşlem panikle biterse (ör. şekil hatası) etkin işlem bir sonraki işleme kadar kalır;
// yalnızca örnekleyicinin o arayı tensör işlemi sayması demektir. İç içe işlemde (bir
// sayaçlı işlemin içinden çağrılan başka biri) içteki bitince etkin işlem sıfırlanır;
// tensör paketinde işlemler birbirini çağırmaz (iç kopyalar arka uca doğrudan gider).
type sayacOlcumu struct {
	k       *sayacKaydi // nil: bu çağrının süresi ölçülmüyor
	bas     int64       // başlangıç tıkı
	agirlik int64       // ölçülen sürenin ağırlığı (temsil ettiği çağrı sayısı)
}

// sayacBasla be cihazında is işleminin (ileri yön) bir çağrısını sayar ve etkin işlem yapar.
// Çağrının süresi ölçülecekse dönen değerin kaydı doludur; saati baslat başlatır. İkisi
// ayrıdır ve ikisi de satır içine alınır: ölçülmeyen çağrıda işlev çağrısı yoktur (çağrı,
// çağıranın yazmaçlarını yığına yazdırdığı için küçük işlemde ölçülür bir paydı; satır içine
// alma sınırı çağrı içeren tek bir işlevde bu kadar işe yetmiyor). Kullanım:
//
//	sy := sayacBasla(be, IslemMatrisCarpimi).baslat()
func sayacBasla(be Backend, is Islem) (s sayacOlcumu) {
	k := &sayaclar[cihazNo(be)&1][0][is]
	ekle64(&k.cagri, 1)
	yaz32i(&etkinNo, k.no)
	if ekle32(&k.geri, -1) <= 0 {
		s.k = k
	}
	return s
}

// baslat ölçülecek çağrının saatini başlatır (bkz. sayacBasla).
func (s sayacOlcumu) baslat() sayacOlcumu {
	if s.k != nil {
		s.olcumBaslat()
	}
	return s
}

// olcumBaslat saati başlatır (soğuk yol); ağırlık, çağrı seçilirken geçerli olan 1/p'dir.
//
//go:noinline
func (s *sayacOlcumu) olcumBaslat() {
	s.agirlik = int64(max(oku32i(&s.k.agr), 1))
	s.bas = saat()
}

// sonrakiOlcum ölçülen sürenin d (tık) uzunluğuna göre p'yi seçer ve sonraki ölçülecek
// çağrıya kadarki aralığı geometrik dağılımdan çeker (her çağrı bağımsız olarak p olasılıkla
// seçilmiş olur; dönemli döngülerde örtüşme olmaz).
func sonrakiOlcum(k *sayacKaydi, d int64) {
	if d >= uzunIslemTik {
		yaz32i(&k.agr, 1)
		yaz32i(&k.geri, 1)
		return
	}
	r := uint32(oku32i(&k.rnd))
	if r == 0 {
		r = 0x9e3779b9 ^ uint32(saat())
	}
	r ^= r << 13
	r ^= r >> 17
	r ^= r << 5
	yaz32i(&k.rnd, int32(r))
	// U ∈ (0, 1]; aralık = 1 + ⌊ln U / ln(1-p)⌋ (geometrik, ortalaması 1/p).
	u := (float64(r>>8) + 1) / (1 << 24)
	aralik := 1 + int32(math.Log(u)/math.Log1p(-1.0/ornekAraligi))
	yaz32i(&k.agr, ornekAraligi)
	yaz32i(&k.geri, aralik)
}

// bitir etkin işlemi sıfırlar ve süreyi (ölçülüyorsa) ekler.
func (s sayacOlcumu) bitir() {
	yaz32i(&etkinNo, 0)
	if s.k != nil {
		s.olcumBitir()
	}
}

// olcumBitir ölçülen süreyi ağırlığıyla ekler (soğuk yol).
//
//go:noinline
func (s sayacOlcumu) olcumBitir() {
	d := saat() - s.bas
	if d < 0 {
		d = 0 // çekirdekler arası saat farkı
	}
	ekle64(&s.k.sure, d*s.agirlik)
	ekle64(&s.k.olcum, 1)
	if (s.k.no-1)>>9 != 0 { // cihaz GPU
		// GPU işlemleri hep ölçülür (ağırlık 1): bir başlatma mikrosaniyeler sürer, iki saat
		// okuması önemsizdir; eşzamanlama (konağa okuma) ise az sayıda ve uzun olabilir,
		// örneklenirse tahmini çok oynak olurdu.
		yaz32i(&s.k.geri, 1)
		return
	}
	sonrakiOlcum(s.k, d)
}

// sayacBayt be cihazında etkin işlemin ayırdığı baytı ekler; işlem çalışmıyorsa cihazın
// "diğer" kaydına (işlem dışı ayırmalar).
func sayacBayt(be Backend, bayt int) {
	n := oku32i(&etkinNo) - 1
	if n < 0 {
		n = int32(cihazNo(be)) << 9 // (cihaz, ileri, IslemDiger)
	}
	ekle64(&sayaclar[(n>>9)&1][(n>>8)&1][n&255].bayt, int64(bayt))
}

// etkinTur çalışan işlemin türüdür (yoksa IslemDiger); track düğümü bununla etiketler.
func etkinTur() Islem {
	if n := oku32i(&etkinNo); n != 0 {
		return Islem(n - 1) // numaranın alt 8 biti işlemdir (kayitNo)
	}
	return IslemDiger
}

// geriSayacli düğümün geri hesabını geri yayılım sayacıyla çalıştırır. Dış (özel) işlemin
// geri hesabı TR kodu çalıştırır ve içindeki tensör işlemleri kendi satırlarında sayılır;
// süresi iki kez sayılmasın diye yalnızca çağrısı sayılır.
func (t *Tensor) geriSayacli(ctx any) {
	c := cihazNo(t.be) & 1
	if t.dis != nil {
		ekle64(&sayaclar[c][1][IslemOzel].cagri, 1)
		t.geriCalistir(ctx)
		return
	}
	k := &sayaclar[c][1][t.islem]
	ekle64(&k.cagri, 1)
	yaz32i(&etkinNo, k.no)
	var s sayacOlcumu
	if ekle32(&k.geri, -1) <= 0 {
		s.k = k
		s.olcumBaslat()
	}
	// Ertelemesiz (ileri işlemlerdeki gibi): geri hesap panikle biterse etkin işlem bir
	// sonraki işleme kadar kalır.
	t.backward()
	s.bitir()
}

// SayacKaydi bir (cihaz türü, yön, işlem) üçlüsünün süreç başından beri biriken
// sayaçlarıdır.
type SayacKaydi struct {
	Cihaz DeviceKind
	Geri  bool // geri yayılım (gradyan hesabı)
	Islem Islem
	// Cagri çağrı sayısıdır (kesin); Olcum süresi ölçülen çağrı sayısıdır.
	Cagri, Olcum int64
	// Sure konakta geçen tahmini toplam süredir (bkz. dosya başı).
	Sure time.Duration
	// Bayt işlemin çıktıları ve gradyanları için ayrılan bayttır.
	Bayt int64
}

// SayacGoruntusu süreç başından beri biriken sayaçların sıfır olmayan kayıtlarını verir.
// Herhangi bir goroutine'den çağrılabilir; okuma kilitsizdir (bkz. dosya başı).
func SayacGoruntusu() []SayacKaydi {
	oran := tikSureOrani()
	var out []SayacKaydi
	for c := range sayaclar {
		for yon := range sayaclar[c] {
			for i := Islem(0); i < IslemSayisi; i++ {
				k := &sayaclar[c][yon][i]
				cagri, bayt := oku64(&k.cagri), oku64(&k.bayt)
				if cagri == 0 && bayt == 0 {
					continue
				}
				out = append(out, SayacKaydi{
					Cihaz: DeviceKind(c), Geri: yon == 1, Islem: i, Cagri: cagri, Olcum: oku64(&k.olcum),
					Sure: time.Duration(float64(oku64(&k.sure)) * oran), Bayt: bayt,
				})
			}
		}
	}
	return out
}

// EtkinIslem şu anda bir tensör işlemi çalışıyorsa onun türünü, yönünü ve doğru verir
// (örnekleyici zamanı tensör işlemi / TR kodu diye ayırır).
func EtkinIslem() (is Islem, geri, calisiyor bool) {
	n := oku32i(&etkinNo)
	if n == 0 {
		return IslemDiger, false, false
	}
	n--
	return Islem(n), (n>>8)&1 == 1, true
}
