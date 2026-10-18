package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"tr_dili/internal/depo"
)

// Belgelerdeki ve eklenti şablonlarındaki TR kodu bozulmasın: her ```tr kod bloğu
// `tr kontrol`dan geçmelidir (hakem mimari 6: kılavuzdaki veritabanı örneği derlenmiyordu).
// Bilerek hatalı yazılmış örnekler ```tr hata ile işaretlenir; onların kontrolden
// GEÇMEMESİ beklenir (belge hatayı gösteriyorsa hata gerçekten oluşmalı).

// belgeler denetlenen belgelerdir (depo köküne göre).
var belgeler = []string{
	"README.md",
	"KULLANIM_KILAVUZU.md",
	"docs/DIL_TANIMI.md",
	"docs/YOL_HARITASI.md",
	"duzenleyici/vscode/README.md",
	"bakim/kiyas/README.md",
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
	for _, b := range belgeler {
		veri, err := os.ReadFile(depo.Yol(b))
		if err != nil {
			t.Fatal(err) // belge taşındıysa listeyi güncelleyin
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
		kod := sablonAc(strings.Join(s.Body, "\n"))
		if strings.HasPrefix(kod, ".") {
			// Metot şablonları (.filtrele { o }) bir listenin ardına yazılır
			kod = "değişken liste = [1, 2, 3]\nliste" + kod
		}
		if gecti, cikti := trKontrol(t, "sablon", kod); !gecti {
			t.Errorf("%q şablonu derlenmiyor:\n%s\n---\n%s", ad, kod, cikti)
		}
	}
}
