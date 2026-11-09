package interp

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// hostVM kilidi tutan bir ana görünüm ve onun host'udur.
func hostVM(t *testing.T) (*VM, *TaskHost) {
	t.Helper()
	v := New()
	var errOut bytes.Buffer
	v.Err = &errOut
	v.acquireGIL()
	t.Cleanup(func() {
		if v.holdsGIL {
			v.releaseGIL()
		}
		if errOut.Len() > 0 {
			t.Errorf("host görevinin hatası yazılmamalı: %q", errOut.String())
		}
	})
	return v, v.NewTaskHost(7)
}

// runOutside Run'ı kilidi tutmayan bir goroutine'den çağırır.
func runOutside(h *TaskHost, fn func(task *VM) (Value, error)) chan [2]any {
	out := make(chan [2]any, 1)
	go func() {
		val, err := h.Run(fn)
		out <- [2]any{val, err}
	}()
	return out
}

// TestTaskHostRunsOnTheLockAndForksFromTheTemplate: görev kilidi tutarak çalışır, yaşayan görev
// sayılır; çatallanma kancası şablon alınırken açandan, her görevde şablondan çalışır (10.4).
func TestTaskHostRunsOnTheLockAndForksFromTheTemplate(t *testing.T) {
	v := New()
	var parents []*VM
	v.AddForkHook(func(parent, _ *VM) { parents = append(parents, parent) })
	v.acquireGIL()
	h := v.NewTaskHost(3)
	v.releaseGIL()
	if len(parents) != 1 || parents[0] != v {
		t.Fatalf("şablon alınırken kanca açandan çalışmalı: %v", parents)
	}
	r := <-runOutside(h, func(task *VM) (Value, error) {
		if !task.holdsGIL || task.liveTasks.Load() != 1 || task.task == nil {
			t.Errorf("görev kilidi tutmalı ve sayılmalı: kilit %v, yaşayan %d", task.holdsGIL, task.liveTasks.Load())
		}
		return NewInt(5), nil
	})
	if r[1] != nil || r[0].(Value).Inspect() != "5" {
		t.Fatalf("görevin sonucu dönmeli: %v", r)
	}
	if len(parents) != 2 || parents[1] != h.template {
		t.Fatalf("görev şablondan çatallanmalı: %v", parents)
	}
	if v.liveTasks.Load() != 0 {
		t.Fatalf("biten görev sayılmamalı: %d", v.liveTasks.Load())
	}
}

// TestTaskHostErrorsReturnAndAreNotWritten: görevin hatası ve Go paniği Run'dan döner, yazılmaz.
func TestTaskHostErrorsReturnAndAreNotWritten(t *testing.T) {
	v, h := hostVM(t)
	v.releaseGIL()
	mine := errors.New("benim hatam")
	if r := <-runOutside(h, func(*VM) (Value, error) { return nil, mine }); r[1] != mine {
		t.Fatalf("görevin hatası dönmeli: %v", r[1])
	}
	r := <-runOutside(h, func(*VM) (Value, error) { panic("kırıldı") })
	if err, _ := r[1].(error); err == nil || !strings.Contains(err.Error(), "kırıldı") {
		t.Fatalf("panik iç hataya dönmeli: %v", r[1])
	}
}

// TestTaskHostCloseCancelsRunningTasks: Close yeni görevi açtırmaz, süreni iptal eder; iptal edilen
// görev güvenli noktasında durur ve ErrInterrupted ile döner (10.9, 28.8).
func TestTaskHostCloseCancelsRunningTasks(t *testing.T) {
	v, h := hostVM(t)
	started, release := make(chan struct{}), make(chan struct{})
	var after bool
	v.releaseGIL()
	out := runOutside(h, func(task *VM) (Value, error) {
		close(started)
		task.Blocking(func() { <-release }) // geri dönüşte iptal denetlenir
		after = true
		return Null, nil
	})
	<-started
	v.acquireGIL()
	if !h.Close(v) || h.Close(v) {
		t.Fatal("ilk Close true, ikincisi false vermeli")
	}
	close(release)
	if !h.Wait(v, 5*time.Second) {
		t.Fatal("iptal edilen görev bitmeliydi")
	}
	if r := <-out; r[1] != ErrInterrupted || after {
		t.Fatalf("iptal edilen görev güvenli noktada durmalı: %v, sürdü %v", r[1], after)
	}
	v.releaseGIL()
	if r := <-runOutside(h, func(*VM) (Value, error) { return Null, nil }); r[1] != ErrHostClosed {
		t.Fatalf("kapanmış host görev açmamalı: %v", r[1])
	}
}

// TestMainEndRunsBeforeTasksAreAwaited: AddMainEnd'in işi program bitince, görevler beklenmeden
// önce çalışır; etkileşimli kabukta çalışmaz.
func TestMainEndRunsBeforeTasksAreAwaited(t *testing.T) {
	for _, interactive := range []bool{false, true} {
		v := New()
		var ran int
		v.AddMainEnd(func(y *VM) {
			ran++
			if !y.holdsGIL || y.liveTasks.Load() != 1 {
				t.Errorf("iş kilitle ve görev beklenmeden çalışmalı: %v %d", y.holdsGIL, y.liveTasks.Load())
			}
		})
		if interactive {
			v.SetInteractive()
		}
		if _, err := v.Run(mustParseProgram(t, "TR_async { TR_sleep(0.2) }")); err != nil {
			t.Fatal(err)
		}
		if want := map[bool]int{false: 1, true: 0}[interactive]; ran != want {
			t.Fatalf("etkileşimli %v: iş %d kez çalıştı, %d bekleniyordu", interactive, ran, want)
		}
	}
}
