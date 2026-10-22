package profiler

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// tensorOps tablo başlığıdır ("Tensör işlemleri"; tip adı kod listesinden).
func tensorOps() string { return messages.Text(messages.H8206, codes.DisplayName(codes.A212)) }

// opName tensör işleminin raporda görünen adıdır. Dildeki bir yerleşiğe ya da metoda
// karşılık gelen işlemler o adla (kod listesinden) yazılır: kullanıcı kendi kodunda gördüğü
// adı görür, ad çevrilince rapor da çevrilir. Karşılığı tek bir ad olmayanlar (işleçler,
// görünümler, kopyalar) açıklama iletisiyle yazılır.
func opName(is tensor.OpKind) string {
	if k, ok := opCode[is]; ok {
		return codes.Name(k)
	}
	switch is {
	case tensor.OpKindAdd:
		return messages.Text(messages.H8207)
	case tensor.OpKindSub:
		return messages.Text(messages.H8208)
	case tensor.OpKindMul:
		return messages.Text(messages.H8209)
	case tensor.OpKindDiv:
		return messages.Text(messages.H8210)
	case tensor.OpKindLess:
		return messages.Text(messages.H8211)
	case tensor.OpKindGreater:
		return messages.Text(messages.H8212)
	case tensor.OpKindLessEq:
		return messages.Text(messages.H8213)
	case tensor.OpKindGreaterEq:
		return messages.Text(messages.H8214)
	case tensor.OpKindMatMul:
		return messages.Text(messages.H8215)
	case tensor.OpKindToDevice:
		return "." + codes.Name(codes.Y414)
	case tensor.OpKindCast:
		return "." + codes.Name(codes.Y415)
	case tensor.OpKindAdamW:
		return codes.Name(codes.T400) + "." + codes.Name(codes.Y500)
	case tensor.OpKindSGD:
		return codes.Name(codes.T401) + "." + codes.Name(codes.Y500)
	}
	if c, ok := opLabel[is]; ok {
		return messages.Text(c)
	}
	return is.Code()
}

// opLabel, dilde tek bir ad olmayan ve işleç olmayan işlemlerin açıklama iletileridir.
// Ölçüt: dilin bir yerleşiğinin/metodunun adı olan işlem ad koduyla (opCode), yalnızca
// açıklaması olan ileti koduyla yazılır; böylece her etiket dile göre çevrilir.
var opLabel = map[tensor.OpKind]messages.Code{
	tensor.OpKindOther:      messages.H8223,
	tensor.OpKindPow:        messages.H8224,
	tensor.OpKindAddScalar:  messages.H8225,
	tensor.OpKindMulScalar:  messages.H8226,
	tensor.OpKindRSubScalar: messages.H8227,
	tensor.OpKindDivScalar:  messages.H8228,
	tensor.OpKindView:       messages.H8229,
	tensor.OpKindToHost:     messages.H8252,
	tensor.OpKindZeroGrad:   messages.H8253,
	tensor.OpKindCopy:       messages.H3096,
	tensor.OpKindCreate:     messages.H3089,
}

// opCode dildeki bir yerleşiğe birebir karşılık gelen işlemlerdir.
var opCode = map[tensor.OpKind]codes.Code{
	tensor.OpKindExp:          codes.T220,
	tensor.OpKindLog:          codes.T221,
	tensor.OpKindSqrt:         codes.G404,
	tensor.OpKindTanh:         codes.T202,
	tensor.OpKindRelu:         codes.T200,
	tensor.OpKindGelu:         codes.T203,
	tensor.OpKindGeluExact:    codes.T203,
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
	tensor.OpKindFused:        codes.T312,
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

// fullName yönüyle birlikte görünen addır: geri yayılım "geri · " önekiyle. Çağrısız "diğer"
// kaydı yalnızca işlem dışında yapılan ayırmalardır (oluşturucular).
func fullName(is tensor.OpKind, restore bool, calls int64) string {
	if is == tensor.OpKindOther && calls == 0 {
		return messages.Text(messages.H8216)
	}
	if restore {
		return messages.Text(messages.H8217, opName(is))
	}
	return opName(is)
}
