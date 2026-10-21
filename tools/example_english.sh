#!/usr/bin/env bash
# example_english.sh: examples/tr dosyasından examples/en eşini üretmeye yardım eder.
#
#   bash tools/example_english.sh draft examples/tr/05_kosullar.tr
#       Türkçe örneğin İngilizce gösterimini (dilin sözcükleri İngilizce, kullanıcı adları,
#       yorumlar ve metinler Türkçe) standart çıktıya yazar. Elle çevirmek için taslaktır.
#   bash tools/example_english.sh store ceviri.tr examples/en/05_conditions.tr
#       Elle çevrilmiş İngilizce gösterimi kodlu saklama biçimine çevirip hedefe yazar.
#
# Çevirirken satır düzenini koruyun (aynı sayıda satır) ve kullanıcı adlarını dilin üye
# adlarıyla çakıştırmayın (name, size, sampler ...): çakışan ad kodlanır, iskelet bozulur.
# Sınama: go test ./cmd/tr/e2e -run TestExamples
set -euo pipefail
TR="${TR:-bin/tr}"
[ -x "$TR" ] || TR="${TR}.exe"
case "${1:-}" in
draft)
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT
	cp "$2" "$tmp/draft.tr"
	"$TR" kodla "$tmp/draft.tr" >/dev/null
	"$TR" göster "$tmp/draft.tr" --dil en
	;;
store)
	"$TR" kodla - --dil en <"$2" >"$3.tmp"
	mv "$3.tmp" "$3"
	;;
*)
	echo "kullanım: $0 draft <tr-dosyası> | store <ingilizce-gösterim> <hedef>" >&2
	exit 2
	;;
esac
