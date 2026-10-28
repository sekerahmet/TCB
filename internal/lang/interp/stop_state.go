package interp

import "github.com/sekerahmet/TCB/internal/lang/codes"

// StateChanges programın durumunu değiştiren işlerin (yerleşik ve üye) kodlarıdır (1.5). Durdurma
// isteği böyle bir işi yarıda kesmez, bırakmaz: iş biter ve program sonraki güvenli noktada durur.
// Durumu değiştirmeyen iş (hesap, dosya okuma, bekleme) bırakılabilir (Abandonable, AwaitNew).
// Liste tek yerde yazılır ve sınanır (stop_state_test.go): bu işlerin hiçbiri Abandonable ya da
// AwaitNew ile beklenmez. Koda bağlı olmayan dil işi indeksle atamadır (liste[i] = x, sözlük[k] = x):
// değeri yerinde değiştirir, derleyicinin atama yolunda bekleme yoktur.
var StateChanges = []codes.Code{
	codes.FnBackward,    // yaprak tensörlerin gradyanlarını biriktirir
	codes.MemStep,       // iyileştiricinin adımı ağırlıkları ve durumunu değiştirir
	codes.FnWriteSlice,  // tensörün diliminin üzerine yazar
	codes.FnWriteFile,   // dosyaya yazar (atomik)
	codes.FnSaveTensors, // tensör dosyasını yazar (atomik)
	codes.MemSave,       // kontrol noktasını kaydeder
	codes.FnRandn,       // rastgele üretecin durumunu ilerletir
	codes.FnSample,      // rastgele üretecin durumunu ilerletir
	codes.MemNext,       // veri yükleyicinin ve okuyucunun konumunu ilerletir
}
