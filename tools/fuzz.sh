#!/usr/bin/env bash
# Bulanıklaştırma: depodaki her Go bulanıklaştırma hedefini (func FuzzX) verilen süre boyunca
# rastgele girdiyle koşar. Haftalık otomatik sınama işi (.github/workflows/fuzz.yml)
# bunu çağırır. Yerelde zayıf makinede tek bir hedefi kısa süre denemek için süzgeç verilir.
#
# Kullanım (depo kökünden): bash tools/fuzz.sh [süre] [süzgeç]
#   süre    her hedefin süresi, Go süre biçimiyle (varsayılan 30s)
#   süzgeç  yalnızca adı bu düzenli ifadeye uyan hedefler (varsayılan: hepsi)
# Bir hedef düşerse düşüren girdi testdata/fuzz/<Hedef>/ altına yazılır; betik onu günlüğe
# basar (otomatik sınama ayrıca yapıt olarak saklar).
set -u
set -o pipefail
cd "$(dirname "$0")/.."

duration="${1:-30s}"
filter="${2:-.}"
if ! [[ "$duration" =~ ^[0-9]+(\.[0-9]+)?(ns|us|ms|s|m|h)$ ]]; then
	echo "geçersiz süre: $duration (ör. 30s, 2m)" >&2
	exit 2
fi

# run_target bir hedefi koşar. Go'nun bulanıklaştırma motoru süre dolarken yanıt vermeyen bir
# işçiyi "context deadline exceeded" ile düşmüş sayabilir (girdi yazılmaz, çökme değildir):
# öyleyse hedef bir kez yeniden koşulur, ikincisi de düşerse düşmüş sayılır.
run_target() {
	local name=$1 dir=$2 output
	output=$(go test -run='^$' -fuzz="^${name}\$" -fuzztime="$duration" "./$dir" 2>&1)
	local code=$?
	echo "$output"
	[ $code -eq 0 ] && return 0
	if echo "$output" | grep -q 'context deadline exceeded' &&
		[ -z "$(git ls-files -o --exclude-standard -- "$dir/testdata/fuzz/$name")" ]; then
		echo "-- $name girdi yazmadan süre aşımıyla düştü (motorun zaman aşımı); bir kez yeniden koşuluyor"
		if [ "${GITHUB_ACTIONS:-}" = "true" ]; then echo "::warning title=Bulanıklaştırma::$name süre aşımıyla düştü, yeniden koşuldu"; fi
		go test -run='^$' -fuzz="^${name}\$" -fuzztime="$duration" "./$dir"
		return
	fi
	return $code
}

failed=0
count=0
# Hedefler: izlenen test dosyalarındaki "func FuzzX(" bildirimleri; klasör paket yoludur.
while IFS=: read -r file _ decl; do
	name=$(echo "$decl" | sed -E 's/^func (Fuzz[A-Za-z0-9_]*)\(.*/\1/')
	echo "$name" | grep -Eq -- "$filter" || continue
	dir=$(dirname "$file")
	count=$((count + 1))
	printf '\n== %s (./%s, %s)\n' "$name" "$dir" "$duration"
	if ! run_target "$name" "$dir"; then
		echo "!! KALDI: $name"
		if [ "${GITHUB_ACTIONS:-}" = "true" ]; then echo "::error title=Bulanıklaştırma::$name düştü"; fi
		failed=1
		# Yeni yazılan düşüren girdileri göster.
		for input in $(git ls-files -o --exclude-standard -- "$dir/testdata/fuzz/$name"); do
			echo "-- düşüren girdi: $input"
			cat "$input"
		done
	fi
done < <(git grep -n -E '^func Fuzz[A-Za-z0-9_]*\(' -- '*_test.go')

if [ $count -eq 0 ]; then
	echo "hiç bulanıklaştırma hedefi bulunamadı (süzgeç: $filter)" >&2
	exit 1
fi
echo
if [ $failed -eq 0 ]; then echo "✅ $count hedef düşmedi"; else echo "❌ düşen hedef var"; fi
exit $failed
