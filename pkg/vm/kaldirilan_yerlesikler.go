package vm

// kaldirilanUyeler Metin ve Liste üzerinde kaldırılmış eş anlamlı özellik ve metot adlarıdır.
var kaldirilanUyeler = map[string]string{
	"boy":      "uzunluk",
	"dönüştür": "haritala",
	"donustur": "haritala",
}

// kaldirilanUyeIpucu kaldırılmış bir metot ya da özellik adı için ipucunu döndürür.
func kaldirilanUyeIpucu(ad string) string {
	if asil, ok := kaldirilanUyeler[ad]; ok {
		return yokIpucu(ad, asil)
	}
	return ""
}
