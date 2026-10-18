package motor

import (
	"path/filepath"
	"strings"
	"testing"

	"tr_dili/internal/depo"
	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/sozcuk"
	"tr_dili/internal/dil/yurut"
)

// Tüm motorlar yüklüyken genel kapsamdaki her yerleşiğin sözlükte bir yerleşik kodu
// (G/T/V/W) olmalı ve kayıtlı ad o kodun adı olmalı; sözlükteki her yerleşik kodu da
// adıyla kayıtlı olmalı. Böylece sözlükte adı olmayan
// bir yerleşik kaydedilemez, sözlükte de kaydı olmayan bir yerleşik kodu kalamaz.
func TestYerlesiklerSozlukleTutarli(t *testing.T) {
	m := yurut.New()
	for _, ad := range m.GlobalEnv.Names() {
		d, _ := m.GlobalEnv.Get(ad)
		b, ok := d.(*yurut.BuiltinVal)
		if !ok {
			continue
		}
		k, ok := kod.Bul(kod.AlanGenel, ad)
		if !ok || !k.Yerlesik() {
			t.Errorf("kayıtlı yerleşik %q sözlükte bir yerleşik kodu değil", ad)
			continue
		}
		if asil, _ := kod.AsilAdAlanda(kod.AlanGenel, ad); b.Name != asil {
			t.Errorf("%q (%s) yerleşiğinin adı %q, sözlükte %q", ad, k, b.Name, asil)
		}
	}
	for _, k := range kod.Kodlar() {
		if !k.Yerlesik() {
			continue
		}
		b := m.Yerlesik(k)
		if b == nil {
			t.Errorf("sözlükteki %s (%s) kayıtlı değil", k, kod.Ad(k))
			continue
		}
		for _, y := range kod.Yazimlar(k) {
			if d, _ := m.GlobalEnv.Get(y); d != b {
				t.Errorf("%s yazımı %q aynı yerleşiğe bağlı değil", k, y)
			}
		}
	}
}

// calistirAyni TR kodunu verilen VM'de çalıştırır (genel tanımlar korunur).
func calistirAyni(t *testing.T, m *yurut.VM, kd string) (yurut.Value, error) {
	t.Helper()
	p := ayristir.New(sozcuk.New(kd))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v\n%s", p.Errors(), kd)
	}
	return m.Run(prog)
}

// Kod listesindeki her üye kodu (Y) bütün yazımlarıyla en az bir tipte çalışmalı: kodu
// olup da hiçbir switch'te karşılanmayan (ya da yalnızca bir yazımı karşılanan) üye
// kalmamalı. Veri kümesi ve okuyucu üyeleri (Y600-Y602) dizin gerektirdiği için
// internal/motor/veri testlerinde denenir.
func TestUyelerSozlukleTutarli(t *testing.T) {
	m := yurut.New()
	kur := strings.Join([]string{
		`değişken u_metin = "abc"`,
		`değişken u_liste = [1, 2]`,
		`değişken u_sozluk = {"a": 1}`,
		`değişken u_tensor = tensör([[1.0, 2.0]], türev_izle: doğru)`,
		`değişken u_iyilestirici = SGD([u_tensor])`,
		`değişken u_parcalayici = parçalayıcı_yükle("` + filepath.ToSlash(depo.Yol("internal/hesap/parca/testdata/p_bayt_bpe.json")) + `")`,
	}, "\n")
	if _, err := calistirAyni(t, m, kur); err != nil {
		t.Fatal(err)
	}
	ornekler := []string{"u_metin", "u_liste", "u_sozluk", "u_tensor", "u_iyilestirici", "u_parcalayici"}
	for _, k := range kod.Kodlar() {
		if k.Alan() != kod.AlanUye || (k >= kod.Y600 && k <= kod.Y602) {
			continue
		}
		for _, y := range kod.Yazimlar(k) {
			bulundu := false
			for _, o := range ornekler {
				if _, err := calistirAyni(t, m, o+"."+y); err == nil {
					bulundu = true
					break
				}
			}
			if !bulundu {
				t.Errorf("üye %s (%q) hiçbir tipte karşılanmıyor", k, y)
			}
		}
	}
	// Motor tiplerinde ASCII yazılmış üye adı hata verir ve Türkçe adı önerir; öneri
	// yalnızca o tipte geçerli bir üye için yapılır.
	for _, c := range []struct{ kod, want string }{
		{"u_tensor.sekil", "Bunu mu demek istediniz: 'şekil'?"},
		{"u_iyilestirici.sifirla", "Bunu mu demek istediniz: 'sıfırla'?"},
		{"u_parcalayici.coz", "Bunu mu demek istediniz: 'çöz'?"},
	} {
		if _, err := calistirAyni(t, m, c.kod); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu: %v", c.kod, c.want, err)
		}
	}
	if _, err := calistirAyni(t, m, "u_tensor.kodla"); err == nil || strings.Contains(err.Error(), "yazmak mı istediniz") {
		t.Errorf("tensörde kodla önerilmemeli: %v", err)
	}
	// Kaldırılan eş yazımlar hata verir ve asıl adı önerir.
	for _, c := range []struct{ kod, want string }{
		{"u_liste.kayıt_ekle", "'kayıt_ekle' TR'de yok; 'ekle' yazmak mı istediniz?"},
		{"u_tensor.türev", "'gradyan'"},
		{"u_parcalayici.sözlük_boyutu", "'sözlük_boyutu' TR'de yok; 'dağarcık_boyutu' yazmak mı istediniz?"},
		{"u_parcalayici.token_kimliği", "'token_kimliği' TR'de yok; 'parça_kimliği' yazmak mı istediniz?"},
		{`u_parcalayici.kodla("a", özel_tokenlar: yanlış)`, "Bunu mu demek istediniz: 'özel_parçalar'?"},
	} {
		if _, err := calistirAyni(t, m, c.kod); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu: %v", c.kod, c.want, err)
		}
	}
	// Dilden kaldırılan yerleşikler, üyeler ve onların eski adları: "TR'den kaldırıldı".
	for _, c := range []struct{ kod, want string }{
		{`veritabanı_aç(":bellek:")`, "'veritabanı_aç' TR'den kaldırıldı"},
		{`veritabanı(":bellek:")`, "'veritabanı' TR'den kaldırıldı"},
		{`web_sunucu(port: 8080)`, "'web_sunucu' TR'den kaldırıldı"},
		{`arayüz_oluştur("a")`, "'arayüz_oluştur' TR'den kaldırıldı"},
		{`pencere_oluştur("a")`, "'pencere_oluştur' TR'den kaldırıldı"},
		{`sinir_ağı([2, 1])`, "'sinir_ağı' TR'den kaldırıldı"},
		{`kayıp_mse([1], [1])`, "'kayıp_mse' TR'den kaldırıldı"},
		{`yz_sor("a")`, "'yz_sor' TR'den kaldırıldı"},
		{`nokta_çarpım([1], [1])`, "'nokta_çarpım' TR'den kaldırıldı"},
		{"u_liste.kaydet", "'kaydet' TR'den kaldırıldı"},
		{"u_tensor.eğit", "'eğit' TR'den kaldırıldı"},
	} {
		if _, err := calistirAyni(t, m, c.kod); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu: %v", c.kod, c.want, err)
		}
	}
	// Kaldırılan adlar sıradan adlardır.
	if v, err := calistirAyni(t, m, "değişken web_sunucu = 3\nweb_sunucu + 1"); err != nil || v.Inspect() != "4" {
		t.Errorf("kaldırılan ad değişken adı olabilmeli: %v %v", v, err)
	}
}
