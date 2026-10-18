package motor

import (
	"strings"
	"testing"

	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
	"tr_dili/pkg/sozluk"
	"tr_dili/pkg/vm"
)

// Tüm motorlar yüklüyken genel kapsamdaki her yerleşiğin sözlükte bir yerleşik kodu
// (G/T/V/W) olmalı ve kayıtlı ad o kodun adı olmalı; sözlükteki her yerleşik kodu da
// bütün yazımlarıyla (eş yazımlar dahil) kayıtlı olmalı. Böylece sözlükte adı olmayan
// bir yerleşik kaydedilemez, sözlükte de kaydı olmayan bir yerleşik kodu kalamaz.
func TestYerlesiklerSozlukleTutarli(t *testing.T) {
	m := vm.New()
	for _, ad := range m.GlobalEnv.Names() {
		d, _ := m.GlobalEnv.Get(ad)
		b, ok := d.(*vm.BuiltinVal)
		if !ok {
			continue
		}
		k, ok := sozluk.Bul(sozluk.AlanGenel, ad)
		if !ok || !k.Yerlesik() {
			t.Errorf("kayıtlı yerleşik %q sözlükte bir yerleşik kodu değil", ad)
			continue
		}
		if asil, _ := sozluk.AsilAdAlanda(sozluk.AlanGenel, ad); b.Name != asil {
			t.Errorf("%q (%s) yerleşiğinin adı %q, sözlükte %q", ad, k, b.Name, asil)
		}
	}
	for _, k := range sozluk.Kodlar() {
		if !k.Yerlesik() {
			continue
		}
		b := m.Yerlesik(k)
		if b == nil {
			t.Errorf("sözlükteki %s (%s) kayıtlı değil", k, sozluk.Ad(k))
			continue
		}
		for _, y := range sozluk.Yazimlar(k) {
			d, _ := m.GlobalEnv.Get(y)
			if sozluk.EsYazimi(k, y) {
				// Eş yazım aynı fonksiyonu kendi adıyla taşır.
				if e, ok := d.(*vm.BuiltinVal); !ok || e == b {
					t.Errorf("%s eş yazımı %q ayrı adlı bir yerleşiğe bağlı olmalı", k, y)
				}
				continue
			}
			if d != b {
				t.Errorf("%s yazımı %q aynı yerleşiğe bağlı değil", k, y)
			}
		}
	}
}

// calistirAyni TR kodunu verilen VM'de çalıştırır (genel tanımlar korunur).
func calistirAyni(t *testing.T, m *vm.VM, kod string) (vm.Value, error) {
	t.Helper()
	p := parser.New(lexer.New(kod))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v\n%s", p.Errors(), kod)
	}
	return m.Run(prog)
}

// Sözlükteki her üye kodu (Y) bütün yazımlarıyla en az bir tipte çalışmalı: kodu olup da
// hiçbir switch'te karşılanmayan (ya da yalnızca bir yazımı karşılanan) üye kalmamalı.
// Veri kümesi ve okuyucu üyeleri (Y600-Y602) dizin gerektirdiği için pkg/motor/veri
// testlerinde denenir.
func TestUyelerSozlukleTutarli(t *testing.T) {
	m := vm.New()
	kur := strings.Join([]string{
		`değişken u_metin = "abc"`,
		`değişken u_liste = [1, 2]`,
		`değişken u_sozluk = {"a": 1}`,
		`değişken u_tensor = tensör([[1.0, 2.0]], türev_izle: doğru)`,
		`değişken u_iyilestirici = SGD([u_tensor])`,
		`değişken u_model = sinir_ağı([2, 1])`,
		`değişken u_vt = veritabanı_aç(":bellek:")`,
		`değişken u_web = web_sunucu(port: 0)`,
		`değişken u_arayuz = arayüz_oluştur("deneme")`,
		`değişken u_parcalayici = parçalayıcı_yükle("../parcalayici/testdata/p_bayt_bpe.json")`,
	}, "\n")
	if _, err := calistirAyni(t, m, kur); err != nil {
		t.Fatal(err)
	}
	ornekler := []string{"u_metin", "u_liste", "u_sozluk", "u_tensor", "u_iyilestirici", "u_model", "u_vt", "u_web", "u_arayuz", "u_parcalayici"}
	for _, k := range sozluk.Kodlar() {
		if k.Alan() != sozluk.AlanUye || (k >= sozluk.Y+600 && k <= sozluk.Y+602) {
			continue
		}
		for _, y := range sozluk.Yazimlar(k) {
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
		{"u_vt.tablo_olustur", "'tablo_olustur' TR'de yok; 'tablo_oluştur' yazmak mı istediniz?"},
		{"u_web.baslat", "'baslat' TR'de yok; 'başlat' yazmak mı istediniz?"},
		{"u_arayuz.baslik_ekle", "'baslik_ekle' TR'de yok; 'başlık_ekle' yazmak mı istediniz?"},
		{"u_model.egit", "'egit' TR'de yok; 'eğit' yazmak mı istediniz?"},
	} {
		if _, err := calistirAyni(t, m, c.kod); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu: %v", c.kod, c.want, err)
		}
	}
	if _, err := calistirAyni(t, m, "u_vt.baslik_ekle"); err == nil || strings.Contains(err.Error(), "başlık_ekle") {
		t.Errorf("veritabanında başlık_ekle önerilmemeli: %v", err)
	}
	// Eş yazımlar yalnızca bugün kabul edildikleri tiplerde çalışır.
	for _, kod := range []string{"u_liste.kayıt_ekle", "u_web.göster", "u_web.goster", "u_metin.kayit_ekle"} {
		if _, err := calistirAyni(t, m, kod); err == nil || !strings.Contains(err.Error(), "bulunamadı") {
			t.Errorf("%s hata vermeliydi: %v", kod, err)
		}
	}
}
