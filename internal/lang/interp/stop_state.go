package interp

import "github.com/sekerahmet/TCB/internal/lang/codes"

// StateChange programın durumunu değiştiren bir işi anlatır. Durdurma isteği (1.5) böyle bir
// işi yarıda kesmez, bırakmaz: iş biter ve program sonraki güvenli noktada durur. Durumu
// değiştirmeyen iş (hesap, dosya okuma, bekleme) bırakılabilir (Abandonable, AwaitNew).
type StateChange struct {
	// Code işin yerleşik ya da üye kodudur (codes.None: koda bağlı olmayan dil işi).
	Code codes.Code
	// What işin neyi değiştirdiğidir.
	What string
}

// StateChanges durumu değiştiren işlerin listesidir (1.5); tek yerde yazılır ve sınanır
// (stop_state_test.go): bu işlerin hiçbiri Abandonable ya da AwaitNew ile beklenmez.
var StateChanges = []StateChange{
	{codes.FnBackward, "yaprak tensörlerin gradyanlarını biriktirir"},
	{codes.MemStep, "iyileştiricinin adımı ağırlıkları ve durumunu değiştirir"},
	{codes.None, "indeksle atama (liste[i] = x, sözlük[k] = x) değeri yerinde değiştirir"},
	{codes.FnWriteSlice, "tensörün diliminin üzerine yazar"},
	{codes.FnWriteFile, "dosyaya yazar (atomik)"},
	{codes.FnSaveTensors, "tensör dosyasını yazar (atomik)"},
	{codes.MemSave, "kontrol noktasını kaydeder"},
	{codes.FnRandn, "rastgele üretecin durumunu ilerletir"},
	{codes.FnSample, "rastgele üretecin durumunu ilerletir"},
	{codes.MemNext, "veri yükleyicinin ve okuyucunun konumunu ilerletir"},
}
