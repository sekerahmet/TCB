package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Görev başına çekirdek durumu.
//
// Çekirdeğin türevsiz kipi bir türev bağlamında (tensor.GradContext) tutulur ve her an tek bir
// bağlam etkindir. Yorumlayıcı kilidini (GIL) her alan görev kendi bağlamını etkin yapar
// (taskHook); böylece bir görevin türevsiz { } bloğu (ya da özel_türev'in türevsiz
// çalışan ileri/geri hesabı) kilidi devrettiğinde aynı anda çalışan başka görevin işlemleri
// çizge dışında kalmaz. Bağlam görev durumunda (interp.GorevDegeri) saklanır; yeni görevler
// türevli kipte başlar (karışık_hassasiyet gibi blok durumu görevden göreve geçmez).

// gradKey görev durumunda (interp.GorevDegeri) türev bağlamının anahtarı.
type gradKey struct{}

// taskGradContext görevin türev bağlamını verir; yoksa oluşturur.
func taskGradContext(m *interp.VM) *tensor.GradContext {
	if c, ok := m.TaskValue(gradKey{}).(*tensor.GradContext); ok {
		return c
	}
	c := tensor.NewGradContext()
	m.SetTaskValue(gradKey{}, c)
	return c
}

// taskHook, kilidi alan görevin türev bağlamını çekirdekte etkin yapar.
func taskHook(m *interp.VM) { tensor.SetGradContext(taskGradContext(m)) }
