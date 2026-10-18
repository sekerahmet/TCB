package ayikla

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"tr_dili/internal/dil/kaynak"
	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/motor/gunluk"
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
	hazir := kaynak.Hazirla(string(data), kaynak.Secenekler{TipDenetimi: true})
	code, prog, errs := hazir.Metin, hazir.Program, hazir.Hatalar
	// Uyarılar (ör. yerleşik ad gölgeleme) programı durdurmaz; hata ayıklama konsoluna yazılır.
	if u := hazir.Uyarilar; len(u) > 0 {
		s.output("stderr", "⚠️ Uyarılar:\n")
		for _, e := range u {
			s.output("stderr", "  • "+e+"\n"+indentLines(turkce.KodCercevesi(code, e), "    "))
		}
	}
	if len(errs) > 0 {
		s.output("stderr", "❌ Program başlatılamadı:\n")
		for _, e := range errs {
			s.output("stderr", "  • "+e+"\n"+indentLines(turkce.KodCercevesi(code, e), "    "))
		}
		s.respondErr(msg, "%s içinde hata var; ayrıntılar hata ayıklama konsolunda", filepath.Base(path))
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
	machine.Err = dapOutput{s, "stderr"} // eşzamanlı görev hataları da hata ayıklama konsoluna
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
		gunluk.TumunuKapat() // bitmemiş ilerleme çubuklarının son satırı konsola
		exitCode := 0
		if c, ok := yurut.CikisIstegiMi(err); ok {
			// Programın istediği çıkış (ör. komut satırı seçeneklerinde --yardım ya da hatalı
			// argüman): mesaj konsola, kod düzenleyiciye.
			exitCode = c.Kod
			kategori := "stderr"
			if c.Kod == 0 {
				kategori = "stdout"
			}
			if c.Mesaj != "" {
				s.output(kategori, s.machine.Maskele(c.Mesaj)+"\n")
			}
		} else if err != nil {
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
