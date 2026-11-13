package interp

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
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

// TestValueAndContextMaskTogether: değer maskesi ve bağlamlı maske aynı özgün metinde bulunur, ortak
// baytı olan aralıklar tek *** olur (19.4). Sıralı iki aşamanın ikisi de birer durumda sızdırırdı:
// kayıtlı değer "://"'yu kapsayınca bağlam bölgeyi bulamazdı, kimlik kısmını kapsayınca değer
// eşleşmezdi. Adressiz metinde sonuç değer maskesinin kendisidir (TestSecretFilter değişmeden geçer).
func TestValueAndContextMaskTogether(t *testing.T) {
	t.Cleanup(ClearRegisteredSecrets)
	for _, tc := range []struct{ secret, text, want string }{
		{"https://ali", "https://ali:ab@h", "***:***@h"},
		{"ali:Vekil2026", "http://ali:Vekil2026@h", "http://***@h"},
		{"postgres", "postgres://postgres@localhost/ana", "***://***@localhost/ana"},
	} {
		v := New()
		if !v.AddSecret(tc.secret) {
			t.Fatalf("%q kaydedilmedi", tc.secret)
		}
		if got := v.Mask(tc.text); got != tc.want {
			t.Errorf("kayıtlı %q: %q → %q, %q bekleniyordu", tc.secret, tc.text, got, tc.want)
		}
		if got := MaskBeforeCut(tc.text); got != tc.want {
			t.Errorf("süreç boyu küme, kayıtlı %q: %q → %q", tc.secret, tc.text, got)
		}
	}
}

// TestEscapedSecretIsMasked: maskeden önce metni kaçışlayan üç dönüşüm (kabın metni QuoteText,
// TR_JSON'un metni, iletinin turkish.Printable'ı) gizli değeri kendi kaçışlı biçimine çevirir; kayıt
// o biçimleri de tutar (19.4). Maskeden önce metni değiştiren yeni bir dönüşüm eklenirse buraya
// satırı eklenir.
func TestEscapedSecretIsMasked(t *testing.T) {
	t.Cleanup(ClearRegisteredSecrets)
	program := "TR_print([t])\nTR_print({\"k\": t})\nTR_print({t: 1})\nTR_print([[t], {\"k\": [t]}])\n" +
		"TR_print(TR_JSON([t, {\"k\": t}]))\n"
	for _, secret := range []string{
		"abc\"def12345", "abc\\def12345", "abc\tdef12345", "abc\rdef12345", "abc\x01def12345",
		"abc\xe2\x80\xa8def12345", // U+2028: yalnız JSON kaçışlar
		"abc\xe2\x80\xaedef12345", // U+202E: yalnız TR'nin metin yazımı kaçışlar
		"abc\"def\x01gh12345",     // tırnak ve denetim harfi birlikte: Printable'ın biçimi QuoteText'inkinden ayrı
	} {
		var output bytes.Buffer
		v := New()
		v.Out, v.Err = &output, &output
		if !v.AddSecret(secret) {
			t.Fatalf("%q kaydedilmedi", secret)
		}
		v.GlobalEnv.Define("t", &StringVal{Val: secret}, false)
		execCode(t, v, program)
		if got := output.String(); strings.Contains(got, "12345") || strings.Count(got, SecretMask) != 7 {
			t.Errorf("%q: kabın metninde kaçışlı biçim açık kaldı: %q", secret, got)
		}
		if got := v.Mask(turkish.Printable("bağlanamadı: " + secret)); got != "bağlanamadı: ***" {
			t.Errorf("%q: iletinin yolu: %q", secret, got)
		}
	}
	// Kaçışlanan harf taşımayan değer ek kayıt doğurmaz.
	v := New()
	v.AddSecret("abcdef12")
	if len(v.secrets) != 1 {
		t.Errorf("düz değer: %d kayıt, 1 bekleniyordu", len(v.secrets))
	}
	// Geçersiz UTF-8: kabın tek yaprağıyken biçim tutar. Sınırdaki yarım diziyi komşu bayt tamamlayıp
	// kaçışlanan bir harf (U+0085) yapınca kaçışlı biçim eşleşmez: sınır sabitlenir, yanılgı değil.
	// Tamamlanan harf kaçışlanmıyorsa (x\xc3 + \xa7…: ç) ham değer baytça eşleşir ve örtülür.
	invalid := New()
	invalid.AddSecret("ab\xffcdef12")
	if got := invalid.Mask(QuoteText("ab\xffcdef12")); got != `"***"` {
		t.Errorf("geçersiz UTF-8, tek yaprak: %q", got)
	}
	edge := New()
	edge.AddSecret("\x85abcdef")
	edge.AddSecret("\xa7abcdef")
	if got := edge.Mask(turkish.Printable("x\xc2\x85abcdef")); !strings.Contains(got, "abcdef") {
		t.Errorf("sınır değişti (U+0085'i tamamlayan komşu): %q", got)
	}
	if got := edge.Mask(turkish.Printable("x\xc3\xa7abcdef")); strings.Contains(got, "abcdef") {
		t.Errorf("ç'yi tamamlayan komşuda ham değer örtülmeli: %q", got)
	}
}
