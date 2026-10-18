package agac

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Dokum bir düğümün anlamını (konum bilgisi olmadan) metin olarak döker. Sözcük birimleri
// (Token alanları) satır/sütun taşıdığı için atlanır; geri kalan her alan yazılır. İki
// kaynağın Dokum çıktısı aynıysa programlar aynı anlama gelir — biçimlendirici bunu
// "anlam değişmedi" denetimi için kullanır.
func Dokum(n Node) string {
	var sb strings.Builder
	dok(&sb, reflect.ValueOf(n), 0)
	return sb.String()
}

// String programın konumdan bağımsız dökümünü verir (bkz. Dokum).
func (p *Program) String() string { return Dokum(p) }

func dok(sb *strings.Builder, v reflect.Value, girinti int) {
	bosluk := strings.Repeat("  ", girinti)
	switch v.Kind() {
	case reflect.Interface, reflect.Ptr:
		if v.IsNil() {
			sb.WriteString("∅") // dilden bağımsız boş değer imi (döküm yalnızca karşılaştırılır)
			return
		}
		dok(sb, v.Elem(), girinti)
	case reflect.Struct:
		sb.WriteString(v.Type().Name() + " {\n")
		for i := 0; i < v.NumField(); i++ {
			alan := v.Type().Field(i)
			if alan.Name == "Token" || !alan.IsExported() {
				continue
			}
			sb.WriteString(bosluk + "  " + alan.Name + ": ")
			dok(sb, v.Field(i), girinti+1)
			sb.WriteString("\n")
		}
		sb.WriteString(bosluk + "}")
	case reflect.Slice:
		sb.WriteString("[\n")
		for i := 0; i < v.Len(); i++ {
			sb.WriteString(bosluk + "  ")
			dok(sb, v.Index(i), girinti+1)
			sb.WriteString("\n")
		}
		sb.WriteString(bosluk + "]")
	case reflect.Map:
		anahtarlar := v.MapKeys()
		sort.Slice(anahtarlar, func(i, j int) bool {
			return fmt.Sprint(anahtarlar[i].Interface()) < fmt.Sprint(anahtarlar[j].Interface())
		})
		sb.WriteString("{\n")
		for _, k := range anahtarlar {
			fmt.Fprintf(sb, "%s  %v: ", bosluk, k.Interface())
			dok(sb, v.MapIndex(k), girinti+1)
			sb.WriteString("\n")
		}
		sb.WriteString(bosluk + "}")
	case reflect.String:
		fmt.Fprintf(sb, "%q", v.String())
	default:
		fmt.Fprintf(sb, "%v", v.Interface())
	}
}
