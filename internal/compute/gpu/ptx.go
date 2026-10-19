package gpu

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ============================================================================
// PTX yazıcısı: çekirdekler Go içinden PTX metni olarak üretilir
// ============================================================================
//
// Her çekirdek tek giriş noktalı bir modüldür. Yazıcı yazmaçları türüne göre numaralar,
// parametreleri bildirim sırasıyla tutar (başlatıcı aynı sırayı kullanır ve imza
// denetlenir), etiketleri tekil üretir. Üretilen metin taban hedef sm_52 içindir ve
// yalnızca benzetimin (benzetim_*.go) de yürüttüğü komut altkümesini kullanır; bu yüzden
// her çekirdek NVIDIA donanımı olmadan da sınanabilir.

// yazmac bir PTX yazmacının adıdır (ör. "%f3").
type reg string

// Yazmaç türleri.
const (
	yzF  = iota // .f32
	yzR         // .b32 (u32/s32)
	yzRD        // .b64 (u64/s64, adres)
	yzP         // .pred
	yzH         // .b16
	yzFD        // .f64
	regClassCount
)

var regInfo = [regClassCount]struct{ dtype, prefix string }{
	yzF: {".f32", "%f"}, yzR: {".b32", "%r"}, yzRD: {".b64", "%rd"},
	yzP: {".pred", "%p"}, yzH: {".b16", "%h"}, yzFD: {".f64", "%fd"},
}

// paramTuru çekirdek parametresinin imzası: adı, bayt boyu, gösterici mi ve çekirdeğin
// gösterdiği belleğe yazıp yazmadığı. Başlatıcı, başlatma yerinin bildirdiği okuma/yazma
// ile imzayı karşılaştırır ve yazılan depolamaların konak gölgesini kendisi siler.
type paramType struct {
	ad       string
	byteN    int
	pointer  bool
	writable bool
}

// yazici bir PTX çekirdeğini kurar.
type emitter struct {
	ad        string
	signature []paramType
	param     []string
	sharedMem []string
	row       []string
	counter   [regClassCount]int
	labelNo   int
	// hedef PTX hedef mimarisi (52: taban sm_52; 80: cp.async kullanan çekirdekler).
	target int
	// dinamik doğruysa paylaşımlı diziler tek bir devingen dizinin (.extern .shared dsm)
	// parçalarıdır (48 KB üstü; başlatmada paylasimBayt kadar açılır). paylasimBayt bütün
	// paylaşımlı dizilerin (16'ya hizalı) toplamı.
	dynamic        bool
	sharedMemBytes int
}

func newEmitter(ad string) *emitter { return &emitter{ad: ad, target: 52} }

func (k *emitter) fresh2(t int) reg {
	k.counter[t]++
	return reg(regInfo[t].prefix + strconv.Itoa(k.counter[t]))
}

func (k *emitter) f() reg  { return k.fresh2(yzF) }
func (k *emitter) r() reg  { return k.fresh2(yzR) }
func (k *emitter) rd() reg { return k.fresh2(yzRD) }
func (k *emitter) p() reg  { return k.fresh2(yzP) }
func (k *emitter) h() reg  { return k.fresh2(yzH) }
func (k *emitter) fd() reg { return k.fresh2(yzFD) }

// terim bir işleneni PTX yazımına çevirir.
func term(a any) string {
	switch v := a.(type) {
	case reg:
		return string(v)
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int32:
		return strconv.Itoa(int(v))
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case float32:
		return fmt.Sprintf("0f%08X", math.Float32bits(v))
	case float64:
		return fmt.Sprintf("0d%016X", math.Float64bits(v))
	}
	panic(fmt.Sprintf("gpu: PTX yazıcısına bilinmeyen işlenen %T (iç hata)", a))
}

func (k *emitter) instruction(on, op string, args []any) {
	var sb strings.Builder
	sb.WriteString("\t")
	sb.WriteString(on)
	sb.WriteString(op)
	for i, a := range args {
		if i == 0 {
			sb.WriteString(" ")
		} else {
			sb.WriteString(", ")
		}
		sb.WriteString(term(a))
	}
	sb.WriteString(";")
	k.row = append(k.row, sb.String())
}

// e bir komut yazar: k.e("add.f32", d, a, b).
func (k *emitter) e(op string, args ...any) { k.instruction("", op, args) }

// eger koşullu komut yazar (@p op ...).
func (k *emitter) ifCase(p reg, op string, args ...any) { k.instruction("@"+string(p)+" ", op, args) }

// degilse koşulun yanlışlığına bağlı komut yazar (@!p op ...).
func (k *emitter) otherwise(p reg, op string, args ...any) {
	k.instruction("@!"+string(p)+" ", op, args)
}

// etiket tekil bir etiket adı üretir; koy ile yerleştirilir.
func (k *emitter) label() string {
	k.labelNo++
	return "L" + strconv.Itoa(k.labelNo)
}

func (k *emitter) put(l string) { k.row = append(k.row, l+":") }

// git koşulsuz dallanma.
func (k *emitter) jump(l string) { k.e("bra", l) }

// gitEger p doğruysa dallanır; gitDegilse p yanlışsa.
func (k *emitter) jumpIf(p reg, l string)     { k.ifCase(p, "bra", l) }
func (k *emitter) jumpUnless(p reg, l string) { k.otherwise(p, "bra", l) }

// engel bloktaki bütün iş parçacıklarını bekler.
func (k *emitter) barrier() { k.e("bar.sync", 0) }

// bel bir bellek işleneni yazar: [taban+ofset].
func mem(base2 reg, offset int) string {
	if offset == 0 {
		return "[" + string(base2) + "]"
	}
	return "[" + string(base2) + "+" + strconv.Itoa(offset) + "]"
}

// ----------------------------------------------------------------------------
// Parametreler
// ----------------------------------------------------------------------------

func (k *emitter) addParam(dtype, ad string, byteN int) {
	k.param = append(k.param, "\t.param "+dtype+" "+ad)
	k.signature = append(k.signature, paramType{ad: ad, byteN: byteN})
}

// gosterici çekirdeğin yalnızca okuduğu bir genel bellek göstericisi parametresi
// bildirir ve yükler.
func (k *emitter) pointer(ad string) reg { return k.declarePointer(ad, false) }

// yazilan çekirdeğin yazdığı (ya da okuyup yazdığı) bir gösterici parametresi bildirir.
func (k *emitter) written(ad string) reg { return k.declarePointer(ad, true) }

func (k *emitter) declarePointer(ad string, writable bool) reg {
	k.param = append(k.param, "\t.param .u64 "+ad)
	k.signature = append(k.signature, paramType{ad: ad, byteN: 8, pointer: true, writable: writable})
	r := k.rd()
	k.e("ld.param.u64", r, "["+ad+"]")
	k.e("cvta.to.global.u64", r, r)
	return r
}

// u32p 32 bitlik tam sayı parametresi bildirir ve yükler.
func (k *emitter) u32p(ad string) reg {
	k.addParam(".u32", ad, 4)
	r := k.r()
	k.e("ld.param.u32", r, "["+ad+"]")
	return r
}

// f32p float32 parametresi bildirir ve yükler.
func (k *emitter) f32p(ad string) reg {
	k.addParam(".f32", ad, 4)
	r := k.f()
	k.e("ld.param.f32", r, "["+ad+"]")
	return r
}

// dizi n elemanlı 32 bitlik tam sayı dizisini değerle geçen parametre bildirir.
func (k *emitter) arr(ad string, n int) {
	n = max(n, 1)
	k.param = append(k.param, fmt.Sprintf("\t.param .align 4 .b8 %s[%d]", ad, 4*n))
	k.signature = append(k.signature, paramType{ad: ad, byteN: 4 * n})
}

// diziOku dizinin i. elemanını yükler.
func (k *emitter) readArr(ad string, i int) reg {
	r := k.r()
	k.e("ld.param.u32", r, fmt.Sprintf("[%s+%d]", ad, 4*i))
	return r
}

// ----------------------------------------------------------------------------
// Paylaşımlı bellek ve özel yazmaçlar
// ----------------------------------------------------------------------------

// ortak bayt boyunda paylaşımlı bellek dizisi bildirir ve 32 bitlik adresini verir.
func (k *emitter) shared(ad string, byteN int) reg {
	ofs := (k.sharedMemBytes + 15) &^ 15
	k.sharedMemBytes = ofs + byteN
	r := k.r()
	if k.dynamic {
		k.e("mov.u32", r, "dsm")
		if ofs > 0 {
			k.e("add.s32", r, r, ofs)
		}
		return r
	}
	k.sharedMem = append(k.sharedMem, fmt.Sprintf("\t.shared .align 16 .b8 %s[%d];", ad, byteN))
	k.e("mov.u32", r, ad)
	return r
}

// ozel bir özel yazmacı (%tid.x, %ctaid.y, ...) okur.
func (k *emitter) special(ad string) reg {
	r := k.r()
	k.e("mov.u32", r, ad)
	return r
}

// genelIndeks ctaid.x·ntid.x + tid.x.
func (k *emitter) globalIndex() reg {
	b, n, t := k.special("%ctaid.x"), k.special("%ntid.x"), k.special("%tid.x")
	i := k.r()
	k.e("mad.lo.s32", i, b, n, t)
	return i
}

// ----------------------------------------------------------------------------
// Adres ve sabit yardımcıları
// ----------------------------------------------------------------------------

// adres taban + idx·boy (idx işaretsiz 32 bit).
func (k *emitter) address(base2, idx reg, size int) reg {
	r := k.rd()
	k.e("mul.wide.u32", r, idx, size)
	k.e("add.s64", r, r, base2)
	return r
}

// adresS taban + idx·boy (idx işaretli 32 bit).
func (k *emitter) addressS(base2, idx reg, size int) reg {
	r := k.rd()
	k.e("mul.wide.s32", r, idx, size)
	k.e("add.s64", r, r, base2)
	return r
}

// sabit float32 sabitini yazmaca koyar.
func (k *emitter) constant(v float32) reg {
	r := k.f()
	k.e("mov.f32", r, v)
	return r
}

// tam 32 bitlik tam sayı sabitini yazmaca koyar.
func (k *emitter) whole(v int) reg {
	r := k.r()
	k.e("mov.u32", r, v)
	return r
}

// ----------------------------------------------------------------------------
// Metin
// ----------------------------------------------------------------------------

// metin modülün PTX metnini verir.
func (k *emitter) text() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, ".version 7.0\n.target sm_%d\n.address_size 64\n\n", k.target)
	if k.dynamic {
		// Devingen paylaşımlı dizi modül düzeyinde bildirilir (ptxas gövdede kabul etmez).
		sb.WriteString(".extern .shared .align 16 .b8 dsm[];\n\n")
	}
	fmt.Fprintf(&sb, ".visible .entry %s(\n", k.ad)
	sb.WriteString(strings.Join(k.param, ",\n"))
	sb.WriteString("\n)\n{\n")
	for t := 0; t < regClassCount; t++ {
		if k.counter[t] > 0 {
			fmt.Fprintf(&sb, "\t.reg %s %s<%d>;\n", regInfo[t].dtype, regInfo[t].prefix, k.counter[t]+1)
		}
	}
	for _, s := range k.sharedMem {
		sb.WriteString(s)
		sb.WriteString("\n")
	}
	for _, s := range k.row {
		sb.WriteString(s)
		sb.WriteString("\n")
	}
	sb.WriteString("\tret;\n}\n")
	return sb.String()
}
