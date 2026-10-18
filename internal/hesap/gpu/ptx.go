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
type yazmac string

// Yazmaç türleri.
const (
	yzF  = iota // .f32
	yzR         // .b32 (u32/s32)
	yzRD        // .b64 (u64/s64, adres)
	yzP         // .pred
	yzH         // .b16
	yzFD        // .f64
	yzSayisi
)

var yazmacBilgi = [yzSayisi]struct{ tur, onek string }{
	yzF: {".f32", "%f"}, yzR: {".b32", "%r"}, yzRD: {".b64", "%rd"},
	yzP: {".pred", "%p"}, yzH: {".b16", "%h"}, yzFD: {".f64", "%fd"},
}

// paramTuru çekirdek parametresinin imzası: adı, bayt boyu, gösterici mi ve çekirdeğin
// gösterdiği belleğe yazıp yazmadığı. Başlatıcı, başlatma yerinin bildirdiği okuma/yazma
// ile imzayı karşılaştırır ve yazılan depolamaların konak gölgesini kendisi siler.
type paramTuru struct {
	ad        string
	bayt      int
	gosterici bool
	yazilir   bool
}

// yazici bir PTX çekirdeğini kurar.
type yazici struct {
	ad       string
	imza     []paramTuru
	param    []string
	paylasim []string
	satir    []string
	sayac    [yzSayisi]int
	etiketNo int
}

func yeniYazici(ad string) *yazici { return &yazici{ad: ad} }

func (k *yazici) yeni(t int) yazmac {
	k.sayac[t]++
	return yazmac(yazmacBilgi[t].onek + strconv.Itoa(k.sayac[t]))
}

func (k *yazici) f() yazmac  { return k.yeni(yzF) }
func (k *yazici) r() yazmac  { return k.yeni(yzR) }
func (k *yazici) rd() yazmac { return k.yeni(yzRD) }
func (k *yazici) p() yazmac  { return k.yeni(yzP) }
func (k *yazici) h() yazmac  { return k.yeni(yzH) }
func (k *yazici) fd() yazmac { return k.yeni(yzFD) }

// terim bir işleneni PTX yazımına çevirir.
func terim(a any) string {
	switch v := a.(type) {
	case yazmac:
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

func (k *yazici) komut(on, op string, args []any) {
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
		sb.WriteString(terim(a))
	}
	sb.WriteString(";")
	k.satir = append(k.satir, sb.String())
}

// e bir komut yazar: k.e("add.f32", d, a, b).
func (k *yazici) e(op string, args ...any) { k.komut("", op, args) }

// eger koşullu komut yazar (@p op ...).
func (k *yazici) eger(p yazmac, op string, args ...any) { k.komut("@"+string(p)+" ", op, args) }

// degilse koşulun yanlışlığına bağlı komut yazar (@!p op ...).
func (k *yazici) degilse(p yazmac, op string, args ...any) { k.komut("@!"+string(p)+" ", op, args) }

// etiket tekil bir etiket adı üretir; koy ile yerleştirilir.
func (k *yazici) etiket() string {
	k.etiketNo++
	return "L" + strconv.Itoa(k.etiketNo)
}

func (k *yazici) koy(l string) { k.satir = append(k.satir, l+":") }

// git koşulsuz dallanma.
func (k *yazici) git(l string) { k.e("bra", l) }

// gitEger p doğruysa dallanır; gitDegilse p yanlışsa.
func (k *yazici) gitEger(p yazmac, l string)    { k.eger(p, "bra", l) }
func (k *yazici) gitDegilse(p yazmac, l string) { k.degilse(p, "bra", l) }

// engel bloktaki bütün iş parçacıklarını bekler.
func (k *yazici) engel() { k.e("bar.sync", 0) }

// bel bir bellek işleneni yazar: [taban+ofset].
func bel(taban yazmac, ofset int) string {
	if ofset == 0 {
		return "[" + string(taban) + "]"
	}
	return "[" + string(taban) + "+" + strconv.Itoa(ofset) + "]"
}

// ----------------------------------------------------------------------------
// Parametreler
// ----------------------------------------------------------------------------

func (k *yazici) paramEkle(tur, ad string, bayt int) {
	k.param = append(k.param, "\t.param "+tur+" "+ad)
	k.imza = append(k.imza, paramTuru{ad: ad, bayt: bayt})
}

// gosterici çekirdeğin yalnızca okuduğu bir genel bellek göstericisi parametresi
// bildirir ve yükler.
func (k *yazici) gosterici(ad string) yazmac { return k.gostericiBildir(ad, false) }

// yazilan çekirdeğin yazdığı (ya da okuyup yazdığı) bir gösterici parametresi bildirir.
func (k *yazici) yazilan(ad string) yazmac { return k.gostericiBildir(ad, true) }

func (k *yazici) gostericiBildir(ad string, yazilir bool) yazmac {
	k.param = append(k.param, "\t.param .u64 "+ad)
	k.imza = append(k.imza, paramTuru{ad: ad, bayt: 8, gosterici: true, yazilir: yazilir})
	r := k.rd()
	k.e("ld.param.u64", r, "["+ad+"]")
	k.e("cvta.to.global.u64", r, r)
	return r
}

// u32p 32 bitlik tam sayı parametresi bildirir ve yükler.
func (k *yazici) u32p(ad string) yazmac {
	k.paramEkle(".u32", ad, 4)
	r := k.r()
	k.e("ld.param.u32", r, "["+ad+"]")
	return r
}

// f32p float32 parametresi bildirir ve yükler.
func (k *yazici) f32p(ad string) yazmac {
	k.paramEkle(".f32", ad, 4)
	r := k.f()
	k.e("ld.param.f32", r, "["+ad+"]")
	return r
}

// dizi n elemanlı 32 bitlik tam sayı dizisini değerle geçen parametre bildirir.
func (k *yazici) dizi(ad string, n int) {
	n = max(n, 1)
	k.param = append(k.param, fmt.Sprintf("\t.param .align 4 .b8 %s[%d]", ad, 4*n))
	k.imza = append(k.imza, paramTuru{ad: ad, bayt: 4 * n})
}

// diziOku dizinin i. elemanını yükler.
func (k *yazici) diziOku(ad string, i int) yazmac {
	r := k.r()
	k.e("ld.param.u32", r, fmt.Sprintf("[%s+%d]", ad, 4*i))
	return r
}

// ----------------------------------------------------------------------------
// Paylaşımlı bellek ve özel yazmaçlar
// ----------------------------------------------------------------------------

// ortak bayt boyunda paylaşımlı bellek dizisi bildirir ve 32 bitlik adresini verir.
func (k *yazici) ortak(ad string, bayt int) yazmac {
	k.paylasim = append(k.paylasim, fmt.Sprintf("\t.shared .align 16 .b8 %s[%d];", ad, bayt))
	r := k.r()
	k.e("mov.u32", r, ad)
	return r
}

// ozel bir özel yazmacı (%tid.x, %ctaid.y, ...) okur.
func (k *yazici) ozel(ad string) yazmac {
	r := k.r()
	k.e("mov.u32", r, ad)
	return r
}

// genelIndeks ctaid.x·ntid.x + tid.x.
func (k *yazici) genelIndeks() yazmac {
	b, n, t := k.ozel("%ctaid.x"), k.ozel("%ntid.x"), k.ozel("%tid.x")
	i := k.r()
	k.e("mad.lo.s32", i, b, n, t)
	return i
}

// ----------------------------------------------------------------------------
// Adres ve sabit yardımcıları
// ----------------------------------------------------------------------------

// adres taban + idx·boy (idx işaretsiz 32 bit).
func (k *yazici) adres(taban, idx yazmac, boy int) yazmac {
	r := k.rd()
	k.e("mul.wide.u32", r, idx, boy)
	k.e("add.s64", r, r, taban)
	return r
}

// adresS taban + idx·boy (idx işaretli 32 bit).
func (k *yazici) adresS(taban, idx yazmac, boy int) yazmac {
	r := k.rd()
	k.e("mul.wide.s32", r, idx, boy)
	k.e("add.s64", r, r, taban)
	return r
}

// sabit float32 sabitini yazmaca koyar.
func (k *yazici) sabit(v float32) yazmac {
	r := k.f()
	k.e("mov.f32", r, v)
	return r
}

// tam 32 bitlik tam sayı sabitini yazmaca koyar.
func (k *yazici) tam(v int) yazmac {
	r := k.r()
	k.e("mov.u32", r, v)
	return r
}

// ----------------------------------------------------------------------------
// Metin
// ----------------------------------------------------------------------------

// metin modülün PTX metnini verir.
func (k *yazici) metin() string {
	var sb strings.Builder
	sb.WriteString(".version 7.0\n.target sm_52\n.address_size 64\n\n")
	fmt.Fprintf(&sb, ".visible .entry %s(\n", k.ad)
	sb.WriteString(strings.Join(k.param, ",\n"))
	sb.WriteString("\n)\n{\n")
	for t := 0; t < yzSayisi; t++ {
		if k.sayac[t] > 0 {
			fmt.Fprintf(&sb, "\t.reg %s %s<%d>;\n", yazmacBilgi[t].tur, yazmacBilgi[t].onek, k.sayac[t]+1)
		}
	}
	for _, s := range k.paylasim {
		sb.WriteString(s)
		sb.WriteString("\n")
	}
	for _, s := range k.satir {
		sb.WriteString(s)
		sb.WriteString("\n")
	}
	sb.WriteString("\tret;\n}\n")
	return sb.String()
}
