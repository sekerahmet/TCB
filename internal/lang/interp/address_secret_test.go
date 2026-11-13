package interp

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// Adresin kimlik kısmı (19.4): her "://"'dan sonra '/', '?', '#' ya da boşluğa kadar süren bölgede son
// '@'den önceki metin kimlik kısmıdır; ':' varsa ilk ':'den sonrası, yoksa tamamı gizlidir; yazıldığı
// gibi ve % kaçışları çözülmüş hâliyle; sayı da gizlidir, 6 karakter sınırı süzgecinkidir.
func TestAddressSecrets(t *testing.T) {
	t.Cleanup(ClearRegisteredSecrets)
	for _, tc := range []struct {
		name  string
		value string
		want  []string
	}{
		// (a) 19.4'ün örnekleri
		{"vekil", "http://ali:Vekil2026@vekil:8080", []string{"Vekil2026"}},
		{"veritabanı", "postgres://uygulama:Vekil2026@db/ana", []string{"Vekil2026"}},
		{"sayı da gizli", "redis://:20262026@onbellek:6379/0", []string{"20262026"}},
		{"':' yoksa tamamı", "https://ghp_abc123def@github.com/k/d", []string{"ghp_abc123def"}},
		// (b) sınırlar
		{"son '@'", "http://u:Vekil@2026@host", []string{"Vekil@2026"}},
		{"'/'dan sonraki '@'", "http://host/a:Vekil2026@x", nil},
		{"'?'den sonraki '@'", "http://host?u=a:Vekil2026@x", nil},
		{"'#'dan sonraki '@'", "http://host#a:Vekil2026@x", nil},
		{"boşluktan sonraki '@'", "http://host a:Vekil2026@x", nil},
		{"sekme sınırdır", "http://a:Vekil2026@x\thttp://b:Baska2026@y", []string{"Vekil2026", "Baska2026"}},
		{"satır sonu sınırdır", "http://a:Vekil2026@x\nhttp://b:Baska2026@y", []string{"Vekil2026", "Baska2026"}},
		{"satır sonundan sonraki '@'", "http://host\nu:Vekil2026@x", nil},
		// (c) % kaçışları
		{"çözülmüş hâli de", "http://u:Vek%40il2026@x", []string{"Vek%40il2026", "Vek@il2026"}},
		{"geçersiz kaçış yalnız yazıldığı gibi", "http://u:Vek%zzil2026@x", []string{"Vek%zzil2026"}},
		{"'+' boşluk olmaz", "http://u:Vek+il2026@x", []string{"Vek+il2026"}},
		{"%2B '+' olur", "http://u:Vek%2Bil2026@x", []string{"Vek%2Bil2026", "Vek+il2026"}},
		// (d) adresi olmayan değer
		{"yol", "D:/veri/model.gguf", nil},
		{"dosya adresi", "file:///C:/x@y", nil},
		{"yolda '@'", "https://www.npmjs.com/package/@scope/x", nil},
		// (e) iki adres
		{"virgülle", "http://a:Vekil2026@x,http://b:Baska2026@y", []string{"Vekil2026", "Baska2026"}},
		{"boşlukla", "http://a:Vekil2026@x http://b:Baska2026@y", []string{"Vekil2026", "Baska2026"}},
		// (g) boş parola
		{"boş parola", "http://ali:@vekil", nil},
		// (h) sınır girdileri
		{"boş değer", "", nil},
		{"yalnız ://", "://", nil},
	} {
		if got := addressSecrets(tc.value); !slices.Equal(got, tc.want) {
			t.Errorf("%s: %q → %q, %q bekleniyordu", tc.name, tc.value, got, tc.want)
		}
	}
	t.Run("6 karakter sınırı", addressSecretsTakeTheFilterLimit)
	t.Run("büyük girdiler", addressSecretsOfLargeValues)
	t.Run("toplu kayıt", addressSecretsBuildTheFilterOnce)
}

// (f) 6 karakter sınırı süzgecindir ve karakterle ölçülür; kısa parola ve ':'siz kısa kimlik girmez.
func addressSecretsTakeTheFilterLimit(t *testing.T) {
	t.Cleanup(ClearRegisteredSecrets)
	for _, tc := range []struct {
		value, secret string
		masked        bool
	}{
		{"http://ali:Vekil@x", "Vekil", false},
		{"http://ali:Vekil2@x", "Vekil2", true},
		{"http://ali:şşşşş@x", "şşşşş", false},
		{"http://ali:şşşşşş@x", "şşşşşş", true},
		{"http://ali:12@vekil", "12", false},
		{"ssh://git@github.com/k/d", "git", false},
	} {
		v := New()
		v.AddAddressSecrets(tc.value)
		if got := v.Mask(tc.secret) == SecretMask; got != tc.masked {
			t.Errorf("%q: maskelendi %v, beklenen %v", tc.value, got, tc.masked)
		}
	}
	// Maske: ':'siz kimlik kısmı her geçişinde örtülür; iki adresin iki parolası.
	v := New()
	v.AddAddressSecrets("postgres://postgres@localhost/ana", "http://a:Vekil2026@x,http://b:Baska2026@y")
	if got := v.Mask("postgres://postgres@localhost/ana"); got != "***://***@localhost/ana" {
		t.Errorf("':'siz kimlik: %q", got)
	}
	if got := v.Mask("http://a:Vekil2026@x,http://b:Baska2026@y"); got != "http://a:***@x,http://b:***@y" {
		t.Errorf("iki adres: %q", got)
	}
}

// (h) büyük girdiler: adressiz 1 MiB'lık değer kayıt vermez, aynı adresin tekrarı tek kayıttır, 10 000
// ayrı adres 10 000 kayıttır ve maske çalışır.
func addressSecretsOfLargeValues(t *testing.T) {
	t.Cleanup(ClearRegisteredSecrets)
	const mib = 1 << 20
	if got := addressSecrets(strings.Repeat("a", mib)); got != nil {
		t.Errorf("adressiz değer: %d kayıt", len(got))
	}
	v := New()
	v.AddAddressSecrets(strings.Repeat("http://u:Vekil2026@x,", mib/len("http://u:Vekil2026@x,")))
	if len(v.secrets) != 1 {
		t.Errorf("aynı adresin tekrarı: %d kayıt, 1 bekleniyordu", len(v.secrets))
	}
	var many strings.Builder
	for i := range 10000 {
		fmt.Fprintf(&many, "http://u:Parola%05d@h ", i)
	}
	v = New()
	v.AddAddressSecrets(many.String())
	if len(v.secrets) != 10000 {
		t.Errorf("10 000 adres: %d kayıt", len(v.secrets))
	}
	if got := v.Mask("a Parola00000 b Parola09999 c"); got != "a *** b *** c" {
		t.Errorf("maske: %q", got)
	}
}

// (i) toplu kayıt: tek çağrının yeni değerleri süzgeci bir kez kurar; aynı değerlerle ikinci çağrı
// kurmaz. Tek tek kaydeden eski yol üç kez kurardı.
func addressSecretsBuildTheFilterOnce(t *testing.T) {
	t.Cleanup(ClearRegisteredSecrets)
	v := New()
	values := []string{"http://a:Birinci26@x", "http://b:Ikinci26@y", "http://c:Ucuncu26@z"}
	v.AddAddressSecrets(values...)
	if v.maskBuilds != 1 {
		t.Errorf("üç yeni adres: süzgeç %d kez kuruldu, 1 bekleniyordu", v.maskBuilds)
	}
	v.AddAddressSecrets(values...)
	if v.maskBuilds != 1 {
		t.Errorf("aynı değerler: süzgeç %d kez kuruldu, 1 bekleniyordu", v.maskBuilds)
	}
}

// TestContextualAddressMask: bağlamlı maske (19.4, K1a). Değer kaydı yokken basılan metindeki her
// adresin kimlik kısmı bağlamından tanınır: ':' varsa ilk ':'den sonrası uzunluğuna bakılmadan, ':'
// yoksa 6 karakter (rune) ve uzun kimlik kısmı örtülür. Kaçışsız '/' ya da '#' taşıyan parola
// kapsanmaz; boşluksuz JSON'da bölge son '@'ye kadar sürer (fazla maske, sabitlenir).
func TestContextualAddressMask(t *testing.T) {
	t.Cleanup(ClearRegisteredSecrets)
	for _, tc := range []struct {
		name, text, want string
	}{
		{"kısa parola", "postgres://u:ab@db/ana", "postgres://u:***@db/ana"},
		{"sayı parola", "http://u:1234@h", "http://u:***@h"},
		{"belirteç", "https://ghp_abc123def@github.com/k/d", "https://***@github.com/k/d"},
		{"belirteç rune sayılır", "https://şşşşşş@h", "https://***@h"},
		{"5 karakterlik belirteç", "https://ghp_a@github.com/k/d", "https://ghp_a@github.com/k/d"},
		{"git", "ssh://git@github.com/k/d", "ssh://git@github.com/k/d"},
		{"yolda '@'", "https://www.npmjs.com/package/@scope/x", "https://www.npmjs.com/package/@scope/x"},
		{"kaçışsız '/'", "http://ali:Vek/il2026@x", "http://ali:Vek/il2026@x"},
		{"kaçışsız '#'", "http://ali:Vek#il2026@x", "http://ali:Vek#il2026@x"},
		{"boş parola", "http://ali:@vekil", "http://ali:@vekil"},
		{"son '@'", "http://u:Vekil@2026@host", "http://u:***@host"},
		{"sekme ve satır sonu", "http://a:pw@x\thttp://b:qq@y\nhttp://c:rr@z", "http://a:***@x\thttp://b:***@y\nhttp://c:***@z"},
		{"iç içe ://", "x://a://b:pw@c", "x://a://b:***@c"},
		{"JSON bedeli", `["http://a:pw@h","u@v"]`, `["http://a:***@v"]`},
		{"'://'sız bağlantı dizgisi", "root:Vekil2026@tcp(db:3306)/app", "root:Vekil2026@tcp(db:3306)/app"},
		{"kaçışlı parola, listenin metni", `["http://u:a\"b@h", "postgres://u:x\\y@db"]`, `["http://u:***@h", "postgres://u:***@db"]`},
		{"boş metin", "", ""},
		{"yalnız ://", "://", "://"},
	} {
		if got := New().Mask(tc.text); got != tc.want {
			t.Errorf("%s: %q → %q, %q bekleniyordu", tc.name, tc.text, got, tc.want)
		}
	}
	plain := strings.Repeat("a", 1<<20)
	if got := New().Mask(plain); got != plain {
		t.Error("adressiz 1 MiB'lık metin değişmemeli")
	}
	var many strings.Builder
	for i := range 10000 {
		fmt.Fprintf(&many, "http://u:p%d@h ", i)
	}
	got := New().Mask(many.String())
	if n := strings.Count(got, "u:***@h "); n != 10000 || strings.Contains(got, "u:p") {
		t.Errorf("10 000 adres: %d örtüldü", n)
	}
	// Bağlamdan tanınan parola kaydedilmez: aynı parola tek başına geçince açık kalır.
	v := New()
	if got := v.Mask("http://u:Vekil2026@h ve Vekil2026"); got != "http://u:***@h ve Vekil2026" {
		t.Errorf("kayıtsız parola: %q", got)
	}
	if v.HasSecrets() {
		t.Error("bağlam gizli değer kaydetmemeli")
	}
}
