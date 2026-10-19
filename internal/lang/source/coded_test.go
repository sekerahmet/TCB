package source

import (
	"errors"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// kodlaCoz Türkçe kaynağın kodlu biçimini verir ve kodlu biçimin çözülünce özgün kaynağa
// döndüğünü sınar.
func kodlaCoz(t *testing.T, tr string) string {
	t.Helper()
	kodlu, err := Encode(tr)
	if err != nil {
		t.Fatalf("%q kodlanamadı: %v", tr, err)
	}
	geri, err := Decode(kodlu, codes.Active())
	if err != nil || geri != tr {
		t.Fatalf("TR → kod → TR aynı değil (%v)\nözgün: %q\nkodlu: %q\ngeri:  %q", err, tr, kodlu, geri)
	}
	return kodlu
}

// Kenar durumlar (17.3, 17.4): beklenen kodlu gövde başlıktan sonraki metindir.
func TestKodlamaKenarDurumlari(t *testing.T) {
	durumlar := []struct{ ad, tr, kodlu string }{
		{"anahtar kelime ve yerleşik", "değişken x = 1\nyazdır(x)\n", "A030 x = 1\nG100(x)\n"},
		{"gölgelenen yerleşik kullanıcının adıdır",
			"fonksiyon f() {\n    değişken toplam = 1\n    sonuç toplam\n}\nyazdır(toplam([1, 2]))\n",
			"A020 f() {\n    A030 toplam = 1\n    A021 toplam\n}\nG100(G400([1, 2]))\n"},
		{"yorum ve ham metin çevrilmez",
			"// eğer yazdır\n/* değişken */ değişken y = `yazdır {eğer}`\n",
			"// eğer yazdır\n/* değişken */ A030 y = `yazdır {eğer}`\n"},
		{"metin içi yerleştirme ve iç içe yerleştirme",
			"yazdır(\"a\\n{toplam([1])} \\{x\\} {\"iç {uzunluk_x(1)} {yazdır}\"}\")\nfonksiyon uzunluk_x(a) { sonuç a }\n",
			"G100(\"a\\n{G400([1])} \\{x\\} {\"iç {uzunluk_x(1)} {G100}\"}\")\nA020 uzunluk_x(a) { A021 a }\n"},
		{"üyeler tanımda ve kullanımda aynı kodla (Y5)",
			"yapı Sepet {\n    uzunluk = 0\n    fonksiyon ekle(x) { kendi.uzunluk += 1 }\n}\ndeğişken s = Sepet(uzunluk: 2)\ns.ekle(1)\n[1].ekle(2)\n",
			"A050 Sepet {\n    Y001 = 0\n    A020 Y004(x) { A051.Y001 += 1 }\n}\nA030 s = Sepet(Y001: 2)\ns.Y004(1)\n[1].Y004(2)\n"},
		{"kurucu bağlamsal sözcüktür",
			"yapı N {\n    a\n    fonksiyon kurucu(a) { kendi.a = a }\n}\n",
			"A050 N {\n    a\n    A020 A102(a) { A051.a = a }\n}\n"},
		{"adlı argüman yalnızca dilin öğesinde",
			"fonksiyon f(eksen = 0) { sonuç eksen }\nyazdır(toplam([1], eksen: 0), f(eksen: 1))\n",
			"A020 f(eksen = 0) { A021 eksen }\nG100(G400([1], T902: 0), f(eksen: 1))\n"},
		{"örtük adlar yalnızca kendi bloklarında",
			"değişken o = 1\nyazdır([1].haritala { o * 2 }, [1].indirge(0) { birikim + o }, (o) => o)\ndene { fırlat \"x\" } yakala { yazdır(hata) }\ndene { fırlat \"y\" } yakala hata { yazdır(hata) }\n",
			"A030 o = 1\nG100([1].Y201 { A104 * 2 }, [1].Y207(0) { A105 + A104 }, (o) => o)\nA060 { A062 \"x\" } A061 { G100(A103) }\nA060 { A062 \"y\" } A061 hata { G100(hata) }\n"},
		{"test ve olarak",
			"aktar \"m\" olarak m\ntest \"t\" { doğrula(doğru) }\ndeğişken test = 1\n",
			"A070 \"m\" A100 m\nA101 \"t\" { G700(A040) }\nA030 test = 1\n"},
		{"tip açıklaması; kabul edilmeyen harf yazımı (3.14) olduğu gibi kalır",
			"değişken a: Tam = 1\ndeğişken b: tam = 2\nyazdır(tip(a) == Tam)\n",
			"A030 a: A200 = 1\nA030 b: tam = 2\nG100(G201(a) == A200)\n"},
		{"Türkçe harfler ve kod biçimine benzeyen adlar",
			"değişken İSİM_ı = \"Iİ\"\ndeğişken g100 = 1\nyazdır(İSİM_ı, g100)\n",
			"A030 İSİM_ı = \"Iİ\"\nA030 g100 = 1\nG100(İSİM_ı, g100)\n"},
		{"kaçış '$' atılır", "değişken $eğer = 3\nyazdır($eğer)\n", "A030 eğer = 3\nG100(eğer)\n"},
	}
	for _, d := range durumlar {
		t.Run(d.ad, func(t *testing.T) {
			kodlu := kodlaCoz(t, d.tr)
			if d.ad == "kaçış '$' atılır" {
				// Kaçış dili sözcüğüyle çakışan ad için gereklidir: çözümde geri yazılır.
				return
			}
			if want := Header + "\n" + d.kodlu; kodlu != want {
				t.Errorf("kodlu biçim:\n%s\nbeklenen:\n%s", kodlu, want)
			}
		})
	}
}

// BOM ve CRLF satır sonları korunur; başlık BOM'dan sonra, gövdenin satır sonu biçimiyle yazılır.
func TestKodlamaBOMVeCRLF(t *testing.T) {
	tr := "\xef\xbb\xbfdeğişken x = 1\r\nyazdır(\"a\r\nb\")\r\n"
	kodlu := kodlaCoz(t, tr)
	if want := "\xef\xbb\xbf" + Header + "\r\nA030 x = 1\r\nG100(\"a\r\nb\")\r\n"; kodlu != want {
		t.Errorf("kodlu: %q, beklenen %q", kodlu, want)
	}
	// Zaten kodlu dosyaya dokunulmaz.
	if yine, err := Encode(kodlu); err != nil || yine != kodlu {
		t.Errorf("kodlu dosya yeniden kodlandı: %q, %v", yine, err)
	}
	// Türkçe dosya çözülmez.
	if ayni, err := Decode(tr, codes.Active()); err != nil || ayni != tr {
		t.Errorf("Türkçe dosya değişti: %q, %v", ayni, err)
	}
}

func TestBaslikKurallari(t *testing.T) {
	for _, m := range []string{Header + "\n", Header, "\xef\xbb\xbf" + Header + "\r\nA030 x = 1"} {
		if b, err := Split(m); err != nil || !b.Coded || b.Join() != m {
			t.Errorf("%q: kodlu okunmalı ve geri birleşmeli (%+v, %v)", m, b, err)
		}
	}
	for _, m := range []string{"//! tr kodlu 2\nA030 x = 1", Header + " \n", "//! tr kodlu\n"} {
		var bh *HeaderError
		if _, err := Split(m); !errors.As(err, &bh) {
			t.Errorf("%q: tanınmayan başlık hatası bekleniyordu, alınan %v", m, err)
		}
		if s := Prepare(m, Options{}); s.Stage != StageSyntax || !strings.Contains(s.Errors[0], "Satır 1, Sütun 1:") {
			t.Errorf("%q: hazırlık başlık hatasını sözdizimi hatası olarak bildirmeli: %+v", m, s.Errors)
		}
	}
	// İlk satır olmayan ya da başka bir '//!' yorumu sıradan yorumdur.
	for _, m := range []string{"\n" + Header + "\n", "//! tr notu\n"} {
		if b, err := Split(m); err != nil || b.Coded {
			t.Errorf("%q: Türkçe kaynak sayılmalı (%+v, %v)", m, b, err)
		}
	}
}

// Kodlu dosya çevrilmeden hazırlanır; konumlar Türkçe gösterime göredir (17.5).
func TestKodluDosyaHazirlanir(t *testing.T) {
	s := Prepare(Header+"\nA030 x = 1\nG100(x, tanımsız)\n", Options{TypeCheck: true})
	if s.Stage != StageType || len(s.Errors) != 1 || !strings.HasPrefix(s.Errors[0], "Satır 2, Sütun 11:") {
		t.Fatalf("tanımsız ad Türkçe gösterimdeki konumla bildirilmeli: %v", s.Errors)
	}
	if s.Text != "değişken x = 1\nyazdır(x, tanımsız)\n" {
		t.Errorf("hata çerçevesinin metni Türkçe gösterim olmalı: %q", s.Text)
	}
	// Yerleştirmedeki kodlar da sütunu kaydırır.
	s = Prepare(Header+"\nG100(\"{G400([1])} {yok_ad}\")\n", Options{TypeCheck: true})
	if len(s.Errors) != 1 || !strings.HasPrefix(s.Errors[0], "Satır 1, Sütun 24:") {
		t.Errorf("yerleştirmedeki ad Türkçe gösterimdeki konumla bildirilmeli: %v", s.Errors)
	}
	// Kod olmayan her ad kullanıcının adıdır, anahtar kelime yazımında bile.
	s = Prepare(Header+"\nA030 eğer = 1\nG100(eğer)\n", Options{TypeCheck: true})
	if s.Stage != "" {
		t.Errorf("kodlu dosyada 'eğer' bir kullanıcı adıdır: %v", s.Errors)
	}
}

func TestKodluDosyaHatalari(t *testing.T) {
	durumlar := map[string]string{
		Header + "\nG999(1)\n":            "'G999' bilinen bir kod değil",
		Header + "\nA030 x = G200([1])\n": "'uzunluk(...)' TR'de bir fonksiyon değil",
	}
	for kaynak, parca := range durumlar {
		s := Prepare(kaynak, Options{TypeCheck: true})
		if !strings.Contains(strings.Join(s.Errors, "\n"), parca) {
			t.Errorf("%q: %q içeren hata bekleniyordu: %v", kaynak, parca, s.Errors)
		}
	}
	// Türkçe yazılmış dosyada kod biçimli ad hatadır ve başlığı gösterir (2.3a).
	s := Prepare("G100(1)\n", Options{})
	if s.Stage != StageSyntax || !strings.Contains(s.Errors[0], Header) {
		t.Errorf("kod biçimli ad hatası başlığı göstermeli: %v", s.Errors)
	}
	// Ayrıştırılamayan kaynak çevrilmez.
	var sh *SyntaxError
	if _, err := Encode("eğer {\n"); !errors.As(err, &sh) {
		t.Errorf("sözdizimi hatası bekleniyordu: %v", err)
	}
}

// sahteDil Türkçe dil dosyasındaki bazı adları değiştirerek ikinci bir yazım dili kurar:
// kaçışın (2.11) başka bir dilde nasıl çalıştığını sınamak için.
func sahteDil(t *testing.T) *codes.Language {
	t.Helper()
	veri := string(codes.TRData())
	for eski, yeni := range map[string]string{
		`"A030": { "ad": "değişken"`: `"A030": { "ad": "tanım"`,
		`"G100": { "ad": "yazdır"`:   `"G100": { "ad": "sayaç"`,
		`"A001": { "ad": "eğer"`:     `"A001": { "ad": "kutu"`,
		`"Y004": { "ad": "ekle"`:     `"Y004": { "ad": "koy"`,
		`"A104": { "ad": "o"`:        `"A104": { "ad": "bu"`,
	} {
		if !strings.Contains(veri, eski) {
			t.Fatalf("dil dosyasında %q bulunamadı", eski)
		}
		veri = strings.Replace(veri, eski, yeni, 1)
	}
	d, err := codes.Load("xx", []byte(veri))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// Hedef dilde bir kullanıcı adı dilin sözcüğüyle çakışırsa '$ad' yazılır (2.11).
func TestKacisBaskaDilde(t *testing.T) {
	d := sahteDil(t)
	tr := "değişken kutu = 1\ndeğişken sayaç = 2\nyazdır(kutu, sayaç)\nyapı K {\n    fonksiyon koy() { sonuç 1 }\n}\ndeğişken bu = 3\nyazdır([1].haritala { o + bu })\n"
	kodlu := kodlaCoz(t, tr)
	got, err := Decode(kodlu, d)
	if err != nil {
		t.Fatal(err)
	}
	want := "tanım $kutu = 1\ntanım $sayaç = 2\nsayaç($kutu, $sayaç)\nyapı K {\n    fonksiyon $koy() { sonuç 1 }\n}\ntanım bu = 3\nsayaç([1].haritala { bu + $bu })\n"
	if got != want {
		t.Errorf("sahte dilde gösterim:\n%s\nbeklenen:\n%s", got, want)
	}
	// Türkçede çakışma yok: kaçış yazılmaz.
	if tr2, _ := Decode(kodlu, codes.Active()); strings.Contains(tr2, "$") {
		t.Errorf("Türkçe gösterimde gereksiz kaçış: %s", tr2)
	}
}
