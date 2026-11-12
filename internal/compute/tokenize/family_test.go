package tokenize

import (
	"errors"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// familyFixture: karakter düzeyinde küçük bir BPE; özel olmayan eklenmiş parçalar iki boşluk parçası
// (20 "\n\n", 24 "▁▁"), üç HTML öğe adı (21 "<b>", 22 "</b>", 23 "<think>") ve kümeye girmeyen
// "<B>" (26); özel parça "<bos>" (25).
const familyFixture = `{"added_tokens":[` +
	`{"id":20,"content":"\n\n","special":false,"normalized":false},` +
	`{"id":21,"content":"<b>","special":false,"normalized":false},` +
	`{"id":22,"content":"</b>","special":false,"normalized":false},` +
	`{"id":23,"content":"<think>","special":false,"normalized":false},` +
	`{"id":24,"content":"▁▁","special":false,"normalized":false},` +
	`{"id":25,"content":"<bos>","special":true,"normalized":false},` +
	`{"id":26,"content":"<B>","special":false,"normalized":false}],` +
	`"model":{"type":"BPE","vocab":{"\n":0," ":1,"<":2,">":3,"b":4,"/":5,"▁":6,"t":7,"h":8,"i":9,"n":10,` +
	`"k":11,"o":12,"s":13,"<unk>":14,"x":15},"merges":[],"unk_token":"<unk>","byte_fallback":false}}`

// fixtureFamily familyFixture'ın kümelerine uyan sahte bir ailedir; karmalar parçaların açık
// listesinden kurulur.
func fixtureFamily() Family {
	ws := []AddedToken{{ID: 20, Content: "\n\n"}, {ID: 24, Content: "▁▁"}}
	pat := []AddedToken{{ID: 21, Content: "<b>"}, {ID: 22, Content: "</b>"}, {ID: 23, Content: "<think>"}}
	return Family{ID: "sahte", Arch: "sahte", Whitespace: &FormatSet{2, setHash(ws)}, Pattern: &FormatSet{3, setHash(pat)},
		Protocol: []string{"<bos>", "<start_of_turn>"}}
}

func familyTokenizer(t *testing.T) *Tokenizer {
	t.Helper()
	p, err := Parse([]byte(familyFixture))
	must(t, err)
	return p
}

const familyText = "\n\nx<b>x</b><B><think>▁▁<bos>"

func trFalseIDs(p *Tokenizer, text string) []int {
	return p.EncodeWithOptions(text, Options{DontParseAdded: true, DontAddSpecial: true})
}

// Kural (16.2, iki kümenin kuralı): boşluk kümesi yalnız boşluk, ▁, sekme, satır sonu ve satır
// başından oluşan içerik; desen kümesi tam olarak ^</?[a-z]+>$. Sınır girdileriyle.
func TestFormatPieceRules(t *testing.T) {
	for _, c := range []struct {
		content    string
		whitespace bool
		pattern    bool
	}{
		{"", false, false}, {" ", true, false}, {"▁", true, false}, {"\r\n", true, false}, {"\t \n", true, false},
		{" a", false, false}, {" ", false, false}, {"<b>", false, true}, {"</b>", false, true},
		{"<think>", false, true}, {"<>", false, false}, {"</>", false, false}, {"<B>", false, false},
		{"<h1>", false, false}, {"<a_b>", false, false}, {"<b> ", false, false}, {"<b>\n", false, false},
		{"<<b>", false, false}, {"<start_of_turn>", false, false},
	} {
		if got := isWhitespacePiece(c.content); got != c.whitespace {
			t.Errorf("isWhitespacePiece(%q) = %v, beklenen %v", c.content, got, c.whitespace)
		}
		if got := formatPattern.MatchString(c.content); got != c.pattern {
			t.Errorf("desen %q = %v, beklenen %v", c.content, got, c.pattern)
		}
	}
}

// Kural (16.2, kapalı küme): karmanın yazımı Python'unkiyle aynı sabit girdide aynı değeri verir
// (Colab 3a'nın betiği ve yerel deneme: sha256 of "2:3c623e\n7:e29681e29681\n10:20\n").
func TestSetHashMatchesPython(t *testing.T) {
	got := setHash([]AddedToken{{ID: 10, Content: " "}, {ID: 2, Content: "<b>"}, {ID: 7, Content: "▁▁"}})
	const want = "cefcefb860323e873a94e291143ed1295100b4c07e5872684758f7852e6cc0d2"
	if got != want {
		t.Errorf("setHash = %s, Python %s", got, want)
	}
	if setHash(nil) != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("boş kümenin karması boş metnin SHA-256'sı olmalı")
	}
}

// Kural (16.2, ailenin biçim parçaları): aile uygulanınca iki kümenin parçaları TR_false'ta da tek
// kimlik olur; özel parça ve kümeye girmeyen parça kapalı kalır. Ailesiz parçalayıcıda hiçbiri.
func TestFamilyFormatPiecesAreOneIDUnderTRFalse(t *testing.T) {
	bare := familyTokenizer(t)
	for _, id := range trFalseIDs(bare, familyText) {
		if id >= 20 {
			t.Errorf("ailesiz parçalayıcıda eklenmiş parça kimliği %d çıktı", id)
		}
	}
	if bare.Family() != "" || !bare.HasWhitespacePieces() {
		t.Errorf("ailesiz: aile %q, boşluk parçası %v", bare.Family(), bare.HasWhitespacePieces())
	}
	p := familyTokenizer(t)
	must(t, p.applyFamily(fixtureFamily()))
	if p.Family() != "sahte" || p.Mismatch() != nil {
		t.Fatalf("aile %q, fark %+v", p.Family(), p.Mismatch())
	}
	got := trFalseIDs(p, familyText)
	for _, id := range []int{20, 21, 22, 23, 24} {
		if !slices.Contains(got, id) {
			t.Errorf("biçim parçası %d tek kimlik değil: %v", id, got)
		}
	}
	for _, id := range []int{25, 26} {
		if slices.Contains(got, id) {
			t.Errorf("kapalı kalması gereken %d çıktı: %v", id, got)
		}
	}
	want := []int{20, 15, 21, 15, 22, 2, 14, 3, 23, 24, 2, 4, 12, 13, 3}
	if !slices.Equal(got, want) {
		t.Errorf("kimlikler %v, beklenen %v", got, want)
	}
	if !slices.Equal(p.Encode(familyText), bare.Encode(familyText)) {
		t.Errorf("TR_true'nun (varsayılan kodlama) sonucu aileyle değişmemeli")
	}
}

// Kural (16.2, kapalı küme; H7184'ün dört kolu): kümesi ailenin resmî dosyasınınkine uymayan dosya
// ailesiz açılır ve fark kümeyi ve nasıl ayrıldığını söyler; iki küme birden ayrılırsa boşluk kümesi.
func TestFamilyMismatchLeavesTheTokenizerFamilyless(t *testing.T) {
	wrong := "0000000000000000000000000000000000000000000000000000000000000000"
	for _, c := range []struct {
		name   string
		change func(*Family)
		want   FamilyMismatch
	}{
		{"boşluk sayısı", func(f *Family) { f.Whitespace.Count = 3 }, FamilyMismatch{"sahte", "whitespace", "count", 2, 3}},
		{"boşluk içeriği", func(f *Family) { f.Whitespace.Hash = wrong }, FamilyMismatch{"sahte", "whitespace", "content", 2, 2}},
		{"desen sayısı", func(f *Family) { f.Pattern.Count = 1 }, FamilyMismatch{"sahte", "pattern", "count", 3, 1}},
		{"desen içeriği", func(f *Family) { f.Pattern.Hash = wrong }, FamilyMismatch{"sahte", "pattern", "content", 3, 3}},
		{"ikisi birden", func(f *Family) { f.Whitespace.Hash = wrong; f.Pattern.Count = 0 }, FamilyMismatch{"sahte", "whitespace", "content", 2, 2}},
	} {
		p := familyTokenizer(t)
		f := fixtureFamily()
		c.change(&f)
		must(t, p.applyFamily(f))
		if p.Family() != "" || p.Mismatch() == nil || *p.Mismatch() != c.want {
			t.Errorf("%s: aile %q, fark %+v, beklenen %+v", c.name, p.Family(), p.Mismatch(), c.want)
		}
		for _, id := range trFalseIDs(p, familyText) {
			if id >= 20 {
				t.Errorf("%s: ailesiz kalan parçalayıcıda %d çıktı", c.name, id)
			}
		}
	}
}

// Kural (16.2, H7185): ailenin bir biçim parçası protokol etiketiyle kesişirse yükleme hatasıdır
// (Go tablosunun yanlışı; sahte tabloyla sınanır).
func TestFamilyOverlapIsAnError(t *testing.T) {
	p := familyTokenizer(t)
	f := fixtureFamily()
	f.Protocol = append(f.Protocol, "<think>")
	err := p.applyFamily(f)
	var o *FamilyOverlapError
	if !errors.As(err, &o) || *o != (FamilyOverlapError{"sahte", "<think>", "<think>"}) {
		t.Fatalf("hata %v, beklenen <think>'in kesişmesi", err)
	}
	m := o.Message("parçalayıcı_yükle")
	want := "parçalayıcı_yükle: \"sahte\" ailesinin biçim parçası \"<think>\", ailenin protokol etiketi \"<think>\" ile kesişiyor"
	if !messages.Has(m, messages.H7185) || !strings.HasPrefix(m.Error(), want) || !strings.Contains(m.Error(), "lütfen bildirin (16.2)") {
		t.Errorf("ileti %q", m.Error())
	}
}

// Kural (16.2, ailesiz yolun iletileri): H7184 kümeyi ve farkı select'le söyler (dört kol); H7178 üç kolda
// klasörü ve ne yapılacağını söyler, folder kolu kopyalanınca çalışan çağrıyı yazar. Klasör dalı yalnız
// dosyanın kümesi ailenin kümesine uyarsa seçilir (compare; test ailesiyle).
func TestFamilylessMessages(t *testing.T) {
	for _, c := range []struct {
		m    FamilyMismatch
		want string
	}{
		{FamilyMismatch{"gemma3", "whitespace", "count", 1, 92}, "yalnız boşluk karakterlerinden oluşan parçalar bu dosyada 1, resmî dosyada 92;"},
		{FamilyMismatch{"gemma3", "whitespace", "content", 92, 92}, "yalnız boşluk karakterlerinden oluşan parçalar sayı aynı (bu dosyada 92, resmî dosyada 92) ama içerikler farklı;"},
		{FamilyMismatch{"gemma3", "pattern", "count", 60, 59}, "HTML öğe adı parçaları bu dosyada 60, resmî dosyada 59;"},
		{FamilyMismatch{"gemma3", "pattern", "content", 59, 59}, "HTML öğe adı parçaları sayı aynı (bu dosyada 59, resmî dosyada 59) ama içerikler farklı;"},
	} {
		err := c.m.Message()
		if !messages.Has(err, messages.H7184) || !strings.Contains(err.Error(), c.want) || !strings.HasPrefix(err.Error(), "\"gemma3\" ailesinin") {
			t.Errorf("%+v: %q", c.m, err.Error())
		}
	}
	for where, want := range map[string]string{
		"alone":  "\"m/k\" klasöründen model ailesi seçilemiyor; güvenilir metinde",
		"gguf":   "\"m/k\" klasöründeki GGUF dosyasının mimarisi ve ön bölme etiketi TR'nin aile tablosunda yok; güvenilir",
		"folder": "parçalayıcıyı modelin klasörüyle açın: `TR_load_tokenizer(\"m/k\")`; ya da güvenilir metinde",
	} {
		if err := WhitespaceError(where, "m/k"); !messages.Has(err, messages.H7178) || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %q", where, err.Error())
		}
	}
	p := familyTokenizer(t)
	f := fixtureFamily()
	if p.compare(f) != nil {
		t.Errorf("uyan küme fark vermemeli")
	}
	f.Pattern.Count = 4
	if m := p.compare(f); m == nil || m.Set != "pattern" {
		t.Errorf("uymayan küme fark vermeli: %+v", m)
	}
	if p.FitsFolderFamily("gemma3_text", "GemmaTokenizer") || p.FitsFolderFamily("yok", "yok") {
		t.Errorf("fikstürün kümesi Gemma'nın resmî kümesine uymaz; tabloda olmayan çift aile seçmez")
	}
}

// Kural (16.2, ailesiz yolun hatası; H7178'in gguf kolu): (mimari, etiket) çifti tabloda olmayan boşluk parçalı
// GGUF ailesizdir, TR_false'ın hatası gguf kolu; aynı parça kümesiz bir ailenin (qwen3) GGUF'unda hata vermez;
// boşluk parçası olmayan ailesiz dosyada da hata yok. Tek başına açılan dosyada klasör aile seçmiyorsa alone.
func TestFamilylessErrorBranches(t *testing.T) {
	ws := map[int]int32{256: ggufTokenUserDefined}
	g := mustGGUF(t, ggufVocab{arch: "llama", pre: "smollm", extra: []string{"\n\n"}, kinds: ws})
	if err := g.FamilylessError("", "", "m"); !messages.Has(err, messages.H7178) || !strings.Contains(err.Error(), "GGUF dosyasının") {
		t.Errorf("ailesiz GGUF: %v", err)
	}
	q := mustGGUF(t, ggufVocab{arch: "qwen3", pre: "qwen2", extra: []string{"\n\n"}, kinds: ws})
	if q.Family() != "qwen3" || q.FamilylessError("", "", "m") != nil {
		t.Errorf("kümesiz ailenin GGUF'u hata vermemeli: %q", q.Family())
	}
	if err := familyTokenizer(t).FamilylessError("gemma3_text", "GemmaTokenizer", "m"); !strings.Contains(err.Error(), "klasöründen model ailesi seçilemiyor") {
		t.Errorf("kümesi uymayan klasör ailesi alone kolu vermeli: %v", err)
	}
	// Kümesiz ailenin klasörü (qwen3) önerilir: klasörle açılınca aile qwen3 olur ve TR_false hata vermez.
	if err := familyTokenizer(t).FamilylessError("qwen3", "Qwen2Tokenizer", "m"); !strings.Contains(err.Error(), "`TR_load_tokenizer(\"m\")`") {
		t.Errorf("aile seçen klasör folder kolu vermeli: %v", err)
	}
	if mustGGUF(t, ggufVocab{arch: "llama", pre: "smollm"}).FamilylessError("", "", "m") != nil {
		t.Errorf("boşluk parçası olmayan ailesiz dosya hata vermemeli")
	}
}

// Kural (16.2, iki tablo; A2 (a)): Go'daki aile tablosu kitaplığın sohbet tablosuyla
// (lib/model_family.tr'nin _sohbet_tablosu) aynı satırları aynı sırayla taşır: aile kimliği, mimari,
// GGUF etiketi, klasörün işareti. Kitaplığın tablosu özel bir fonksiyondur; test kaynağı okur
// (TestChatTableIsUnique gibi).
func TestFamilyTableMatchesTheLibraryChatTable(t *testing.T) {
	src, err := os.ReadFile(reporoot.Path("lib", "model_family.tr"))
	must(t, err)
	rows := regexp.MustCompile(`_Sohbet\("([^"]*)", "([^"]*)", "([^"]*)", "([^"]*)", \[`).FindAllStringSubmatch(string(src), -1)
	var lib, gotab [][4]string
	for _, r := range rows {
		lib = append(lib, [4]string{r[1], r[2], r[3], r[4]})
	}
	for _, f := range Families() {
		gotab = append(gotab, [4]string{f.ID, f.Arch, f.GGUFTag, f.FolderMarker})
	}
	if len(lib) < 2 || !slices.Equal(lib, gotab) {
		t.Errorf("kitaplığın tablosu %q, Go'nun tablosu %q", lib, gotab)
	}
}

// Kural (16.2): kümesi olmayan aile (llama3, qwen3) bugünkü davranışı değiştirmez; tablodaki iki
// satır seçim anahtarlarıyla bulunur.
func TestFamilyWithoutSetsChangesNothing(t *testing.T) {
	bare := familyTokenizer(t)
	p := familyTokenizer(t)
	f, ok := GGUFFamily("qwen3", "qwen2")
	if !ok || f.ID != "qwen3" || f.Whitespace != nil || f.Pattern != nil {
		t.Fatalf("qwen3 satırı: %+v %v", f, ok)
	}
	must(t, p.applyFamily(f))
	if p.Family() != "qwen3" || !slices.Equal(trFalseIDs(p, familyText), trFalseIDs(bare, familyText)) {
		t.Errorf("kümesiz aile kodlamayı değiştirmemeli")
	}
	if f, ok := FolderFamily("llama", "<|begin_of_text|>"); !ok || f.ID != "llama3" {
		t.Errorf("llama3 satırı klasör anahtarıyla bulunmadı: %+v", f)
	}
	if _, ok := GGUFFamily("qwen2", "qwen2"); ok {
		t.Errorf("tabloda olmayan çift aile vermemeli")
	}
}
