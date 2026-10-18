// İzleme Masası yönlendirme düğmelerinin eşlemesi: web görünümündeki düğmenin data-komut
// değeri (DAP isteğinin adı) → VS Code komutu. vscode modülüne bağlı değildir; böylece
// eşleme Node ile ayrıca sınanır (test/izleme_komutlari_test.js).
'use strict';

// Düğme → VS Code'un kendi hata ayıklama komutu. Oturum etkin oturumsa bu komutlar kullanılır:
// araç çubuğu, çağrı yığını ve editördeki sarı satır da güncellenir.
const IZLEME_KOMUTLARI = Object.freeze({
    next: 'workbench.action.debug.stepOver',
    stepIn: 'workbench.action.debug.stepInto',
    stepOut: 'workbench.action.debug.stepOut',
    continue: 'workbench.action.debug.continue',
    stop: 'workbench.action.debug.stop'
});

// izlemeKomutuCoz düğmeden gelen komutun nasıl uygulanacağını söyler:
//   { tur: 'vscode', komut }  VS Code komutu çalıştırılır (oturum etkin oturumsa)
//   { tur: 'durdur' }         oturum vscode.debug.stopDebugging ile durdurulur
//   { tur: 'dap', istek, argumanlar }  istek doğrudan oturuma gönderilir
//   undefined                 bilinmeyen komut (yok sayılır)
function izlemeKomutuCoz(komut, oturumEtkin) {
    if (!Object.prototype.hasOwnProperty.call(IZLEME_KOMUTLARI, komut)) {
        return undefined;
    }
    if (oturumEtkin) {
        return { tur: 'vscode', komut: IZLEME_KOMUTLARI[komut] };
    }
    if (komut === 'stop') {
        return { tur: 'durdur' };
    }
    return { tur: 'dap', istek: komut, argumanlar: { threadId: 1 } };
}

module.exports = { IZLEME_KOMUTLARI, izlemeKomutuCoz };
