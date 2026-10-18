#!/usr/bin/env bash
# Bulanıklaştırma: depodaki her Go bulanıklaştırma hedefini (func FuzzX) verilen süre boyunca
# rastgele girdiyle koşar. Haftalık otomatik sınama işi (.github/workflows/bulaniklastirma.yml)
# bunu çağırır. Yerelde zayıf makinede tek bir hedefi kısa süre denemek için süzgeç verilir.
#
# Kullanım (depo kökünden): bash tools/bulaniklastir.sh [süre] [süzgeç]
#   süre    her hedefin süresi, Go süre biçimiyle (varsayılan 30s)
#   süzgeç  yalnızca adı bu düzenli ifadeye uyan hedefler (varsayılan: hepsi)
# Bir hedef düşerse düşüren girdi testdata/fuzz/<Hedef>/ altına yazılır; betik onu günlüğe
# basar (otomatik sınamada dosya kaybolur, girdi günlükte kalır).
set -u
set -o pipefail
cd "$(dirname "$0")/.."

sure="${1:-30s}"
suzgec="${2:-.}"
if ! [[ "$sure" =~ ^[0-9]+(\.[0-9]+)?(ns|us|ms|s|m|h)$ ]]; then
	echo "geçersiz süre: $sure (ör. 30s, 2m)" >&2
	exit 2
fi

hata=0
sayi=0
# Hedefler: izlenen test dosyalarındaki "func FuzzX(" bildirimleri; klasör paket yoludur.
while IFS=: read -r dosya _ satir; do
	ad=$(echo "$satir" | sed -E 's/^func (Fuzz[A-Za-z0-9_]*)\(.*/\1/')
	echo "$ad" | grep -Eq -- "$suzgec" || continue
	klasor=$(dirname "$dosya")
	sayi=$((sayi + 1))
	printf '\n== %s (./%s, %s)\n' "$ad" "$klasor" "$sure"
	if ! go test -run='^$' -fuzz="^${ad}\$" -fuzztime="$sure" "./$klasor"; then
		echo "!! KALDI: $ad"
		if [ "${GITHUB_ACTIONS:-}" = "true" ]; then echo "::error title=Bulanıklaştırma::$ad düştü"; fi
		hata=1
		# Yeni yazılan düşüren girdileri göster.
		for girdi in $(git ls-files -o --exclude-standard -- "$klasor/testdata/fuzz/$ad"); do
			echo "-- düşüren girdi: $girdi"
			cat "$girdi"
		done
	fi
done < <(git grep -n -E '^func Fuzz[A-Za-z0-9_]*\(' -- '*_test.go')

if [ $sayi -eq 0 ]; then
	echo "hiç bulanıklaştırma hedefi bulunamadı (süzgeç: $suzgec)" >&2
	exit 1
fi
echo
if [ $hata -eq 0 ]; then echo "✅ $sayi hedef düşmedi"; else echo "❌ düşen hedef var"; fi
exit $hata
