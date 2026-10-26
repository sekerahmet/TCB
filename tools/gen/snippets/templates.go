package main

import "github.com/sekerahmet/TCB/internal/lang/codes"

// Şablonların verisi. Gövde (Source) Türkçe, geçerli TR kodudur ve source.Show ile her dile çevrilir;
// kullanıcının dolduracağı yerler "ph_" adlarıdır ve Holes'ta VS Code şablon durağına (${1:varsayılan})
// çevrilir. Bir deliğin metnindeki %sözcük% wordTable'dan dilin sözcüğüyle gelir; durak varsayılanında
// dil öğesi (anahtar kelime, yerleşik adı) bulunmaz, o kodun kendisine yazılır.

// prefix kısayolun kaynağıdır: bir kodun dildeki adı (Code) ya da wordTable'daki bir sözcük (Word).
type prefix struct {
	Code codes.Code
	Word string
}

// template bir kod şablonudur.
type template struct {
	ID          string
	Label       map[string]string
	Description map[string]string
	Prefix      []prefix
	Member      bool // kısayol ayrıca ".ad" olarak da sunulur (metot şablonları)
	End         bool // son satırın sonuna $0 (imleç son durağı) eklenir
	Source      string
	Holes       map[string]string
}

// wordTable dile bağlı sözcüklerdir (kullanıcının seçeceği ad ve metin önerileri); satırı olmayan dil
// İngilizceyi kullanır.
var wordTable = map[string]map[string]string{
	"tr": {
		"name": "isim", "param": "parametre", "condition": "koşul", "value": "değer",
		"one": "bir", "few": "birkaç", "other": "diğer",
		"acc": "birikim", "x": "x", "i": "i", "Name": "İsim", "field": "alan", "method": "metod",
		"err": "hata", "caught": "Hata yakalandı:", "list_comp": "liste_üret", "grad": "türev",
		"optimizer": "iyileştirici", "loss": "kayıp", "step": "adım", "step_label": "adım", "loss_label": "kayıp",
	},
	"en": {
		"name": "name", "param": "parameter", "condition": "condition", "value": "value",
		"one": "one", "few": "a few", "other": "other",
		"acc": "acc", "x": "x", "i": "i", "Name": "Name", "field": "field", "method": "method",
		"err": "err", "caught": "Caught error:", "list_comp": "list_comp", "grad": "grad",
		"optimizer": "optimizer", "loss": "loss", "step": "step", "step_label": "step", "loss_label": "loss",
	},
}

var templates = []template{
	{
		ID:          "function",
		Label:       map[string]string{"tr": "Fonksiyon Tanımla", "en": "Define Function"},
		Description: map[string]string{"tr": "Yeni bir TR fonksiyonu tanımlar", "en": "Defines a new TR function"},
		Prefix:      []prefix{{Code: codes.KwFunc}},
		Source:      "fonksiyon ph_name(ph_param) {\n\tsonuç ph_return\n}\n",
		Holes:       map[string]string{"ph_name": "${1:%name%}", "ph_param": "${2:%param%}", "ph_return": "${3:${2:%param%}}"},
	},
	{
		ID:          "if",
		Label:       map[string]string{"tr": "Eğer - Değilse Koşulu", "en": "If - Else"},
		Description: map[string]string{"tr": "Eğer / değilse koşul bloğu", "en": "If / else conditional block"},
		Prefix:      []prefix{{Code: codes.KwIf}},
		Source:      "eğer ph_condition {\n\tph_then\n} değilse {\n\tph_else\n}\n",
		Holes:       map[string]string{"ph_condition": "${1:%condition%}", "ph_then": "${2}", "ph_else": "${3}"},
	},
	{
		ID:          "match",
		Label:       map[string]string{"tr": "Eşle (Pattern Matching)", "en": "Match (Pattern Matching)"},
		Description: map[string]string{"tr": "Durum ve aralık eşleştirme bloğu", "en": "Case and range matching block"},
		Prefix:      []prefix{{Code: codes.KwMatch}},
		Source:      "eşle ph_value {\n\tph_first => ph_first_result\n\tph_second => ph_second_result\n\tvarsayılan => ph_default_result\n}\n",
		Holes: map[string]string{
			"ph_value": "${1:%value%}", "ph_first": "${2:1}", "ph_first_result": "${3:\"%one%\"}",
			"ph_second": "${4:2..10}", "ph_second_result": "${5:\"%few%\"}", "ph_default_result": "${6:\"%other%\"}",
		},
	},
	{
		ID:          "filter",
		Label:       map[string]string{"tr": "Filtrele (o-lambda)", "en": "Filter (it-lambda)"},
		Description: map[string]string{"tr": "Sondaki blokla filtreleme; o, o anki elemandır: liste.filtrele { o > 2 }", "en": "Filtering with a trailing block; it is the current element"},
		Prefix:      []prefix{{Code: codes.MemFilter}},
		Member:      true,
		Source:      "ph_list.filtrele { o > ph_limit }\n",
		Holes:       map[string]string{"ph_list": "", "ph_limit": "${1:2}"},
	},
	{
		ID:          "map",
		Label:       map[string]string{"tr": "Haritala (o-lambda)", "en": "Map (it-lambda)"},
		Description: map[string]string{"tr": "Sondaki blokla dönüştürme; o, o anki elemandır: liste.haritala { o * o }", "en": "Transforming with a trailing block; it is the current element"},
		Prefix:      []prefix{{Code: codes.MemMap}},
		Member:      true,
		Source:      "ph_list.haritala { o * ph_factor }\n",
		Holes:       map[string]string{"ph_list": "", "ph_factor": "${1:2}"},
	},
	{
		ID:          "reduce",
		Label:       map[string]string{"tr": "İndirge", "en": "Reduce"},
		Description: map[string]string{"tr": "Listeyi soldan sağa tek değere indirger: liste.indirge(0, (birikim, x) => birikim + x)", "en": "Reduces a list from left to right to a single value"},
		Prefix:      []prefix{{Code: codes.MemReduce}},
		Member:      true,
		Source:      "ph_list.indirge(ph_start, (ph_acc, ph_x) => ph_body)\n",
		Holes:       map[string]string{"ph_list": "", "ph_start": "${1:0}", "ph_acc": "%acc%", "ph_x": "%x%", "ph_body": "${2:%acc% + %x%}"},
	},
	{
		ID:          "for",
		Label:       map[string]string{"tr": "Döngü (İçinde)", "en": "Loop (In)"},
		Description: map[string]string{"tr": "Liste veya aralık üzerinde döngü", "en": "Loop over a list or a range"},
		Prefix:      []prefix{{Code: codes.KwFor}},
		Source:      "döngü ph_item içinde ph_range {\n\tyazdır(ph_item)\n}\n",
		Holes:       map[string]string{"ph_item": "${1:%i%}", "ph_range": "${2:1..=10}"},
	},
	{
		ID:          "list_comprehension",
		Label:       map[string]string{"tr": "Liste Üreteci (List Comprehension)", "en": "List Comprehension"},
		Description: map[string]string{"tr": "Tek satırda koşullu liste üreteci", "en": "Conditional list comprehension on one line"},
		Prefix:      []prefix{{Word: "list_comp"}},
		Source:      "[ph_result döngü ph_item içinde ph_range eğer ph_filter]\n",
		Holes: map[string]string{
			"ph_result": "${1:%x% * %x%}", "ph_item": "${2:%x%}", "ph_range": "${3:1..=10}", "ph_filter": "${4:%x% % 2 == 0}",
		},
	},
	{
		ID:          "struct",
		Label:       map[string]string{"tr": "Yapı Tanımla", "en": "Define Struct"},
		Description: map[string]string{"tr": "Yeni bir yapı ve metot tanımlar", "en": "Defines a new struct and a method"},
		Prefix:      []prefix{{Code: codes.KwStruct}},
		Source:      "yapı ph_Name {\n\tph_field: Metin\n\n\tfonksiyon ph_method() {\n\t\tsonuç kendi.ph_field\n\t}\n}\n",
		Holes:       map[string]string{"ph_Name": "${1:%Name%}", "ph_field": "${2:%field%}", "ph_method": "${3:%method%}"},
	},
	{
		ID:          "try",
		Label:       map[string]string{"tr": "Dene - Yakala (Hata Yönetimi)", "en": "Try - Catch (Error Handling)"},
		Description: map[string]string{"tr": "Hata yakalama bloğu", "en": "Error catching block"},
		Prefix:      []prefix{{Code: codes.KwTry}},
		Source:      "dene {\n\tph_body\n} yakala ph_error {\n\tyazdır(ph_message)\n}\n",
		Holes:       map[string]string{"ph_body": "${1}", "ph_error": "%err%", "ph_message": "\"%caught% {%err%}\""},
	},
	{
		ID:          "tensor",
		Label:       map[string]string{"tr": "Tensör ve Eğitim Döngüsü (Otomatik Türev)", "en": "Tensor and Training Loop (Automatic Differentiation)"},
		Description: map[string]string{"tr": "Tensörler, AdamW iyileştiricisi ve geri_yayılım ile küçük bir eğitim döngüsü (doğrusal model)", "en": "A small training loop (linear model) with tensors, the AdamW optimizer and backward"},
		Prefix:      []prefix{{Code: codes.FnTensor}, {Word: "grad"}},
		End:         true,
		Source: "değişken ph_X = tensör(ph_xs)\n" +
			"değişken ph_Y = tensör(ph_ys)\n" +
			"değişken ph_w = tensör(türevsiz(rastgele_normal([ph_shape], tohum: 1) * 0.1), türev_izle: doğru)\n" +
			"değişken ph_b = tensör([0.0], türev_izle: doğru)\n" +
			"değişken ph_optimizer = AdamW([ph_w, ph_b], öğrenme_oranı: ph_rate, ağırlık_çürümesi: 0)\n" +
			"döngü ph_step içinde 0..ph_steps {\n" +
			"\tph_optimizer.sıfırla()\n" +
			"\tdeğişken ph_loss = ortalama((ph_X @ ph_w + ph_b - ph_Y) ** 2)\n" +
			"\tgeri_yayılım(ph_loss)\n" +
			"\tph_optimizer.adım()\n" +
			"\teğer ph_step % 100 == 0 { yazdır(\"ph_step_label {ph_step}: ph_loss_label = {ph_loss.değer}\") }\n" +
			"}\n" +
			"yazdır(türevsiz { sonuç ph_X @ ph_w + ph_b })\n",
		Holes: map[string]string{
			"ph_X": "X", "ph_Y": "Y", "ph_w": "w", "ph_b": "b",
			"ph_xs": "${1:[[0.0], [1.0], [2.0], [3.0]]}", "ph_ys": "${2:[[-1.0], [1.0], [3.0], [5.0]]}",
			"ph_shape": "${3:1, 1}", "ph_rate": "${4:0.1}", "ph_steps": "${5:300}",
			"ph_optimizer": "%optimizer%", "ph_loss": "%loss%", "ph_step": "%step%",
			"ph_step_label": "%step_label%", "ph_loss_label": "%loss_label%",
		},
	},
}
