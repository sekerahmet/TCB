//go:build unix

package turkish

import "syscall"

// systemErrorReason POSIX hata kodlarından bilinenleri Türkçe anlatır; bilinmeyende "" döner.
func systemErrorReason(code syscall.Errno) string {
	switch code {
	case syscall.ENOSPC:
		return "diskte yer kalmadı (yer açıp yeniden deneyin)"
	case syscall.EDQUOT:
		return "disk kotanız doldu (yer açıp yeniden deneyin)"
	case syscall.EROFS:
		return "dosya sistemi salt okunur"
	case syscall.ENOTDIR:
		return "yolun bir parçası klasör değil, dosya"
	case syscall.ENAMETOOLONG:
		return "yol çok uzun (daha kısa bir klasör ya da dosya adı kullanın)"
	case syscall.EMFILE, syscall.ENFILE:
		return "aynı anda çok fazla dosya açık (açık dosyaları kapatıp yeniden deneyin)"
	case syscall.EBUSY, syscall.ETXTBSY:
		return "dosya başka bir program tarafından kullanılıyor (o programı kapatıp yeniden deneyin)"
	case syscall.ENOTEMPTY:
		return "klasör boş değil"
	case syscall.EIO:
		return "disk okunurken ya da yazılırken donanım hatası oluştu"
	case syscall.ELOOP:
		return "yolda sonsuz döngüye giren bir sembolik bağ var"
	case syscall.EFBIG:
		return "dosya bu dosya sistemi için çok büyük"
	}
	return ""
}
