//go:build !windows && !linux

package tensor

// hostMap bu platformda yoktur; çağıran öbeği kullanır.
func hostMap(int) (mem []byte, free func(), ok bool) { return nil, nil, false }
