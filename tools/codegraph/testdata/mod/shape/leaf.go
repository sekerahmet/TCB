package shape

func leaf() {}

type leafBox struct{}

func (leafBox) step() {}

type embedBox struct{ leafBox }

type genBox[T any] struct{}

func (genBox[T]) genStep() {}
