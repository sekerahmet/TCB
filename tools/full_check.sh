#!/usr/bin/env bash
# Tam denetim: bir işin "bitti" sayılması için gereken bütün sınamalar, tek komutla. Otomatik
# sınama (.github/workflows/ci.yml) da bu betiği çağırır; "bitti"nin tek tanımı burasıdır.
#
# Kullanım (depo kökünden): bash tools/tam_denetim.sh [--hizli] [--kip=KİP]
#   (bayraksız)       dagitim ve go-esleri kipleri art arda (yerelde birleştirmeden önce)
#   --kip=dagitim     kullanıcıya giden yapılandırma: düz `go build` (GOAMD64=v1), AVX2
#                     çekirdekleri çalışma anında seçilir. Durağan denetimler, ARM derlemesi,
#                     Go testleri, tr testleri (tests/, lib/, examples/) ve örneklerin hepsi.
#   --kip=go-esleri   AVX2 çekirdekleri kapalı (TR_CPU_AVX2=0): assembly'siz Go eşleri. Derleme,
#                     Go testleri ve dil tanımı testleri (tr test tests).
#   --kip=v3          isteğe bağlı: GOAMD64=v3 derleme ile dagitim'in testleri (kullanıcıya
#                     önerilmez; derleyicinin v3 kod üretimini denemek için)
#   --kip=yaris       yarış dedektörü (go test -race -short) eşzamanlı kod taşıyan paketlerde;
#                     cgo ve bir C derleyicisi ister (otomatik sınamada Linux'ta koşar)
#   --kip=uzun        Go testleri -short olmadan (her .tr dosyasının İngilizce gidiş-dönüşü
#                     gibi uzun sınamalar dahil)
#   --hizli           --kip=dagitim ile aynı (iş sırasında; birleştirmeden önce bayraksız koşulur)
#
# Durağan denetimler: üretilmiş dosyalar güncel mi (her üreticinin -denetle bayrağı olmalı),
# gofmt, go vet, go.mod/go.sum (go mod verify, go mod tidy -diff), ARM (arm64) derlemesi ve
# vet'i, GPU'suz (-tags gpusuz) Linux program dosyasının statik olması, VS Code eklentisinin
# JavaScript dosyaları ve package.json. Go testleri -short ile koşar (uzun kipi hariç): dil
# tanımı testleri (tests/) Go testinin içinde yinelenmez, burada `tr test tests` ile koşar.
# Örnekler depoda iz bırakmaz: izlenen örnekler ve test verisi geçici bir klasöre kopyalanıp
# orada çalışır (koşu klasörleri, .trt dosyaları oraya yazılır; her koşu baştan başlar); sonda
# depodaki değişikliklerin (git status) denetimin başındakiyle aynı olduğu denetlenir.
#
# Ortam değişkenleri: ATLANACAK_ORNEKLER (boşlukla ayrılmış örnek dosya adları; dış ağ
# gerektiren bir örnek eklenirse), ORNEK_ZAMAN_ASIMI (bir örneğin en uzun süresi, saniye; 300),
# TAM_DENETIM_P (go test'in aynı anda derleyip koşacağı paket sayısı, -p; zayıf makinede 1–2).
# TR_CPU_AVX2'yi kip belirler; ortamda başka bir değer varsa yok sayıldığı yazılır.
# Hız ölçümü bu betikte YOK: kesin ölçüm boş makinede ayrı yapılır (tools/bench/kos.py).
set -u
# Borunun çıkış kodu, düşen ilk komutun kodudur; kodu bilerek ele alınan borular aşağıda
# PIPESTATUS ile okunur.
set -o pipefail
cd "$(dirname "$0")/.."

kipler="dagitim go-esleri"
for arguman in "$@"; do
	case "$arguman" in
	--hizli | --kip=dagitim) kipler=dagitim ;;
	--kip=go-esleri | --kip=v1) kipler=go-esleri ;;
	--kip=v3) kipler=v3 ;;
	--kip=yaris) kipler=yaris ;;
	--kip=uzun) kipler=uzun ;;
	*)
		echo "bilinmeyen argüman: $arguman (kullanım: bash tools/tam_denetim.sh [--hizli] [--kip=dagitim|go-esleri|v3|yaris|uzun])" >&2
		exit 2
		;;
	esac
done

hata=0
kalanlar=()
sureler=()
adim_adi=""
adim_basi=$SECONDS
# adim yeni bir adım başlatır; bir öncekinin süresi sondaki özete yazılır.
adim() {
	adim_bitir
	adim_adi=$1
	adim_basi=$SECONDS
	printf '\n== %s\n' "$1"
}
adim_bitir() {
	if [ -n "$adim_adi" ]; then sureler+=("$(printf '%4d sn  %s' $((SECONDS - adim_basi)) "$adim_adi")"); fi
	adim_adi=""
}
kalir() {
	echo "!! KALDI: $1"
	# Otomatik sınamada kalan adım özet sayfasında da görünsün.
	if [ "${GITHUB_ACTIONS:-}" = "true" ]; then echo "::error title=Tam denetim::$1"; fi
	kalanlar+=("$1")
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
TR="$PWD/bin/tr${uzanti}"
p_bayragi=()
if [ -n "${TAM_DENETIM_P:-}" ]; then p_bayragi=(-p "$TAM_DENETIM_P"); fi

# Kip başına ortam: GOAMD64 (derleyicinin kod üretimi) ve TR_CPU_AVX2 (çalışma anındaki
# assembly çekirdekleri). Kullanıcıya giden ikili düz `go build`'dir: GOAMD64=v1, AVX2 açık.
if [ -n "${TR_CPU_AVX2+x}" ]; then
	echo "not: ortamdaki TR_CPU_AVX2=${TR_CPU_AVX2} yok sayıldı; değeri kip belirler"
fi
if [ -n "${GOAMD64+x}" ]; then
	echo "not: ortamdaki GOAMD64=${GOAMD64} yok sayıldı; değeri kip belirler"
fi
kip_ortami() {
	case "$1" in
	v3) export GOAMD64=v3 TR_CPU_AVX2="" ;;
	go-esleri) export GOAMD64=v1 TR_CPU_AVX2=0 ;;
	*) export GOAMD64=v1 TR_CPU_AVX2="" ;;
	esac
}

# Örneklerin bıraktığı izi yakalamak için depodaki değişikliklerin başlangıçtaki hali.
depo_durumu() { git status --porcelain=v1 --untracked-files=all 2>/dev/null; }
bas_durum=$(depo_durumu)

durgan_denetimler() {
	adim "Üretilmiş dosyalar güncel mi"
	for arac in tools/gen/*/; do
		if grep -q -- '-denetle' "$arac"*.go 2>/dev/null; then
			go run "./$arac" -denetle >/dev/null || kalir "$arac güncel değil (go run ./$arac ile üretin)"
		else
			kalir "$arac -denetle bayrağı yok: üretilmiş dosyalarının güncelliği denetlenemiyor"
		fi
	done

	adim "Biçim (gofmt), go vet, go.mod"
	# Depodaki (izlenen ve henüz eklenmemiş) bütün Go dosyaları; .claude/ altındaki çalışma
	# ağaçları başka depolardır. Silinmiş ama henüz commitlenmemiş dosyalar atlanır.
	if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
		bicimsiz=$(git ls-files -co --exclude-standard -- '*.go' ':!:.claude/**' |
			while IFS= read -r f; do if [ -f "$f" ]; then printf '%s\n' "$f"; fi; done | xargs -r -d '\n' gofmt -l)
	else
		bicimsiz=$(gofmt -l ./cmd ./internal ./lib ./tools)
	fi
	[ $? -eq 0 ] || kalir "gofmt çalışmadı"
	[ -z "$bicimsiz" ] || kalir "gofmt ile biçimlenmesi gereken dosyalar: $bicimsiz"
	go vet ./... || kalir "go vet"
	go mod verify >/dev/null || kalir "go mod verify (modül önbelleği go.sum ile uyuşmuyor)"
	go mod tidy -diff >/dev/null || kalir "go.mod/go.sum düzenli değil (go mod tidy ile düzeltin)"

	adim "ARM (arm64) derlemesi ve vet"
	for os in linux darwin windows; do
		GOOS=$os GOARCH=arm64 go build ./... || kalir "$os/arm64 derlemesi"
	done
	for os in linux darwin; do
		GOOS=$os GOARCH=arm64 go vet ./... || kalir "$os/arm64 go vet"
	done

	# Varsayılan Linux program dosyası GPU sürücüsünü yükleyebilmek için glibc'ye dinamik bağlıdır;
	# "gpusuz" etiketli derleme statik kalmalıdır (bkz. docs/mimari.md, GPU arka ucu).
	adim "GPU'suz Linux program dosyası statik mi (-tags gpusuz)"
	statik_klasor=$(mktemp -d)
	if GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags gpusuz -o "$statik_klasor/tr" ./cmd/tr; then
		go run ./tools/checkstatic "$statik_klasor/tr" || kalir "gpusuz Linux program dosyası statik değil"
	else
		kalir "gpusuz Linux derlemesi"
	fi
	rm -rf "$statik_klasor"

	adim "VS Code eklentisi (JavaScript ve package.json)"
	if command -v node >/dev/null 2>&1; then
		shopt -s nullglob
		for js in editors/vscode/*.js editors/vscode/media/*.js editors/vscode/test/*.js; do
			node --check "$js" || kalir "node --check $js"
		done
		shopt -u nullglob
		for json in editors/vscode/package.json editors/vscode/package.nls.json; do
			[ -f "$json" ] || continue
			node -e "JSON.parse(require('fs').readFileSync(process.argv[1], 'utf8'))" "$json" || kalir "$json geçerli JSON değil"
		done
		echo "eklenti dosyaları geçerli"
	elif [ "${GITHUB_ACTIONS:-}" = "true" ]; then
		kalir "node bulunamadı; eklenti denetlenemedi"
	else
		echo "node bulunamadı; eklenti denetimi atlandı (otomatik sınamada zorunlu)"
	fi
}

# go_testleri kipin ortamıyla Go testlerini koşar; ek bayraklar (ör. -short) argümandır.
go_testleri() {
	local ad=$1
	shift
	adim "Go testleri ($ad: GOAMD64=$GOAMD64, TR_CPU_AVX2=${TR_CPU_AVX2:-otomatik})"
	go test "${p_bayragi[@]}" "$@" ./... 2>&1 | grep -v '^ok\|no test files'
	[ "${PIPESTATUS[0]}" -eq 0 ] || kalir "go test ($ad)"
}

tr_derle() {
	mkdir -p bin && go build -o "$TR" ./cmd/tr
}

# ornekler örnekleri geçici bir klasörde, depodaki düzenle (examples/, tests/data/, docs/) çalıştırır.
ornekler() {
	local calisma f ad bas cikti kod sure
	calisma=$(mktemp -d)
	git ls-files -z -- examples tests/data docs | while IFS= read -r -d '' f; do
		mkdir -p "$calisma/$(dirname "$f")" && cp "$f" "$calisma/$f"
	done
	sure="${ORNEK_ZAMAN_ASIMI:-300}"
	(
		cd "$calisma" || exit 1
		# Uyarılar Türkçe aranır (aşağıdaki 'Uyar'); ortamın dil ayarı bunu körleştirmesin.
		export TR_DIL=tr
		if cikti=$("$TR" test examples 2>&1); then
			echo "$cikti" | tail -1
		else
			echo "$cikti"
			echo "!! KALDI: tr test examples"
		fi
		for f in examples/*.tr; do
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
				echo "!! KALDI: $f $sure saniyede bitmedi"
			elif [ $kod -ne 0 ]; then
				echo "!! KALDI: $f çıkış kodu $kod: $(echo "$cikti" | head -20)"
			elif echo "$cikti" | grep -q 'Uyar'; then
				echo "!! KALDI: $f uyarı verdi: $(echo "$cikti" | grep 'Uyar' | head -3)"
			else
				echo "✓ $ad ($((SECONDS - bas)) sn)"
			fi
		done
	) | while IFS= read -r satir; do
		case "$satir" in "!! KALDI: "*) echo "${satir#!! KALDI: }" >>"$calisma.kalan" ;; esac
		echo "$satir"
	done
	if [ -f "$calisma.kalan" ]; then
		while IFS= read -r satir; do kalanlar+=("$satir"); done <"$calisma.kalan"
		[ "${GITHUB_ACTIONS:-}" != "true" ] || while IFS= read -r satir; do echo "::error title=Tam denetim::$satir"; done <"$calisma.kalan"
		hata=1
	fi
	rm -rf "$calisma" "$calisma.kalan"
}

for kip in $kipler; do
	kip_ortami "$kip"
	case "$kip" in
	dagitim | v3)
		[ "$kip" = v3 ] || durgan_denetimler
		adim "Derleme (go build, GOAMD64=$GOAMD64)"
		go build ./... || kalir "go build"
		go_testleri "$kip" -short
		adim "TR testleri ve örnekler ($kip)"
		if tr_derle; then
			ozetle "tr test tests" "$TR" test tests
			ozetle "tr test lib" "$TR" test lib
			ornekler
		else
			kalir "tr derlemesi"
		fi
		;;
	go-esleri)
		adim "Derleme (go build, GOAMD64=$GOAMD64)"
		go build ./... || kalir "go build"
		go_testleri go-esleri -short
		adim "Dil tanımı testleri (go-esleri)"
		if tr_derle; then ozetle "tr test tests (go-esleri)" "$TR" test tests; else kalir "tr derlemesi"; fi
		;;
	yaris)
		# Eşzamanlı kod: yorumlayıcı (görevler), motorlar, veri hattı (önden yükleme), tensör
		# (işçi havuzu, atomik sayaçlar), parçalayıcı (paralel toplu kodlama), GPU (eşzamansız
		# benzetici).
		adim "Yarış dedektörü (go test -race -short)"
		CGO_ENABLED=1 go test "${p_bayragi[@]}" -race -short ./internal/lang/interp ./internal/stdlib/... \
			./internal/compute/dataload ./internal/compute/tensor ./internal/compute/tokenize \
			./internal/compute/gpu 2>&1 | grep -v '^ok\|no test files'
		[ "${PIPESTATUS[0]}" -eq 0 ] || kalir "go test -race"
		;;
	uzun)
		go_testleri uzun
		;;
	esac
done
adim_bitir

if [ "$(depo_durumu)" != "$bas_durum" ]; then
	kalir "denetim depoda iz bıraktı (git status değişti): $(diff <(echo "$bas_durum") <(depo_durumu) | head -10)"
fi

echo
echo "Adım süreleri:"
printf '%s\n' "${sureler[@]}"
if [ $hata -eq 0 ]; then
	echo "✅ Tam denetim geçti"
else
	echo "❌ Tam denetimde kalan var:"
	printf '  - %s\n' "${kalanlar[@]}"
fi
exit $hata
