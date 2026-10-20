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
	"strings"
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
			s.output("stderr", "  • "+e+"\n"+indentLines(turkish.CodeFrame(code, e), "    "))
		}
	}
	if len(errs) > 0 {
		s.output("stderr", "❌ "+messages.Text(messages.H8123)+"\n")
		for _, e := range errs {
			s.output("stderr", "  • "+e+"\n"+indentLines(turkish.CodeFrame(code, e), "    "))
		}
		s.respondErr(msg, messages.Text(messages.H8124, filepath.Base(path)))
		s.event("terminated", nil)
		return
	}

	machine := interp.New()
	machine.SourceFile = path
	machine.SourceCode = code
	machine.Out = dapOutput{s, "stdout"}
	machine.Err = dapOutput{s, "stderr"} // eşzamanlı görev hataları da hata ayıklama konsoluna
	machine.Stdin = dapNoStdin{}
	machine.SetArgs(a.Args)

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
		s.dbg = dbg
	}
	s.machine = machine
	s.prog = prog
	s.programPath = path
	s.viewURI = a.ViewURI
	s.respond(msg, nil)
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
		exitCode := 0
		if c, ok := interp.IsExitRequest(err); ok {
			// Programın istediği çıkış (ör. komut satırı seçeneklerinde --yardım ya da hatalı
			// argüman): mesaj konsola, kod düzenleyiciye.
			exitCode = c.Code
			category := "stderr"
			if c.Code == 0 {
				category = "stdout"
			}
			if c.Message != "" {
				s.output(category, s.machine.Mask(c.Message)+"\n")
			}
		} else if err != nil {
			exitCode = 1
			text := "❌ " + messages.Text(messages.H8126) + "\n"
			for _, m := range interp.ErrorItems(err, s.machine.SourceCode) {
				text += "  • " + m.Message + "\n" + indentLines(m.Frame, "    ")
			}
			s.output("stderr", s.machine.Mask(text))
		} else if u := s.machine.UnusedArgsWarning(); u != "" {
			s.output("stderr", u+"\n")
		}
		s.event("exited", map[string]interface{}{"exitCode": exitCode})
		s.event("terminated", nil)
	}()
}
func indentLines(text, prefix string) string {
	if text == "" {
		return ""
	}
	var sb strings.Builder
	for _, l := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		sb.WriteString(prefix + l + "\n")
	}
	return sb.String()
}
