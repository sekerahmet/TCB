package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"tr_dili/internal/depo"
	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/denetci"
	"tr_dili/internal/dil/sozcuk"
)

// Belgelerdeki ve eklenti şablonlarındaki TR kodu bozulmasın: her ```tr kod bloğu
// `tr kontrol`dan geçmelidir (hakem mimari 6: kılavuzdaki veritabanı örneği derlenmiyordu).
// Bilerek hatalı yazılmış örnekler ```tr hata ile işaretlenir; onların kontrolden
// GEÇMEMESİ beklenir (belge hatayı gösteriyorsa hata gerçekten oluşmalı).

// uretilmisBelge kod listesinden üretilen başvuru belgesidir; içindeki örnekler kod
// listesinin testlerinde (internal/motor) çalıştırılır, burada yalnızca bağlantıları denetlenir.
const uretilmisBelge = "docs/kodlar.md"

// belgeListesi depodaki belgelerdir (depo köküne göre, bölü çizgisiyle): kökteki README,
// docs/ altındaki bütün .md dosyaları, eklentinin ve kıyas setinin README'leri. Yeni bir
// belge docs/ altına eklendiğinde kendiliğinden denetime girer.
func belgeListesi(t *testing.T) []string {
	t.Helper()
	liste := []string{"README.md", "duzenleyici/vscode/README.md", "bakim/kiyas/README.md"}
	for _, desen := range []string{"docs/*.md", "docs/*/*.md"} {
		d, err := filepath.Glob(depo.Yol(desen))
		if err != nil {
			t.Fatal(err)
		}
		for _, yol := range d {
			goreli, err := filepath.Rel(depo.Kok(), yol)
			if err != nil {
				t.Fatal(err)
			}
			liste = append(liste, filepath.ToSlash(goreli))
		}
	}
	if len(liste) < 20 {
		t.Fatalf("yalnızca %d belge bulundu; docs/ yapısı değişmiş olabilir", len(liste))
	}
	return liste
}

var trBlokRe = regexp.MustCompile("(?ms)^```tr( hata)?[ \t]*\r?\n(.*?)^```")

// trKontrol kodu geçici bir dosyaya yazıp `tr kontrol` ile denetler.
func trKontrol(t *testing.T, ad, kod string) (bool, string) {
	t.Helper()
	dosya := filepath.Join(t.TempDir(), ad+".tr")
	if err := os.WriteFile(dosya, []byte(kod), 0644); err != nil {
		t.Fatal(err)
	}
	cikti, err := exec.Command(trIkilisi, "kontrol", dosya).CombinedOutput()
	return err == nil, string(cikti)
}

func TestBelgelerdekiTRKodBloklari(t *testing.T) {
	toplam := 0
	for _, b := range belgeListesi(t) {
		if b == uretilmisBelge {
			continue
		}
		veri, err := os.ReadFile(depo.Yol(b))
		if err != nil {
			t.Fatal(err)
		}
		metin := string(veri)
		for _, m := range trBlokRe.FindAllStringSubmatchIndex(metin, -1) {
			toplam++
			satir := strings.Count(metin[:m[0]], "\n") + 1
			hataBekleniyor := m[2] >= 0
			kod := metin[m[4]:m[5]]
			ad := fmt.Sprintf("%s_%d", strings.NewReplacer("/", "_", ".", "_").Replace(b), satir)
			gecti, cikti := trKontrol(t, ad, kod)
			switch {
			case !hataBekleniyor && !gecti:
				t.Errorf("%s:%d: kod bloğu `tr kontrol`dan geçmiyor:\n%s", b, satir, cikti)
			case hataBekleniyor && gecti:
				t.Errorf("%s:%d: ```tr hata bloğu hata vermeli ama `tr kontrol`dan geçiyor", b, satir)
			}
		}
	}
	if toplam < 20 {
		t.Fatalf("yalnızca %d kod bloğu bulundu; belge biçimi değişmiş olabilir", toplam)
	}
}

var (
	sablonVarsayilanRe = regexp.MustCompile(`\$\{\d+:([^${}]*)\}`)
	sablonBosRe        = regexp.MustCompile(`\$\{\d+\}|\$\d+`)
)

// sablonAc bir VS Code şablonunu varsayılan değerleriyle açar (Tab'a hiç basılmamış gibi).
func sablonAc(govde string) string {
	for {
		yeni := sablonVarsayilanRe.ReplaceAllString(govde, "$1")
		if yeni == govde {
			break
		}
		govde = yeni
	}
	return sablonBosRe.ReplaceAllString(govde, "")
}

// adYerTutucuRe yer tutucunun varsayılanı tek bir ad mı (eğer ${1:koşul})?
var adYerTutucuRe = regexp.MustCompile(`^[\p{L}_][\p{L}\p{N}_]*$`)

// yerTutuculariTanimla şablonun kullanıcının dolduracağı ad yer tutucularını (eğer
// ${1:koşul}, eşle ${1:değer}) şablon kendisi tanımlamıyorsa kodun başında tanımlar:
// tanımsız ad çalışmadan önce hatadır (5.14), şablon ise bu adların kullanıcıda var
// olduğunu varsayar.
func yerTutuculariTanimla(govde, kod string) string {
	tanimli, _ := denetci.UstDuzeyAdlari(ayristir.New(sozcuk.New(kod)).ParseProgram())
	var on strings.Builder
	goruldu := map[string]bool{}
	for _, m := range sablonVarsayilanRe.FindAllStringSubmatch(govde, -1) {
		ad := m[1]
		if !adYerTutucuRe.MatchString(ad) || goruldu[ad] || slices.Contains(tanimli, ad) || sozcuk.LookupIdent(ad) != sozcuk.TOKEN_IDENT {
			continue
		}
		goruldu[ad] = true
		fmt.Fprintf(&on, "değişken %s = 1\n", ad)
	}
	return on.String() + kod
}

// Eklentinin kod şablonları varsayılan değerleriyle açıldığında derlenebilir kod üretmelidir
// (hakem mimari 3: eşle şablonu derlenmeyen kod üretiyordu).
func TestEklentiSablonlariDerlenir(t *testing.T) {
	veri, err := os.ReadFile(depo.Yol("duzenleyici/vscode/snippets/tr.code-snippets"))
	if err != nil {
		t.Fatal(err)
	}
	var sablonlar map[string]struct {
		Prefix []string `json:"prefix"`
		Body   []string `json:"body"`
	}
	if err := json.Unmarshal(veri, &sablonlar); err != nil {
		t.Fatalf("şablon dosyası geçerli JSON değil: %v", err)
	}
	if len(sablonlar) < 10 {
		t.Fatalf("yalnızca %d şablon bulundu", len(sablonlar))
	}
	for ad, s := range sablonlar {
		govde := strings.Join(s.Body, "\n")
		kod := sablonAc(govde)
		if strings.HasPrefix(kod, ".") {
			// Metot şablonları (.filtrele { o }) bir listenin ardına yazılır
			kod = "değişken liste = [1, 2, 3]\nliste" + kod
		}
		kod = yerTutuculariTanimla(govde, kod)
		if gecti, cikti := trKontrol(t, "sablon", kod); !gecti {
			t.Errorf("%q şablonu derlenmiyor:\n%s\n---\n%s", ad, kod, cikti)
		}
	}
}
