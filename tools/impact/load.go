package main

import "github.com/sekerahmet/TCB/internal/devtools/goload"

// module tip denetimi bitmiş modüldür (yükleme ortak pakettedir: tools/internal/goload); sorgular
// onun üzerindeki yöntemlerdir.
type module struct{ *goload.Module }

// load dir'deki modülü yükler.
func load(dir string) (*module, error) {
	m, err := goload.Load(dir, goload.Options{})
	if err != nil {
		return nil, err
	}
	return &module{m}, nil
}
