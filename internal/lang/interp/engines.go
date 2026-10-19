package interp

import "sync"

// Ek motorlar (tensör, veri, parçalayıcı, JSON/CSV ...) kendi yerleşiklerini her yeni
// VM'ye MotorEkle ile kaydeder. internal/lang/interp bu motorları içe aktarmaz; hepsini doğru sırayla
// tanıtan paket github.com/sekerahmet/TCB/internal/stdlib'dur.
var (
	isEngine sync.Mutex
	engines  []func(*VM)
)

// AddEngine adds a registration function that New calls for every VM after the core
// builtins. Registrations run in the order they were added.
func AddEngine(save func(v *VM)) {
	isEngine.Lock()
	defer isEngine.Unlock()
	engines = append(engines, save)
}

func (v *VM) registerEngines() {
	isEngine.Lock()
	records := make([]func(*VM), len(engines))
	copy(records, engines)
	isEngine.Unlock()
	for _, save := range records {
		save(v)
	}
}

// AddTaskHook adds a hook that runs whenever one of this VM's tasks (the main program or a
// concurrent block) takes the interpreter lock, on the goroutine and with the view of the task
// that took it. Engines use it to adjust process-wide state (e.g. the tensor core's no-grad
// mode) to the task holding the lock; the hook reads task state with TaskValue. Call it only
// during engine registration (before the program starts). Keep the hook short: it runs every
// time the lock changes hands.
func (v *VM) AddTaskHook(f func(*VM)) {
	v.taskHooks = append(v.taskHooks, f)
}
