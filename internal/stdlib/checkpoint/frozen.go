package checkpoint

import (
	"slices"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Donma durumu (dil tanımı 15.34, 20.4): nokta her modelin donmuş parametrelerinin adlarını
// kaydeder (frozen, sürüm 4); sürdürme ve geri dönüş aynı kümeyi kurar: adı listede olan
// parametre donar, öbür parametreler türev izler. Sürüm 3 dosyasında liste yoktur: hiçbir
// parametre donmuş değil. Parametre, türev izlemek için kurulmuş ya da açılmış yapraktır (15.33);
// hiç türev izlememiş durum tensörü (tablo, tampon) listeye girmez ve dokunulmaz.

// frozenNames modelin donmuş parametrelerinin adlarıdır (kayıt sırasıyla; yoksa nil).
func (n *object) frozenNames() []string {
	var out []string
	for i, p := range n.params {
		if t := p.Tensor(); t.IsParameter() && !t.RequiresGrad() {
			out = append(out, n.names[i])
		}
	}
	return out
}

// frozenMatch kayıttaki donmuş adların modelin parametreleri olduğunu, hiçbir şey yüklemeden
// denetler: olmayan ya da parametre olmayan ad kaydın bozukluğudur.
func (n *object) frozenMatch(fn any, y *pending) error {
	for _, name := range y.record.Frozen {
		i := slices.Index(n.names, name)
		if i < 0 || !n.params[i].Tensor().IsParameter() {
			return corruptContent{messages.New(messages.H6993, fn, n.name, name)}
		}
	}
	return nil
}

// applyFrozen kayıttaki donma durumunu modelin parametrelerine kurar.
func (n *object) applyFrozen(frozen []string) {
	for i, p := range n.params {
		t := p.Tensor()
		switch {
		case !t.IsParameter():
		case slices.Contains(frozen, n.names[i]):
			t.Freeze()
		default:
			t.SetRequiresGrad(true)
		}
	}
}
