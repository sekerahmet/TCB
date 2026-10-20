package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Komut satırı seçenekleri (docs/spec/19_surec_ve_kayit.md): argümanlar komut_satırı'na
// gelir; --yardım standart çıktıya yazılır ve 0 ile, hatalı argüman standart hataya yazılır
// ve 2 ile biter. İkisinde de ertele blokları çalışır; argüman uyarısı verilmez.
func TestCommandLineOptions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "egit.tr", `ertele yazdır("ertelenen")
değişken ayar = komut_satırı({adım: 10, oran: 0.5, hızlı: yanlış}, açıklama: "Deneme eğitimi")
yazdır("adım={ayar.adım} oran={ayar.oran} hızlı={ayar.hızlı}")
`)
	for _, command := range [][]string{
		{"egit.tr", "--adım", "20", "--oran=0.25", "--hızlı"},
		{"çalıştır", "egit.tr", "--adım", "20", "--oran=0.25", "--hızlı"},
		{"hata_ayıkla", "egit.tr", "--adım", "20", "--oran=0.25", "--hızlı"},
	} {
		s := tr(t, dir, command...)
		wait(t, s, 0, []string{"adım=20 oran=0.25 hızlı=doğru", "ertelenen"}, nil)
		if strings.Contains(s.stderr, "Uyarı") {
			t.Errorf("%v: okunan argümanlar için uyarı olmamalı: %q", command, s.stderr)
		}
	}
	s := tr(t, dir, "egit.tr", "--yardım")
	wait(t, s, 0, []string{"Kullanım: tr egit.tr [seçenekler]", "Deneme eğitimi", "--adım <Tam>", "(varsayılan: 10)", "ertelenen"}, nil)
	if s.stderr != "" || strings.Contains(s.stdout, "adım=") {
		t.Errorf("yardım programı bitirmeli, standart hata boş kalmalı: %+v", s)
	}
	s = tr(t, dir, "egit.tr", "--adim", "5")
	wait(t, s, 2, []string{"ertelenen"}, []string{"Hata: bilinmeyen seçenek '--adim'. Bunu mu demek istediniz: '--adım'? Seçenekler için: tr egit.tr --yardım"})
	if strings.Contains(s.stderr, "Çalışma Zamanı") || strings.Contains(s.stdout, "Hata:") {
		t.Errorf("kullanım hatası çalışma hatası gibi yazılmamalı: %+v", s)
	}
	// Derlenmiş uygulamada kullanım satırı uygulamanın adını gösterir.
	exit := filepath.Join(t.TempDir(), "uyg.exe")
	wait(t, tr(t, dir, "derle", "egit.tr", exit), 0, nil, nil)
	wait(t, run(t, exit, dir, "", nil, "-h"), 0, []string{"Kullanım: uyg.exe [seçenekler]"}, nil)
	wait(t, run(t, exit, dir, "", nil, "--oran", "0,5"), 2, nil, []string{"ondalık ayırıcı nokta olmalı", "Seçenekler için: uyg.exe --yardım"})
}

// Gizli değerler: gizli adlı ortam değişkeninin ve seçeneğin değeri yazdır'da, hata
// mesajında ve günlükte *** olur; dosyaya yazılan veri (dosya_yaz) değişmez.
func TestSecretValuesMaskedInOutput(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "g.tr", `değişken ayar = komut_satırı({api_key: ""})
değişken belirteç = ortam_değişkeni("HF_TOKEN")
yazdır("belirteç:", belirteç, "anahtar:", ayar.api_key)
değişken g = günlük("kayit/g.jsonl")
g.ekle({belirteç: belirteç})
dosya_yaz("veri.txt", belirteç)
fırlat "bozuk: " + ayar.api_key
`)
	s := run(t, trBinary, dir, "", []string{"HF_TOKEN=hf_cokgizli123"}, "g.tr", "--api_key", "sk-anahtar-456")
	wait(t, s, 1, []string{"belirteç: *** anahtar: ***"}, []string{"bozuk: ***"})
	for _, secret := range []string{"hf_cokgizli123", "sk-anahtar-456"} {
		if strings.Contains(s.stdout+s.stderr, secret) {
			t.Errorf("%q çıktıda görünmemeli:\n%s\n%s", secret, s.stdout, s.stderr)
		}
	}
	logContent, _ := os.ReadFile(filepath.Join(dir, "kayit", "g.jsonl"))
	if strings.Contains(string(logContent), "hf_cokgizli123") || !strings.Contains(string(logContent), `"belirteç":"***"`) {
		t.Errorf("günlükte gizli değer maskelenmeli:\n%s", logContent)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "veri.txt")); string(data) != "hf_cokgizli123" {
		t.Errorf("veri çıktısı değişmemeli: %q", data)
	}
}

// İlerleme çubuğu boruya (terminal değil) yazarken standart çıktıya karışmaz, sonda özet
// satırı yazılır; program hatayla bitse de çubuk biter, bitiş günlüğe yazılır.
func TestProgressOnCommandLine(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "i.tr", `değişken g = günlük("i.jsonl")
değişken ç = ilerleme(10, günlük: g)
döngü i içinde 0..4 { ç.güncelle({kayıp: 1.5}) }
yazdır("bitti mi:", ç)
fırlat "patladı"
`)
	s := tr(t, dir, "i.tr")
	wait(t, s, 1, []string{"bitti mi: İlerleme(adım 4/10)"}, []string{"adım 4/10 · kayıp 1,5 ·", "sürdü\n❌ Çalışma Zamanı Hatası:", "patladı"})
	if strings.Contains(s.stdout, "adım 4/10 ·") || strings.Contains(s.stderr, "\r") {
		t.Errorf("çubuk standart çıktıya ve (terminal değilken) satır başı karakteriyle yazılmamalı: %+v", s)
	}
	content, _ := os.ReadFile(filepath.Join(dir, "i.jsonl"))
	if !strings.Contains(string(content), `"tür":"bitiş"`) {
		t.Errorf("bitiş kaydı yazılmalı:\n%s", content)
	}
}
