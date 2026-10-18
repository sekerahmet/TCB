#!/usr/bin/env bash
# Tam denetim: bir işin "bitti" sayılması için gereken bütün sınamalar, tek komutla. Otomatik
# sınama (.github/workflows/ci.yml) da bu betiği çağırır; "bitti"nin tek tanımı burasıdır.
#
# Kullanım (depo kökünden): bash bakim/tam_denetim.sh [--hizli] [--kip=v3|v1]
#   (bayraksız)  durağan denetimler; Go testleri GOAMD64=v3 ve GOAMD64=v1 ile; tr (v3) ile dil
#                tanımı testleri, örnek testleri ve örneklerin hepsi
#   --hizli      GOAMD64=v1 testlerini atlar (yalnızca iş sırasında; birleştirmeden önce tam
#                koşulmalı). --kip=v3 ile aynıdır.
#   --kip=v3     yalnızca GOAMD64=v3: durağan denetimler, Go testleri, tr testleri ve örnekler
#   --kip=v1     yalnızca GOAMD64=v1: derleme, Go testleri, tr testleri ve örnekler; işlemci
#                düzeyinden bağımsız durağan denetimler (üretilmiş dosyalar, gofmt, vet,
#                eklenti) atlanır. Otomatik sınamanın her işi bir kip koşar.
#
# Durağan denetimler: üretilmiş dosyalar güncel mi, gofmt, go vet, GPU'suz (-tags gpusuz) Linux
# ikilisinin statik olması, VS Code eklentisinin
# JavaScript dosyaları ve package.json. Go testleri -short ile koşar: dil tanımı testleri
# (testler/) Go testinin içinde yinelenmez, burada bir kez `tr test testler` ile koşar.
# Ortam değişkenleri: ATLANACAK_ORNEKLER (boşlukla ayrılmış örnek dosya adları; dış ağ
# gerektiren bir örnek eklenirse), ORNEK_ZAMAN_ASIMI (bir örneğin en uzun süresi, saniye; 300).
# Hız ölçümü bu betikte YOK: kesin ölçüm boş makinede ayrı yapılır (bakim/kiyas/kos.py).
set -u
# Borunun çıkış kodu, düşen ilk komutun kodudur; kodu bilerek ele alınan borular aşağıda
# PIPESTATUS ile okunur.
set -o pipefail
cd "$(dirname "$0")/.."

kip=hepsi
for arguman in "$@"; do
	case "$arguman" in
	--hizli | --kip=v3) kip=v3 ;;
	--kip=v1) kip=v1 ;;
	*)
		echo "bilinmeyen argüman: $arguman (kullanım: bash bakim/tam_denetim.sh [--hizli] [--kip=v3|v1])" >&2
		exit 2
		;;
	esac
done

hata=0
adim() { printf '\n== %s\n' "$1"; }
kalir() {
	echo "!! KALDI: $1"
	# Otomatik sınamada kalan adım özet sayfasında da görünsün.
	if [ "${GITHUB_ACTIONS:-}" = "true" ]; then echo "::error title=Tam denetim::$1"; fi
	hata=1
}

# ozetle komutu çalıştırır: geçerse çıktısının son satırını, kalırsa tamamını gösterir.
ozetle() {
	local ad=$1 cikti
	shift
	if cikti=$("$@" 2>&1); then
		echo "$cikti" | tail -1
	else
		echo "$cikti"
		kalir "$ad"
	fi
}

uzanti=""
case "$(uname -s)" in MINGW* | MSYS* | CYGWIN*) uzanti=".exe" ;; esac
TR="./ikili/tr${uzanti}"

case "$kip" in
hepsi) go_kipleri="v3 v1" ;;
*) go_kipleri="$kip" ;;
esac
# Derleme, vet ve tr ikilisi kipin işlemci düzeyiyle (bayraksız kipte v3) yapılır.
export GOAMD64="${go_kipleri%% *}"

if [ "$kip" != v1 ]; then
	adim "Üretilmiş dosyalar güncel mi"
	for arac in bakim/uret/*/; do
		if grep -q -- '-denetle' "$arac"*.go 2>/dev/null; then
			go run "./$arac" -denetle >/dev/null || kalir "$arac güncel değil (go run ./$arac ile üretin)"
		fi
	done

	adim "Biçim (gofmt) ve go vet"
	# Depodaki (izlenen ve henüz eklenmemiş) bütün Go dosyaları; .claude/ altındaki çalışma
	# ağaçları başka depolardır. Silinmiş ama henüz commitlenmemiş dosyalar atlanır.
	if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
		bicimsiz=$(git ls-files -co --exclude-standard -- '*.go' ':!:.claude/**' |
			while IFS= read -r f; do if [ -f "$f" ]; then printf '%s\n' "$f"; fi; done | xargs gofmt -l)
	else
		bicimsiz=$(gofmt -l ./cmd ./internal ./kitaplik ./bakim)
	fi
	[ $? -eq 0 ] || kalir "gofmt çalışmadı"
	[ -z "$bicimsiz" ] || kalir "gofmt ile biçimlenmesi gereken dosyalar: $bicimsiz"
	go vet ./... || kalir "go vet"

	# Varsayılan Linux ikilisi GPU sürücüsünü yükleyebilmek için glibc'ye dinamik bağlıdır;
	# "gpusuz" etiketli derleme statik kalmalıdır (bkz. docs/mimari.md, GPU arka ucu).
	adim "GPU'suz Linux ikilisi statik mi (-tags gpusuz)"
	statik_klasor=$(mktemp -d)
	if GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags gpusuz -o "$statik_klasor/tr" ./cmd/tr; then
		go run ./bakim/statik_denetim "$statik_klasor/tr" || kalir "gpusuz Linux ikilisi statik değil"
	else
		kalir "gpusuz Linux derlemesi"
	fi
	rm -rf "$statik_klasor"

	adim "VS Code eklentisi (JavaScript ve package.json)"
	if command -v node >/dev/null 2>&1; then
		for js in duzenleyici/vscode/*.js duzenleyici/vscode/media/*.js duzenleyici/vscode/test/*.js; do
			node --check "$js" || kalir "node --check $js"
		done
		for json in duzenleyici/vscode/package.json duzenleyici/vscode/package.nls.json; do
			[ -f "$json" ] || continue
			node -e "JSON.parse(require('fs').readFileSync(process.argv[1], 'utf8'))" "$json" || kalir "$json geçerli JSON değil"
		done
		echo "eklenti dosyaları geçerli"
	elif [ "${GITHUB_ACTIONS:-}" = "true" ]; then
		kalir "node bulunamadı; eklenti denetlenemedi"
	else
		echo "node bulunamadı; eklenti denetimi atlandı (otomatik sınamada zorunlu)"
	fi
fi

adim "Derleme (go build, GOAMD64=$GOAMD64)"
go build ./... || kalir "go build"

for k in $go_kipleri; do
	adim "Go testleri (GOAMD64=$k, -short)"
	GOAMD64=$k go test -short ./... 2>&1 | grep -v '^ok\|no test files'
	[ "${PIPESTATUS[0]}" -eq 0 ] || kalir "go test (GOAMD64=$k)"
done

adim "TR testleri ve örnekler (tr, GOAMD64=$GOAMD64)"
if mkdir -p ikili && go build -o "$TR" ./cmd/tr; then
	ozetle "tr test testler" "$TR" test testler
	ozetle "tr test ornekler" "$TR" test ornekler
	sure="${ORNEK_ZAMAN_ASIMI:-300}"
	for f in ornekler/*.tr; do
		ad=$(basename "$f")
		case " ${ATLANACAK_ORNEKLER:-} " in *" $ad "*)
			echo "↷ $ad atlandı"
			continue
			;;
		esac
		bas=$SECONDS
		# Standart çıktı atılır; standart hata (uyarılar, hata mesajları) denetlenir.
		cikti=$(timeout "$sure" "$TR" "$f" </dev/null 2>&1 >/dev/null)
		kod=$?
		if [ $kod -eq 124 ]; then
			kalir "$f $sure saniyede bitmedi"
		elif [ $kod -ne 0 ]; then
			kalir "$f çıkış kodu $kod: $(echo "$cikti" | head -20)"
		elif echo "$cikti" | grep -q 'Uyar'; then
			kalir "$f uyarı verdi: $(echo "$cikti" | grep 'Uyar' | head -3)"
		else
			echo "✓ $ad ($((SECONDS - bas)) sn)"
		fi
	done
else
	kalir "tr derlemesi"
fi

echo
if [ $hata -eq 0 ]; then echo "✅ Tam denetim geçti"; else echo "❌ Tam denetimde kalan var"; fi
exit $hata
