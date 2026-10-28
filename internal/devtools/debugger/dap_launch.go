package debugger

import (
	"encoding/json"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/logging"
	"os"
	"path/filepath"
)

// ---------------------------------------------------------------------------
// Başlatma
// ---------------------------------------------------------------------------

func (s *dapServer) launch(msg *dapMessage) {
	var a struct {
		Program     string `json:"program"`
		StopOnEntry bool   `json:"stopOnEntry"`
		NoDebug     bool   `json:"noDebug"`
		Cwd         string `json:"cwd"`
		// Args programa iletilen komut satırı argümanlarıdır (düzenleyicinin başlatma ayarı).
		Args []string `json:"args"`
		// ViewURI programın düzenleyicideki görünümünün adresidir (kodlu dosyanın
		// Türkçe görünümü; bkz. dapServer.viewURI). Satırlar görünümün satırlarıdır.
		ViewURI string `json:"viewURI"`
	}
	if err := json.Unmarshal(msg.Arguments, &a); err != nil || a.Program == "" {
		s.respondErr(msg, messages.Text(messages.H8120))
		return
	}
	if s.sessionExists() {
		s.respondErr(msg, messages.Text(messages.H8127))
		return
	}
	// Çalışma klasörü önce: göreli program yolu onun içinde çözülür (düzenleyicinin 'cwd'si).
	if a.Cwd != "" {
		_ = os.Chdir(a.Cwd)
	}
	path, _ := filepath.Abs(a.Program)
	data, err := os.ReadFile(path)
	if err != nil {
		s.respondErr(msg, messages.Text(messages.H8121, path, turkish.FileErrorReason(path, err)))
		return
	}
	ready := source.Prepare(string(data), source.Options{TypeCheck: true})
	code, prog, errs := ready.Text, ready.Program, ready.Errors
	// Uyarılar (ör. yerleşik ad gölgeleme) programı durdurmaz; hata ayıklama konsoluna yazılır.
	if u := ready.Warnings; len(u) > 0 {
		s.output("stderr", "⚠️ "+messages.Text(messages.H8122)+"\n")
		for _, e := range u {
			s.output("stderr", "  • "+e.Error()+"\n"+turkish.IndentLines(turkish.ErrorFrame(code, e), "    "))
		}
	}
	if len(errs) > 0 {
		s.output("stderr", "❌ "+messages.Text(messages.H8123)+"\n")
		for _, e := range errs {
			s.output("stderr", "  • "+e.Error()+"\n"+turkish.IndentLines(turkish.ErrorFrame(code, e), "    "))
		}
		s.respondErr(msg, messages.Text(messages.H8124, filepath.Base(path)))
		s.event("terminated", nil)
		return
	}

	// Aktarılan modüller de çalışmadan önce denetlenir (1.3); hata varsa hiçbir satır çalışmaz.
	if err := interp.CheckImportedModules(path, prog); err != nil {
		s.launchFailed(msg, path, err, code)
		return
	}

	machine := interp.New()
	machine.SourceFile = path
	machine.SourceCode = code
	machine.Out = dapOutput{s, "stdout"}
	machine.Err = dapOutput{s, "stderr"} // eşzamanlı görev hataları da hata ayıklama konsoluna
	machine.Stdin = dapNoStdin{}
	machine.SetArgs(a.Args)
	// Argüman verilip programın en dışından ulaşılan bir komut_satırı çağrısı yoksa program
	// çalışmaz (1.4): düzenleyicinin "args" ayarı da bu kurala bağlıdır.
	if verdict := machine.CheckArguments(prog); verdict != interp.ArgumentsUsable {
		s.output("stderr", "❌ "+machine.ArgumentsMessage(verdict)+"\n")
		s.respondErr(msg, messages.Text(messages.H8124, filepath.Base(path)))
		s.event("terminated", nil)
		return
	}

	if !a.NoDebug {
		dbg := NewDebugger(path, code, a.StopOnEntry)
		dbg.Reader = nil
		dbg.dap = s
		dbg.PauseReason = ""
		if a.StopOnEntry {
			dbg.PauseReason = messages.Text(messages.H8125)
			dbg.pauseKind = "entry"
		}
		machine.Debugger = dbg
		s.mu.Lock()
		s.dbg = dbg
		s.mu.Unlock()
	}
	s.mu.Lock()
	s.machine = machine
	s.prog = prog
	s.programPath = path
	s.viewURI = a.ViewURI
	s.mu.Unlock()
	s.respond(msg, nil)
}

// sessionExists oturumun programı zaten kurulmuş ya da başlamış mı: ikinci launch reddedilir.
func (s *dapServer) sessionExists() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started || s.machine != nil
}

func (s *dapServer) start() {
	s.mu.Lock()
	if s.started || s.machine == nil {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	go func() {
		defer close(s.finished)
		_, err := s.machine.Run(s.prog)
		s.machine.WaitAsync()
		logging.CloseAll() // bitmemiş ilerleme çubuklarının son satırı konsola
		end := s.machine.End(err)
		if end.Message != "" {
			// Programın istediği çıkış (ör. komut satırı seçeneklerinde --yardım ya da hatalı
			// argüman): mesaj konsola, kod düzenleyiciye.
			category := "stderr"
			if end.MessageToOutput {
				category = "stdout"
			}
			s.output(category, s.machine.Mask(end.Message)+"\n")
		}
		for _, failure := range end.Errors {
			text := "❌ " + messages.Text(messages.H8126) + "\n"
			for _, m := range interp.ErrorItems(failure, s.machine.SourceCode) {
				text += "  • " + m.Message + "\n" + turkish.IndentLines(m.Frame, "    ")
			}
			s.output("stderr", s.machine.Mask(text))
		}
		s.event("exited", map[string]interface{}{"exitCode": end.Code})
		s.event("terminated", nil)
	}()
}

// launchFailed çalışmadan önceki bir hatayı (modül denetimi) hata ayıklama konsoluna yazar ve
// oturumu bitirir.
func (s *dapServer) launchFailed(msg *dapMessage, path string, err error, code string) {
	text := "❌ " + messages.Text(messages.H8126) + "\n"
	for _, m := range interp.ErrorItems(err, code) {
		text += "  • " + m.Message + "\n" + turkish.IndentLines(m.Frame, "    ")
	}
	s.output("stderr", text)
	s.respondErr(msg, messages.Text(messages.H8124, filepath.Base(path)))
	s.event("terminated", nil)
}
