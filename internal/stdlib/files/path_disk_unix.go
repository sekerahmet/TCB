//go:build linux || darwin || freebsd

package files

import "syscall"

// freeBytesForUser Statfs'in Bavail * Bsize'ıdır: yöneticiye ayrılan pay düşülmüş boş yer
// (alanların tipleri işletim sistemine göre değişir). macOS'ta sınanmaz; derlenmesi tam
// denetimin darwin/arm64 derlemesinde görülür.
func freeBytesForUser(dir string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
