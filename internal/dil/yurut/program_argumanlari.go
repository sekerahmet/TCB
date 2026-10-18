package yurut

import (
	"fmt"
	"strings"
)

// Betiğe komut satırında verilen argümanlar ('tr betik.tr --adım 5 veri' → ["--adım", "5",
// "veri"]). 'tr betik.tr', 'tr çalıştır', 'tr hata_ayıkla', 'tr derle' ile üretilen uygulama
// ve VS Code başlatma ayarındaki "args" aynı yoldan buraya gelir. Program argümanları
// okuyan yerleşik (ileride komut_satırı) Argumanlar ile alır; okunmayan argümanlar program
// bitince uyarıyla bildirilir (sessizce yok sayılmasın).

// ArgumanlariAyarla programın argümanlarını kaydeder (program başlamadan, bir kez).
func (v *VM) ArgumanlariAyarla(argumanlar []string) {
	v.argumanlar = append([]string(nil), argumanlar...)
	v.argumanlarOkundu.Store(false)
}

// Argumanlar programın argümanlarının bir kopyasını verir ve onları okunmuş sayar.
func (v *VM) Argumanlar() []string {
	v.argumanlarOkundu.Store(true)
	return append([]string(nil), v.argumanlar...)
}

// KullanilmayanArgumanUyarisi program argümanları okumadan bittiyse standart hataya
// yazılacak uyarıdır; argüman verilmediyse ya da okunduysa boştur.
func (v *VM) KullanilmayanArgumanUyarisi() string {
	if len(v.argumanlar) == 0 || v.argumanlarOkundu.Load() {
		return ""
	}
	return fmt.Sprintf("⚠️ Uyarı: programa verilen %d argüman kullanılmadı (%s); program komut satırı argümanlarını okumuyor",
		len(v.argumanlar), strings.Join(v.argumanlar, " "))
}
