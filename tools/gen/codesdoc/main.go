// codesdoc, dilin bütün kodlarının açıklamalı başvuru belgesini (docs/codes.md) kod
// listesinden üretir: dilden bağımsız tanımlar (internal/lang/codes/definitions.json: tür, tipler, imza,
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
// Layer: tools (tools); kullanıcıya gitmez.
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

// docPath depo köküne göre üretilen dosyadır.
var docPath = filepath.Join("docs", "codes.md")

func main() {
	check := flag.Bool("denetle", false, "dosyayı yazma; güncel değilse 1 koduyla çık")
	root := flag.String("kok", ".", "depo kökü")
	flag.Parse()

	path := filepath.Join(*root, docPath)
	fresh, err := Generate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
		os.Exit(1)
	}
	if *check {
		old, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
			os.Exit(1)
		}
		if !bytes.Equal(bytes.ReplaceAll(old, []byte("\r\n"), []byte("\n")), fresh) {
			fmt.Fprintf(os.Stderr, "%s güncel değil; go run ./tools/gen/codesdoc ile yeniden üretin\n", path)
			os.Exit(1)
		}
		fmt.Printf("%s güncel\n", path)
		return
	}
	if err := os.WriteFile(path, fresh, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s yazıldı\n", path)
}

// section bir ailenin bir grubudur (group < 0: bütün aile).
type section struct {
	letter byte
	group  int
	title  string
}

// families belgenin bölümleridir; sıra kod sırasıdır.
var families = []struct {
	letter   byte
	title    string
	sections []section
}{
	{'A', "A — Dilin çekirdeği", []section{
		{'A', 0, "Anahtar kelimeler (A0xx)"},
		{'A', 1, "Bağlamsal sözcükler (A1xx)"},
		{'A', 2, "Tip adları (A2xx)"},
	}},
	{'G', "G — Genel yerleşikler", []section{
		{'G', 1, "Giriş, çıkış ve dosya (G1xx)"},
		{'G', 2, "Değer, tip ve dönüşüm (G2xx)"},
		{'G', 3, "Liste (G3xx)"},
		{'G', 4, "Matematik ve istatistik (G4xx)"},
		{'G', 5, "Zaman ve eşzamanlılık (G5xx)"},
		{'G', 6, "Veri biçimleri (G6xx)"},
		{'G', 7, "Test (G7xx)"},
	}},
	{'K', "K — Komutlar (tr, kabuk, hata ayıklayıcı)", []section{
		{'K', 0, "tr alt komutları (K0xx)"},
		{'K', 1, "Komut satırı seçenekleri (K1xx)"},
		{'K', 2, "Kabuk komutları (K2xx)"},
		{'K', 3, "Hata ayıklayıcı komutları (K3xx)"},
	}},
	{'T', "T — Tensör ve yapay zekâ", []section{
		{'T', 1, "Tensör oluşturma ve görünüm (T1xx)"},
		{'T', 2, "Etkinleştirme ve eleman bazlı matematik (T2xx)"},
		{'T', 3, "Türev (T3xx)"},
		{'T', 4, "İyileştiriciler (T4xx)"},
		{'T', 5, "Katmanlar, kayıplar, hazır model (T5xx)"},
		{'T', 6, "Vektör yardımcıları ve dil modeli (T6xx)"},
		{'T', 7, "Kaydetme, yükleme, veri türleri (T7xx)"},
		{'T', 9, "Adlı argümanlar (T9xx)"},
	}},
	{'V', "V — Veri ve parçalayıcı", []section{
		{'V', 1, "Veri kümesi (V1xx)"},
		{'V', 2, "Parçalayıcı (V2xx)"},
		{'V', 9, "Adlı argümanlar (V9xx)"},
	}},
	{'W', "W — Web, veritabanı, arayüz (kaldırıldı)", []section{
		{'W', 1, "Web istemcisi ve sunucusu (W1xx)"},
		{'W', 2, "Veritabanı (W2xx)"},
		{'W', 3, "Görsel arayüz (W3xx)"},
		{'W', 9, "Adlı argümanlar (W9xx)"},
	}},
	{'Y', "Y — Metot ve özellikler (x.ad)", []section{
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

// typeName bir tip ifadesini adlarla yazar: "Tam | Ondalık".
func typeName(t codes.TypeExpr) string {
	if t.Empty() {
		return "Herhangi"
	}
	parts := make([]string, len(t.Types))
	for i, k := range t.Types {
		parts[i] = codes.Name(k)
	}
	return strings.Join(parts, " | ")
}

// defaultValue bir adlı argümanın varsayılanını TR yazımıyla verir.
func defaultValue(v any) string {
	switch x := v.(type) {
	case nil:
		return codes.Name(codes.A042) // yok
	case bool:
		if x {
			return codes.Name(codes.A040)
		}
		return codes.Name(codes.A041)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case string:
		return strconv.Quote(x)
	}
	return fmt.Sprint(v)
}

// arguments imzanın parantez içini yazar: "Tam, [Tam], adlı: Tip = varsayılan".
func arguments(im codes.Signature) string {
	var parts []string
	for _, p := range im.Params {
		s := typeName(p)
		switch {
		case p.Optional:
			s = "[" + s + "]"
		case p.Many:
			s = "[" + s + "...]"
		case p.AtLeastOne:
			s = s + "..."
		}
		parts = append(parts, s)
	}
	for _, a := range im.Named {
		s := codes.Name(a.Code) + ": " + typeName(a.Type)
		if a.HasDefault {
			s += " = " + defaultValue(a.Default)
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

func returns(im codes.Signature) string {
	if im.Return.Empty() {
		return ""
	}
	return " → " + typeName(im.Return)
}

// signatureLines bir kodun imzasını adlarla yazar (üyelerde tip başına bir satır).
func signatureLines(t codes.Definition) []string {
	name := codes.Name(t.Code)
	switch t.Kind {
	case codes.KindFunction:
		return []string{name + "(" + arguments(t.Signature) + ")" + returns(t.Signature)}
	case codes.KindConstant:
		return []string{name + returns(t.Signature)}
	case codes.KindMethod, codes.KindProperty:
		var out []string
		for _, u := range t.Members {
			s := codes.Name(u.Type) + "." + name
			if t.Kind == codes.KindMethod {
				s += "(" + arguments(u.Signature) + ")"
			}
			out = append(out, s+returns(u.Signature))
		}
		return out
	}
	return nil
}

// codeLabel "G400 `toplam`" biçimidir.
func codeLabel(k codes.Code) string { return fmt.Sprintf("%s `%s`", k, codes.Name(k)) }

// movedForm taşınan bir kodun yeni yazımını verir: x.uzunluk, liste.filtrele(...), toplam(...).
func movedForm(y codes.Code) string {
	if y.Namespace() == codes.NamespaceMember {
		return codes.MemberSpelling(y)
	}
	return codes.Name(y) + "(...)"
}

// Generate belgenin içeriğini üretir.
func Generate() ([]byte, error) {
	if hatalar := codes.CheckDefinitions(codes.Active()); len(hatalar) > 0 {
		return nil, fmt.Errorf("tanımlar tutarsız: %v", hatalar[0])
	}
	d := codes.Active()
	allCodes := append(d.Codes(), d.DeadCodes()...)
	sort.Slice(allCodes, func(i, j int) bool { return allCodes[i] < allCodes[j] })

	// Ters bağlar: adlı argümanı kullananlar, tipin üyeleri.
	usedBy := map[codes.Code][]codes.Code{}
	typeMembers := map[codes.Code][]codes.Code{}
	for _, k := range d.Codes() {
		t, _ := codes.DefinitionOf(k)
		signatures := []codes.Signature{t.Signature}
		for _, u := range t.Members {
			signatures = append(signatures, u.Signature)
			typeMembers[u.Type] = append(typeMembers[u.Type], k)
		}
		seen := map[codes.Code]bool{}
		for _, im := range signatures {
			for _, a := range im.Named {
				if !seen[a.Code] {
					seen[a.Code] = true
					usedBy[a.Code] = append(usedBy[a.Code], k)
				}
			}
		}
	}
	list := func(ks []codes.Code) string {
		parts := make([]string, len(ks))
		for i, k := range ks {
			parts[i] = codeLabel(k)
		}
		return strings.Join(parts, ", ")
	}

	var b bytes.Buffer
	b.WriteString("# TR kod listesi\n\n")
	b.WriteString("<!-- Bu dosya internal/lang/codes/definitions.json ve internal/lang/codes/tr.json'dan üretilir; elle düzenlemeyin: go run ./tools/gen/codesdoc -->\n\n")
	b.WriteString("Dilin kullanıcıya görünen her adı sabit bir **koda** bağlıdır (bkz. `docs/spec/17_kodlar.md` §17). ")
	b.WriteString("Bu belge her kodun ne işe yaradığını, imzasını ve örneğini verir; \"hangi kod ne işe yarar\" sorusunun tek başvurusudur. ")
	b.WriteString("Yeni bir ad gerekmeden önce burada aynı işi yapan bir kod olup olmadığına bakın.\n\n")
	b.WriteString("İmzalarda parametrelerin yalnızca tipleri yazılır: `[Tip]` isteğe bağlı, `[Tip...]` sıfır ya da çok, `Tip...` bir ya da çok argümandır; ")
	b.WriteString("`ad: Tip = varsayılan` adlı argümandır, `→` dönüş tipidir. Metot ve özellikler `Tip.ad` biçiminde, geçerli oldukları her tip için ayrı satırda yazılır.\n\n")
	b.WriteString("**Değer dönüştüren ya da değere soru soran iş metottur** (`liste.sırala()`, `metin.uzunluk`); **matematik ve tensör işi fonksiyondur** (`toplam(liste)`, `relu(x)`). ")
	b.WriteString("Kaldırılan kodların numarası bir daha kullanılmaz; adları yazılınca hata yeni biçimi önerir.\n\n")
	counts := map[codes.Kind]int{}
	for _, k := range d.Codes() {
		t, _ := codes.DefinitionOf(k)
		counts[t.Kind]++
	}
	fmt.Fprintf(&b, "Toplam %d kod: %d anahtar kelime, %d bağlamsal sözcük, %d tip, %d fonksiyon, %d sabit, %d metot, %d özellik, %d adlı argüman; ayrıca %d taşınmış ve %d kaldırılmış kod (numaraları bir daha kullanılmaz).\n\n",
		len(d.Codes()), counts[codes.KindKeyword], counts[codes.KindContextual], counts[codes.KindType], counts[codes.KindFunction],
		counts[codes.KindConstant], counts[codes.KindMethod], counts[codes.KindProperty], counts[codes.KindNamedArg], len(d.MovedCodes()), len(d.RemovedCodes()))
	b.WriteString("## İçindekiler\n\n")
	for _, a := range families {
		fmt.Fprintf(&b, "- %s: ", a.title)
		var parts []string
		for _, sec := range a.sections {
			parts = append(parts, sec.title)
		}
		b.WriteString(strings.Join(parts, " · ") + "\n")
	}
	b.WriteString("- Boru hattıyla kullanılabilen fonksiyonlar\n\n")

	for _, a := range families {
		fmt.Fprintf(&b, "## %s\n\n", a.title)
		for _, sec := range a.sections {
			var ks []codes.Code
			for _, k := range allCodes {
				if k.Letter() == sec.letter && k.Group() == sec.group {
					ks = append(ks, k)
				}
			}
			if len(ks) == 0 {
				continue
			}
			fmt.Fprintf(&b, "### %s\n\n", sec.title)
			for _, k := range ks {
				g, _ := d.Entry(k)
				t, _ := codes.DefinitionOf(k)
				if g.Removed {
					fmt.Fprintf(&b, "#### %s `%s` — kaldırıldı\n\n%s\n\n", k, g.Name, g.Description)
					continue
				}
				if g.MovedTo != codes.None {
					fmt.Fprintf(&b, "#### %s `%s` — kaldırıldı\n\n", k, g.Name)
					fmt.Fprintf(&b, "Taşındı: %s → `%s`. %s\n\n", codeLabel(g.MovedTo), movedForm(g.MovedTo), g.Description)
					continue
				}
				fmt.Fprintf(&b, "#### %s `%s`\n\n", k, g.Name)
				if im := signatureLines(t); len(im) > 0 {
					for _, s := range im {
						fmt.Fprintf(&b, "    %s\n", s)
					}
					b.WriteString("\n")
				}
				b.WriteString(g.Description + "\n\n")
				if g.Example != "" {
					fmt.Fprintf(&b, "```tr\n%s\n```\n\n", g.Example)
				}
				var extras []string
				if t.Kind == codes.KindNamedArg && len(usedBy[k]) > 0 {
					extras = append(extras, "Kullananlar: "+list(usedBy[k]))
				}
				if t.Kind == codes.KindType && len(typeMembers[k]) > 0 {
					extras = append(extras, "Üyeleri: "+list(typeMembers[k]))
				}
				if len(t.Related) > 0 {
					extras = append(extras, "İlişkili: "+list(t.Related))
				}
				var oldNames, previous []string
				for _, e := range g.Old {
					oldNames = append(oldNames, "`"+e+"`")
				}
				for _, tk := range d.MovedCodes() {
					if tg, _ := d.Entry(tk); tg.MovedTo == k {
						form := tg.Name + "(...)"
						if tk.Namespace() == codes.NamespaceMember {
							form = "x." + tg.Name + "()"
						}
						previous = append(previous, fmt.Sprintf("`%s` (%s, kaldırıldı)", form, tk))
					}
				}
				if len(oldNames) > 0 {
					extras = append(extras, "Eski adlar: "+strings.Join(oldNames, ", "))
				}
				if len(previous) > 0 {
					extras = append(extras, "Önceki biçim: "+strings.Join(previous, ", "))
				}
				if len(extras) > 0 {
					b.WriteString(strings.Join(extras, " · ") + "\n\n")
				}
			}
		}
	}
	// Boru hattı (x |> f, docs/spec/04_operatorler.md 4.9): ilk konumlu parametresi borudaki değeri alan
	// fonksiyonlar. Metotlar boruyla değil '.' ile zincirlenir.
	b.WriteString("## Boru hattıyla kullanılabilen fonksiyonlar\n\n")
	b.WriteString("`x |> f(a)` ≡ `f(x, a)` (bkz. `docs/spec/04_operatorler.md` 4.9). Boru yalnızca fonksiyonlarla çalışır; metotlar `.` ile zincirlenir (`l.filtrele { o > 1 }.haritala { o * 2 } |> toplam`). ")
	b.WriteString("Borudaki değeri ilk konumlu argüman olarak alabilen yerleşik fonksiyonlar (kullanıcı fonksiyonları da kullanılabilir):\n\n")
	var pipeable []string
	for _, k := range d.Codes() {
		t, _ := codes.DefinitionOf(k)
		if t.Kind == codes.KindFunction && len(t.Signature.Params) > 0 {
			pipeable = append(pipeable, "`"+codes.Name(k)+"`")
		}
	}
	b.WriteString(strings.Join(pipeable, ", ") + "\n")
	return append(bytes.TrimRight(b.Bytes(), "\n"), '\n'), nil
}
