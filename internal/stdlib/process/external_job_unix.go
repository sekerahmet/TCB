//go:build !windows

package process

import (
	"os/exec"
	"syscall"
)

// processGroup programın kendi süreç grubudur (21.5). Grup kimliği programın süreç kimliğidir; önder
// bitse de grupta süreç kaldıkça kimlik başkasına verilmez.
type processGroup struct{ id int }

func (g *processGroup) prepare(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func (g *processGroup) started(c *exec.Cmd) error {
	g.id = c.Process.Pid
	return nil
}

// request nazik isteği gruba yollar; gitmediyse false verir.
func (g *processGroup) request() bool { return syscall.Kill(-g.id, syscall.SIGTERM) == nil }

// kill grubu öldürür.
func (g *processGroup) kill() { _ = syscall.Kill(-g.id, syscall.SIGKILL) }

// release Unix'te bırakılacak bir şey yoktur.
func (g *processGroup) release(bool) {}
