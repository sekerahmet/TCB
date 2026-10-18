package tensor

import (
	ct "tr_dili/pkg/tensor"
	"tr_dili/pkg/vm"
)

// Görev başına çekirdek durumu.
//
// Çekirdeğin türevsiz kipi bir türev bağlamında (ct.GradContext) tutulur ve her an tek bir
// bağlam etkindir. Yorumlayıcı kilidini (GIL) her alan görev kendi bağlamını etkin yapar
// (gorevKancasi); böylece bir görevin türevsiz { } bloğu (ya da özel_türev'in türevsiz
// çalışan ileri/geri hesabı) kilidi devrettiğinde aynı anda çalışan başka görevin işlemleri
// çizge dışında kalmaz. Bağlam görev durumunda (vm.GorevDegeri) saklanır; yeni görevler
// türevli kipte başlar (karışık_hassasiyet gibi blok durumu görevden göreve geçmez).

// gradAnahtari görev durumunda (vm.GorevDegeri) türev bağlamının anahtarı.
type gradAnahtari struct{}

// gorevGradBaglami görevin türev bağlamını verir; yoksa oluşturur.
func gorevGradBaglami(m *vm.VM) *ct.GradContext {
	if c, ok := m.GorevDegeri(gradAnahtari{}).(*ct.GradContext); ok {
		return c
	}
	c := ct.NewGradContext()
	m.GorevDegeriAyarla(gradAnahtari{}, c)
	return c
}

// gorevKancasi, kilidi alan görevin türev bağlamını çekirdekte etkin yapar.
func gorevKancasi(m *vm.VM) { ct.SetGradContext(gorevGradBaglami(m)) }
