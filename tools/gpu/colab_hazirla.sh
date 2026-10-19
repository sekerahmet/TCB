#!/usr/bin/env bash
# Colab paketi: GPU sınamalarını ve kıyası bulut GPU makinesinde tek seferde koşturmak için
# gereken dosyaları verilen klasöre koyar. Linux (x86-64) için çapraz derler; cgo gerekmez.
#
# Kullanım (depo kökünden): bash tools/gpu/colab_hazirla.sh HEDEF_KLASOR
#
# HEDEF_KLASOR'e yazılanlar:
#   tr             TR yorumlayıcısı (linux/amd64, GOAMD64=v3, CGO_ENABLED=0)
#   gpu_sinama     GPU arka ucunun Go sınama ikilisi (go test -c ./internal/compute/gpu)
#   kiyas_gpu.tr   kıyas modeli (TR)
#   gpt_egitim.tr  SimpleStories GPT eğitimi (dilin kitaplığıyla: tr/katman, tr/iyilestirici)
#   kiyas_gpu.py   kıyas modeli (karşılaştırma tarafı)
#   mikro_kiyas.py mikro kıyaslar (karşılaştırma tarafı; TR tarafı gpu_sinama'da TestMicroBench)
#   mikro_tablo.py iki tarafın mikro kıyas çıktılarını tek tabloya çevirir
#   COLAB.md       hücre hücre çalıştırma adımları
#   tests/       dil tanımı testleri (depodaki tests/ klasörünün kopyası)
#   SURUM.txt      derlenen commit ve tarih
# Klasör yoksa oluşturulur; içindeki aynı adlı dosyaların üzerine yazılır, başka dosyaya
# dokunulmaz.
set -euo pipefail
cd "$(dirname "$0")/../.."

if [ $# -ne 1 ]; then
	echo "kullanım: bash tools/gpu/colab_hazirla.sh HEDEF_KLASOR" >&2
	exit 2
fi
hedef=$1
mkdir -p "$hedef"

export GOOS=linux GOARCH=amd64 GOAMD64=v3 CGO_ENABLED=0
echo "== tr derleniyor (linux/amd64, v3)"
go build -trimpath -o "$hedef/tr" ./cmd/tr
echo "== GPU sınama ikilisi derleniyor"
go test -c -trimpath -o "$hedef/gpu_sinama" ./internal/compute/gpu
unset GOOS GOARCH GOAMD64 CGO_ENABLED

cp tools/gpu/kiyas_gpu.tr tools/gpu/gpt_egitim.tr tools/gpu/kiyas_gpu.py tools/gpu/mikro_kiyas.py tools/gpu/mikro_tablo.py tools/gpu/COLAB.md "$hedef/"
# Dil tanımı testleri (15.26–15.30 GPU kuralları GPU'lu makinede koşar).
mkdir -p "$hedef/testler"
cp -r tests/. "$hedef/testler/"
{
	echo "commit: $(git rev-parse --short HEAD 2>/dev/null || echo bilinmiyor)"
	echo "tarih:  $(date -u +%Y-%m-%dT%H:%M:%SZ)"
} >"$hedef/SURUM.txt"
echo "== hazır: $hedef"
ls -l "$hedef"
