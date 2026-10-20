// İzleme Masası yönlendirme düğmelerinin eşlemesi: web görünümündeki düğmenin data-komut
// değeri (DAP isteğinin adı) → VS Code komutu. vscode modülüne bağlı değildir; böylece
// eşleme Node ile ayrıca sınanır (test/watch_commands_test.js).
'use strict';

// Düğme → VS Code'un kendi hata ayıklama komutu. Oturum etkin oturumsa bu komutlar kullanılır:
// araç çubuğu, çağrı yığını ve editördeki sarı satır da güncellenir.
const WATCH_COMMANDS = Object.freeze({
    next: 'workbench.action.debug.stepOver',
    stepIn: 'workbench.action.debug.stepInto',
    stepOut: 'workbench.action.debug.stepOut',
    continue: 'workbench.action.debug.continue',
    stop: 'workbench.action.debug.stop'
});

// resolveWatchCommand düğmeden gelen komutun nasıl uygulanacağını söyler:
//   { kind: 'vscode', command }  VS Code komutu çalıştırılır (oturum etkin oturumsa)
//   { kind: 'durdur' }         oturum vscode.debug.stopDebugging ile durdurulur
//   { kind: 'dap', request, args }  request doğrudan oturuma gönderilir
//   undefined                 bilinmeyen komut (yok sayılır)
function resolveWatchCommand(command, sessionActive) {
    if (!Object.prototype.hasOwnProperty.call(WATCH_COMMANDS, command)) {
        return undefined;
    }
    if (sessionActive) {
        return { kind: 'vscode', command: WATCH_COMMANDS[command] };
    }
    if (command === 'stop') {
        return { kind: 'durdur' };
    }
    return { kind: 'dap', request: command, args: { threadId: 1 } };
}

module.exports = { WATCH_COMMANDS, resolveWatchCommand };
