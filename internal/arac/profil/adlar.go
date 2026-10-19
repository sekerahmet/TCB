package profil

import (
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/hesap/tensor"
)

// tensorIslemleri tablo başlığıdır ("Tensör işlemleri"; tip adı kod listesinden).
func tensorIslemleri() string { return kod.Ad(kod.A212) + " işlemleri" }

// islemAdi tensör işleminin raporda görünen adıdır. Dildeki bir yerleşiğe ya da metoda
// karşılık gelen işlemler o adla (kod listesinden) yazılır: kullanıcı kendi kodunda gördüğü
// adı görür, ad çevrilince rapor da çevrilir. Karşılığı tek bir ad olmayanlar (işleçler,
// görünümler, kopyalar) Türkçe açıklamasıyla yazılır.
func islemAdi(is tensor.Islem) string {
	if k, ok := islemKodu[is]; ok {
		return kod.Ad(k)
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
		return "." + kod.Ad(kod.Y414)
	case tensor.IslemTurDonusumu:
		return "." + kod.Ad(kod.Y415)
	case tensor.IslemAdamW:
		return kod.Ad(kod.T400) + "." + kod.Ad(kod.Y500)
	case tensor.IslemSGD:
		return kod.Ad(kod.T401) + "." + kod.Ad(kod.Y500)
	}
	return is.String()
}

// islemKodu dildeki bir yerleşiğe birebir karşılık gelen işlemlerdir.
var islemKodu = map[tensor.Islem]kod.Kod{
	tensor.IslemUstel:           kod.T220,
	tensor.IslemLog:             kod.T221,
	tensor.IslemKarekok:         kod.G404,
	tensor.IslemTanh:            kod.T202,
	tensor.IslemRelu:            kod.T200,
	tensor.IslemGelu:            kod.T203,
	tensor.IslemSigmoid:         kod.T201,
	tensor.IslemSinus:           kod.T222,
	tensor.IslemKosinus:         kod.T223,
	tensor.IslemToplam:          kod.G400,
	tensor.IslemOrtalama:        kod.G410,
	tensor.IslemEnBuyuk:         kod.G402,
	tensor.IslemEnBuyukIndeks:   kod.T230,
	tensor.IslemSoftmax:         kod.T210,
	tensor.IslemLogSoftmax:      kod.T211,
	tensor.IslemGomme:           kod.T500,
	tensor.IslemCaprazEntropi:   kod.T510,
	tensor.IslemKatmanNormu:     kod.T501,
	tensor.IslemRMSNormu:        kod.T502,
	tensor.IslemDikkat:          kod.T503,
	tensor.IslemBirlestirme:     kod.T121,
	tensor.IslemGradyanKirpma:   kod.T402,
	tensor.IslemOzel:            kod.T310,
	tensor.IslemSilu:            kod.T204,
	tensor.IslemTanjant:         kod.T224,
	tensor.IslemArksinus:        kod.T225,
	tensor.IslemArkkosinus:      kod.T226,
	tensor.IslemArktanjant:      kod.T227,
	tensor.IslemAtan2:           kod.T228,
	tensor.IslemBirikimliToplam: kod.T231,
	tensor.IslemEnBuyukK:        kod.T232,
	tensor.IslemSiralamaIndeksi: kod.T233,
	tensor.IslemKosulluSecim:    kod.T234,
	tensor.IslemSinh:            kod.T240,
	tensor.IslemCosh:            kod.T241,
	tensor.IslemLog1p:           kod.T242,
	tensor.IslemLog2:            kod.T243,
	tensor.IslemLog10:           kod.T244,
	tensor.IslemUstelEksiBir:    kod.T245,
	tensor.IslemMutlak:          kod.G403,
	tensor.IslemTaban:           kod.G406,
	tensor.IslemTavan:           kod.G407,
	tensor.IslemIsaret:          kod.G408,
	tensor.IslemSinirlama:       kod.G409,
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
