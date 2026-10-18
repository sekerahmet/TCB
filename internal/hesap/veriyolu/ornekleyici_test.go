package veriyolu

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func toplulariAl(t *testing.T, o *Ornekleyici, n int) [][]int32 {
	t.Helper()
	var ti TopluIs
	var sonuc [][]int32
	for i := 0; i < n; i++ {
		if err := o.Sonraki(&ti); err != nil {
			t.Fatal(err)
		}
		sonuc = append(sonuc, append([]int32(nil), ti.Veri...))
	}
	return sonuc
}

func TestKumeTanima(t *testing.T) {
	d := kodluKume(t, 100, 250, 7)
	_ = os.WriteFile(filepath.Join(d, "notlar.txt"), []byte("x"), 0o644)
	k, err := KumeAc(d, KumeSecenekleri{})
	if err != nil {
		t.Fatal(err)
	}
	if k.ParcaSayisi() != 3 || k.ToplamToken() != 357 || k.Meta() != nil {
		t.Fatalf("küme yanlış: %d parça, %d token", k.ParcaSayisi(), k.ToplamToken())
	}
	ps := k.Parcalar()
	if ps[0].Ad != parcaAdi(0) || ps[1].Token != 250 {
		t.Fatalf("parça bilgisi yanlış: %+v", ps)
	}
	// Tek dosya da küme olabilir.
	k1, err := KumeAc(ps[1].Yol, KumeSecenekleri{})
	if err != nil || k1.ParcaSayisi() != 1 || k1.ToplamToken() != 250 {
		t.Fatalf("tek dosyalı küme: %v", err)
	}
	if _, err := KumeAc(t.TempDir(), KumeSecenekleri{}); err == nil {
		t.Fatal("boş dizin kabul edildi")
	}
}

func TestKumeMeta(t *testing.T) {
	kok := t.TempDir()
	tr := filepath.Join(kok, "train")
	_ = os.Mkdir(tr, 0o755)
	u16Yaz(t, tr, "a.npy", sozdeU16(300, 1))
	u16Yaz(t, tr, "b.npy", sozdeU16(500, 2))
	meta := map[string]any{
		"tokenizer": "/bir/yer/tok.json",
		"files":     map[string]any{"a": map[string]any{"tokens": 800}},
		"parts":     map[string]any{"a.npy": map[string]any{"tokens": 300}, "b.npy": map[string]any{"tokens": 500}},
		"valid":     map[string]any{"tokens": 10, "docs": 2},
		"progress":  map[string]any{},
	}
	b, _ := json.Marshal(meta)
	_ = os.WriteFile(filepath.Join(kok, "meta.json"), b, 0o644)
	k, err := KumeAc(tr, KumeSecenekleri{})
	if err != nil {
		t.Fatal(err)
	}
	if k.Meta() == nil || k.Meta().TokenizerYolu() != "/bir/yer/tok.json" || k.Meta().Valid.Tokens != 10 {
		t.Fatal("meta okunmadı")
	}
	if k.ToplamToken() != 800 {
		t.Fatal("toplam yanlış")
	}
	// meta.json ile başlık uyuşmazsa parça açılırken hata.
	meta["parts"] = map[string]any{"a.npy": map[string]any{"tokens": 301}}
	b, _ = json.Marshal(meta)
	_ = os.WriteFile(filepath.Join(kok, "meta.json"), b, 0o644)
	k, err = KumeAc(tr, KumeSecenekleri{})
	if err != nil {
		t.Fatal(err)
	}
	if k.Parcalar()[1].Token != 500 { // b.npy meta'da yok → başlıktan
		t.Fatal("eksik meta kaydı başlıktan okunmadı")
	}
	if _, err := k.ParcaAc(context.Background(), 0); err == nil || !strings.Contains(err.Error(), "meta.json") {
		t.Fatalf("uyuşmazlık hatası bekleniyordu: %v", err)
	}
	k2, err := KumeAc(tr, KumeSecenekleri{MetaYok: true})
	if err != nil || k2.Meta() != nil || k2.ToplamToken() != 800 {
		t.Fatal("MetaYok çalışmadı")
	}
}

func TestOrnekleyiciTekrarUretilebilir(t *testing.T) {
	parcaSizintiDenetle(t)
	d := kodluKume(t, 1000, 2000, 1500, 333, 10)
	k, _ := KumeAc(d, KumeSecenekleri{})
	sec := OrnekleyiciSecenekleri{B: 4, T: 16, Tohum: 42}
	o1, err := k.Ornekleyici(sec)
	if err != nil {
		t.Fatal(err)
	}
	o2, _ := k.Ornekleyici(sec)
	donemBasina := int(o1.DonemPencereSayisi()) / 4
	a := toplulariAl(t, o1, 3*donemBasina)
	b := toplulariAl(t, o2, 3*donemBasina)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("aynı tohum farklı toplu işler verdi")
	}
	if o1.Donem() < 2 {
		t.Fatalf("üç dönem geçilmedi: %d", o1.Donem())
	}
	sec.Tohum = 43
	o3, _ := k.Ornekleyici(sec)
	c := toplulariAl(t, o3, 3*donemBasina)
	if reflect.DeepEqual(a, c) {
		t.Fatal("farklı tohum aynı toplu işleri verdi")
	}
	// Eşleme kapalı küme aynı veriyi vermeli.
	kr, _ := KumeAc(d, KumeSecenekleri{Parca: ParcaSecenekleri{EslemeKapali: true}})
	sec.Tohum = 42
	o4, _ := kr.Ornekleyici(sec)
	if !reflect.DeepEqual(a, toplulariAl(t, o4, 3*donemBasina)) {
		t.Fatal("ReadAt yolu eşleme yolundan farklı")
	}
	for _, o := range []*Ornekleyici{o1, o2, o3, o4} {
		_ = o.Kapat()
	}
}

func TestOrnekleyiciPencereSinirVeKaydirma(t *testing.T) {
	parcaSizintiDenetle(t)
	uzunluklar := []int{1000, 2000, 1500, 333, 10, 17}
	d := kodluKume(t, uzunluklar...)
	k, _ := KumeAc(d, KumeSecenekleri{})
	const B, T = 3, 16
	o, err := k.Ornekleyici(OrnekleyiciSecenekleri{B: B, T: T, Tohum: 9})
	if err != nil {
		t.Fatal(err)
	}
	defer o.Kapat()
	beklenen := 0
	for _, n := range uzunluklar {
		beklenen += n / (T + 1)
	}
	if int(o.DonemPencereSayisi()) != beklenen {
		t.Fatalf("dönem pencere sayısı %d != %d", o.DonemPencereSayisi(), beklenen)
	}
	var ti TopluIs
	var girdi, hedef []int32
	for adim := 0; adim < 3*beklenen/B; adim++ {
		if err := o.Sonraki(&ti); err != nil {
			t.Fatal(err)
		}
		if ti.B != B || ti.T != T || len(ti.Veri) != B*(T+1) {
			t.Fatalf("toplu iş boyutu yanlış: %d×%d, %d", ti.B, ti.T, len(ti.Veri))
		}
		girdi, hedef = ti.Ayir(girdi, hedef)
		for r := 0; r < B; r++ {
			s := ti.Satir(r)
			p, bas := int(s[0])/1_000_000, int(s[0])%1_000_000
			for j, v := range s {
				if int(v) != p*1_000_000+bas+j {
					t.Fatalf("pencere bitişik değil ya da parça sınırını aşıyor: %v", s)
				}
			}
			if bas+T+1 > uzunluklar[p] {
				t.Fatalf("pencere parça sonunu aşıyor: parça %d, baş %d", p, bas)
			}
			for j := 0; j < T; j++ {
				if ti.Hedef(r)[j] != ti.Girdi(r)[j]+1 || hedef[r*T+j] != girdi[r*T+j]+1 || girdi[r*T+j] != s[j] {
					t.Fatal("hedef girdiden bir kaydırılmış değil")
				}
			}
		}
	}
	// Tek bir dönemi ayrıca denetle: tam bir dönemde her parçadaki
	// pencereler üst üste binmemeli ve sayıları floor(n/(T+1)) olmalı.
	// Her grup boyu için (1 = parça parça, 10 = tüm parçalar tek grup).
	for _, grup := range []int{1, 2, 4, 10} {
		kapsamDenetle(t, k, uzunluklar, T, grup)
	}
	// Dönemler arasında parça sırası değişmeli (6 parçada aynı kalma
	// olasılığı tohuma bağlı ama bu tohumda değişir).
	var siralar [][]int
	for donem := 0; donem < 3; donem++ {
		siralar = append(siralar, o.donemSirasi(donem, nil))
	}
	if reflect.DeepEqual(siralar[0], siralar[1]) && reflect.DeepEqual(siralar[1], siralar[2]) {
		t.Fatal("parça sırası dönemler arasında karışmıyor")
	}
}

func kapsamDenetle(t *testing.T, k *Kume, uzunluklar []int, T, grup int) {
	t.Helper()
	o2, err := k.Ornekleyici(OrnekleyiciSecenekleri{B: 1, T: T, Tohum: 9, GrupBoyu: grup})
	if err != nil {
		t.Fatal(err)
	}
	defer o2.Kapat()
	beklenen := int(o2.DonemPencereSayisi())
	sayi := map[int]int{}
	kapsam := map[int]map[int]bool{}
	var bir TopluIs
	for i := 0; i < beklenen; i++ {
		if err := o2.Sonraki(&bir); err != nil {
			t.Fatal(err)
		}
		p, bas := int(bir.Veri[0])/1_000_000, int(bir.Veri[0])%1_000_000
		sayi[p]++
		if kapsam[p] == nil {
			kapsam[p] = map[int]bool{}
		}
		for j := 0; j <= T; j++ {
			if kapsam[p][bas+j] {
				t.Fatalf("bir dönemde pencereler üst üste biniyor: parça %d konum %d", p, bas+j)
			}
			kapsam[p][bas+j] = true
		}
	}
	if err := o2.Sonraki(&bir); err != nil || o2.Donem() != 1 {
		t.Fatalf("dönem sonunda bir sonraki pencere 1. dönemden olmalı: %d %v", o2.Donem(), err)
	}
	for p, n := range uzunluklar {
		if sayi[p] != n/(T+1) {
			t.Fatalf("grup %d, parça %d: %d pencere, beklenen %d", grup, p, sayi[p], n/(T+1))
		}
	}
}

func TestOrnekleyiciGrupKarisimi(t *testing.T) {
	parcaSizintiDenetle(t)
	d := kodluKume(t, 4000, 4000, 4000, 4000, 4000, 4000, 4000, 4000)
	k, _ := KumeAc(d, KumeSecenekleri{})
	// Toplu iş başına farklı parça sayısının ortalaması.
	ortalama := func(grup int) float64 {
		o, err := k.Ornekleyici(OrnekleyiciSecenekleri{B: 8, T: 15, Tohum: 21, GrupBoyu: grup})
		if err != nil {
			t.Fatal(err)
		}
		defer o.Kapat()
		if want := (8 + grup - 1) / grup; o.GrupSayisi() != want {
			t.Fatalf("grup sayısı %d, beklenen %d", o.GrupSayisi(), want)
		}
		var ti TopluIs
		toplam, adim := 0, int(o.DonemPencereSayisi())/8
		for i := 0; i < adim; i++ {
			if err := o.Sonraki(&ti); err != nil {
				t.Fatal(err)
			}
			farkli := map[int32]bool{}
			for r := 0; r < ti.B; r++ {
				farkli[ti.Satir(r)[0]/1_000_000] = true
			}
			toplam += len(farkli)
		}
		return float64(toplam) / float64(adim)
	}
	tek, dort := ortalama(1), ortalama(4)
	if tek > 1.1 {
		t.Fatalf("grup 1'de toplu işler çoğunlukla tek parçadan gelmeli: %.2f", tek)
	}
	if dort < 3 {
		t.Fatalf("grup 4'te satırlar birden çok parçadan gelmeli: ortalama %.2f farklı parça", dort)
	}
	// Tekrar üretilebilirlik ve durumdan sürdürme gruplarla da korunur.
	sec := OrnekleyiciSecenekleri{B: 8, T: 15, Tohum: 21, GrupBoyu: 3}
	o1, _ := k.Ornekleyici(sec)
	defer o1.Kapat()
	ilk := toplulariAl(t, o1, 300)
	o2, _ := k.Ornekleyici(sec)
	defer o2.Kapat()
	if !reflect.DeepEqual(ilk, toplulariAl(t, o2, 300)) {
		t.Fatal("gruplu örnekleme tekrar üretilemiyor")
	}
	var ti TopluIs
	_ = o1.Sonraki(&ti)
	durum := ti.Durum
	sonra := toplulariAl(t, o1, 100)
	sec.Baslangic = &durum
	o3, err := k.Ornekleyici(sec)
	if err != nil {
		t.Fatal(err)
	}
	defer o3.Kapat()
	if !reflect.DeepEqual(sonra, toplulariAl(t, o3, 100)) {
		t.Fatal("gruplu örneklemede durumdan sürdürme bozuk")
	}
	if _, err := k.Ornekleyici(OrnekleyiciSecenekleri{B: 1, T: 5, GrupBoyu: -1}); err == nil {
		t.Fatal("negatif grup boyu kabul edildi")
	}
}

func TestOrnekleyiciDurumdanSurdurme(t *testing.T) {
	parcaSizintiDenetle(t)
	d := kodluKume(t, 500, 800, 300)
	k, _ := KumeAc(d, KumeSecenekleri{})
	sec := OrnekleyiciSecenekleri{B: 5, T: 8, Tohum: 3}
	o, _ := k.Ornekleyici(sec)
	defer o.Kapat()
	var ti TopluIs
	var durum Durum
	for i := 0; i < 37; i++ {
		if err := o.Sonraki(&ti); err != nil {
			t.Fatal(err)
		}
		durum = ti.Durum
	}
	sonra := toplulariAl(t, o, 50)
	sec.Baslangic = &durum
	o2, err := k.Ornekleyici(sec)
	if err != nil {
		t.Fatal(err)
	}
	defer o2.Kapat()
	if !reflect.DeepEqual(sonra, toplulariAl(t, o2, 50)) {
		t.Fatal("durumdan sürdürme aynı toplu işleri vermedi")
	}
	sec.Baslangic = &Durum{Sira: 99}
	if _, err := k.Ornekleyici(sec); err == nil {
		t.Fatal("geçersiz durum kabul edildi")
	}
}

func TestOrnekleyiciHatalar(t *testing.T) {
	d := kodluKume(t, 10, 12)
	k, _ := KumeAc(d, KumeSecenekleri{})
	if _, err := k.Ornekleyici(OrnekleyiciSecenekleri{B: 1, T: 50}); err == nil {
		t.Fatal("hiçbir parçaya sığmayan T kabul edildi")
	}
	if _, err := k.Ornekleyici(OrnekleyiciSecenekleri{B: 0, T: 5}); err == nil {
		t.Fatal("B=0 kabul edildi")
	}
	o, _ := k.Ornekleyici(OrnekleyiciSecenekleri{B: 1, T: 5})
	ctx, iptal := context.WithCancel(context.Background())
	iptal()
	var ti TopluIs
	if err := o.Doldur(ctx, &ti); !errors.Is(err, context.Canceled) {
		t.Fatalf("iptal hatası bekleniyordu: %v", err)
	}
	_ = o.Kapat()
	if err := o.Sonraki(&ti); !errors.Is(err, ErrKapali) {
		t.Fatalf("ErrKapali bekleniyordu: %v", err)
	}
}

func TestSiraliOkuyucu(t *testing.T) {
	parcaSizintiDenetle(t)
	uzunluklar := []int{100, 5, 37}
	d := kodluKume(t, uzunluklar...)
	k, _ := KumeAc(d, KumeSecenekleri{})
	const T = 8
	so, err := k.Sirali(SiraliSecenekleri{B: 5, T: T})
	if err != nil {
		t.Fatal(err)
	}
	defer so.Kapat()
	var beklenen [][]int32
	for p, n := range uzunluklar {
		for bas := 0; bas+T+1 <= n; bas += T {
			s := make([]int32, T+1)
			for j := range s {
				s[j] = int32(p*1_000_000 + bas + j)
			}
			beklenen = append(beklenen, s)
		}
	}
	if so.PencereSayisi() != int64(len(beklenen)) {
		t.Fatalf("pencere sayısı %d != %d", so.PencereSayisi(), len(beklenen))
	}
	for tur := 0; tur < 2; tur++ {
		var alinan [][]int32
		var ti TopluIs
		for {
			err := so.Doldur(context.Background(), &ti)
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			for r := 0; r < ti.B; r++ {
				alinan = append(alinan, append([]int32(nil), ti.Satir(r)...))
			}
		}
		if !reflect.DeepEqual(alinan, beklenen) {
			t.Fatalf("sıralı okuma yanlış:\n%v\n%v", alinan, beklenen)
		}
		if len(beklenen)%5 == 0 {
			t.Fatal("test son toplu işin eksik olmasını denetlemeli")
		}
		if err := so.Doldur(context.Background(), &ti); err != io.EOF {
			t.Fatal("bitişten sonra io.EOF bekleniyordu")
		}
		_ = so.Basa()
	}
	// Adım T+1: üst üste binmeyen pencereler.
	so2, _ := k.Sirali(SiraliSecenekleri{B: 100, T: T, Adim: T + 1})
	defer so2.Kapat()
	var ti TopluIs
	if err := so2.Doldur(context.Background(), &ti); err != nil {
		t.Fatal(err)
	}
	if ti.B != 100/9+37/9 || ti.Satir(1)[0] != 9 {
		t.Fatalf("adım yanlış: B=%d, %v", ti.B, ti.Satir(1))
	}
}
