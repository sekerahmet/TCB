package ayikla

import (
	"fmt"

	"tr_dili/internal/dil/yurut"
)

// Terminal hata ayıklayıcısının komut tabloları ve yardım metni. Her komutun tek yazımı
// vardır (F tuşları dışında); eski kısaltmalar ve eşler yalnızca ipucu verir.

// tusRehberi komut isteminde gösterilen kısa tuş listesidir.
const tusRehberi = "F5:devam | F10:geç | F11:adım | Shift+F11:dön | değiştir | zıpla | tablo | izle"

// eskiTuslar önceki sürümün terminal tuşlarıdır (f5 adım, f6 geç, f7 dön, f8 devam). Bu
// tuşlar artık düzenleyicinin hata ayıklama tuşlarıyla aynı anlamda kullanılır; eski
// anlamla yazılanlar başka bir iş yapmaz, yeni tuşu söyler.
var eskiTuslar = map[string]string{"f6": "f10 (geç)", "f7": "shift+f11 (dön)", "f8": "f5 (devam)"}

// eskiKomutlar komutların eski kısaltmaları, Türkçe karaktersiz ve İngilizce eşleridir. Her
// komutun tek yazımı vardır (F tuşları dışında); bunlar çalışmaz, asıl komutu söyler.
var eskiKomutlar = map[string]string{
	"adim": "adım", "a": "adım", "s": "adım", "step": "adım",
	"gec": "geç", "g": "geç", "n": "geç", "next": "geç",
	"don": "dön", "o": "dön", "out": "dön",
	"d": "devam", "c": "devam", "continue": "devam",
	"degistir": "değiştir", "ata": "değiştir", "set": "değiştir", "kalem": "değiştir",
	"zipla": "zıpla", "git": "zıpla", "goto": "zıpla", "j": "zıpla",
	"k": "kesme", "b": "kesme", "break": "kesme",
	"watch": "izle", "w": "izle",
	"incele": "tablo", "t": "tablo",
	"yigin": "yığın", "stack": "yığın", "bt": "yığın",
	"yaz": "hesapla", "p": "hesapla", "?": "hesapla",
	"göster": "ekran", "goster": "ekran", "l": "ekran",
	"yardim": "yardım", "h": "yardım", "help": "yardım",
	"cikis": "çıkış", "q": "çıkış", "quit": "çıkış",
}

func (d *Debugger) printDebugHelp() {
	fmt.Println("\n  🛠️  TR HATA AYIKLAYICI KOMUT REHBERİ:")
	fmt.Println("  (Tuşlar düzenleyicinin hata ayıklama tuşlarıyla aynıdır; tuş adını yazıp Enter'a basın.)")
	fmt.Println("  • f11 / adım / Enter      : Tek adım ilerle (Fonksiyonların içine girer)")
	fmt.Println("  • f10 / geç               : Üstten geç (Fonksiyonu tek adımda çalıştırır)")
	fmt.Println("  • shift+f11 / dön         : Mevcut fonksiyondan çıkana kadar çalıştır")
	fmt.Println("  • f5 / devam              : Sonraki kesme veya izleme noktasına kadar devam et")
	fmt.Println("  • değiştir x = 100        : ✏️ Çalışma anında 'x' değişkeninin değerini canlı değiştir!")
	fmt.Println("  • zıpla 15                : ⏭️ Çalışma sırasını 15. satıra geri/ileri taşı!")
	fmt.Println("  • izle bakiye < 0         : 👁️ (Watchpoint) Koşul sağlandığında veya değişken değiştiğinde dur!")
	fmt.Println("  • kesme 20                : 🔴 20. satıra kesme noktası (Breakpoint) ekle/kaldır")
	fmt.Printf("  • tablo liste_adı         : 📋 %s, %s ya da %s değerini tablo görünümünde aç\n", yurut.VAL_LIST, yurut.VAL_TENSOR, yurut.VAL_DICT)
	fmt.Println("  • yığın                   : 📚 Çağrı yığınını (Call Stack) göster")
	fmt.Println("  • hesapla x * 2           : 💡 Bir TR ifadesini anında hesapla")
	fmt.Println("  • çıkış                   : Hata ayıklayıcıyı sonlandır")
}
