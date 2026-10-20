package messages

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

//go:embed definitions.json
var definitionsFile []byte

//go:embed tr.json
var trFile []byte

//go:embed en.json
var enFile []byte

// languageFiles gömülü dil dosyalarıdır; tr kaynak dildir ve her kodun metni oradadır. Başka
// bir dilde metni olmayan kod Türkçe gösterilir.
var languageFiles = map[string][]byte{"tr": trFile, "en": enFile}

// Catalog is one language's message templates, checked against the definitions.
type Catalog struct {
	Language  string
	templates map[Code][]segment
	params    map[Code][]string // tanımdaki parametre adları (eksik değerin {ad?} gösterimi için)
}

// Definitions is the language-independent part: the parameter names of every code, in the
// order New takes their values.
type Definitions struct {
	Params map[Code][]string
	Codes  []Code // artan sırada
}

var (
	loadOnce    sync.Once
	definitions *Definitions
	catalogs    map[string]*Catalog
)

// load gömülü dosyaları ilk iletide bir kez okur. Dosyalar depoyla birlikte gelir ve testlerle
// denetlenir; okunamazlarsa bu TR'nin kendi hatasıdır (panik).
func load() {
	loadOnce.Do(func() {
		d, err := ParseDefinitions(definitionsFile)
		if err != nil {
			panic(fmt.Sprintf("messages: definitions.json: %v", err))
		}
		definitions = d
		catalogs = map[string]*Catalog{}
		for lang, data := range languageFiles {
			c, err := ParseCatalog(lang, data, d)
			if err != nil {
				panic(fmt.Sprintf("messages: %s.json: %v", lang, err))
			}
			catalogs[lang] = c
		}
	})
}

// activeLanguage süreç dilidir: LanguageEnvVar ilk okunuşta sabitlenir (codes paketi de bu
// değeri kullanır). Bir süreç tek dil konuşur; erken kurulup geç gösterilen bir ileti ile adlar
// iki dile bölünmez. Testler değişkeni bu paketin içinden değiştirir.
var activeLanguage = sync.OnceValue(func() string {
	if lang := os.Getenv(LanguageEnvVar); lang != "" {
		return lang
	}
	return "tr"
})

// ActiveLanguage returns the process's language code, read once from LanguageEnvVar (empty: tr).
// codes uses the same value, so names and messages are always in one language.
func ActiveLanguage() string { return activeLanguage() }

// activeCatalog etkin dilin kataloğudur (bilinmeyen dilde Türkçe).
func activeCatalog() *Catalog {
	load()
	if c, ok := catalogs[activeLanguage()]; ok {
		return c
	}
	return catalogs["tr"]
}

// catalogFor iletiyi gösterecek kataloğudur: etkin dil iletiyi ve içindeki bütün kodlu hataları
// çeviriyorsa o, değilse kaynak dil. Eksik çeviri iletinin bütününü Türkçeye düşürür; çerçevesi
// İngilizce içi Türkçe bir satır okunmazdı.
func catalogFor(code Code, args []any) *Catalog {
	c := activeCatalog()
	if c.Language != "tr" && !c.translates(code, args) {
		return catalogs["tr"]
	}
	return c
}

// translates kodun ve değerlerindeki kodlu hataların hepsinin bu dilde metni olduğudur.
func (c *Catalog) translates(code Code, args []any) bool {
	if _, ok := c.templates[code]; !ok {
		return false
	}
	for _, a := range args {
		if !c.translatesValue(a) {
			return false
		}
	}
	return true
}

// translatesValue bir değerin bu dilde gösterilebildiğidir: kodlu hata ise metni vardır.
func (c *Catalog) translatesValue(a any) bool {
	switch e := a.(type) {
	case *Error:
		return c.translates(e.Code, e.Args)
	case *PosError:
		if code, args, ok := e.message(); ok {
			return c.translates(code, args)
		}
		return c.translatesValue(e.Err)
	}
	return true
}

// ParseDefinitions reads definitions.json: {"H7003": {"param": ["fn", "path"]}, ...}, codes
// in ascending order, parameter names unique and non-empty.
func ParseDefinitions(data []byte) (*Definitions, error) {
	var raw map[string]struct {
		Param []string `json:"param"`
	}
	if err := strictUnmarshal(data, &raw); err != nil {
		return nil, err
	}
	keys, err := orderedKeys(data)
	if err != nil {
		return nil, err
	}
	d := &Definitions{Params: make(map[Code][]string, len(raw))}
	for _, k := range keys {
		c, err := Parse(k)
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		for _, p := range raw[k].Param {
			if p == "" || seen[p] || strings.ContainsAny(p, "{}| ") {
				return nil, fmt.Errorf("%s: parametre adı %q boş, yinelenmiş ya da geçersiz", k, p)
			}
			seen[p] = true
		}
		d.Params[c] = raw[k].Param
		d.Codes = append(d.Codes, c)
	}
	return d, nil
}

// ParseCatalog reads a language file ({"H7003": "metin", ...}) and compiles every template
// against the definitions: placeholders must name a parameter of the code; in the source
// language (tr) every code has a text that uses every parameter.
func ParseCatalog(lang string, data []byte, d *Definitions) (*Catalog, error) {
	var raw map[string]string
	if err := strictUnmarshal(data, &raw); err != nil {
		return nil, err
	}
	keys, err := orderedKeys(data)
	if err != nil {
		return nil, err
	}
	c := &Catalog{Language: lang, templates: make(map[Code][]segment, len(raw)), params: d.Params}
	for _, k := range keys {
		code, err := Parse(k)
		if err != nil {
			return nil, err
		}
		params, ok := d.Params[code]
		if !ok {
			return nil, fmt.Errorf("%s: definitions.json'da tanımlı değil", k)
		}
		segs, used, err := compile(raw[k], params)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", k, err)
		}
		// Her dilin metni kodun bütün parametrelerini kullanır: çeviride düşen bir değer
		// (dosya yolu, sayı) kullanıcıya hiç ulaşmazdı.
		for i, p := range params {
			if !used[i] {
				return nil, fmt.Errorf("%s: %q parametresi metinde kullanılmıyor", k, p)
			}
		}
		c.templates[code] = segs
	}
	if lang == "tr" {
		for _, code := range d.Codes {
			if _, ok := c.templates[code]; !ok {
				return nil, fmt.Errorf("%s: tanımlı ama Türkçe metni yok", code)
			}
		}
	}
	return c, nil
}

// strictUnmarshal bilinmeyen alanları ve sondaki fazlalığı reddeder.
func strictUnmarshal(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("JSON nesnesinden sonra fazladan veri var")
	}
	return nil
}

// orderedKeys üst düzey nesnenin anahtarlarını dosyadaki sırayla verir; kodlar artan sırada
// ve tekil olmalıdır (her kod bir satır, aranınca bulunsun).
func orderedKeys(data []byte) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	var keys []string
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		keys = append(keys, t.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil, err
		}
	}
	if !sort.StringsAreSorted(keys) {
		return nil, fmt.Errorf("kodlar artan sırada olmalı")
	}
	for i := 1; i < len(keys); i++ {
		if keys[i] == keys[i-1] {
			return nil, fmt.Errorf("%s iki kez tanımlı", keys[i])
		}
	}
	return keys, nil
}
