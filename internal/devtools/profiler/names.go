package profiler

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// tensorOps tablo başlığıdır ("Tensör işlemleri"; tip adı kod listesinden).
func tensorOps() string { return messages.Text(messages.H8206, codes.NameRef(codes.TypeTensor)) }

// opName tensör işleminin raporda görünen adıdır. Dildeki bir yerleşiğe ya da metoda
// karşılık gelen işlemler o adla (kod listesinden) yazılır: kullanıcı kendi kodunda gördüğü
// adı görür, ad çevrilince rapor da çevrilir. Karşılığı tek bir ad olmayanlar (işleçler,
// görünümler, kopyalar) açıklama iletisiyle yazılır.
func opName(is tensor.OpKind) string {
	if k, ok := opCode[is]; ok {
		return codes.Source().Name(k)
	}
	switch is {
	case tensor.OpKindToDevice:
		return "." + codes.Source().Name(codes.MemToDevice)
	case tensor.OpKindCast:
		return "." + codes.Source().Name(codes.MemToDtype)
	case tensor.OpKindAdamW:
		return codes.Source().Name(codes.FnAdamW) + "." + codes.Source().Name(codes.MemStep)
	case tensor.OpKindSGD:
		return codes.Source().Name(codes.FnSGD) + "." + codes.Source().Name(codes.MemStep)
	}
	if c, ok := opLabel[is]; ok {
		return messages.Text(c)
	}
	return is.Code()
}

// opLabel, dilde tek bir ad olmayan işlemlerin (işleçler dahil) açıklama iletileridir.
// Ölçüt: dilin bir yerleşiğinin/metodunun adı olan işlem ad koduyla (opCode), yalnızca
// açıklaması olan ileti koduyla yazılır; böylece her etiket dile göre çevrilir.
var opLabel = map[tensor.OpKind]messages.Code{
	tensor.OpKindAdd:        messages.H8207,
	tensor.OpKindSub:        messages.H8208,
	tensor.OpKindMul:        messages.H8209,
	tensor.OpKindDiv:        messages.H8210,
	tensor.OpKindLess:       messages.H8211,
	tensor.OpKindGreater:    messages.H8212,
	tensor.OpKindLessEq:     messages.H8213,
	tensor.OpKindGreaterEq:  messages.H8214,
	tensor.OpKindMatMul:     messages.H8215,
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
	tensor.OpKindAssign:     messages.H8284,
}

// implicitLabels örtük iş türlerinin adlarıdır.
var implicitLabels = [tensor.ImplicitKindCount]messages.Code{messages.H3096, messages.H8285, messages.H3097, messages.H8286}

// implicitName örtük iş türünün insana gösterilen adıdır (üst işlemin adı ayrı sütundadır).
func implicitName(k tensor.ImplicitKind) string {
	if k < tensor.ImplicitKindCount {
		return messages.Text(implicitLabels[k])
	}
	return k.Code()
}

// opCode dildeki bir yerleşiğe birebir karşılık gelen işlemlerdir.
var opCode = map[tensor.OpKind]codes.Code{
	tensor.OpKindExp:          codes.FnExp,
	tensor.OpKindLog:          codes.FnLog,
	tensor.OpKindSqrt:         codes.FnSqrt,
	tensor.OpKindTanh:         codes.FnTanh,
	tensor.OpKindRelu:         codes.FnRelu,
	tensor.OpKindGelu:         codes.FnGelu,
	tensor.OpKindGeluExact:    codes.FnGelu,
	tensor.OpKindSigmoid:      codes.FnSigmoid,
	tensor.OpKindSin:          codes.FnSin,
	tensor.OpKindCos:          codes.FnCos,
	tensor.OpKindSum:          codes.FnSum,
	tensor.OpKindMean:         codes.FnMean,
	tensor.OpKindMax:          codes.FnMax,
	tensor.OpKindArgMax:       codes.FnArgmax,
	tensor.OpKindSoftmax:      codes.FnSoftmax,
	tensor.OpKindLogSoftmax:   codes.FnLogSoftmax,
	tensor.OpKindEmbedding:    codes.FnEmbedding,
	tensor.OpKindCrossEntropy: codes.FnCrossEntropy,
	tensor.OpKindLayerNorm:    codes.FnLayerNorm,
	tensor.OpKindRMSNorm:      codes.FnRmsNorm,
	tensor.OpKindAttention:    codes.FnAttention,
	tensor.OpKindConcat:       codes.FnConcat,
	tensor.OpKindClipGrad:     codes.FnClipGrad,
	tensor.OpKindCustom:       codes.FnCustomGrad,
	tensor.OpKindFused:        codes.FnFuse,
	tensor.OpKindSilu:         codes.FnSilu,
	tensor.OpKindTan:          codes.FnTan,
	tensor.OpKindAsin:         codes.FnAsin,
	tensor.OpKindAcos:         codes.FnAcos,
	tensor.OpKindAtan:         codes.FnAtan,
	tensor.OpKindAtan2:        codes.FnAtan2,
	tensor.OpKindCumSum:       codes.FnCumsum,
	tensor.OpKindTopK:         codes.FnTopk,
	tensor.OpKindArgSort:      codes.FnArgsort,
	tensor.OpKindWhere:        codes.FnWhere,
	tensor.OpKindSinh:         codes.FnSinh,
	tensor.OpKindCosh:         codes.FnCosh,
	tensor.OpKindLog1p:        codes.FnLog1p,
	tensor.OpKindLog2:         codes.FnLog2,
	tensor.OpKindLog10:        codes.FnLog10,
	tensor.OpKindExpm1:        codes.FnExpm1,
	tensor.OpKindAbs:          codes.FnAbs,
	tensor.OpKindFloor:        codes.FnFloor,
	tensor.OpKindCeil:         codes.FnCeil,
	tensor.OpKindSign:         codes.FnSign,
	tensor.OpKindClamp:        codes.FnClamp,
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
