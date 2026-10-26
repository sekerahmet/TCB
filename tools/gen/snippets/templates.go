package main

import (
	_ "embed"
	"encoding/json"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Şablonların verisi data.json'dadır (Go metnine Türkçe yazılmaz). Her şablonun gövdesi (source)
// Türkçe, geçerli TR kodudur ve source.Show ile her dile çevrilir; kullanıcının dolduracağı yerler
// "ph_" adlarıdır ve holes'ta VS Code şablon durağına (${1:varsayılan}) çevrilir. Bir deliğin
// metnindeki %sözcük% "words" tablosundan dilin sözcüğüyle gelir; durak varsayılanında dil öğesi
// (anahtar kelime, yerleşik adı) bulunmaz, o kodun kendisine yazılır.

//go:embed data.json
var dataJSON []byte

// prefix kısayolun kaynağıdır: bir kodun dildeki adı (Code, kimlik: TR_kw_func) ya da words
// tablosundaki bir sözcük (Word).
type prefix struct {
	Code string `json:"code"`
	Word string `json:"word"`
}

// template bir kod şablonudur.
type template struct {
	ID          string            `json:"id"`
	Label       map[string]string `json:"label"`
	Description map[string]string `json:"description"`
	Prefix      []prefix          `json:"prefix"`
	Member      bool              `json:"member"` // kısayol ayrıca ".ad" olarak da sunulur (metot şablonları)
	End         bool              `json:"end"`    // son satırın sonuna $0 (imleç son durağı) eklenir
	Source      string            `json:"source"`
	Holes       map[string]string `json:"holes"`
}

// wordTable dile bağlı sözcüklerdir (kullanıcının seçeceği ad ve metin önerileri); satırı olmayan dil
// İngilizceyi kullanır. templates data.json'daki şablonlardır.
var (
	wordTable map[string]map[string]string
	templates []template
)

func init() {
	var data struct {
		Words     map[string]map[string]string `json:"words"`
		Templates []template                   `json:"templates"`
	}
	if err := json.Unmarshal(dataJSON, &data); err != nil {
		panic("snippets: data.json: " + err.Error())
	}
	wordTable, templates = data.Words, data.Templates
}

// code kısayolun kodudur.
func (p prefix) code() (codes.Code, error) { return codes.Parse(p.Code) }
