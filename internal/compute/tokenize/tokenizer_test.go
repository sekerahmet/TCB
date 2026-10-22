package tokenize

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Altın veriler (testdata/generate.py ile üretilmiş)
// ============================================================================

const smallPath = "testdata/tokenizer_byte_bpe.json"

// syntheticTokenizers testdata'daki parçalayıcıların adlarıdır.
var syntheticTokenizers = []string{
	"byte_bpe", "metaspace_bpe", "llama_style", "word_bpe", "prefixed_bpe",
	"wordpiece", "unigram", "gpt2_style", "llama3_style",
}

// goldenFile golden_<name>.json biçimidir. Kodlama: [girdi sırası, kimlikler,
// özel tanımadan kimlikler, özel eklemeden kimlikler]. Çift: [a, b, kimlikler,
// tür kimlikleri, özel maske, özelsiz kimlikler, özelsiz tür kimlikleri].
// Çözme: [kimlikler, metin, özel atlanmış metin].
type goldenFile struct {
	VocabSize int                 `json:"sozluk_boyutu"`
	Encoding  [][]json.RawMessage `json:"kodlama"`
	Pair      [][]json.RawMessage `json:"cift"`
	Decoding  [][]json.RawMessage `json:"cozme"`
}

type goldenEncoding struct {
	Text    string
	IDs     []int
	IDsFlat []int
}

type goldenData struct {
	VocabSize int
	Encoding  []goldenEncoding
}

var (
	inputOne    sync.Once
	inputs      []string
	inputErr    error
	smallOne    sync.Once
	smallP      *Tokenizer
	smallErr    error
	smallGolden goldenData
)

func loadInputs(t testing.TB) []string {
	t.Helper()
	inputOne.Do(func() {
		var d struct {
			Inputs []string `json:"girdiler"`
		}
		data, err := os.ReadFile("testdata/inputs.json")
		if err == nil {
			err = json.Unmarshal(data, &d)
		}
		inputs, inputErr = d.Inputs, err
	})
	if inputErr != nil {
		t.Fatalf("girdiler okunamadı: %v", inputErr)
	}
	return inputs
}

func loadGolden(t testing.TB, ad string) (*Tokenizer, *goldenFile) {
	t.Helper()
	p, err := Load("testdata/tokenizer_" + ad + ".json")
	if err != nil {
		t.Fatalf("%s yüklenemedi: %v", ad, err)
	}
	data, err := os.ReadFile("testdata/golden_" + ad + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var a goldenFile
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatalf("golden_%s.json çözümlenemedi: %v", ad, err)
	}
	return p, &a
}

func decode[T any](t testing.TB, r json.RawMessage) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(r, &v); err != nil {
		t.Fatalf("altın veri çözümlenemedi: %v", err)
	}
	return v
}

// loadSmall bayt düzeyi BPE parçalayıcısını ve altın kodlamalarını verir.
func loadSmall(t testing.TB) (*Tokenizer, *goldenData) {
	t.Helper()
	g := loadInputs(t)
	smallOne.Do(func() {
		var a *goldenFile
		smallP, a = loadGolden(t, "byte_bpe")
		smallGolden.VocabSize = a.VocabSize
		for _, k := range a.Encoding {
			smallGolden.Encoding = append(smallGolden.Encoding, goldenEncoding{
				Text: g[decode[int](t, k[0])], IDs: decode[[]int](t, k[1]), IDsFlat: decode[[]int](t, k[2]),
			})
		}
	})
	if smallErr != nil {
		t.Fatal(smallErr)
	}
	return smallP, &smallGolden
}

func equalInt(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equal32(a []int32, b []int) bool { return equalInt(ToInt(nil, a), b) }

func equalBool(a []bool, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != (b[i] != 0) {
			return false
		}
	}
	return true
}

// TestSyntheticTokenizers her sentetik parçalayıcıyı altın çıktılarla
// karşılaştırır: tek metin kodlama (üç seçenekle), çift kodlama (tür
// kimlikleri ve özel maske dahil) ve çözme (özel atlamalı ve atlamasız).
func TestSyntheticTokenizers(t *testing.T) {
	g := loadInputs(t)
	for _, ad := range syntheticTokenizers {
		t.Run(ad, func(t *testing.T) {
			p, a := loadGolden(t, ad)
			if p.VocabSize() != a.VocabSize {
				t.Errorf("sözlük boyutu %d, beklenen %d", p.VocabSize(), a.VocabSize)
			}
			failure, total := 0, 0
			report := func(f string, x ...any) {
				failure++
				if failure <= 8 {
					t.Errorf(f, x...)
				}
			}
			for _, k := range a.Encoding {
				m := g[decode[int](t, k[0])]
				options := []Options{{}, {DontParseSpecial: true}, {DontAddSpecial: true}}
				for i, s := range options {
					total++
					expected := decode[[]int](t, k[1+i])
					if got := p.EncodeWithOptions(m, s); !equalInt(got, expected) {
						report("Kodla(%q, %+v)\n alınan: %v\nbeklenen: %v", m, s, got, expected)
					}
				}
			}
			for _, c := range a.Pair {
				total += 2
				x, y := g[decode[int](t, c[0])], g[decode[int](t, c[1])]
				k := p.EncodePair(x, &y, Options{})
				if !equal32(k.IDs, decode[[]int](t, c[2])) || !equal32(k.TypeIDs, decode[[]int](t, c[3])) || !equalBool(k.SpecialMask, decode[[]int](t, c[4])) {
					report("KodlaCift(%q, %q)\n alınan: %v %v %v\nbeklenen: %s %s %s", x, y, k.IDs, k.TypeIDs, k.SpecialMask, c[2], c[3], c[4])
				}
				k2 := p.EncodePair(x, &y, Options{DontAddSpecial: true})
				if !equal32(k2.IDs, decode[[]int](t, c[5])) || !equal32(k2.TypeIDs, decode[[]int](t, c[6])) {
					report("KodlaCift özelsiz(%q, %q)\n alınan: %v %v\nbeklenen: %s %s", x, y, k2.IDs, k2.TypeIDs, c[5], c[6])
				}
			}
			for _, c := range a.Decoding {
				total += 2
				ids := decode[[]int](t, c[0])
				if got, b := p.Decode(ids, false), decode[string](t, c[1]); got != b {
					report("Coz(%v, false)\n alınan: %q\nbeklenen: %q", ids, got, b)
				}
				if got, b := p.Decode(ids, true), decode[string](t, c[2]); got != b {
					report("Coz(%v, true)\n alınan: %q\nbeklenen: %q", ids, got, b)
				}
			}
			if failure > 0 {
				t.Fatalf("%d / %d karşılaştırma uyuşmadı", failure, total)
			}
			t.Logf("%d karşılaştırma eşleşti", total)
		})
	}
}

// TestComponents tek tek bileşenleri (normalleştirici, ön bölücü, çözücü)
// altın çıktılarla karşılaştırır.
func TestComponents(t *testing.T) {
	g := loadInputs(t)[1:140]
	data, err := os.ReadFile("testdata/components.json")
	if err != nil {
		t.Fatal(err)
	}
	var records [][]json.RawMessage
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatal(err)
	}
	compareComponent(t, g, records)
}

// TestComponentsWide, TR_PARCALAYICI_BILESEN_ALTIN ile verilen geniş altın
// veriyle (ör. bütün Unicode kod noktaları) bileşenleri karşılaştırır.
// Biçim: {"girdiler": [...], "kayitlar": [components.json kayıtları]}.
func TestComponentsWide(t *testing.T) {
	path := os.Getenv("TR_PARCALAYICI_BILESEN_ALTIN")
	if path == "" {
		t.Skip("TR_PARCALAYICI_BILESEN_ALTIN ayarlı değil")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var d struct {
		Inputs  []string            `json:"girdiler"`
		Records [][]json.RawMessage `json:"kayitlar"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatal(err)
	}
	compareComponent(t, d.Inputs, d.Records)
}

func compareComponent(t *testing.T, g []string, records [][]json.RawMessage) {
	counter := map[string]int{}
	failure := 0
	report := func(f string, x ...any) {
		failure++
		if failure <= 15 {
			t.Errorf(f, x...)
		}
	}
	for _, k := range records {
		kind := decode[string](t, k[0])
		def := k[1]
		switch kind {
		case "normalizer":
			n, err := NewNormalizer(def)
			if err != nil {
				report("%s: %v", def, err)
				continue
			}
			for i, b := range decode[[]string](t, k[2]) {
				counter[kind]++
				if got := n.Normalize(g[i]); got != b {
					report("%s\n girdi: %q\n alınan: %q\nbeklenen: %q", def, g[i], got, b)
				}
			}
		case "pre_tokenizer":
			o, err := NewPreTokenizer(def)
			if err != nil {
				report("%s: %v", def, err)
				continue
			}
			for i, b := range decode[[][]string](t, k[2]) {
				counter[kind]++
				var got []string
				for _, x := range o.PreTokenize(nil, []Piece{{Text: g[i], Start: true}}) {
					got = append(got, x.Content())
				}
				if len(got) != len(b) || (len(got) > 0 && !reflect.DeepEqual(got, b)) {
					report("%s\n girdi: %q\n alınan: %q\nbeklenen: %q", def, g[i], got, b)
				}
			}
		case "decoder":
			c, err := NewDecoder(def)
			if err != nil {
				report("%s: %v", def, err)
				continue
			}
			lists := decode[[][]string](t, k[2])
			for i, b := range decode[[]string](t, k[3]) {
				counter[kind]++
				if got := strings.Join(c.DecodeChain(append([]string(nil), lists[i]...)), ""); got != b {
					report("%s\n girdi: %q\n alınan: %q\nbeklenen: %q", def, lists[i], got, b)
				}
			}
		}
	}
	if failure > 0 {
		t.Fatalf("%d karşılaştırma uyuşmadı (%v)", failure, counter)
	}
	t.Logf("eşleşen karşılaştırmalar: %v", counter)
}

// Kodlama sonra çözme, bayt düzeyi parçalayıcıda metnin kendisini vermeli.
func TestRoundTrip(t *testing.T) {
	p, a := loadSmall(t)
	// Bu ek tokenlar tasarım gereği gidiş-dönüşü bozar: boşluk yutan
	// (lstrip/rstrip) ya da çözücüde bayt eşlemesine takılan içerikler.
	breakers := []string{"<|maske|>", "[KELİME]", "çaycı", "Özel"}
	for _, k := range a.Encoding {
		skip := false
		for _, b := range breakers {
			skip = skip || strings.Contains(k.Text, b)
		}
		if skip {
			continue
		}
		if got := p.Decode(p.Encode(k.Text), false); got != k.Text {
			t.Errorf("gidiş-dönüş bozuk: %q -> %q", k.Text, got)
		}
	}
}

// Ön bölme parçalarının birleşimi girdinin kendisi olmalı.
func TestPreTokenizeCovers(t *testing.T) {
	for _, m := range loadInputs(t) {
		if got := strings.Join(SplitByPattern(m), ""); got != m {
			t.Errorf("ön bölme metni kapsamıyor: %q -> %q", m, got)
		}
	}
}

func TestPreTokenizeExamples(t *testing.T) {
	cases := []struct {
		text  string
		piece []string
	}{
		{"Ankara'ya gitti", []string{"Ankara", "'ya", " gitti"}},
		{"İzmir’e 3'te", []string{"İzmir", "’e", " 3", "'te"}},
		{"2024", []string{"2", "0", "2", "4"}},
		{"a   b", []string{"a", "  ", " b"}},
		{"a \n b", []string{"a", " \n", " b"}},
		{"son   ", []string{"son", "   "}},
		{"a\nb", []string{"a", "\n", "b"}},
		{"hey!!! ...", []string{"hey", "!!!", " ..."}},
		{" 'ya", []string{" '", "ya"}},
		{"", nil},
	}
	for _, d := range cases {
		if got := SplitByPattern(d.text); !reflect.DeepEqual(got, d.piece) {
			t.Errorf("DesenleBol(%q) = %q, beklenen %q", d.text, got, d.piece)
		}
	}
}

// Elle yazılmış hızlı eşleştirici ile genel düzenli ifade motoru aynı
// parçaları vermeli.
func TestFastPatternMatchesEngine(t *testing.T) {
	expr, err := CompileRegex(SplitPattern)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range loadInputs(t) {
		var got []string
		expr.FindAll(m, func(b, e int) { got = append(got, m[b:e]) })
		if want := SplitByPattern(m); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: motor %q, hızlı yol %q", m, got, want)
		}
	}
}

// Birleştirmeler eski "a b" metin biçiminde de okunabilmeli.
func TestStringFormMerges(t *testing.T) {
	_, a := loadSmall(t)
	data, err := os.ReadFile(smallPath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	model := raw["model"].(map[string]any)
	var texts []string
	for _, m := range model["merges"].([]any) {
		c := m.([]any)
		texts = append(texts, c[0].(string)+" "+c[1].(string))
	}
	model["merges"] = texts
	fresh, _ := json.Marshal(raw)
	p, err := Parse(fresh)
	if err != nil {
		t.Fatalf("metin biçimli birleştirmeler okunamadı: %v", err)
	}
	for _, k := range a.Encoding {
		if got := p.Encode(k.Text); !equalInt(got, k.IDs) {
			t.Fatalf("metin biçimli birleştirmelerle Kodla(%q) = %v, beklenen %v", k.Text, got, k.IDs)
		}
	}
}

func TestUnsupportedConfig(t *testing.T) {
	data, err := os.ReadFile(smallPath)
	if err != nil {
		t.Fatal(err)
	}
	replace := func(f func(m map[string]any)) []byte {
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatal(err)
		}
		f(raw)
		b, _ := json.Marshal(raw)
		return b
	}
	// template tek özel tokenlı bir TemplateProcessing verir.
	template := func(id int64) map[string]any {
		return map[string]any{"type": "TemplateProcessing",
			"single":         []any{map[string]any{"SpecialToken": map[string]any{"id": "<s>", "type_id": 0}}, map[string]any{"Sequence": map[string]any{"id": "A", "type_id": 0}}},
			"pair":           []any{map[string]any{"Sequence": map[string]any{"id": "A", "type_id": 0}}},
			"special_tokens": map[string]any{"<s>": map[string]any{"id": "<s>", "ids": []any{id}, "tokens": []any{"<s>"}}}}
	}
	// Her durum: değişiklik ve hata iletisinde geçmesi gereken bileşen/tür adı.
	cases := map[string]struct {
		f    func(m map[string]any)
		code messages.Code
	}{
		"normalizer": {func(m map[string]any) { m["normalizer"] = map[string]any{"type": "Benzersiz"} }, messages.H5202},
		"ön bölücü": {func(m map[string]any) {
			pt := m["pre_tokenizer"].(map[string]any)["pretokenizers"].([]any)
			pt[0] = map[string]any{"type": "UnicodeScripts"}
		}, messages.H5202},
		"desen": {func(m map[string]any) {
			pt := m["pre_tokenizer"].(map[string]any)["pretokenizers"].([]any)
			pt[0].(map[string]any)["pattern"] = map[string]any{"Regex": `(a)\1`}
		}, messages.H5264},
		"model türü":     {func(m map[string]any) { m["model"].(map[string]any)["type"] = "Benzersiz" }, messages.H5202},
		"dropout":        {func(m map[string]any) { m["model"].(map[string]any)["dropout"] = 0.1 }, messages.H5216},
		"çözücü":         {func(m map[string]any) { m["decoder"] = map[string]any{"type": "CTC"} }, messages.H5202},
		"son işlem":      {func(m map[string]any) { m["post_processor"] = map[string]any{"type": "Yok"} }, messages.H5202},
		"dolgu":          {func(m map[string]any) { m["padding"] = map[string]any{"length": 8} }, messages.H5276},
		"şablon kimliği": {func(m map[string]any) { m["post_processor"] = template(1 << 30) }, messages.H5278},
		"taşan kimlik":   {func(m map[string]any) { m["post_processor"] = template(1<<32 + 1) }, messages.H5231},
		"cls kimliği": {func(m map[string]any) {
			m["post_processor"] = map[string]any{"type": "BertProcessing", "sep": []any{"[SEP]", 1 << 30}, "cls": []any{"[CLS]", 0}}
		}, messages.H5278},
		"birleştirme": {func(m map[string]any) {
			mm := m["model"].(map[string]any)
			mm["merges"] = append(mm["merges"].([]any), []any{"yokyok", "zzz"})
		}, messages.H5219},
	}
	for ad, d := range cases {
		_, err := Parse(replace(d.f))
		if err == nil {
			t.Errorf("%s: hata bekleniyordu", ad)
		} else if !messages.Has(err, d.code) || !utf8.ValidString(err.Error()) {
			t.Errorf("%s: hata %q, %s kodunu içermeli", ad, err.Error(), d.code)
		}
	}
	if _, err := Parse(replace(func(map[string]any) {})); err != nil {
		t.Fatalf("değiştirilmemiş dosya hata verdi: %v", err)
	}
}

func TestLossyUTF8(t *testing.T) {
	cases := []struct {
		input  []byte
		output string
	}{
		{[]byte("abc"), "abc"},
		{[]byte{0xE2, 0x82, 'a'}, "�a"},
		{[]byte{0xE2, 0x82}, "�"},
		{[]byte{0x80, 0x80}, "��"},
		{[]byte{0xC4}, "�"},
		{[]byte{0xC4, 0x9F}, "ğ"},
		{[]byte{0xED, 0xA0, 0x80}, "���"},
		{[]byte{0xF0, 0x9F, 0x98}, "�"},
		{[]byte{0xF0, 0x9F, 0x98, 0x80}, "😀"},
		{[]byte{0xF4, 0x90, 0x80, 0x80}, "����"},
		{[]byte{0xFF, 'x'}, "�x"},
	}
	for _, d := range cases {
		if got := lossyUTF8(d.input); got != d.output {
			t.Errorf("kayipliUTF8(% x) = %q, beklenen %q", d.input, got, d.output)
		}
	}
}

func TestInvalidIDs(t *testing.T) {
	p, _ := loadSmall(t)
	if got := p.Decode([]int{-1, -100, 1 << 40}, false); got != "" {
		t.Fatalf("geçersiz kimlikler atlanmalıydı, alınan %q", got)
	}
}

func TestBatchAndParallel(t *testing.T) {
	p, a := loadSmall(t)
	var texts []string
	var expected [][]int
	for repeat := 0; repeat < 5; repeat++ {
		for _, k := range a.Encoding {
			texts = append(texts, k.Text)
			expected = append(expected, k.IDs)
		}
	}
	for _, worker := range []int{1, 3, 0} {
		got := p.EncodeBatch(texts, Options{}, worker)
		for i := range got {
			if !equalInt(got[i], expected[i]) {
				t.Fatalf("isci=%d: TopluKodla[%d] uyuşmadı", worker, i)
			}
		}
		decoded := p.DecodeBatch(got, false, worker)
		for i := range decoded {
			if decoded[i] != p.Decode(got[i], false) {
				t.Fatalf("isci=%d: TopluCoz[%d] uyuşmadı", worker, i)
			}
		}
		g32 := p.EncodeBatch32(texts[:50], Options{}, worker)
		for i := range g32 {
			if !equalInt(p.EncodeAppend(nil, texts[i], true), expected[i]) || p.Decode32(g32[i], false) != decoded[i] {
				t.Fatalf("isci=%d: TopluKodla32[%d] uyuşmadı", worker, i)
			}
		}
	}
	eot, ok := p.TokenToID("<|endoftext|>")
	if !ok {
		t.Fatal("<|endoftext|> bulunamadı")
	}
	joined := p.JoinDocuments([]string{"bir", "iki"}, eot, 0)
	expectedB := append(append(append(p.Encode("bir"), eot), p.Encode("iki")...), eot)
	if got := ToInt(nil, joined); !equalInt(got, expectedB) {
		t.Fatalf("BelgeleriBirlestir = %v, beklenen %v", got, expectedB)
	}
}

// Önbellek kapalıyken de sonuçlar aynı olmalı; uzun parçalar yığın yolunu sınar.
func TestUncachedAndLong(t *testing.T) {
	p, err := Load(smallPath)
	if err != nil {
		t.Fatal(err)
	}
	_, a := loadSmall(t)
	p.SetCacheCapacity(0)
	for _, k := range a.Encoding {
		if got := p.Encode(k.Text); !equalInt(got, k.IDs) {
			t.Fatalf("önbelleksiz Kodla(%q) uyuşmadı", k.Text)
		}
	}
	// Kısa ve uzun birleştirme yolları aynı sonucu vermeli.
	bm := p.model.(*bpeModel)
	for _, k := range a.Encoding {
		for _, s := range SplitByPattern(k.Text) {
			var sem []int32
			for i := 0; i < len(s); i++ {
				if id := bm.byteCharID[s[i]]; id >= 0 {
					sem = append(sem, id)
				}
			}
			k1 := bm.joinShort(append([]int32(nil), sem...))
			if len(sem) == 0 {
				continue
			}
			k2 := bm.joinLong(append([]int32(nil), sem...))
			if !reflect.DeepEqual(k1, k2) {
				t.Fatalf("kısa/uzun birleştirme farklı: %q: %v / %v", s, k1, k2)
			}
		}
	}
}

// ============================================================================
// Gerçek parçalayıcıya karşı sınama (isteğe bağlı)
// ============================================================================
//
// TR_PARCALAYICI_YOLU ayarlıysa çalışır. TR_PARCALAYICI_ALTIN ayrıca ayarlıysa
// (her satırı {"metin":..., "idler":[...]} olan JSONL dosyası), kodlama ve
// çözme sonuçları bu altın çıktılarla birebir karşılaştırılır.

func TestRealTokenizer(t *testing.T) {
	path := os.Getenv("TR_PARCALAYICI_YOLU")
	if path == "" {
		t.Skip("TR_PARCALAYICI_YOLU ayarlı değil")
	}
	p, err := Load(path)
	if err != nil {
		t.Fatalf("gerçek parçalayıcı yüklenemedi: %v", err)
	}
	t.Logf("sözlük: %d, birleştirme: %d", p.VocabSize(), p.MergeCount())
	// Kayıpsız (normalleştiricisiz, bayt düzeyi) parçalayıcıda gidiş-dönüş.
	if p.byteDecoder && p.normalizer == nil && p.postProcessor == nil {
		for _, m := range loadInputs(t) {
			if got := p.Decode(p.Encode(m), false); got != m {
				t.Errorf("gidiş-dönüş bozuk: %q -> %q", m, got)
			}
		}
	}
	goldenPath := os.Getenv("TR_PARCALAYICI_ALTIN")
	if goldenPath == "" {
		return
	}
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	// Satır biçimi: {"metin": metin ya da null, "idler": [...], "cozme": metin}
	// metin null değilse Kodla(metin) == idler; her satırda Coz(idler) == cozme.
	type record struct {
		Text     *string `json:"metin"`
		IDs      []int   `json:"idler"`
		Decoding string  `json:"cozme"`
		Skip     bool    `json:"ozel_atla"`
	}
	var records []record
	var texts []string
	var textRecord []int
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var k record
		if err := json.Unmarshal([]byte(line), &k); err != nil {
			t.Fatal(err)
		}
		if k.Text != nil {
			texts = append(texts, *k.Text)
			textRecord = append(textRecord, len(records))
		}
		records = append(records, k)
	}
	got := p.EncodeBatch(texts, Options{}, 0)
	matched, bad := 0, 0
	totalBytes := 0
	for i := range texts {
		totalBytes += len(texts[i])
		b := records[textRecord[i]].IDs
		if equalInt(got[i], b) {
			matched++
		} else if bad++; bad <= 10 {
			t.Errorf("uyuşmazlık: %q\n alınan: %v\nbeklenen: %v", texts[i], got[i], b)
		}
	}
	decodeMatched := 0
	for _, k := range records {
		if p.Decode(k.IDs, k.Skip) == k.Decoding {
			decodeMatched++
		} else if bad++; bad <= 10 {
			t.Errorf("çözme uyuşmazlığı: %v\n alınan: %q\nbeklenen: %q", k.IDs, p.Decode(k.IDs, k.Skip), k.Decoding)
		}
	}
	t.Logf("kodlama: %d / %d eşleşti (%d bayt); çözme: %d / %d eşleşti",
		matched, len(texts), totalBytes, decodeMatched, len(records))
	if matched != len(texts) || decodeMatched != len(records) {
		t.Fatalf("%d kodlama, %d çözme uyuşmadı", len(texts)-matched, len(records)-decodeMatched)
	}
}

// ============================================================================
// Hız ölçümleri
// ============================================================================

// benchText ölçüm metnini verir: TR_PARCALAYICI_OLCUM_METNI ayarlıysa o
// dosyanın içeriği, değilse örnek metnin ~1 MB'a tekrarlanmış hâli.
func benchText(tb testing.TB) string {
	if path := os.Getenv("TR_PARCALAYICI_OLCUM_METNI"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			tb.Fatal(err)
		}
		return string(data)
	}
	data, err := os.ReadFile("testdata/sample_text.txt")
	if err != nil {
		tb.Fatal(err)
	}
	// ~1 MB'lık metin; her tekrarda bir sayı katarak önbelleğin tamamen
	// isabet etmesini biraz bozarız.
	var sb strings.Builder
	for i := 0; sb.Len() < 1<<20; i++ {
		sb.Write(data)
		fmt.Fprintf(&sb, " %d\n", i)
	}
	return sb.String()
}

func benchTokenizer(b *testing.B) *Tokenizer {
	if path := os.Getenv("TR_PARCALAYICI_YOLU"); path != "" {
		p, err := Load(path)
		if err != nil {
			b.Fatal(err)
		}
		return p
	}
	p, err := Load(filepath.FromSlash(smallPath))
	if err != nil {
		b.Fatal(err)
	}
	return p
}

func BenchmarkEncode(b *testing.B) {
	p := benchTokenizer(b)
	text := benchText(b)
	b.SetBytes(int64(len(text)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Encode32(nil, text, Options{})
	}
}

func BenchmarkEncodeUncached(b *testing.B) {
	p := benchTokenizer(b)
	p.SetCacheCapacity(0)
	text := benchText(b)
	b.SetBytes(int64(len(text)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Encode32(nil, text, Options{})
	}
}

func BenchmarkEncodeBatchParallel(b *testing.B) {
	p := benchTokenizer(b)
	text := benchText(b)
	lines := strings.SplitAfter(text, "\n")
	b.SetBytes(int64(len(text)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.EncodeBatch32(lines, Options{}, 0)
	}
}

func BenchmarkDecode(b *testing.B) {
	p := benchTokenizer(b)
	text := benchText(b)
	ids := p.Encode(text)
	b.SetBytes(int64(len(text)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Decode(ids, false)
	}
}

// Bayt düzeyi ön bölücünün deseni bir kez derlenir; parçalayıcılar eşzamanlı yüklenebilir
// (eşzamanlı görevler; yarış dedektörüyle otomatik sınamada denetlenir).
func TestParallelByteLevelLoad(t *testing.T) {
	js := `{"version":"1.0","truncation":null,"padding":null,"added_tokens":[],
"normalizer":null,"pre_tokenizer":{"type":"ByteLevel","add_prefix_space":false,"trim_offsets":true,"use_regex":true},
"post_processor":null,"decoder":null,
"model":{"type":"WordLevel","unk_token":"<unk>","vocab":{"<unk>":0}}}`
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := Parse([]byte(js))
			if err == nil {
				p.Encode("merhaba dünya")
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}
