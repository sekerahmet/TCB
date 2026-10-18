package web

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
	"tr_dili/pkg/kod"
	"unicode/utf8"

	"tr_dili/pkg/motor/internal/jsonyaz"
	"tr_dili/pkg/vm"
)

// Bu dosya TR programlarının dışarıya HTTP isteği atan yerleşiklerini (web_al,
// web_gönder) ve onların ortak istemcisini içerir. yz_sor gibi başka motorlar da aynı
// istemciyi (Iste) kullanır; böylece boyut sınırı, zaman aşımı ve hata biçimi tek yerde
// tanımlanır.

// EnFazlaGovde bir HTTP yanıt gövdesinin okunabilecek en büyük boyutudur. Daha büyük
// yanıtlar belleği tüketmek yerine açık bir hatayla reddedilir. Testler küçültebilir.
var EnFazlaGovde int64 = 50 << 20

// webZamanAsimi web_al ve web_gönder isteklerinin toplam süre sınırıdır (bağlanma,
// gönderme, gövdeyi okuma ve JSON gövdeyi TR değerine çevirme dahil). Testler kısaltabilir.
var webZamanAsimi = 30 * time.Second

// EnFazlaJSONDegeri bir JSON gövdesinin (web_al/web_gönder yanıtı, web sunucusuna gelen
// istek) TR değerine çevrilebilecek en çok değer sayısıdır. Gövde boyutu sınırı tek başına
// belleği sınırlamaz: 50 MB'lık "[0,0,…]" 26 milyon değer, yaklaşık 1 GB bellek demektir.
// 5 milyon değer birkaç yüz MB'tır. Testler küçültebilir.
var EnFazlaJSONDegeri = 5_000_000

// jsonDegerSayisi JSON metnindeki değer sayısının bir üst sınırını tek geçişte, ayırmadan
// sayar: metin dışındaki her virgül ve açılan parantez bir değerdir.
func jsonDegerSayisi(b []byte) int {
	n := 1
	metinde, kacis := false, false
	for _, c := range b {
		if metinde {
			switch {
			case kacis:
				kacis = false
			case c == '\\':
				kacis = true
			case c == '"':
				metinde = false
			}
			continue
		}
		switch c {
		case '"':
			metinde = true
		case ',', '[', '{':
			n++
		}
	}
	return n
}

// jsonGibi gövdenin bir JSON nesnesi ya da dizisi gibi başlayıp başlamadığını söyler.
func jsonGibi(b []byte) bool {
	t := bytes.TrimLeft(b, " \t\r\n")
	return len(t) > 0 && (t[0] == '{' || t[0] == '[')
}

// JSONSinirli JSON gövdesini EnFazlaJSONDegeri sınırıyla TR değerine çevirir. Sınır aşılırsa
// çevirmeye hiç başlanmadan açık bir hata döner.
func JSONSinirli(b []byte) (vm.Value, error) {
	if jsonGibi(b) {
		if n := jsonDegerSayisi(b); n > EnFazlaJSONDegeri {
			return nil, &IstekHatasi{
				Neden: fmt.Sprintf("JSON'da çok fazla değer var (yaklaşık %d; en fazla %d çevrilir)", n, EnFazlaJSONDegeri),
				Buyuk: true,
			}
		}
	}
	return vm.JSONToTR(b)
}

// Yanit bir HTTP isteğinin durum kodu ve (sınırla okunmuş) gövdesidir.
type Yanit struct {
	Durum int
	Metin string // Durum kodunun açıklaması, ör. "404 Not Found"
	Govde []byte
}

// Basarili durum kodunun 2xx olup olmadığını söyler.
func (y *Yanit) Basarili() bool { return y.Durum >= 200 && y.Durum < 300 }

// GovdeOzeti hata mesajlarında gösterilecek kısa, tek satırlık gövde özetidir.
func (y *Yanit) GovdeOzeti(enFazla int) string {
	s := strings.Join(strings.Fields(string(y.Govde)), " ")
	if len(s) <= enFazla {
		return s
	}
	kes := enFazla
	for kes > 0 && !utf8.RuneStart(s[kes]) {
		kes--
	}
	return s[:kes] + "..."
}

// IstekHatasi ağ düzeyindeki bir hatadır (bağlanılamadı, zaman aşımı, gövde çok büyük).
// Durum kodu hataları bu tipte değildir; onları çağıran Yanit.Durum ile değerlendirir.
type IstekHatasi struct {
	Neden     string // Kullanıcıya gösterilecek Türkçe açıklama
	ZamanAsim bool
	Buyuk     bool
	Ic        error
}

func (e *IstekHatasi) Error() string { return e.Neden }
func (e *IstekHatasi) Unwrap() error { return e.Ic }

// Iste bir HTTP isteği atar ve yanıt gövdesini en çok EnFazlaGovde bayt okur. Ağ
// beklemesi sırasında yorumlayıcı kilidi bırakılır (makine nil değilse); böylece
// program kendi başlattığı sunucuya istek atabilir. govde nil ise gövdesiz istek atılır.
func Iste(makine *vm.VM, yontem, adres string, govde []byte, icerikTipi string, zamanAsimi time.Duration) (*Yanit, error) {
	var okuyucu io.Reader
	if govde != nil {
		okuyucu = bytes.NewReader(govde)
	}
	istek, err := http.NewRequest(yontem, adres, okuyucu)
	if err != nil {
		return nil, &IstekHatasi{Neden: fmt.Sprintf("geçersiz adres: %v", err), Ic: err}
	}
	if icerikTipi != "" {
		istek.Header.Set("Content-Type", icerikTipi)
	}
	istemci := &http.Client{Timeout: zamanAsimi}
	var yanit *Yanit
	calistir := func() { yanit, err = isteYurut(istemci, istek, zamanAsimi) }
	if makine != nil {
		makine.Blocking(calistir)
	} else {
		calistir()
	}
	return yanit, err
}

// isteYurut isteği gönderir ve gövdeyi sınırla okur.
func isteYurut(istemci *http.Client, istek *http.Request, zamanAsimi time.Duration) (*Yanit, error) {
	resp, err := istemci.Do(istek)
	if err != nil {
		return nil, agHatasi(err, zamanAsimi)
	}
	defer resp.Body.Close()
	sinir := EnFazlaGovde
	govde, err := io.ReadAll(io.LimitReader(resp.Body, sinir+1))
	if err != nil {
		return nil, agHatasi(err, zamanAsimi)
	}
	if int64(len(govde)) > sinir {
		return nil, &IstekHatasi{
			Neden: fmt.Sprintf("yanıt gövdesi çok büyük (en fazla %s okunabilir)", baytMetni(sinir)),
			Buyuk: true,
		}
	}
	return &Yanit{Durum: resp.StatusCode, Metin: resp.Status, Govde: govde}, nil
}

// agHatasi Go'nun ağ hatasını Türkçe açıklamalı bir IstekHatasi'na çevirir.
func agHatasi(err error, zamanAsimi time.Duration) error {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return &IstekHatasi{
			Neden:     fmt.Sprintf("zaman aşımı: %s içinde yanıt alınamadı", sureMetni(zamanAsimi)),
			ZamanAsim: true,
			Ic:        err,
		}
	}
	return &IstekHatasi{Neden: err.Error(), Ic: err}
}

// sureMetni süreyi Türkçe kısa biçimde yazar ("30 sn", "500 ms").
func sureMetni(d time.Duration) string {
	if d >= time.Second && d%time.Second == 0 {
		return fmt.Sprintf("%d sn", int64(d/time.Second))
	}
	return fmt.Sprintf("%d ms", d.Milliseconds())
}

// baytMetni bayt sayısını okunur biçimde yazar ("50 MB", "1 KB", "100 bayt").
func baytMetni(n int64) string {
	switch {
	case n >= 1<<20 && n%(1<<20) == 0:
		return fmt.Sprintf("%d MB", n>>20)
	case n >= 1<<10 && n%(1<<10) == 0:
		return fmt.Sprintf("%d KB", n>>10)
	}
	return fmt.Sprintf("%d bayt", n)
}

// webIstegi web_al ve web_gönder'in ortak gövdesidir: isteği atar, 2xx dışı durum
// kodunu açık hataya çevirir, gövde JSON ise TR değerine, değilse metne dönüştürür. İstek
// ve JSON çevirisi yorumlayıcı kilidi bırakılarak, toplam webZamanAsimi içinde yapılır
// (çeviri yeni değerler üretir, paylaşılan hiçbir TR değerine dokunmaz).
func webIstegi(makine *vm.VM, fn, yontem, adres string, govde []byte) (vm.Value, error) {
	icerikTipi := ""
	if govde != nil {
		icerikTipi = "application/json"
	}
	var (
		yanit   *Yanit
		cozulen vm.Value
		err     error
	)
	calistir := func() {
		bas := time.Now()
		yanit, err = Iste(nil, yontem, adres, govde, icerikTipi, webZamanAsimi)
		if err != nil || !yanit.Basarili() || !jsonGibi(yanit.Govde) {
			return
		}
		kalan := webZamanAsimi - time.Since(bas)
		type sonuc struct {
			v   vm.Value
			err error
		}
		bitti := make(chan sonuc, 1)
		g := yanit.Govde
		go func() {
			v, e := JSONSinirli(g)
			bitti <- sonuc{v, e}
		}()
		select {
		case s := <-bitti:
			var ih *IstekHatasi
			if errors.As(s.err, &ih) {
				err = ih
			} else if s.err == nil {
				cozulen = s.v
			}
		case <-time.After(kalan):
			// Çeviri arka planda biter (EnFazlaJSONDegeri ile sınırlı) ve sonucu atılır.
			err = &IstekHatasi{
				Neden:     fmt.Sprintf("zaman aşımı: yanıt %s içinde okunup çözülemedi", sureMetni(webZamanAsimi)),
				ZamanAsim: true,
			}
		}
	}
	if makine != nil {
		makine.Blocking(calistir)
	} else {
		calistir()
	}
	if err != nil {
		etiket := "Web bağlantı hatası"
		if yontem == http.MethodPost {
			etiket = "Web gönderim hatası"
		}
		return nil, fmt.Errorf("%s (%s): %v", etiket, adres, err)
	}
	if !yanit.Basarili() {
		ozet := yanit.GovdeOzeti(200)
		if ozet != "" {
			ozet = ": " + ozet
		}
		return nil, fmt.Errorf("%s: %s adresi %s durum kodu döndürdü%s", fn, adres, yanit.Metin, ozet)
	}
	if cozulen != nil {
		return cozulen, nil
	}
	return &vm.StringVal{Val: string(yanit.Govde)}, nil
}

// istemciKaydet web_al ve web_gönder yerleşiklerini kaydeder.
func istemciKaydet(v *vm.VM) {
	v.AdlisizKaydet(kod.W100, func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("web_al(url) 1 parametre bekler")
		}
		return webIstegi(makine, "web_al", http.MethodGet, args[0].Inspect(), nil)
	})

	v.AdlisizKaydet(kod.W101, func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("web_gönder(url, veri) 2 parametre bekler")
		}
		payload, err := jsonyaz.Yaz(args[1])
		if err != nil {
			return nil, fmt.Errorf("web_gönder: gönderilecek veri JSON'a çevrilemedi: %v", err)
		}
		return webIstegi(makine, "web_gönder", http.MethodPost, args[0].Inspect(), payload)
	})
}
