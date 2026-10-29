package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Komut satırı seçenekleri (docs/spec/19_surec_ve_kayit.md): argümanlar komut_satırı'na
// gelir; --help standart çıktıya yazılır ve 0 ile, hatalı argüman standart hataya yazılır
// ve 2 ile biter. İkisinde de ertele blokları çalışır; argüman uyarısı verilmez.
func TestCommandLineOptions(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "egit.tr", `TR_defer TR_print("ertelenen")
TR_var ayar = TR_command_line({"adım": 10, "oran": 0.5, "hızlı": TR_false}, TR_description: "Deneme eğitimi")
TR_print("adım=\(ayar["adım"]) oran=\(ayar["oran"]) hızlı=\(ayar["hızlı"])")
`)
	for _, command := range [][]string{
		{"egit.tr", "--adım", "20", "--oran=0.25", "--hızlı"},
		{"run", "egit.tr", "--adım", "20", "--oran=0.25", "--hızlı"},
		{"debug", "egit.tr", "--adım", "20", "--oran=0.25", "--hızlı"},
	} {
		s := tr(t, dir, command...)
		testkit.Wait(t, s, 0, []string{"adım=20 oran=0.25 hızlı=true", "ertelenen"}, nil)
		if strings.Contains(s.Stderr, "Uyarı") {
			t.Errorf("%v: okunan argümanlar için uyarı olmamalı: %q", command, s.Stderr)
		}
	}
	s := tr(t, dir, "egit.tr", "--help")
	testkit.Wait(t, s, 0, []string{"Kullanım: tr egit.tr [seçenekler]", "Deneme eğitimi", "--adım <Tam>", "(varsayılan: 10)", "ertelenen"}, nil)
	if s.Stderr != "" || strings.Contains(s.Stdout, "adım=") {
		t.Errorf("yardım programı bitirmeli, standart hata boş kalmalı: %+v", s)
	}
	s = tr(t, dir, "egit.tr", "--adim", "5")
	testkit.Wait(t, s, 2, []string{"ertelenen"}, []string{"Hata: bilinmeyen seçenek '--adim'. Bunu mu demek istediniz: '--adım'? Seçenekler için: tr egit.tr --help"})
	if strings.Contains(s.Stderr, "Çalışma Zamanı") || strings.Contains(s.Stdout, "Hata:") {
		t.Errorf("kullanım hatası çalışma hatası gibi yazılmamalı: %+v", s)
	}
	// Derlenmiş uygulamada kullanım satırı uygulamanın adını gösterir.
	exit := filepath.Join(t.TempDir(), "uyg.exe")
	testkit.Wait(t, tr(t, dir, "build", "egit.tr", exit), 0, nil, nil)
	testkit.Wait(t, testkit.RunBinary(t, exit, dir, "", nil, "-h"), 0, []string{"Kullanım: uyg.exe [seçenekler]"}, nil)
	testkit.Wait(t, testkit.RunBinary(t, exit, dir, "", nil, "--oran", "0,5"), 2, nil, []string{"ondalık ayırıcı nokta olmalı", "Seçenekler için: uyg.exe --help"})
}

// Gizli değerler: gizli adlı ortam değişkeninin ve seçeneğin değeri yazdır'da, hata
// mesajında ve günlükte *** olur; dosyaya yazılan veri (dosya_yaz) değişmez.
func TestSecretValuesMaskedInOutput(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "g.tr", `TR_var ayar = TR_command_line({"api_key": ""})
TR_var belirteç = TR_env("HF_TOKEN")
TR_print("belirteç:", belirteç, "anahtar:", ayar["api_key"])
TR_var g = TR_Logger("kayit/g.jsonl")
g.TR_append({"belirteç": belirteç})
TR_write_file("veri.txt", belirteç)
TR_throw "bozuk: " + ayar["api_key"]
`)
	s := testkit.RunBinary(t, trBinary, dir, "", []string{"HF_TOKEN=hf_cokgizli123"}, "g.tr", "--api_key", "sk-anahtar-456")
	testkit.Wait(t, s, 1, []string{"belirteç: *** anahtar: ***"}, []string{"bozuk: ***"})
	for _, secret := range []string{"hf_cokgizli123", "sk-anahtar-456"} {
		if strings.Contains(s.Stdout+s.Stderr, secret) {
			t.Errorf("%q çıktıda görünmemeli:\n%s\n%s", secret, s.Stdout, s.Stderr)
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
	t.Skip("katman 2: değer metninin kalıp sözcükleri (İlerleme(adım 4/10) → Progress(step 4/10)) tek dile inince açılır")
	dir := t.TempDir()
	writeTR(t, dir, "i.tr", `TR_var g = TR_Logger("i.jsonl")
TR_var ç = TR_Progress(10, TR_log: g)
TR_for i TR_in 0..4 { ç.TR_update({"loss": 1.5}) }
TR_print("bitti mi:", ç)
TR_throw "patladı"
`)
	s := tr(t, dir, "i.tr")
	testkit.Wait(t, s, 1, []string{"bitti mi: İlerleme(adım 4/10)"}, []string{"adım 4/10 · kayıp 1,5 ·", "sürdü\n❌ Çalışma Zamanı Hatası:", "patladı"})
	if strings.Contains(s.Stdout, "adım 4/10 ·") || strings.Contains(s.Stderr, "\r") {
		t.Errorf("çubuk standart çıktıya ve (terminal değilken) satır başı karakteriyle yazılmamalı: %+v", s)
	}
	content, _ := os.ReadFile(filepath.Join(dir, "i.jsonl"))
	if !strings.Contains(string(content), `"kind":"end"`) {
		t.Errorf("bitiş kaydı yazılmalı:\n%s", content)
	}
}
