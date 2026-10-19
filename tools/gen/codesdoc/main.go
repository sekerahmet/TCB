// kod_belgesi, dilin bütün kodlarının açıklamalı başvuru belgesini (docs/kodlar.md) kod
// listesinden üretir: dilden bağımsız tanımlar (internal/lang/codes/tanim.json: tür, tipler, imza,
// ilişkili kodlar, taşınma) ve Türkçe dil dosyası (internal/lang/codes/tr.json: ad, eski adlar,
// açıklama, örnek). Belge elle düzenlenmez; bir kod eklendiğinde ya da değiştiğinde
// yeniden üretilir.
//
// Kullanım (depo kökünden):
//
//	go run ./tools/gen/codesdoc           // dosyayı yeniden yazar
//	go run ./tools/gen/codesdoc -denetle  // güncel değilse 1 koduyla çıkar
//
// Aynı denetim `go test ./tools/gen/...` ile de yapılır.
//
// Katman: araçlar (tools); kullanıcıya gitmez.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// belgeYolu depo köküne göre üretilen dosyadır.
var belgeYolu = filepath.Join("docs", "kodlar.md")

func main() {
	denetle := flag.Bool("denetle", false, "dosyayı yazma; güncel değilse 1 koduyla çık")
	kok := flag.String("kok", ".", "depo kökü")
	flag.Parse()

	yol := filepath.Join(*kok, belgeYolu)
	yeni, err := Uret()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
		os.Exit(1)
	}
	if *denetle {
		eski, err := os.ReadFile(yol)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
			os.Exit(1)
		}
		if !bytes.Equal(bytes.ReplaceAll(eski, []byte("\r\n"), []byte("\n")), yeni) {
			fmt.Fprintf(os.Stderr, "%s güncel değil; go run ./tools/gen/codesdoc ile yeniden üretin\n", yol)
			os.Exit(1)
		}
		fmt.Printf("%s güncel\n", yol)
		return
	}
	if err := os.WriteFile(yol, yeni, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s yazıldı\n", yol)
}

// bolum bir ailenin bir grubudur (grup < 0: bütün aile).
type bolum struct {
	harf   byte
	grup   int
	baslik string
}

// aileler belgenin bölümleridir; sıra kod sırasıdır.
var aileler = []struct {
	harf     byte
	baslik   string
	bolumler []bolum
}{
	{'A', "A — Dilin çekirdeği", []bolum{
		{'A', 0, "Anahtar kelimeler (A0xx)"},
		{'A', 1, "Bağlamsal sözcükler (A1xx)"},
		{'A', 2, "Tip adları (A2xx)"},
	}},
	{'G', "G — Genel yerleşikler", []bolum{
		{'G', 1, "Giriş, çıkış ve dosya (G1xx)"},
		{'G', 2, "Değer, tip ve dönüşüm (G2xx)"},
		{'G', 3, "Liste (G3xx)"},
		{'G', 4, "Matematik ve istatistik (G4xx)"},
		{'G', 5, "Zaman ve eşzamanlılık (G5xx)"},
		{'G', 6, "Veri biçimleri (G6xx)"},
		{'G', 7, "Test (G7xx)"},
	}},
	{'T', "T — Tensör ve yapay zekâ", []bolum{
		{'T', 1, "Tensör oluşturma ve görünüm (T1xx)"},
		{'T', 2, "Etkinleştirme ve eleman bazlı matematik (T2xx)"},
		{'T', 3, "Türev (T3xx)"},
		{'T', 4, "İyileştiriciler (T4xx)"},
		{'T', 5, "Katmanlar, kayıplar, hazır model (T5xx)"},
		{'T', 6, "Vektör yardımcıları ve dil modeli (T6xx)"},
		{'T', 7, "Kaydetme, yükleme, veri türleri (T7xx)"},
		{'T', 9, "Adlı argümanlar (T9xx)"},
	}},
	{'V', "V — Veri ve parçalayıcı", []bolum{
		{'V', 1, "Veri kümesi (V1xx)"},
		{'V', 2, "Parçalayıcı (V2xx)"},
		{'V', 9, "Adlı argümanlar (V9xx)"},
	}},
	{'W', "W — Web, veritabanı, arayüz (kaldırıldı)", []bolum{
		{'W', 1, "Web istemcisi ve sunucusu (W1xx)"},
		{'W', 2, "Veritabanı (W2xx)"},
		{'W', 3, "Görsel arayüz (W3xx)"},
		{'W', 9, "Adlı argümanlar (W9xx)"},
	}},
	{'Y', "Y — Metot ve özellikler (x.ad)", []bolum{
		{'Y', 0, "Birden çok tipte ortak (Y0xx)"},
		{'Y', 1, "Metin (Y1xx)"},
		{'Y', 2, "Liste (Y2xx)"},
		{'Y', 3, "Sözlük (Y3xx)"},
		{'Y', 4, "Tensör (Y4xx)"},
		{'Y', 5, "İyileştirici (Y5xx)"},
		{'Y', 6, "Veri kümesi, okuyucu, parçalayıcı (Y6xx)"},
		{'Y', 7, "Veritabanı (Y7xx)"},
		{'Y', 8, "Web sunucusu (Y8xx)"},
		{'Y', 9, "Görsel arayüz (Y9xx)"},
	}},
}

// tipAdi bir tip ifadesini adlarla yazar: "Tam | Ondalık".
func tipAdi(t codes.TipIfadesi) string {
	if t.Bos() {
		return "Herhangi"
	}
	parca := make([]string, len(t.Tipler))
	for i, k := range t.Tipler {
		parca[i] = codes.Ad(k)
	}
	return strings.Join(parca, " | ")
}

// varsayilan bir adlı argümanın varsayılanını TR yazımıyla verir.
func varsayilan(v any) string {
	switch x := v.(type) {
	case nil:
		return codes.Ad(codes.A042) // yok
	case bool:
		if x {
			return codes.Ad(codes.A040)
		}
		return codes.Ad(codes.A041)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case string:
		return strconv.Quote(x)
	}
	return fmt.Sprint(v)
}

// argumanlar imzanın parantez içini yazar: "Tam, [Tam], adlı: Tip = varsayılan".
func argumanlar(im codes.Imza) string {
	var parca []string
	for _, p := range im.Param {
		s := tipAdi(p)
		switch {
		case p.Istege:
			s = "[" + s + "]"
		case p.Cok:
			s = "[" + s + "...]"
		case p.EnAzBir:
			s = s + "..."
		}
		parca = append(parca, s)
	}
	for _, a := range im.Adli {
		s := codes.Ad(a.Kod) + ": " + tipAdi(a.Tip)
		if a.VarsayilanVar {
			s += " = " + varsayilan(a.Varsayilan)
		}
		parca = append(parca, s)
	}
	return strings.Join(parca, ", ")
}

func donus(im codes.Imza) string {
	if im.Donus.Bos() {
		return ""
	}
	return " → " + tipAdi(im.Donus)
}

// imzaSatirlari bir kodun imzasını adlarla yazar (üyelerde tip başına bir satır).
func imzaSatirlari(t codes.Tanim) []string {
	ad := codes.Ad(t.Kod)
	switch t.Tur {
	case codes.TurFonksiyon:
		return []string{ad + "(" + argumanlar(t.Imza) + ")" + donus(t.Imza)}
	case codes.TurSabit:
		return []string{ad + donus(t.Imza)}
	case codes.TurMetot, codes.TurOzellik:
		var out []string
		for _, u := range t.Uyeler {
			s := codes.Ad(u.Tip) + "." + ad
			if t.Tur == codes.TurMetot {
				s += "(" + argumanlar(u.Imza) + ")"
			}
			out = append(out, s+donus(u.Imza))
		}
		return out
	}
	return nil
}

// kodAdi "G400 `toplam`" biçimidir.
func kodAdi(k codes.Kod) string { return fmt.Sprintf("%s `%s`", k, codes.Ad(k)) }

// tasinmisBicim taşınan bir kodun yeni yazımını verir: x.uzunluk, liste.filtrele(...), toplam(...).
func tasinmisBicim(y codes.Kod) string {
	if y.Alan() == codes.AlanUye {
		return codes.UyeYazimi(y)
	}
	return codes.Ad(y) + "(...)"
}

// Uret belgenin içeriğini üretir.
func Uret() ([]byte, error) {
	if hatalar := codes.TanimDenetle(codes.Etkin()); len(hatalar) > 0 {
		return nil, fmt.Errorf("tanımlar tutarsız: %v", hatalar[0])
	}
	d := codes.Etkin()
	tumu := append(d.Kodlar(), d.Olular()...)
	sort.Slice(tumu, func(i, j int) bool { return tumu[i] < tumu[j] })

	// Ters bağlar: adlı argümanı kullananlar, tipin üyeleri.
	kullananlar := map[codes.Kod][]codes.Kod{}
	uyeleri := map[codes.Kod][]codes.Kod{}
	for _, k := range d.Kodlar() {
		t, _ := codes.Tanimi(k)
		imzalar := []codes.Imza{t.Imza}
		for _, u := range t.Uyeler {
			imzalar = append(imzalar, u.Imza)
			uyeleri[u.Tip] = append(uyeleri[u.Tip], k)
		}
		gorulen := map[codes.Kod]bool{}
		for _, im := range imzalar {
			for _, a := range im.Adli {
				if !gorulen[a.Kod] {
					gorulen[a.Kod] = true
					kullananlar[a.Kod] = append(kullananlar[a.Kod], k)
				}
			}
		}
	}
	listele := func(ks []codes.Kod) string {
		parca := make([]string, len(ks))
		for i, k := range ks {
			parca[i] = kodAdi(k)
		}
		return strings.Join(parca, ", ")
	}

	var b bytes.Buffer
	b.WriteString("# TR kod listesi\n\n")
	b.WriteString("<!-- Bu dosya internal/lang/codes/tanim.json ve internal/lang/codes/tr.json'dan üretilir; elle düzenlemeyin: go run ./tools/gen/codesdoc -->\n\n")
	b.WriteString("Dilin kullanıcıya görünen her adı sabit bir **koda** bağlıdır (bkz. `docs/spec/17_kodlar.md` §17). ")
	b.WriteString("Bu belge her kodun ne işe yaradığını, imzasını ve örneğini verir; \"hangi kod ne işe yarar\" sorusunun tek başvurusudur. ")
	b.WriteString("Yeni bir ad gerekmeden önce burada aynı işi yapan bir kod olup olmadığına bakın.\n\n")
	b.WriteString("İmzalarda parametrelerin yalnızca tipleri yazılır: `[Tip]` isteğe bağlı, `[Tip...]` sıfır ya da çok, `Tip...` bir ya da çok argümandır; ")
	b.WriteString("`ad: Tip = varsayılan` adlı argümandır, `→` dönüş tipidir. Metot ve özellikler `Tip.ad` biçiminde, geçerli oldukları her tip için ayrı satırda yazılır.\n\n")
	b.WriteString("**Değer dönüştüren ya da değere soru soran iş metottur** (`liste.sırala()`, `metin.uzunluk`); **matematik ve tensör işi fonksiyondur** (`toplam(liste)`, `relu(x)`). ")
	b.WriteString("Kaldırılan kodların numarası bir daha kullanılmaz; adları yazılınca hata yeni biçimi önerir.\n\n")
	sayilar := map[codes.Tur]int{}
	for _, k := range d.Kodlar() {
		t, _ := codes.Tanimi(k)
		sayilar[t.Tur]++
	}
	fmt.Fprintf(&b, "Toplam %d kod: %d anahtar kelime, %d bağlamsal sözcük, %d tip, %d fonksiyon, %d sabit, %d metot, %d özellik, %d adlı argüman; ayrıca %d taşınmış ve %d kaldırılmış kod (numaraları bir daha kullanılmaz).\n\n",
		len(d.Kodlar()), sayilar[codes.TurAnahtarKelime], sayilar[codes.TurBaglamsal], sayilar[codes.TurTip], sayilar[codes.TurFonksiyon],
		sayilar[codes.TurSabit], sayilar[codes.TurMetot], sayilar[codes.TurOzellik], sayilar[codes.TurAdliArguman], len(d.Tasinanlar()), len(d.Kaldirilanlar()))
	b.WriteString("## İçindekiler\n\n")
	for _, a := range aileler {
		fmt.Fprintf(&b, "- %s: ", a.baslik)
		var parca []string
		for _, bl := range a.bolumler {
			parca = append(parca, bl.baslik)
		}
		b.WriteString(strings.Join(parca, " · ") + "\n")
	}
	b.WriteString("- Boru hattıyla kullanılabilen fonksiyonlar\n\n")

	for _, a := range aileler {
		fmt.Fprintf(&b, "## %s\n\n", a.baslik)
		for _, bl := range a.bolumler {
			var ks []codes.Kod
			for _, k := range tumu {
				if k.Harf() == bl.harf && k.Grup() == bl.grup {
					ks = append(ks, k)
				}
			}
			if len(ks) == 0 {
				continue
			}
			fmt.Fprintf(&b, "### %s\n\n", bl.baslik)
			for _, k := range ks {
				g, _ := d.Girdisi(k)
				t, _ := codes.Tanimi(k)
				if g.Kaldirildi {
					fmt.Fprintf(&b, "#### %s `%s` — kaldırıldı\n\n%s\n\n", k, g.Ad, g.Aciklama)
					continue
				}
				if g.Tasindi != codes.Yok {
					fmt.Fprintf(&b, "#### %s `%s` — kaldırıldı\n\n", k, g.Ad)
					fmt.Fprintf(&b, "Taşındı: %s → `%s`. %s\n\n", kodAdi(g.Tasindi), tasinmisBicim(g.Tasindi), g.Aciklama)
					continue
				}
				fmt.Fprintf(&b, "#### %s `%s`\n\n", k, g.Ad)
				if im := imzaSatirlari(t); len(im) > 0 {
					for _, s := range im {
						fmt.Fprintf(&b, "    %s\n", s)
					}
					b.WriteString("\n")
				}
				b.WriteString(g.Aciklama + "\n\n")
				if g.Ornek != "" {
					fmt.Fprintf(&b, "```tr\n%s\n```\n\n", g.Ornek)
				}
				var ek []string
				if t.Tur == codes.TurAdliArguman && len(kullananlar[k]) > 0 {
					ek = append(ek, "Kullananlar: "+listele(kullananlar[k]))
				}
				if t.Tur == codes.TurTip && len(uyeleri[k]) > 0 {
					ek = append(ek, "Üyeleri: "+listele(uyeleri[k]))
				}
				if len(t.Iliskili) > 0 {
					ek = append(ek, "İlişkili: "+listele(t.Iliskili))
				}
				var eskiler, onceki []string
				for _, e := range g.Eski {
					eskiler = append(eskiler, "`"+e+"`")
				}
				for _, tk := range d.Tasinanlar() {
					if tg, _ := d.Girdisi(tk); tg.Tasindi == k {
						bicim := tg.Ad + "(...)"
						if tk.Alan() == codes.AlanUye {
							bicim = "x." + tg.Ad + "()"
						}
						onceki = append(onceki, fmt.Sprintf("`%s` (%s, kaldırıldı)", bicim, tk))
					}
				}
				if len(eskiler) > 0 {
					ek = append(ek, "Eski adlar: "+strings.Join(eskiler, ", "))
				}
				if len(onceki) > 0 {
					ek = append(ek, "Önceki biçim: "+strings.Join(onceki, ", "))
				}
				if len(ek) > 0 {
					b.WriteString(strings.Join(ek, " · ") + "\n\n")
				}
			}
		}
	}
	// Boru hattı (x |> f, docs/spec/04_operatorler.md 4.9): ilk konumlu parametresi borudaki değeri alan
	// fonksiyonlar. Metotlar boruyla değil '.' ile zincirlenir.
	b.WriteString("## Boru hattıyla kullanılabilen fonksiyonlar\n\n")
	b.WriteString("`x |> f(a)` ≡ `f(x, a)` (bkz. `docs/spec/04_operatorler.md` 4.9). Boru yalnızca fonksiyonlarla çalışır; metotlar `.` ile zincirlenir (`l.filtrele { o > 1 }.haritala { o * 2 } |> toplam`). ")
	b.WriteString("Borudaki değeri ilk konumlu argüman olarak alabilen yerleşik fonksiyonlar (kullanıcı fonksiyonları da kullanılabilir):\n\n")
	var boru []string
	for _, k := range d.Kodlar() {
		t, _ := codes.Tanimi(k)
		if t.Tur == codes.TurFonksiyon && len(t.Imza.Param) > 0 {
			boru = append(boru, "`"+codes.Ad(k)+"`")
		}
	}
	b.WriteString(strings.Join(boru, ", ") + "\n")
	return append(bytes.TrimRight(b.Bytes(), "\n"), '\n'), nil
}
