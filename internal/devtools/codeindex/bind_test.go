package codeindex

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// bindSource üye erişimleri, parametreler, adlı argümanlar ve komut_satırı anahtarları için bir proje.
const bindSource = `yapı Dikkat {
    baş
    kv_baş
    fonksiyon kurucu(boyut, baş, kv_baş = yok) {
        kendi.baş = baş
        kendi.kv_baş = kv_baş
    }
    fonksiyon say() {
        sonuç kendi.baş
    }
}

fonksiyon yap(baş) {
    değişken d = Dikkat(8, baş)
    sonuç d.baş
}

fonksiyon gez(x) {
    sonuç x.baş
}

fonksiyon gölge(baş) {
    değişken baş = 3
    sonuç baş
}

değişken a = Dikkat(8, 2, kv_baş: 1)
değişken e = yap(baş: 4)
değişken ayar = komut_satırı({baş: 8, adım: 1}, açıklamalar: {baş: "baş sayısı"})
`

// uses bir düğümün başvurularını "rol@satır:sütun/güven/dayanak" olarak sıralı verir.
func uses(r *Reader, id string) []string {
	var out []string
	for _, o := range r.Occurrences(id) {
		s := fmt.Sprintf("%s@%d:%d/%s", o.Role, o.Pos.Line, o.Pos.Col, o.Confidence)
		if o.Basis != "" {
			s += "/" + string(o.Basis)
		}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func wantUses(t *testing.T, r *Reader, id string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if got := uses(r, id); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("%s\n got  %v\n want %v", id, got, want)
	}
}

func TestMemberAccessBindsToTheField(t *testing.T) {
	r := buildSource(t, bindSource)
	if len(r.Index.Skipped) != 0 {
		t.Fatalf("the fixture must parse: %v", r.Index.Skipped)
	}
	wantUses(t, r, "field:m.tr#Dikkat.baş",
		"definition@2:5/EXTRACTED",
		"write@5:15/EXTRACTED/self",      // kendi.baş = baş
		"read@9:21/EXTRACTED/self",       // sonuç kendi.baş
		"read@15:13/INFERRED/local_type", // d.baş: yapıcıdan tek atamalı yerel
		"read@19:13/INFERRED/name_only")  // x.baş: alıcı bilinmiyor, yalnızca ad tutuyor
}

func TestParameterAndNamedArgumentOccurrences(t *testing.T) {
	r := buildSource(t, bindSource)
	if n := r.Node("parameter:m.tr#Dikkat.kurucu.kv_baş"); n == nil || n.Signature != "kv_baş = yok" {
		t.Errorf("the default value text is part of the signature: %+v", n)
	}
	wantUses(t, r, "parameter:m.tr#Dikkat.kurucu.baş",
		"definition@4:29/EXTRACTED", "read@5:21/EXTRACTED")
	wantUses(t, r, "parameter:m.tr#Dikkat.kurucu.kv_baş",
		"definition@4:34/EXTRACTED", "read@6:24/EXTRACTED", "argument_name@27:27/EXTRACTED")
	wantUses(t, r, "parameter:m.tr#yap.baş",
		"definition@13:15/EXTRACTED", "read@14:28/EXTRACTED", "argument_name@28:18/EXTRACTED")
}

func TestShadowedParameterIsInferred(t *testing.T) {
	r := buildSource(t, bindSource)
	for _, u := range uses(r, "parameter:m.tr#gölge.baş") {
		if strings.HasPrefix(u, "read") && !strings.Contains(u, "INFERRED") {
			t.Errorf("a parameter rebound in the body is only a guess: %s", u)
		}
	}
}

func TestCommandLineOptionKeys(t *testing.T) {
	r := buildSource(t, bindSource)
	wantUses(t, r, "option:m.tr#baş", "option_key@29:31/EXTRACTED", "option_key@29:63/EXTRACTED")
	if n := r.Node("option:m.tr#adım"); n == nil || n.Kind != KindOption {
		t.Errorf("each key is an option: %+v", n)
	}
}

func TestNameOnlyGuessesStayOutOfReferences(t *testing.T) {
	r := buildSource(t, bindSource)
	res := r.References("Dikkat.baş", QueryOptions{})
	for _, u := range res.Users {
		if u.Pos.Line == 19 {
			t.Errorf("a name-only guess is not a reference: %+v", u)
		}
	}
	if res.Counts.Total != 3 {
		t.Errorf("self write, self read and the typed local: %d", res.Counts.Total)
	}
}

// Kural: komut satırı seçeneğinin adı çalışma anındaki adıdır: dilin bir üye adı olan çıplak anahtar dosyanın dilinde
// yazılır (K3: İngilizce kodlu betikte `{Y500: 100}` → `--step`, Türkçe betikte `--adım`); kullanıcının kendi adı
// (`run`) ve kod taşımayan, düz yazılmış `adım` (Türkçe gösterimde `$adım`) olduğu gibi kalır (bağımsız denetim grup 7). Konum Türkçe gösterimdeki anahtarın yeridir; iki dilin eş betikleri ayrı dosyada, ayrı
// düğümlerdir (yerleşik adım yerleşiği onları birleştirir).
func TestOptionNameFollowsTheFilesLanguage(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{
		"en.tr":  "//! tr kodlu 2 dil=en\nA030 config = G801({Y500: 100, run: \"trial\"}, G911: {Y500: \"steps\"})\n",
		"en2.tr": "//! tr kodlu 2 dil=en\nA030 c = G801({adım: 1, Y500: 2, run: 3})\n",
		"tr.tr":  "değişken ayar = komut_satırı({adım: 100, run: \"deneme\"})\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := Build(dir, Options{})
	if err != nil || len(idx.Skipped) > 0 {
		t.Fatal(err, idx.Skipped)
	}
	r := NewReader(idx)
	for _, id := range []string{"option:en.tr#step", "option:en.tr#run", "option:tr.tr#adım", "option:tr.tr#run"} {
		if n := r.Node(id); n == nil || n.Kind != KindOption {
			t.Errorf("%s is an option", id)
		}
	}
	if n := r.Node("option:en.tr#adım"); n != nil {
		t.Errorf("an English script's option is --step, not --adım: %+v", n)
	}
	// İngilizce betikte düz yazılmış `adım` anahtarı kod taşımaz: `--adım`'dır, kodlu Y500 `--step`; iki ayrı seçenek.
	wantUses(t, r, "option:en2.tr#adım", "option_key@2:28/EXTRACTED")
	wantUses(t, r, "option:en2.tr#step", "option_key@2:38/EXTRACTED")
	wantUses(t, r, "option:en.tr#step", "option_key@2:33/EXTRACTED", "option_key@2:73/EXTRACTED")
}

// memberGuessFiles alıcısı bilinmeyen üye erişimleri için bir proje: katman.tr'de aynı adlı beş metot (`ileri`: aday
// sınırı 4 aşılır) ve beş alan (`ağırlık`), iki metot (`geri`: sınır içi); kullan.tr katmanı aktarır, başka.tr aktarmaz.
var memberGuessFiles = map[string]string{
	"katman.tr": func() string {
		var b strings.Builder
		for i := 1; i <= 5; i++ {
			fmt.Fprintf(&b, "yapı Katman%d {\n    ağırlık = %d\n    fonksiyon ileri() {\n        sonuç kendi.ağırlık\n    }\n", i, i)
			if i <= 2 {
				b.WriteString("    fonksiyon geri() {\n        sonuç 0\n    }\n")
			}
			b.WriteString("}\n")
		}
		return b.String()
	}(),
	"kullan.tr": "aktar \"katman\"\n\nfonksiyon çalış(p) {\n    yazdır(p.ileri())\n    p.ağırlık = 2\n    yazdır(p.geri())\n}\n",
	"başka.tr":  "fonksiyon başka(p) {\n    yazdır(p.ileri())\n    yazdır(p.ağırlık)\n}\n",
}

// Kural: alıcısı bilinmeyen üye erişimi (`p.ileri()`, `p.ağırlık = 2`) aynı adlı ve erişimin dosyasından görünen her
// alana, ve aynı adlı görünen metot 4'ten çoksa her metoda "olası" başvurudur: kullananlar göstermez, yeniden adlandırma
// "olası"da rolüyle (çağrı, yazma, okuma) listeler; metodun dosyasını görmeyen dosyadaki aynı adlı erişim hiç sayılmaz.
// Sınır içindeki metotlar (`geri`, 2) ada göre eşleşen adaydır (kenarı vardır). Bu davranış, tahminin kullanım yeri
// başına bir kez saklanıp sorguda çözülmesiyle değişmez (TestIndexSizeGrowsLinearlyWithSameNamedMembers).
func TestMemberGuessesAreLikelyOnlyWhereTheMemberIsVisible(t *testing.T) {
	dir := t.TempDir()
	for name, text := range memberGuessFiles {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := Build(dir, Options{})
	if err != nil || len(idx.Skipped) > 0 {
		t.Fatal(err, idx.Skipped)
	}
	r := NewReader(idx)
	likely := func(q string) []string {
		var out []string
		for _, c := range r.RenameImpact(q, "yeni", RenameOptions{NoText: true}).Likely {
			out = append(out, fmt.Sprintf("%s:%d %s %s", c.Pos.File, c.Pos.Line, c.Role, c.Basis))
		}
		sort.Strings(out)
		return out
	}
	for q, want := range map[string]string{
		"Katman3.ileri":   "kullan.tr:4 call name_only",
		"Katman3.ağırlık": "kullan.tr:5 write name_only",
		"Katman1.geri":    "kullan.tr:6 call name_match",
	} {
		if got := strings.Join(likely(q), "; "); got != want {
			t.Errorf("rename %s: likely %q, want %q (başka.tr does not import katman.tr)", q, got, want)
		}
	}
	for _, q := range []string{"Katman3.ileri", "Katman3.ağırlık"} {
		for _, u := range r.References(q, QueryOptions{}).Users {
			if u.Pos.File != "katman.tr" {
				t.Errorf("references %s: a name-only guess is not a reference: %+v", q, u)
			}
		}
	}
}
