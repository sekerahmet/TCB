//go:build windows

package turkce

import "syscall"

// sistemHatasiNedeni Windows hata kodlarından bilinenleri Türkçe anlatır; bilinmeyende "" döner.
// Kodlar: https://learn.microsoft.com/windows/win32/debug/system-error-codes
func sistemHatasiNedeni(kod syscall.Errno) string {
	switch kod {
	case 4: // ERROR_TOO_MANY_OPEN_FILES
		return "aynı anda çok fazla dosya açık (açık dosyaları kapatıp yeniden deneyin)"
	case 15: // ERROR_INVALID_DRIVE
		return "böyle bir sürücü yok"
	case 19: // ERROR_WRITE_PROTECT
		return "disk yazmaya karşı korumalı"
	case 21: // ERROR_NOT_READY
		return "sürücü hazır değil (çıkarılabilir disk ya da ağ sürücüsü takılı mı?)"
	case 23, 1117: // ERROR_CRC, ERROR_IO_DEVICE
		return "disk okunurken ya da yazılırken donanım hatası oluştu"
	case 32: // ERROR_SHARING_VIOLATION
		return "dosya başka bir program tarafından kullanılıyor (o programı kapatıp yeniden deneyin)"
	case 33: // ERROR_LOCK_VIOLATION
		return "dosyanın bir bölümü başka bir program tarafından kilitlenmiş (o programı kapatıp yeniden deneyin)"
	case 39, 112: // ERROR_HANDLE_DISK_FULL, ERROR_DISK_FULL
		return "diskte yer kalmadı (yer açıp yeniden deneyin)"
	case 53, 67: // ERROR_BAD_NETPATH, ERROR_BAD_NET_NAME
		return "ağ yolu bulunamadı"
	case 123: // ERROR_INVALID_NAME
		return "dosya adı geçersiz (Windows'ta \\ / : * ? \" < > | karakterleri dosya adında kullanılamaz)"
	case 145: // ERROR_DIR_NOT_EMPTY
		return "klasör boş değil"
	case 206: // ERROR_FILENAME_EXCED_RANGE
		return "yol çok uzun (daha kısa bir klasör ya da dosya adı kullanın)"
	case 267: // ERROR_DIRECTORY
		return "klasör adı geçersiz ya da yolun bir parçası klasör değil"
	case 1392: // ERROR_FILE_CORRUPT
		return "dosya bozuk ve okunamıyor"
	}
	return ""
}
