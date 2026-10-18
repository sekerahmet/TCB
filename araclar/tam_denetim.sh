#!/usr/bin/env bash
# Tam denetim: bir işin "bitti" sayılması için gereken bütün sınamalar, tek komutla.
# Kullanım (depo kökünden): bash araclar/tam_denetim.sh [--hizli]
#   --hizli  GOAMD64=v1 testlerini atlar (yalnızca iş sırasında; birleştirmeden önce tam koşulmalı).
# Hız ölçümü bu betikte YOK: kesin ölçüm boş makinede ayrı yapılır (kiyas/kos.py).
set -u
cd "$(dirname "$0")/.."

hata=0
adim() { printf '\n== %s\n' "$1"; }
kalir() { echo "!! KALDI: $1"; hata=1; }

uzanti=""
case "$(uname -s)" in MINGW*|MSYS*|CYGWIN*) uzanti=".exe" ;; esac
TR="./tr${uzanti}"

adim "Üretilmiş dosyalar güncel mi"
for arac in araclar/*/; do
	if grep -q -- '-denetle' "$arac"*.go 2>/dev/null; then
		go run "./$arac" -denetle >/dev/null || kalir "$arac güncel değil (go run ./$arac ile üretin)"
	fi
done

adim "Biçim ve vet"
bicimsiz=$(gofmt -l ./cmd ./pkg ./araclar 2>/dev/null)
[ -z "$bicimsiz" ] || kalir "gofmt: $bicimsiz"
go vet ./... || kalir "go vet"

adim "Go testleri (GOAMD64=v3)"
GOAMD64=v3 go test ./... 2>&1 | grep -v '^ok\|no test files' ; [ "${PIPESTATUS[0]}" -eq 0 ] || kalir "go test (v3)"

if [ "${1:-}" != "--hizli" ]; then
	adim "Go testleri (GOAMD64=v1)"
	GOAMD64=v1 go test ./... 2>&1 | grep -v '^ok\|no test files' ; [ "${PIPESTATUS[0]}" -eq 0 ] || kalir "go test (v1)"
fi

adim "TR testleri ve örnekler"
GOAMD64=v3 go build -o "$TR" ./cmd/tr || kalir "derleme"
"$TR" test testler | tail -1 || kalir "tr test testler"
"$TR" test ornekler | tail -1 || kalir "tr test ornekler"
for f in ornekler/0*.tr; do
	cikti=$(timeout 300 "$TR" "$f" </dev/null 2>&1 >/dev/null)
	kod=$?
	[ $kod -eq 0 ] || kalir "$f çıkış kodu $kod: $(echo "$cikti" | head -3)"
	echo "$cikti" | grep -q 'Uyar' && kalir "$f uyarı verdi"
done

echo
if [ $hata -eq 0 ]; then echo "✅ Tam denetim geçti"; else echo "❌ Tam denetimde kalan var"; fi
exit $hata
