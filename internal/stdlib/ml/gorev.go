package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Görev başına çekirdek durumu.
//
// Çekirdeğin türevsiz kipi bir türev bağlamında (tensor.GradContext) tutulur ve her an tek bir
// bağlam etkindir. Yorumlayıcı kilidini (GIL) her alan görev kendi bağlamını etkin yapar
// (gorevKancasi); böylece bir görevin türevsiz { } bloğu (ya da özel_türev'in türevsiz
// çalışan ileri/geri hesabı) kilidi devrettiğinde aynı anda çalışan başka görevin işlemleri
// çizge dışında kalmaz. Bağlam görev durumunda (yurut.GorevDegeri) saklanır; yeni görevler
// türevli kipte başlar (karışık_hassasiyet gibi blok durumu görevden göreve geçmez).

// gradAnahtari görev durumunda (yurut.GorevDegeri) türev bağlamının anahtarı.
type gradAnahtari struct{}

// gorevGradBaglami görevin türev bağlamını verir; yoksa oluşturur.
func gorevGradBaglami(m *interp.VM) *tensor.GradContext {
	if c, ok := m.TaskValue(gradAnahtari{}).(*tensor.GradContext); ok {
		return c
	}
	c := tensor.NewGradContext()
	m.SetTaskValue(gradAnahtari{}, c)
	return c
}

// gorevKancasi, kilidi alan görevin türev bağlamını çekirdekte etkin yapar.
func gorevKancasi(m *interp.VM) { tensor.SetGradContext(gorevGradBaglami(m)) }
