package ayikla

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/denetci"
	"tr_dili/internal/dil/sozcuk"
	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
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
		// Args programa iletilen komut satırı argümanlarıdır (VS Code başlatma ayarı).
		Args []string `json:"args"`
	}
	if err := json.Unmarshal(msg.Arguments, &a); err != nil || a.Program == "" {
		s.respondErr(msg, "Hata ayıklanacak .tr dosyası belirtilmedi ('program')")
		return
	}
	path, _ := filepath.Abs(a.Program)
	data, err := os.ReadFile(path)
	if err != nil {
		s.respondErr(msg, "Hata ayıklanacak program okunamadı (%s): %s", path, turkce.DosyaHatasiNedeni(path, err))
		return
	}
	code := strings.TrimPrefix(string(data), "\xef\xbb\xbf")

	p := ayristir.New(sozcuk.New(code))
	prog := p.ParseProgram()
	errs := p.Errors()
	if len(errs) == 0 {
		chk := denetci.New()
		errs = chk.CheckProgram(prog)
		// Uyarılar (ör. yerleşik ad gölgeleme) programı durdurmaz; Debug Console'a yazılır.
		if u := chk.Warnings(); len(u) > 0 {
			s.output("stderr", "⚠️ Uyarılar:\n")
			for _, e := range u {
				s.output("stderr", "  • "+e+"\n"+indentLines(turkce.KodCercevesi(code, e), "    "))
			}
		}
	}
	if len(errs) > 0 {
		s.output("stderr", "❌ Program başlatılamadı:\n")
		for _, e := range errs {
			s.output("stderr", "  • "+e+"\n"+indentLines(turkce.KodCercevesi(code, e), "    "))
		}
		s.respondErr(msg, "%s içinde hata var; ayrıntılar Debug Console'da", filepath.Base(path))
		s.event("terminated", nil)
		return
	}

	if a.Cwd != "" {
		_ = os.Chdir(a.Cwd)
	}
	machine := yurut.New()
	machine.SourceFile = path
	machine.SourceCode = code
	machine.Out = dapOutput{s, "stdout"}
	machine.Err = dapOutput{s, "stderr"} // eşzamanlı görev hataları da Debug Console'a
	machine.Stdin = dapNoStdin{}
	machine.ArgumanlariAyarla(a.Args)

	if !a.NoDebug {
		dbg := NewDebugger(path, code, a.StopOnEntry)
		dbg.Reader = nil
		dbg.dap = s
		dbg.PauseReason = ""
		if a.StopOnEntry {
			dbg.PauseReason = "Programın başında durdu"
			dbg.pauseKind = "entry"
		}
		machine.Debugger = dbg
		s.dbg = dbg
	}
	s.machine = machine
	s.prog = prog
	s.programPath = path
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
		exitCode := 0
		if err != nil {
			exitCode = 1
			metin := "❌ Çalışma Zamanı Hatası:\n"
			for _, m := range yurut.HataMaddeleri(err, s.machine.SourceCode) {
				metin += "  • " + m.Mesaj + "\n" + indentLines(m.Cerceve, "    ")
			}
			s.output("stderr", s.machine.Maskele(metin))
		} else if u := s.machine.KullanilmayanArgumanUyarisi(); u != "" {
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
