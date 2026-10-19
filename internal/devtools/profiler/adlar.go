package profiler

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// tensorIslemleri tablo başlığıdır ("Tensör işlemleri"; tip adı kod listesinden).
func tensorIslemleri() string { return codes.Name(codes.A212) + " işlemleri" }

// islemAdi tensör işleminin raporda görünen adıdır. Dildeki bir yerleşiğe ya da metoda
// karşılık gelen işlemler o adla (kod listesinden) yazılır: kullanıcı kendi kodunda gördüğü
// adı görür, ad çevrilince rapor da çevrilir. Karşılığı tek bir ad olmayanlar (işleçler,
// görünümler, kopyalar) Türkçe açıklamasıyla yazılır.
func islemAdi(is tensor.OpKind) string {
	if k, ok := islemKodu[is]; ok {
		return codes.Name(k)
	}
	switch is {
	case tensor.OpKindAdd:
		return "toplama (+)"
	case tensor.OpKindSub:
		return "çıkarma (-)"
	case tensor.OpKindMul:
		return "çarpma (*)"
	case tensor.OpKindDiv:
		return "bölme (/)"
	case tensor.OpKindLess:
		return "karşılaştırma (<)"
	case tensor.OpKindGreater:
		return "karşılaştırma (>)"
	case tensor.OpKindLessEq:
		return "karşılaştırma (<=)"
	case tensor.OpKindGreaterEq:
		return "karşılaştırma (>=)"
	case tensor.OpKindMatMul:
		return "matris çarpımı (@)"
	case tensor.OpKindToDevice:
		return "." + codes.Name(codes.Y414)
	case tensor.OpKindCast:
		return "." + codes.Name(codes.Y415)
	case tensor.OpKindAdamW:
		return codes.Name(codes.T400) + "." + codes.Name(codes.Y500)
	case tensor.OpKindSGD:
		return codes.Name(codes.T401) + "." + codes.Name(codes.Y500)
	}
	return is.String()
}

// islemKodu dildeki bir yerleşiğe birebir karşılık gelen işlemlerdir.
var islemKodu = map[tensor.OpKind]codes.Code{
	tensor.OpKindExp:          codes.T220,
	tensor.OpKindLog:          codes.T221,
	tensor.OpKindSqrt:         codes.G404,
	tensor.OpKindTanh:         codes.T202,
	tensor.OpKindRelu:         codes.T200,
	tensor.OpKindGelu:         codes.T203,
	tensor.OpKindSigmoid:      codes.T201,
	tensor.OpKindSin:          codes.T222,
	tensor.OpKindCos:          codes.T223,
	tensor.OpKindSum:          codes.G400,
	tensor.OpKindMean:         codes.G410,
	tensor.OpKindMax:          codes.G402,
	tensor.OpKindArgMax:       codes.T230,
	tensor.OpKindSoftmax:      codes.T210,
	tensor.OpKindLogSoftmax:   codes.T211,
	tensor.OpKindEmbedding:    codes.T500,
	tensor.OpKindCrossEntropy: codes.T510,
	tensor.OpKindLayerNorm:    codes.T501,
	tensor.OpKindRMSNorm:      codes.T502,
	tensor.OpKindAttention:    codes.T503,
	tensor.OpKindConcat:       codes.T121,
	tensor.OpKindClipGrad:     codes.T402,
	tensor.OpKindCustom:       codes.T310,
	tensor.OpKindSilu:         codes.T204,
	tensor.OpKindTan:          codes.T224,
	tensor.OpKindAsin:         codes.T225,
	tensor.OpKindAcos:         codes.T226,
	tensor.OpKindAtan:         codes.T227,
	tensor.OpKindAtan2:        codes.T228,
	tensor.OpKindCumSum:       codes.T231,
	tensor.OpKindTopK:         codes.T232,
	tensor.OpKindArgSort:      codes.T233,
	tensor.OpKindWhere:        codes.T234,
	tensor.OpKindSinh:         codes.T240,
	tensor.OpKindCosh:         codes.T241,
	tensor.OpKindLog1p:        codes.T242,
	tensor.OpKindLog2:         codes.T243,
	tensor.OpKindLog10:        codes.T244,
	tensor.OpKindExpm1:        codes.T245,
	tensor.OpKindAbs:          codes.G403,
	tensor.OpKindFloor:        codes.G406,
	tensor.OpKindCeil:         codes.G407,
	tensor.OpKindSign:         codes.G408,
	tensor.OpKindClamp:        codes.G409,
}

// tamAd yönüyle birlikte görünen addır: geri yayılım "geri · " önekiyle. Çağrısız "diğer"
// kaydı yalnızca işlem dışında yapılan ayırmalardır (oluşturucular).
func tamAd(is tensor.OpKind, geri bool, cagri int64) string {
	if is == tensor.OpKindOther && cagri == 0 {
		return "işlem dışı ayırmalar"
	}
	if geri {
		return "geri · " + islemAdi(is)
	}
	return islemAdi(is)
}
