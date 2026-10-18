//go:build amd64

package tensor

// islemciBekleAsm dönerek beklerken çağrılır (PAUSE; bkz. bekle_amd64.s).
//
//go:noescape
func islemciBekleAsm()

// islemciBekle dönerek beklemede kısa bir duraklama yapar. Satır içine
// alınmayan bir Go işlevi olduğundan girişindeki yığın denetimi aynı zamanda
// bir işbirlikçi kesme noktasıdır: çöp toplayıcı dünyayı durdurmak
// istediğinde dönen işçi burada hemen durur. (Assembly içindeki PAUSE
// döngüsünde zaman uyumsuz kesme güvenli değildir; kesme noktası olmadan her
// GC duraklaması milisaniyeler sürüyordu.)
//
//go:noinline
func islemciBekle() { islemciBekleAsm() }
