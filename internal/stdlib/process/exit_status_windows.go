//go:build windows

package process

import "os"

// exitSignal Windows'ta sinyal yoktur: program çıkış koduyla biter (21.4).
func exitSignal(*os.ProcessState) (string, bool) { return "", false }
