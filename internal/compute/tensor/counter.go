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
// cihazdaki gerçek süresini GPU arka ucunun kendi profili ölçer (internal/compute/gpu,
// profil_arayuz.go); canlı profil ikisini yan yana gösterir.

// OpKind profil sayaçlarının işlem türüdür. Sayaçlar işlem türü, yön (ileri / geri yayılım)
// ve cihaz türüne göre ayrı tutulur.
type OpKind uint8

// İşlem türleri. İkili işlemler BinOp, tekli işlemler UnOp sırasıyla aynı sıradadır (IslemToplama +
// Islem(op), IslemUstel + Islem(op); sayac_test.go denetler).
const (
	OpKindOther OpKind = iota
	// İkili işlemler (BinOp sırası).
	OpKindAdd
	OpKindSub
	OpKindMul
	OpKindDiv
	OpKindAtan2
	OpKindLess
	OpKindGreater
	OpKindLessEq
	OpKindGreaterEq
	// Tekli işlemler (UnOp sırası).
	OpKindExp
	OpKindLog
	OpKindSqrt
	OpKindPow
	OpKindTanh
	OpKindRelu
	OpKindGelu
	OpKindSigmoid
	OpKindAddScalar
	OpKindMulScalar
	OpKindRSubScalar
	OpKindDivScalar
	OpKindSin
	OpKindCos
	OpKindSilu
	OpKindTan
	OpKindAsin
	OpKindAcos
	OpKindAtan
	OpKindSinh
	OpKindCosh
	OpKindLog1p
	OpKindLog2
	OpKindLog10
	OpKindExpm1
	OpKindFloor
	OpKindCeil
	OpKindSign
	OpKindAbs
	// Diğer işlemler.
	OpKindMatMul
	OpKindSum
	OpKindMean
	OpKindMax
	OpKindArgMax
	OpKindSoftmax
	OpKindLogSoftmax
	OpKindEmbedding
	OpKindCrossEntropy
	OpKindLayerNorm
	OpKindRMSNorm
	OpKindAttention
	OpKindClamp
	OpKindWhere
	OpKindCumSum
	OpKindTopK
	OpKindArgSort
	OpKindConcat
	OpKindView
	OpKindCopy
	OpKindCast
	OpKindToDevice
	OpKindToHost
	OpKindCreate
	OpKindZeroGrad
	OpKindSGD
	OpKindAdamW
	OpKindClipGrad
	OpKindCustom
	// OpKindCount işlem türlerinin sayısıdır (dizi boyu).
	OpKindCount
)

// opCodes dosya biçimlerinde (profil.json) kullanılan, yazım dilinden bağımsız
// değişmez anahtarlardır; islemAdlariTR insana gösterilen açıklamalardır.
var opCodes = [OpKindCount]string{
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

var opNamesTR = [OpKindCount]string{
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
func (i OpKind) Code() string {
	if i < OpKindCount {
		return opCodes[i]
	}
	return opCodes[OpKindOther]
}

// String işlemin Türkçe açıklamasıdır (ör. "matris çarpımı").
func (i OpKind) String() string {
	if i < OpKindCount {
		return opNamesTR[i]
	}
	return opNamesTR[OpKindOther]
}

// longOpTicks, son ölçümü bundan uzun süren işlemin her çağrısının ölçüldüğü sınırdır (saat
// tıkı; 3 GHz'lik sayaçta ~10 µs, monoton saatte 30 µs). Bu uzunluktaki bir işlemde iki saat
// okuması (~15 ns) önemsizdir. Sınır yalnızca verimi etkiler, tahmin her durumda yansızdır.
const longOpTicks = 30000

// sampleRange kısa işlemlerde ortalama kaç çağrıda bir sürenin ölçüldüğüdür (1/p).
const sampleRange = 32

// counterRecord bir (cihaz, yön, işlem) üçlüsünün sayaçlarıdır.
type counterRecord struct {
	call        int64 // çağrı sayısı (kesin)
	dur         int64 // ağırlıklandırılmış saat tıkı (tahmini toplam süre)
	measurement int64 // süresi ölçülen çağrı sayısı
	byteN       int64 // ayrılan bayt (çıktılar ve gradyanlar)
	back        int32 // sonraki ölçüme kalan çağrı (≤ 0: bu çağrı ölçülür)
	wt          int32 // geçerli ağırlık 1/p (0: henüz ölçülmedi, p = 1)
	rnd         int32 // aralık seçen sözde rastgele durum
	no          int32 // kaydın kimliği (kayitNo: cihaz, yön, işlem); init'te yazılır, değişmez
}

// Sayaç tablosu: [cihaz (0 cpu, 1 diğer)][yön (0 ileri, 1 geri)][işlem]. İşlem ekseni 256
// uzunluktadır: Islem (uint8) ile indekslemede sınır denetimi gerekmez.
var counters [2][2][256]counterRecord

func init() {
	for c := range counters {
		for y := range counters[c] {
			for i := range counters[c][y] {
				counters[c][y][i].no = recordNo(c, y, OpKind(i))
			}
		}
	}
}

// activeNo o anda çalışan işlemin kaydının numarasıdır (kayitNo; 0: tensör işlemi yok). Bayt
// sayacı ayırmayı ona yazar; track düğümü onunla etiketler (geri yayılım düğümün işlem
// türüyle sayılır; sayaçsız işlemlerin düğümü "diğer" olur); örnekleyici (tr izle) zamanı
// tensör işlemi / TR kodu diye ona bakarak ayırır. Gösterici değil numaradır: küresel bir
// göstericiye her yazma, çöp toplayıcı işaretlerken yazma engelinden (write barrier) geçerdi;
// çok iş parçacıklı, çok ayırmalı küçük işlemlerde bu, sayaçların yükünü üçe katlıyordu.
var activeNo int32

// recordNo (cihaz, yön, işlem) kaydının numarasıdır (0 "yok" anlamına gelir: +1).
func recordNo(c, direction int, is OpKind) int32 { return int32(c<<9|direction<<8|int(is)) + 1 }

// deviceNo sayaç tablosundaki cihaz indeksidir.
func deviceNo(be Backend) int {
	if _, ok := be.(cpuBackend); ok {
		return 0
	}
	return 1
}

// counterMeasure bir işlemin başladığı anın bilgisidir; işlem bitince bitir ile kapatılır:
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
type counterMeasure struct {
	k      *counterRecord // nil: bu çağrının süresi ölçülmüyor
	begin  int64          // başlangıç tıkı
	weight int64          // ölçülen sürenin ağırlığı (temsil ettiği çağrı sayısı)
}

// counterStart be cihazında is işleminin (ileri yön) bir çağrısını sayar ve etkin işlem yapar.
// Çağrının süresi ölçülecekse dönen değerin kaydı doludur; saati baslat başlatır. İkisi
// ayrıdır ve ikisi de satır içine alınır: ölçülmeyen çağrıda işlev çağrısı yoktur (çağrı,
// çağıranın yazmaçlarını yığına yazdırdığı için küçük işlemde ölçülür bir paydı; satır içine
// alma sınırı çağrı içeren tek bir işlevde bu kadar işe yetmiyor). Kullanım:
//
//	sy := sayacBasla(be, IslemMatrisCarpimi).baslat()
func counterStart(be Backend, is OpKind) (s counterMeasure) {
	k := &counters[deviceNo(be)&1][0][is]
	add64(&k.call, 1)
	write32i(&activeNo, k.no)
	if add32(&k.back, -1) <= 0 {
		s.k = k
	}
	return s
}

// start ölçülecek çağrının saatini başlatır (bkz. sayacBasla).
func (s counterMeasure) start() counterMeasure {
	if s.k != nil {
		s.startMeasure()
	}
	return s
}

// startMeasure saati başlatır (soğuk yol); ağırlık, çağrı seçilirken geçerli olan 1/p'dir.
//
//go:noinline
func (s *counterMeasure) startMeasure() {
	s.weight = int64(max(read32i(&s.k.wt), 1))
	s.begin = clock()
}

// nextMeasure ölçülen sürenin d (tık) uzunluğuna göre p'yi seçer ve sonraki ölçülecek
// çağrıya kadarki aralığı geometrik dağılımdan çeker (her çağrı bağımsız olarak p olasılıkla
// seçilmiş olur; dönemli döngülerde örtüşme olmaz).
func nextMeasure(k *counterRecord, d int64) {
	if d >= longOpTicks {
		write32i(&k.wt, 1)
		write32i(&k.back, 1)
		return
	}
	r := uint32(read32i(&k.rnd))
	if r == 0 {
		r = 0x9e3779b9 ^ uint32(clock())
	}
	r ^= r << 13
	r ^= r >> 17
	r ^= r << 5
	write32i(&k.rnd, int32(r))
	// U ∈ (0, 1]; aralık = 1 + ⌊ln U / ln(1-p)⌋ (geometrik, ortalaması 1/p).
	u := (float64(r>>8) + 1) / (1 << 24)
	span := 1 + int32(math.Log(u)/math.Log1p(-1.0/sampleRange))
	write32i(&k.wt, sampleRange)
	write32i(&k.back, span)
}

// finish etkin işlemi sıfırlar ve süreyi (ölçülüyorsa) ekler.
func (s counterMeasure) finish() {
	write32i(&activeNo, 0)
	if s.k != nil {
		s.endMeasure()
	}
}

// endMeasure ölçülen süreyi ağırlığıyla ekler (soğuk yol).
//
//go:noinline
func (s counterMeasure) endMeasure() {
	d := clock() - s.begin
	if d < 0 {
		d = 0 // çekirdekler arası saat farkı
	}
	add64(&s.k.dur, d*s.weight)
	add64(&s.k.measurement, 1)
	if (s.k.no-1)>>9 != 0 { // cihaz GPU
		// GPU işlemleri hep ölçülür (ağırlık 1): bir başlatma mikrosaniyeler sürer, iki saat
		// okuması önemsizdir; eşzamanlama (konağa okuma) ise az sayıda ve uzun olabilir,
		// örneklenirse tahmini çok oynak olurdu.
		write32i(&s.k.back, 1)
		return
	}
	nextMeasure(s.k, d)
}

// counterBytes be cihazında etkin işlemin ayırdığı baytı ekler; işlem çalışmıyorsa cihazın
// "diğer" kaydına (işlem dışı ayırmalar).
func counterBytes(be Backend, byteN int) {
	n := read32i(&activeNo) - 1
	if n < 0 {
		n = int32(deviceNo(be)) << 9 // (cihaz, ileri, IslemDiger)
	}
	add64(&counters[(n>>9)&1][(n>>8)&1][n&255].byteN, int64(byteN))
}

// activeKind çalışan işlemin türüdür (yoksa IslemDiger); track düğümü bununla etiketler.
func activeKind() OpKind {
	if n := read32i(&activeNo); n != 0 {
		return OpKind(n - 1) // numaranın alt 8 biti işlemdir (kayitNo)
	}
	return OpKindOther
}

// backwardCounted düğümün geri hesabını geri yayılım sayacıyla çalıştırır. Dış (özel) işlemin
// geri hesabı TR kodu çalıştırır ve içindeki tensör işlemleri kendi satırlarında sayılır;
// süresi iki kez sayılmasın diye yalnızca çağrısı sayılır.
func (t *Tensor) backwardCounted(ctx any) {
	c := deviceNo(t.be) & 1
	if t.outer != nil {
		add64(&counters[c][1][OpKindCustom].call, 1)
		t.runBackward(ctx)
		return
	}
	k := &counters[c][1][t.opKind]
	add64(&k.call, 1)
	write32i(&activeNo, k.no)
	var s counterMeasure
	if add32(&k.back, -1) <= 0 {
		s.k = k
		s.startMeasure()
	}
	// Ertelemesiz (ileri işlemlerdeki gibi): geri hesap panikle biterse etkin işlem bir
	// sonraki işleme kadar kalır.
	t.backward()
	s.finish()
}

// CounterRecord bir (cihaz türü, yön, işlem) üçlüsünün süreç başından beri biriken
// sayaçlarıdır.
type CounterRecord struct {
	Device   DeviceKind
	Backward bool // geri yayılım (gradyan hesabı)
	OpKind   OpKind
	// Calls çağrı sayısıdır (kesin); Olcum süresi ölçülen çağrı sayısıdır.
	Calls, Measured int64
	// Duration konakta geçen tahmini toplam süredir (bkz. dosya başı).
	Duration time.Duration
	// Bytes işlemin çıktıları ve gradyanları için ayrılan bayttır.
	Bytes int64
}

// CounterSnapshot süreç başından beri biriken sayaçların sıfır olmayan kayıtlarını verir.
// Herhangi bir goroutine'den çağrılabilir; okuma kilitsizdir (bkz. dosya başı).
func CounterSnapshot() []CounterRecord {
	ratio := tickDurationRatio()
	var out []CounterRecord
	for c := range counters {
		for direction := range counters[c] {
			for i := OpKind(0); i < OpKindCount; i++ {
				k := &counters[c][direction][i]
				call, byteN := read64(&k.call), read64(&k.byteN)
				if call == 0 && byteN == 0 {
					continue
				}
				out = append(out, CounterRecord{
					Device: DeviceKind(c), Backward: direction == 1, OpKind: i, Calls: call, Measured: read64(&k.measurement),
					Duration: time.Duration(float64(read64(&k.dur)) * ratio), Bytes: byteN,
				})
			}
		}
	}
	return out
}

// ActiveOp şu anda bir tensör işlemi çalışıyorsa onun türünü, yönünü ve doğru verir
// (örnekleyici zamanı tensör işlemi / TR kodu diye ayırır).
func ActiveOp() (is OpKind, back, running bool) {
	n := read32i(&activeNo)
	if n == 0 {
		return OpKindOther, false, false
	}
	n--
	return OpKind(n), (n>>8)&1 == 1, true
}
