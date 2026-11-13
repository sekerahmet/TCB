package interp

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// fakeLiveLine silinip çizildiğini ortak çıktıya işaretleyen bir canlı satırdır.
type fakeLiveLine struct{ w *bytes.Buffer }

func (s *fakeLiveLine) Clear() { s.w.WriteString("[sil]") }
func (s *fakeLiveLine) Draw()  { s.w.WriteString("[çiz]") }

func execCode(t *testing.T, vmx *VM, codeVal string) {
	t.Helper()
	prog := mustParseProgram(t, codeVal)
	if _, err := vmx.Run(prog); err != nil {
		t.Fatal(err)
	}
}

// Etkin bir canlı satır (ilerleme çubuğu) varken yazdır, girdi istemi ve hata çıktısı
// önce satırı siler, metni yazar, satırı yeniden çizer; canlı satır kaldırılınca düz yazılır.
func TestLiveLineHook(t *testing.T) {
	var output bytes.Buffer
	vmx := New()
	vmx.Out, vmx.Err = &output, &output
	vmx.Stdin = bytes.NewBufferString("ali\n")
	vmx.SetLiveLine(&fakeLiveLine{w: &output})
	execCode(t, vmx, "TR_print(\"a\", 1)\nTR_var ad = TR_input(\"ad: \")\n")
	vmx.reportAsyncError(3, errors.New("bozuk"))
	expected := "[sil]a 1\n[çiz][sil]ad: [çiz][sil]⚠️  Eşzamanlı görev hatası (satır 3'te başlatılan): bozuk\n[çiz]"
	if output.String() != expected {
		t.Errorf("çıktı\n%q\nbeklenen\n%q", output.String(), expected)
	}
	output.Reset()
	vmx.SetLiveLine(nil)
	execCode(t, vmx, "TR_print(\"b\")\n")
	if output.String() != "b\n" {
		t.Errorf("canlı satır kaldırılınca düz yazılmalı: %q", output.String())
	}
	// Canlı satırın kendi çizimi de aynı kilit altında
	vmx.UpdateLiveLine(func() { output.WriteString("çubuk") })
	if output.String() != "b\nçubuk" {
		t.Errorf("CanliSatirGuncelle: %q", output.String())
	}
}

// Gizli değer süzgeci: kayıtlı değerler çıktı noktalarında (yazdır, hata çıktısı) *** ile
// maskelenir; değerin kendisi (yazdır'ın döndürdüğü, değişkenler) değişmez.
// Kısa değerler kaydedilmez; biri ötekini içeren değerlerde uzunu bütünüyle maskelenir.
func TestSecretFilter(t *testing.T) {
	var output bytes.Buffer
	vmx := New()
	vmx.Out, vmx.Err = &output, &output
	if vmx.Mask("hf_gizli123") != "hf_gizli123" {
		t.Error("kayıt yokken metin değişmemeli")
	}
	if vmx.AddSecret("abc") {
		t.Error("kısa değer kaydedilmemeli")
	}
	if !vmx.AddSecret("hf_gizli123") || !vmx.AddSecret("hf_gizli123_uzun") || !vmx.AddSecret("hf_gizli123") {
		t.Fatal("gizli değer kaydedilemedi")
	}
	vmx.GlobalEnv.Define("anahtar", &StringVal{Val: "hf_gizli123"}, false)
	execCode(t, vmx, "TR_var d = \"anahtar: \\(anahtar), uzun: hf_gizli123_uzun, abc\"\nTR_print(d)\n")
	if got := output.String(); got != "anahtar: ***, uzun: ***, abc\n" {
		t.Errorf("yazdır çıktısı maskelenmeli: %q", got)
	}
	if d, _ := vmx.GlobalEnv.Get("d"); d.Inspect() != "anahtar: hf_gizli123, uzun: hf_gizli123_uzun, abc" {
		t.Errorf("yazdırılan değer (veri) değişmemeli: %s", d.Inspect())
	}
	output.Reset()
	vmx.reportAsyncError(1, errors.New("istek reddedildi: hf_gizli123"))
	if got := output.String(); !strings.Contains(got, "istek reddedildi: ***") {
		t.Errorf("hata çıktısı maskelenmeli: %q", got)
	}
}

// En uzun gizli değerin boyu bayt olarak verilir; süzgece girmeyen kısa değer sayılmaz.
func TestLongestSecret(t *testing.T) {
	v := New()
	if got := v.LongestSecret(); got != 0 {
		t.Errorf("süzgeç boşken %d", got)
	}
	v.AddSecret("kısa")
	v.AddSecret("uzun_değer")
	v.AddSecret("altı_harf")
	if got, want := v.LongestSecret(), len("uzun_değer"); got != want {
		t.Errorf("%d, beklenen %d", got, want)
	}
}

// Değer özetinin süreç boyu kümesi (19.4): bir VM'nin kaydı öbür VM'nin hata iletisindeki özette de
// örtülür; ClearRegisteredSecrets VM'lerin kaydettiğini siler, süreç başında taranan kalır; uzun
// değer önce denenir; kümede olan değerin yeniden kaydı değiştiriciyi yeniden kurdurmaz.
func TestProgramSecretsAcrossVMs(t *testing.T) {
	t.Cleanup(ClearRegisteredSecrets)
	ClearRegisteredSecrets()
	summary := func(text string) string {
		return ArgTypeError(codes.FnZeros, 1, &StringVal{Val: text}, codes.TypeInt).Error()
	}
	key := "sk-vmler-arasi-0123456789abcdefghijklmnopqrst"
	if len(key) != 45 {
		t.Fatalf("anahtar %d karakter", len(key))
	}
	a := New()
	a.AddSecret(key)
	_ = New() // B: kendi süzgecinde anahtar yok; özet süreç boyu kümeden maskelenir
	if got := summary(key); strings.Contains(got, key[:8]) {
		t.Errorf("A'nın kaydı B'nin özetinde örtülmeli: %q", got)
	}
	ClearRegisteredSecrets()
	if got := summary(key); !strings.Contains(got, key[:39]) || strings.Contains(got, SecretMask) {
		t.Errorf("boşaltmadan sonra özet anahtarın ilk 39 karakterini taşımalı: %q", got)
	}
	address := "postgres://uygulama_kullanicisi:Surecbasi26@db.sirket.local/ana"
	a.AddStartAddressSecrets(address)
	ClearRegisteredSecrets()
	if got := summary(address); strings.Contains(got, "Surecba") {
		t.Errorf("süreç başı parçası boşaltmadan sonra da örtülü kalmalı: %q", got)
	}
	// Örtüşme: uzun değer süreç başı parçasında, başı olan kısa değer VM'lerin parçasında; iki kayıt
	// sırasında da uzunun hiçbir parçası açık kalmaz.
	a.AddStartAddressSecrets("https://ghp_abc123def456@github.com")
	a.AddSecret("ghp_abc123")
	a.AddSecret("ghp_xyz789")
	a.AddStartAddressSecrets("https://ghp_xyz789uvw012@github.com")
	for text, rest := range map[string]string{"anahtar: ghp_abc123def456 son": "def456", "anahtar: ghp_xyz789uvw012 son": "uvw012"} {
		if got := summary(text); strings.Contains(got, rest) {
			t.Errorf("uzun değer bütünüyle maskelenmeli: %q", got)
		}
	}
	// Yeniden kayıt kümeyi "değişti" diye işaretlemez; yeni değer işaretler, değiştirici bir kez kurulur.
	programSecrets.mask("")
	before := programSecrets.replacer.Load()
	a.AddSecret("ghp_abc123")
	a.AddStartAddressSecrets("https://ghp_abc123def456@github.com")
	if programSecrets.changed.Load() {
		t.Error("kümede olan değerin yeniden kaydı kümeyi değiştirmemeli")
	}
	programSecrets.mask("")
	if programSecrets.replacer.Load() != before {
		t.Error("değiştirici yeniden kurulmamalı")
	}
	a.AddSecret("yeni-gizli-2026")
	if !programSecrets.changed.Load() {
		t.Error("yeni değer kümeyi değiştirmeli")
	}
	programSecrets.mask("")
	if after := programSecrets.replacer.Load(); after == before || programSecrets.changed.Load() {
		t.Error("değiştirici bir sonraki maskede bir kez kurulmalı")
	}
}
