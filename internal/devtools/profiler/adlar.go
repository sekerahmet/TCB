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
func islemAdi(is tensor.Islem) string {
	if k, ok := islemKodu[is]; ok {
		return codes.Name(k)
	}
	switch is {
	case tensor.IslemToplama:
		return "toplama (+)"
	case tensor.IslemCikarma:
		return "çıkarma (-)"
	case tensor.IslemCarpma:
		return "çarpma (*)"
	case tensor.IslemBolme:
		return "bölme (/)"
	case tensor.IslemKucuk:
		return "karşılaştırma (<)"
	case tensor.IslemBuyuk:
		return "karşılaştırma (>)"
	case tensor.IslemKucukEsit:
		return "karşılaştırma (<=)"
	case tensor.IslemBuyukEsit:
		return "karşılaştırma (>=)"
	case tensor.IslemMatrisCarpimi:
		return "matris çarpımı (@)"
	case tensor.IslemCihazaTasima:
		return "." + codes.Name(codes.Y414)
	case tensor.IslemTurDonusumu:
		return "." + codes.Name(codes.Y415)
	case tensor.IslemAdamW:
		return codes.Name(codes.T400) + "." + codes.Name(codes.Y500)
	case tensor.IslemSGD:
		return codes.Name(codes.T401) + "." + codes.Name(codes.Y500)
	}
	return is.String()
}

// islemKodu dildeki bir yerleşiğe birebir karşılık gelen işlemlerdir.
var islemKodu = map[tensor.Islem]codes.Code{
	tensor.IslemUstel:           codes.T220,
	tensor.IslemLog:             codes.T221,
	tensor.IslemKarekok:         codes.G404,
	tensor.IslemTanh:            codes.T202,
	tensor.IslemRelu:            codes.T200,
	tensor.IslemGelu:            codes.T203,
	tensor.IslemSigmoid:         codes.T201,
	tensor.IslemSinus:           codes.T222,
	tensor.IslemKosinus:         codes.T223,
	tensor.IslemToplam:          codes.G400,
	tensor.IslemOrtalama:        codes.G410,
	tensor.IslemEnBuyuk:         codes.G402,
	tensor.IslemEnBuyukIndeks:   codes.T230,
	tensor.IslemSoftmax:         codes.T210,
	tensor.IslemLogSoftmax:      codes.T211,
	tensor.IslemGomme:           codes.T500,
	tensor.IslemCaprazEntropi:   codes.T510,
	tensor.IslemKatmanNormu:     codes.T501,
	tensor.IslemRMSNormu:        codes.T502,
	tensor.IslemDikkat:          codes.T503,
	tensor.IslemBirlestirme:     codes.T121,
	tensor.IslemGradyanKirpma:   codes.T402,
	tensor.IslemOzel:            codes.T310,
	tensor.IslemSilu:            codes.T204,
	tensor.IslemTanjant:         codes.T224,
	tensor.IslemArksinus:        codes.T225,
	tensor.IslemArkkosinus:      codes.T226,
	tensor.IslemArktanjant:      codes.T227,
	tensor.IslemAtan2:           codes.T228,
	tensor.IslemBirikimliToplam: codes.T231,
	tensor.IslemEnBuyukK:        codes.T232,
	tensor.IslemSiralamaIndeksi: codes.T233,
	tensor.IslemKosulluSecim:    codes.T234,
	tensor.IslemSinh:            codes.T240,
	tensor.IslemCosh:            codes.T241,
	tensor.IslemLog1p:           codes.T242,
	tensor.IslemLog2:            codes.T243,
	tensor.IslemLog10:           codes.T244,
	tensor.IslemUstelEksiBir:    codes.T245,
	tensor.IslemMutlak:          codes.G403,
	tensor.IslemTaban:           codes.G406,
	tensor.IslemTavan:           codes.G407,
	tensor.IslemIsaret:          codes.G408,
	tensor.IslemSinirlama:       codes.G409,
}

// tamAd yönüyle birlikte görünen addır: geri yayılım "geri · " önekiyle. Çağrısız "diğer"
// kaydı yalnızca işlem dışında yapılan ayırmalardır (oluşturucular).
func tamAd(is tensor.Islem, geri bool, cagri int64) string {
	if is == tensor.IslemDiger && cagri == 0 {
		return "işlem dışı ayırmalar"
	}
	if geri {
		return "geri · " + islemAdi(is)
	}
	return islemAdi(is)
}
