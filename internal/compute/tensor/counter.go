package tensor

import "time"

// ============================================================================
// Profil sayaçları: her zaman açık, çok ucuz tensör işlemi sayaçları
// ============================================================================
//
// Canlı profil (trlang watch, trlang profile) tensör işlemlerinin türüne, yönüne (ileri / geri
// yayılım) ve cihazına göre çağrı sayısını, süresini ve ayırdığı baytı bu sayaçlardan
// okur. Sayaçlar her zaman açıktır; açma/kapama yoktur. Bir işlemin maliyeti:
//
//   - Çağrı sayısı ve bayt kesindir: her çağrıda bir toplama.
//   - Süre her çağrıda ölçülür (CPU'da ve GPU'da): iki saat okuması, toplam kesindir
//     (yükün ölçüsü: counter_bench_test.go ve docs/architecture.md). Kilitsiz çalışan
//     eşzamanlı çekirdeklerin süreleri üst üste sayılır; toplam duvar süresini aşabilir.
//     Gövdesi panikle biten çağrı sayılır, süresi eklenmez.
//   - Saat amd64'te işlemcinin değişmez zaman damgası sayacıdır (okuması ~7 ns, çözünürlüğü
//     nanosaniyenin altında); yoksa Go'nun monoton saati. Tıklar görüntü alınırken duvar
//     saatiyle süreye çevrilir (counter_clock*.go).
//
// Eşzamanlılık: sayaçlar kilitsizdir. Yarış denetimsiz derlemede düz okuma ve yazma
// kullanılır (counter_plain.go): aynı anda iki goroutine'den çağrılan işlemlerde nadiren bir
// artırım kaybolabilir; TR görevleri yorumlayıcı kilidiyle sıraya girdiğinden pratikte
// olmaz. Kelime boyundaki değerler bozulmaz. Yarış denetimli derlemede (-race) aynı yollar
// atomik işlemlerle çalışır (counter_race.go); davranış aynıdır.
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
	OpKindGeluExact
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
	OpKindFused
	OpKindAssign
	// OpKindCount işlem türlerinin sayısıdır (dizi boyu).
	OpKindCount
)

// opCodes dosya biçimlerinde (profile.json) kullanılan, yazım dilinden bağımsız
// değişmez anahtarlardır: Go adları gibi İngilizce tanımlayıcı, OpKind sabitinin adı küçük harfle
// ve alt çizgiyle (OpKindLessEq → less_eq); PyTorch'un işlem adı tek sözcükse onun yazımı
// (matmul, argmax, cumsum, topk, argsort). İnsana gösterilen ad profiler paketindedir (opName):
// tensör paketi kullanıcıya görünen işlem adı tutmaz.
var opCodes = [OpKindCount]string{
	"other", "add", "sub", "mul", "div", "atan2", "less", "greater", "less_eq",
	"greater_eq", "exp", "log", "sqrt", "pow", "tanh", "relu", "gelu", "sigmoid",
	"add_scalar", "mul_scalar", "rsub_scalar", "div_scalar", "sin", "cos",
	"silu", "tan", "asin", "acos", "atan", "sinh", "cosh", "log1p", "log2",
	"log10", "expm1", "floor", "ceil", "sign", "abs", "gelu_exact", "matmul", "sum",
	"mean", "max", "argmax", "softmax", "log_softmax", "embedding", "cross_entropy",
	"layer_norm", "rms_norm", "attention", "clamp", "where", "cumsum",
	"topk", "argsort", "concat", "view", "copy", "cast",
	"to_device", "to_host", "create", "zero_grad", "sgd", "adamw",
	"clip_grad", "custom", "fused", "assign",
}

// Code işlemin dosya biçimlerindeki değişmez anahtarıdır (ör. "matmul").
func (i OpKind) Code() string {
	if i < OpKindCount {
		return opCodes[i]
	}
	return opCodes[OpKindOther]
}

// String işlemin değişmez anahtarıdır (Code ile aynı); hata ayıklama çıktısı içindir, kullanıcıya
// görünen ad değildir.
func (i OpKind) String() string { return i.Code() }

// counterRecord bir (cihaz, yön, işlem) üçlüsünün sayaçlarıdır.
type counterRecord struct {
	call        int64 // çağrı sayısı (kesin)
	dur         int64 // saat tıkı (toplam süre)
	measurement int64 // süresi ölçülen çağrı sayısı
	byteN       int64 // ayrılan bayt (çıktılar ve gradyanlar)
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
// türüyle sayılır; sayaçsız işlemlerin düğümü "diğer" olur); örnekleyici (trlang watch) zamanı
// tensör işlemi / TR kodu diye ona bakarak ayırır. Gösterici değil numaradır: küresel bir
// göstericiye her yazma, çöp toplayıcı işaretlerken yazma engelinden (write barrier) geçerdi;
// çok iş parçacıklı, çok ayırmalı küçük işlemlerde bu, sayaçların yükünü üçe katlıyordu.
var activeNo int32

// preparedNo son counterStart'ın kaydının numarasıdır; newKernel onu çekirdeğe alıp sıfırlar
// (Kernel.Abandon). activeNo'dan ayrıdır: onu kilitsiz çalışan başka çekirdeğin finish'i
// sıfırlayabilir; bunu yalnız kilit altında çağrılan counterStart yazar.
var preparedNo int32

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
//	sy := counterStart(be, OpKindMatMul).start()
//	... hesap ...
//	r := track(out, ...) // düğüm etkin işlemin türüyle etiketlenir
//	sy.finish()
//	return r
//
// Ertelemeli kapatma (defer) kullanılmaz: küçük işlemlerde payı ölçülür büyüklükteydi.
// İşlem panikle biterse (ör. şekil hatası) etkin işlem bir sonraki işleme kadar kalır;
// yalnızca örnekleyicinin o arayı tensör işlemi sayması demektir; çağrı sayılmış, süresi
// eklenmemiş olur. İç içe işlemde (bir sayaçlı işlemin içinden çağrılan başka biri) içteki
// bitince etkin işlem sıfırlanır; tensör paketinde işlemler birbirini çağırmaz (iç kopyalar
// arka uca doğrudan gider).
type counterMeasure struct {
	k     *counterRecord // nil: sayaç başlamadı (sıfır değer; finish yalnız etkin işlemi sıfırlar)
	begin int64          // başlangıç tıkı
}

// counterStart be cihazında is işleminin (ileri yön) bir çağrısını sayar ve etkin işlem yapar;
// saati start başlatır. Kullanım:
//
//	sy := counterStart(be, OpKindMatMul).start()
func counterStart(be Backend, is OpKind) (s counterMeasure) {
	k := &counters[deviceNo(be)&1][0][is]
	add64(&k.call, 1)
	write32i(&activeNo, k.no)
	write32i(&preparedNo, k.no)
	s.k = k
	return s
}

// start çağrının saatini başlatır (bkz. counterStart).
func (s counterMeasure) start() counterMeasure {
	s.begin = clock()
	return s
}

// finish etkin işlemi sıfırlar ve süreyi ekler.
func (s counterMeasure) finish() {
	write32i(&activeNo, 0)
	if s.k == nil {
		return
	}
	d := clock() - s.begin
	if d < 0 {
		d = 0 // çekirdekler arası saat farkı
	}
	add64(&s.k.dur, d)
	add64(&s.k.measurement, 1)
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
func (n *node) backwardCounted(ctx any) {
	c := deviceNo(n.be) & 1
	if n.outer != nil {
		add64(&counters[c][1][OpKindCustom].call, 1)
		store64u(&backwardPos, 0) // dış geri hesap TR kodudur: süresi kendi satırlarındadır
		n.outer.back(ctx)
		return
	}
	k := &counters[c][1][n.opKind]
	add64(&k.call, 1)
	write32i(&activeNo, k.no)
	store64u(&backwardPos, n.pos)
	s := counterMeasure{k: k}.start()
	// Ertelemesiz (ileri işlemlerdeki gibi): geri hesap panikle biterse etkin işlem bir
	// sonraki işleme kadar kalır.
	n.backward(n)
	s.finish()
}

// CounterRecord bir (cihaz türü, yön, işlem) üçlüsünün süreç başından beri biriken
// sayaçlarıdır.
type CounterRecord struct {
	Device   DeviceKind
	Backward bool // geri yayılım (gradyan hesabı)
	OpKind   OpKind
	// Calls çağrı sayısıdır (kesin); Measured süresi ölçülen çağrı sayısıdır (gövdesi panikle
	// biten çağrı ve dış işlemin geri hesabı dışında Calls'a eşit).
	Calls, Measured int64
	// Duration konakta geçen toplam süredir (bkz. dosya başı).
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
