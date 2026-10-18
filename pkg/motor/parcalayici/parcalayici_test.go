package parcalayici_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"tr_dili/pkg/hataayiklama"
	"tr_dili/pkg/lexer"
	_ "tr_dili/pkg/motor"
	pc "tr_dili/pkg/parcalayici"
	"tr_dili/pkg/parser"
	"tr_dili/pkg/vm"
)

const (
	bpeYolu  = "../../parcalayici/testdata/p_bayt_bpe.json"
	bertYolu = "../../parcalayici/testdata/p_wordpiece.json"
)

func calistir(t *testing.T, kod string) (*vm.VM, error) {
	t.Helper()
	p := parser.New(lexer.New(kod))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := vm.New()
	m.Out = &bytes.Buffer{}
	_, err := m.Run(prog)
	return m, err
}

func mustCalistir(t *testing.T, kod string) *vm.VM {
	t.Helper()
	m, err := calistir(t, kod)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v\nkod:\n%s", err, kod)
	}
	return m
}

func al(t *testing.T, m *vm.VM, ad string) vm.Value {
	t.Helper()
	v, ok := m.GlobalEnv.Get(ad)
	if !ok {
		t.Fatalf("%s yok", ad)
	}
	return v
}

func tamlar(t *testing.T, v vm.Value) []int {
	t.Helper()
	l, ok := v.(*vm.ListVal)
	if !ok {
		t.Fatalf("liste bekleniyordu: %s", v.Inspect())
	}
	out := []int{}
	for _, e := range l.Elements {
		out = append(out, int(e.(*vm.IntVal).Val))
	}
	return out
}

func goParcalayici(t *testing.T, yol string) *pc.Parcalayici {
	t.Helper()
	p, err := pc.Yukle(yol)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TR'deki kodla/çöz sonuçları Go kütüphanesiyle birebir aynıdır.
func TestKodlaCozGoIleAyni(t *testing.T) {
	g := goParcalayici(t, bpeYolu)
	metinler := []string{"Merhaba dünya!", "İstanbul'da hava güzel <|endoftext|>", ""}
	m := mustCalistir(t, `
değişken p = parçalayıcı_yükle("`+bpeYolu+`")
değişken tek = p.kodla("Merhaba dünya!")
değişken toplu = p.kodla(["Merhaba dünya!", "İstanbul'da hava güzel <|endoftext|>", ""])
değişken çözülen = p.çöz(toplu)
değişken özelli = p.çöz(toplu[1], özel_parçalar: doğru)
değişken özelsiz = p.çöz(toplu[1])
değişken boyut = p.dağarcık_boyutu
değişken eot = p.parça_kimliği("<|endoftext|>")
değişken yok_token = p.parça_kimliği("böyle-bir-token-yok")
`)
	if got := tamlar(t, al(t, m, "tek")); !reflect.DeepEqual(got, g.Kodla(metinler[0])) {
		t.Fatalf("kodla: %v, Go: %v", got, g.Kodla(metinler[0]))
	}
	toplu := al(t, m, "toplu").(*vm.ListVal)
	for i, s := range metinler {
		if got := tamlar(t, toplu.Elements[i]); !reflect.DeepEqual(got, g.Kodla(s)) {
			t.Fatalf("toplu kodla %d: %v, Go: %v", i, got, g.Kodla(s))
		}
	}
	cozulen := al(t, m, "çözülen").(*vm.ListVal)
	for i, s := range metinler {
		if got := cozulen.Elements[i].Inspect(); got != g.Coz(g.Kodla(s), true) {
			t.Fatalf("toplu çöz %d: %q", i, got)
		}
	}
	if got := al(t, m, "özelli").Inspect(); got != metinler[1] {
		t.Fatalf("özel_parçalar: doğru ile çöz: %q", got)
	}
	if got := al(t, m, "özelsiz").Inspect(); strings.Contains(got, "<|endoftext|>") {
		t.Fatalf("varsayılan çöz özel tokenları atlamalı: %q", got)
	}
	if got := al(t, m, "boyut").(*vm.IntVal).Val; int(got) != g.SozlukBoyutu() {
		t.Fatalf("sözlük boyutu %d", got)
	}
	id, _ := g.TokenKimligi("<|endoftext|>")
	if got := al(t, m, "eot").(*vm.IntVal).Val; int(got) != id {
		t.Fatalf("token_kimliği %d", got)
	}
	if al(t, m, "yok_token") != vm.NULL {
		t.Fatal("olmayan token yok dönmeli")
	}
}

// özel_parçalar: yanlış son işleyicinin eklemelerini kapatır; ikinci: çift kodlar.
func TestOzelTokenlarVeIkinci(t *testing.T) {
	g := goParcalayici(t, bertYolu)
	m := mustCalistir(t, `
değişken p = parçalayıcı_yükle("`+bertYolu+`")
değişken a = p.kodla("merhaba dünya")
değişken b = p.kodla("merhaba dünya", özel_parçalar: yanlış)
değişken c = p.kodla("soru", ikinci: "bağlam")
değişken d = p.kodla(["soru", "bir"], ikinci: ["bağlam", "iki"], özel_parçalar: yanlış)
`)
	if got := tamlar(t, al(t, m, "a")); !reflect.DeepEqual(got, g.Kodla("merhaba dünya")) {
		t.Fatalf("a: %v", got)
	}
	if got, b := tamlar(t, al(t, m, "b")), g.KodlaSecenekli("merhaba dünya", pc.Secenekler{OzelEklenmesin: true}); !reflect.DeepEqual(got, b) {
		t.Fatalf("b: %v, Go: %v", got, b)
	}
	ik := "bağlam"
	k := g.KodlaCift("soru", &ik, pc.Secenekler{})
	beklenen := []int{}
	for _, x := range k.Idler {
		beklenen = append(beklenen, int(x))
	}
	if got := tamlar(t, al(t, m, "c")); !reflect.DeepEqual(got, beklenen) {
		t.Fatalf("c: %v, Go: %v", got, beklenen)
	}
	d := al(t, m, "d").(*vm.ListVal)
	ik2 := "iki"
	k2 := g.KodlaCift("bir", &ik2, pc.Secenekler{OzelEklenmesin: true})
	beklenen = []int{}
	for _, x := range k2.Idler {
		beklenen = append(beklenen, int(x))
	}
	if got := tamlar(t, d.Elements[1]); !reflect.DeepEqual(got, beklenen) {
		t.Fatalf("d[1]: %v, Go: %v", got, beklenen)
	}
}

// Tensör (ör. maks_indeks sonucu ya da okuyucunun girdi tensörü) doğrudan çözülebilir.
func TestCozTensor(t *testing.T) {
	m := mustCalistir(t, `
değişken p = parçalayıcı_yükle("`+bpeYolu+`")
değişken k = p.kodla("bir iki")
değişken tek = p.çöz(tensör(k))
değişken iki = p.çöz(tensör([k, k]))
`)
	if got := al(t, m, "tek").Inspect(); got != "bir iki" {
		t.Fatalf("tek: %q", got)
	}
	if got := al(t, m, "iki").Inspect(); got != `["bir iki", "bir iki"]` {
		t.Fatalf("iki: %s", got)
	}
}

// riskliParcalayici bayt düzeyi BPE dosyasının ön bölme desenini, kabul edilen ama bazı
// girdilerde aşırı geri izleyen bir desenle değiştirip geçici bir dosyaya yazar.
func riskliParcalayici(t *testing.T) string {
	t.Helper()
	veri, err := os.ReadFile(bpeYolu)
	if err != nil {
		t.Fatal(err)
	}
	var ham map[string]any
	if err := json.Unmarshal(veri, &ham); err != nil {
		t.Fatal(err)
	}
	pt := ham["pre_tokenizer"].(map[string]any)["pretokenizers"].([]any)
	pt[0].(map[string]any)["pattern"] = map[string]any{"Regex": `(a|a)*b|\s+|\S`}
	yeni, _ := json.Marshal(ham)
	yol := filepath.Join(t.TempDir(), "riskli.json")
	if err := os.WriteFile(yol, yeni, 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(yol)
}

// stderrYakala f çalışırken standart hataya yazılanları döndürür.
func stderrYakala(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	eski := os.Stderr
	os.Stderr = w
	bitti := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		bitti <- string(b)
	}()
	f()
	os.Stderr = eski
	w.Close()
	return <-bitti
}

// Düzenli ifade adım sınırı aşılırsa yarım sonuç yerine yol gösterici bir hata oluşur;
// riskli desen uyarısı standart hataya dosya başına bir kez yazılır.
func TestAdimSiniriVeUyari(t *testing.T) {
	yol := riskliParcalayici(t)
	kotu := strings.Repeat("a", 40)
	var m *vm.VM
	uyari := stderrYakala(t, func() {
		m = mustCalistir(t, `
değişken p = parçalayıcı_yükle("`+yol+`")
değişken p2 = parçalayıcı_yükle("`+yol+`")
değişken iyi = p.kodla("merhaba dünya")
değişken h1 = ""
dene { p.kodla("`+kotu+`") } yakala e { h1 = e }
değişken h2 = ""
dene { p.kodla(["iyi", "`+kotu+`"]) } yakala e { h2 = e }
`)
	})
	if strings.Count(uyari, "Uyarı: parçalayıcı") != 1 || !strings.Contains(uyari, "riskli desen") {
		t.Fatalf("uyarı bir kez yazılmalıydı:\n%s", uyari)
	}
	for _, ad := range []string{"h1", "h2"} {
		h := al(t, m, ad).Inspect()
		if !strings.Contains(h, "adım sınırını") || !strings.Contains(h, "daha küçük parçalara") {
			t.Fatalf("%s: adım sınırı hatası bekleniyordu: %q", ad, h)
		}
	}
	if !strings.Contains(al(t, m, "h2").Inspect(), "listenin 2. metni") {
		t.Fatalf("toplu hatada metnin sırası belirtilmeli: %s", al(t, m, "h2").Inspect())
	}
}

func TestYuklemeHatalari(t *testing.T) {
	d := t.TempDir()
	yaz := func(ad, icerik string) string {
		y := filepath.Join(d, ad)
		if err := os.WriteFile(y, []byte(icerik), 0o644); err != nil {
			t.Fatal(err)
		}
		return filepath.ToSlash(y)
	}
	desteksiz := yaz("desteksiz.json", `{"model": {"type": "Bilinmeyen", "vocab": {}}}`)
	riskli := yaz("riskli.json", `{"pre_tokenizer": {"type": "Split", "pattern": {"Regex": "(a+)+"}, "behavior": "Isolated", "invert": false},
		"model": {"type": "WordLevel", "vocab": {"a": 0, "[UNK]": 1}, "unk_token": "[UNK]"}}`)
	bozuk := yaz("bozuk.json", `{"model": `)
	durumlar := []struct{ kod, beklenen string }{
		{`parçalayıcı_yükle("yok_boyle_bir_dosya.json")`, "yüklenemedi: böyle bir dosya ya da klasör yok"},
		{`parçalayıcı_yükle("` + filepath.ToSlash(d) + `")`, "bir klasör"},
		{`parçalayıcı_yükle("` + desteksiz + `")`, "parçalayıcı türü desteklenmiyor"},
		{`parçalayıcı_yükle("` + riskli + `")`, "güvenlik gereği kabul edilmiyor"},
		{`parçalayıcı_yükle("` + bozuk + `")`, "geçerli bir tokenizer.json dosyası değil"},
		{`parçalayıcı_yükle(5)`, "yol bir metin olmalıdır"},
		{`parçalayıcı_yükle("` + bpeYolu + `").kodla(5)`, "metin ya da metin listesi"},
		{`parçalayıcı_yükle("` + bpeYolu + `").kodla(["a", 1])`, "2. eleman Tam"},
		{`parçalayıcı_yükle("` + bpeYolu + `").kodla("a", özel_parçlar: doğru)`, "Bunu mu demek istediniz: 'özel_parçalar'"},
		{`parçalayıcı_yükle("` + bpeYolu + `").kodla("a", özel_parçalar: 1)`, "doğru ya da yanlış"},
		{`parçalayıcı_yükle("` + bpeYolu + `").kodla(["a"], ikinci: ["b", "c"])`, "eşit uzunlukta"},
		{`parçalayıcı_yükle("` + bpeYolu + `").çöz([1, "a"])`, "tam sayı olmalıdır"},
		{`parçalayıcı_yükle("` + bpeYolu + `").çöz(tensör([[[1]]]))`, "1 boyutlu"},
		{`parçalayıcı_yükle("` + bpeYolu + `").çözz([1])`, "Bunu mu demek istediniz: 'çöz'"},
	}
	for _, dd := range durumlar {
		_, err := calistir(t, dd.kod)
		if err == nil || !strings.Contains(err.Error(), dd.beklenen) {
			t.Errorf("%s\n hata: %v\n beklenen: %q", dd.kod, err, dd.beklenen)
		}
	}
}

func TestIzlemeMasasi(t *testing.T) {
	m := mustCalistir(t, `değişken p = parçalayıcı_yükle("`+bpeYolu+`")`)
	iz, ok := al(t, m, "p").(hataayiklama.IzlemeDegeri)
	if !ok {
		t.Fatal("IzlemeDegeri değil")
	}
	if iz.IzlemeEtiketi() != "Parçalayıcı (dağarcık 1005)" {
		t.Fatalf("etiket %q", iz.IzlemeEtiketi())
	}
	cols, rows, err := iz.TabloVerisi()
	if err != nil || len(cols) != 3 || len(rows) == 0 || rows[0][1] != "<|endoftext|>" {
		t.Fatalf("tablo %v %v %v", cols, rows, err)
	}
}
