//go:build !windows && !linux

package tensor

// hostBlockLimit bu platformda blok yoktur.
func hostBlockLimit() int64 { return 0 }

// hostMap bu platformda yoktur; çağıran öbeği kullanır.
func hostMap(int) (mem []byte, free func(), ok bool) { return nil, nil, false }
