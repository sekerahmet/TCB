package gpu

import (
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// internalf TR'nin kendi hatasıyla ulaşılabilen bir durumu (üretilen kodun bozuk olması, çağıranın
// değişmezi bozması) iç hata olarak bildirir: kullanıcı ortak "lütfen bildirin" iletisini görür,
// İngilizce ayrıntı geliştirici içindir.
func internalf(form string, a ...any) *messages.Error {
	return messages.Internalf(form, a...)
}
