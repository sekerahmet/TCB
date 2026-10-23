package specref

import "strings"

// Fence Markdown kod çitinin durumudur (CommonMark 0.31 §4.5): en çok üç boşluk girintili, en az üç ``` ya da ~~~ ile
// açılır; aynı karakterden en az o kadar uzun, ardında bilgi metni olmayan bir çitle kapanır. Açık ``` bloğundaki ~~~
// satırı (ya da tersi) bloğun içeriğidir. codegraph'ın Markdown ayrıştırıcısı ve madde tarayıcısı bu tek kuralı
// kullanır (bağımsız denetim T6: biri ~~~'yi tanıyordu, öbürü tanımıyordu).
type Fence struct{ open string }

// Line satır bir çit satırıysa (açan ya da kapatan) true verir ve durumu günceller.
func (f *Fence) Line(line string) bool {
	t := strings.TrimLeft(line, " ")
	if len(line)-len(t) > 3 || t == "" || t[0] != '`' && t[0] != '~' {
		return false
	}
	run := t[:len(t)-len(strings.TrimLeft(t, t[:1]))]
	switch {
	case len(run) < 3:
		return false
	case f.open == "":
		if run[0] == '`' && strings.Contains(t[len(run):], "`") {
			return false // ``` çitinin bilgi metninde ` olamaz: satır içi kod
		}
		f.open = run
		return true
	case run[0] == f.open[0] && len(run) >= len(f.open) && strings.TrimSpace(t[len(run):]) == "":
		f.open = ""
		return true
	}
	return false
}

// In bir çit bloğunun içinde mi.
func (f *Fence) In() bool { return f.open != "" }
