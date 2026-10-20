#!/usr/bin/env bash
# Tam denetim: bir işin "bitti" sayılması için gereken bütün sınamalar, tek komutla. Otomatik
# sınama (.github/workflows/ci.yml) da bu betiği çağırır; "bitti"nin tek tanımı burasıdır.
#
# Kullanım (depo kökünden): bash tools/full_check.sh [--hizli] [--kip=KİP]
#   (bayraksız)       dagitim ve go-esleri kipleri art arda (yerelde birleştirmeden önce)
#   --kip=dagitim     kullanıcıya giden yapılandırma: düz `go build` (GOAMD64=v1), AVX2
#                     çekirdekleri çalışma anında seçilir. Durağan denetimler, ARM derlemesi,
#                     Go testleri, tr testleri (tests/, lib/, examples/) ve örneklerin hepsi.
#   --kip=go-esleri   AVX2 çekirdekleri kapalı (TR_CPU_AVX2=0): assembly'siz Go eşleri. Derleme,
#                     assembly'nin bulunduğu paketin ve onun çekirdeklerini en yoğun kullanan
#                     paketin Go testleri (esleri_paketleri) ve dil tanımı testleri (tr test tests).
#                     Assembly yalnızca internal/compute/tensor'dadır; öbür paketlerin Go kodu iki
#                     kipte aynıdır, bütün modülü ikinci kez koşmak denetimi ~3 dk uzatıyordu.
#   --kip=v3          isteğe bağlı: GOAMD64=v3 derleme ile dagitim'in testleri (kullanıcıya
#                     önerilmez; derleyicinin v3 kod üretimini denemek için)
#   --kip=yaris       yarış dedektörü (go test -race -short) eşzamanlı kod taşıyan paketlerde;
#                     cgo ve bir C derleyicisi ister (otomatik sınamada Linux'ta koşar)
#   --kip=uzun        Go testleri -short olmadan (her .tr dosyasının İngilizce gidiş-dönüşü
#                     gibi uzun sınamalar dahil)
#   --hizli           --kip=dagitim ile aynı (iş sırasında; birleştirmeden önce bayraksız koşulur)
#
# Durağan denetimler: üretilmiş dosyalar güncel mi (her üreticinin -check bayrağı olmalı),
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
# Hız ölçümü bu betikte YOK: kesin ölçüm boş makinede ayrı yapılır (tools/bench/run.py).
set -u
# Borunun çıkış kodu, düşen ilk komutun kodudur; kodu bilerek ele alınan borular aşağıda
# PIPESTATUS ile okunur.
set -o pipefail
cd "$(dirname "$0")/.."

modes="dagitim go-esleri"
for arg in "$@"; do
	case "$arg" in
	--hizli | --kip=dagitim) modes=dagitim ;;
	--kip=go-esleri | --kip=v1) modes=go-esleri ;;
	--kip=v3) modes=v3 ;;
	--kip=yaris) modes=yaris ;;
	--kip=uzun) modes=uzun ;;
	*)
		echo "bilinmeyen argüman: $arg (kullanım: bash tools/full_check.sh [--hizli] [--kip=dagitim|go-esleri|v3|yaris|uzun])" >&2
		exit 2
		;;
	esac
done

failed=0
failures=()
durations=()
step_name=""
step_start=$SECONDS
# step yeni bir adım başlatır; bir öncekinin süresi sondaki özete yazılır.
step() {
	step_end
	step_name=$1
	step_start=$SECONDS
	printf '\n== %s\n' "$1"
}
step_end() {
	if [ -n "$step_name" ]; then durations+=("$(printf '%4d sn  %s' $((SECONDS - step_start)) "$step_name")"); fi
	step_name=""
}
fail() {
	echo "!! KALDI: $1"
	# Otomatik sınamada kalan adım özet sayfasında da görünsün.
	if [ "${GITHUB_ACTIONS:-}" = "true" ]; then echo "::error title=Tam denetim::$1"; fi
	failures+=("$1")
	failed=1
}

# summarize komutu çalıştırır: geçerse çıktısının son satırını, kalırsa tamamını gösterir.
summarize() {
	local name=$1 output
	shift
	if output=$("$@" 2>&1); then
		echo "$output" | tail -1
	else
		echo "$output"
		fail "$name"
	fi
}

ext=""
case "$(uname -s)" in MINGW* | MSYS* | CYGWIN*) ext=".exe" ;; esac
TR="$PWD/bin/tr${ext}"
p_flag=()
if [ -n "${TAM_DENETIM_P:-}" ]; then p_flag=(-p "$TAM_DENETIM_P"); fi

# Kip başına ortam: GOAMD64 (derleyicinin kod üretimi) ve TR_CPU_AVX2 (çalışma anındaki
# assembly çekirdekleri). Kullanıcıya giden ikili düz `go build`'dir: GOAMD64=v1, AVX2 açık.
if [ -n "${TR_CPU_AVX2+x}" ]; then
	echo "not: ortamdaki TR_CPU_AVX2=${TR_CPU_AVX2} yok sayıldı; değeri kip belirler"
fi
if [ -n "${GOAMD64+x}" ]; then
	echo "not: ortamdaki GOAMD64=${GOAMD64} yok sayıldı; değeri kip belirler"
fi
mode_env() {
	case "$1" in
	v3) export GOAMD64=v3 TR_CPU_AVX2="" ;;
	go-esleri) export GOAMD64=v1 TR_CPU_AVX2=0 ;;
	*) export GOAMD64=v1 TR_CPU_AVX2="" ;;
	esac
}

# Örneklerin bıraktığı izi yakalamak için depodaki değişikliklerin başlangıçtaki hali.
repo_state() { git status --porcelain=v1 --untracked-files=all 2>/dev/null; }
initial_state=$(repo_state)

static_checks() {
	step "Üretilmiş dosyalar güncel mi"
	for tool in tools/gen/*/; do
		if grep -q -- '"check"' "$tool"*.go 2>/dev/null; then
			go run "./$tool" -check >/dev/null || fail "$tool güncel değil (go run ./$tool ile üretin)"
		else
			fail "$tool -check bayrağı yok: üretilmiş dosyalarının güncelliği denetlenemiyor"
		fi
	done

	step "Biçim (gofmt), go vet, go.mod"
	# Depodaki (izlenen ve henüz eklenmemiş) bütün Go dosyaları; .claude/ altındaki çalışma
	# ağaçları başka depolardır. Silinmiş ama henüz commitlenmemiş dosyalar atlanır.
	if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
		unformatted=$(git ls-files -co --exclude-standard -- '*.go' ':!:.claude/**' |
			while IFS= read -r f; do if [ -f "$f" ]; then printf '%s\n' "$f"; fi; done | xargs -r -d '\n' gofmt -l)
	else
		unformatted=$(gofmt -l ./cmd ./internal ./lib ./tools)
	fi
	[ $? -eq 0 ] || fail "gofmt çalışmadı"
	[ -z "$unformatted" ] || fail "gofmt ile biçimlenmesi gereken dosyalar: $unformatted"
	go vet ./... || fail "go vet"
	go mod verify >/dev/null || fail "go mod verify (modül önbelleği go.sum ile uyuşmuyor)"
	go mod tidy -diff >/dev/null || fail "go.mod/go.sum düzenli değil (go mod tidy ile düzeltin)"

	step "ARM (arm64) derlemesi ve vet"
	for os in linux darwin windows; do
		GOOS=$os GOARCH=arm64 go build ./... || fail "$os/arm64 derlemesi"
	done
	for os in linux darwin; do
		GOOS=$os GOARCH=arm64 go vet ./... || fail "$os/arm64 go vet"
	done

	# Varsayılan Linux program dosyası GPU sürücüsünü yükleyebilmek için glibc'ye dinamik bağlıdır;
	# "gpusuz" etiketli derleme statik kalmalıdır (bkz. docs/architecture.md, GPU arka ucu).
	step "GPU'suz Linux program dosyası statik mi (-tags gpusuz)"
	static_dir=$(mktemp -d)
	if GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags gpusuz -o "$static_dir/tr" ./cmd/tr; then
		go run ./tools/checkstatic "$static_dir/tr" || fail "gpusuz Linux program dosyası statik değil"
	else
		fail "gpusuz Linux derlemesi"
	fi
	rm -rf "$static_dir"

	step "VS Code eklentisi (JavaScript ve package.json)"
	if command -v node >/dev/null 2>&1; then
		shopt -s nullglob
		for js in editors/vscode/*.js editors/vscode/media/*.js editors/vscode/test/*.js; do
			node --check "$js" || fail "node --check $js"
		done
		shopt -u nullglob
		for json in editors/vscode/package.json editors/vscode/package.nls.json; do
			[ -f "$json" ] || continue
			node -e "JSON.parse(require('fs').readFileSync(process.argv[1], 'utf8'))" "$json" || fail "$json geçerli JSON değil"
		done
		echo "eklenti dosyaları geçerli"
	elif [ "${GITHUB_ACTIONS:-}" = "true" ]; then
		fail "node bulunamadı; eklenti denetlenemedi"
	else
		echo "node bulunamadı; eklenti denetimi atlandı (otomatik sınamada zorunlu)"
	fi
}

# esleri_paketleri go-esleri kipinde Go testleri koşan paketlerdir: assembly çekirdeklerinin
# paketi (Go eşleri burada sınanır) ve çekirdekleri en çok kullanan dil paketi (ml). Assembly
# başka pakete eklenirse buraya da eklenir (TestGoEquivalentPackagesListed korur).
esleri_paketleri=(./internal/compute/tensor ./internal/stdlib/ml)

# go_tests kipin ortamıyla Go testlerini koşar; ek bayraklar (ör. -short) argümandır. Paketler
# test_paketleri dizisindedir (varsayılan bütün modül).
test_paketleri=(./...)
go_tests() {
	local name=$1
	shift
	step "Go testleri ($name: GOAMD64=$GOAMD64, TR_CPU_AVX2=${TR_CPU_AVX2:-otomatik})"
	go test "${p_flag[@]}" "$@" "${test_paketleri[@]}" 2>&1 | grep -v '^ok\|no test files'
	[ "${PIPESTATUS[0]}" -eq 0 ] || fail "go test ($name)"
}

build_tr() {
	mkdir -p bin && go build -o "$TR" ./cmd/tr
}

# extension_tests VS Code eklentisinin davranış testlerini (editors/vscode/test/*_test.js) derlenen
# yorumlayıcıyla koşar. Yorumlayıcı mutlak yolla verilir; bir test "atlandı" derse denetim kalır
# (koşulmayan test hiçbir şeyi korumaz).
extension_tests() {
	local js output
	command -v node >/dev/null 2>&1 || { [ "${GITHUB_ACTIONS:-}" = "true" ] && fail "node bulunamadı; eklenti testleri koşulamadı"; return; }
	for js in editors/vscode/test/*_test.js; do
		if ! output=$(TR_YORUMLAYICI="$TR" node "$js" 2>&1); then
			echo "$output"
			fail "eklenti testi: $js"
			continue
		fi
		echo "$output" | tail -1
		if echo "$output" | grep -q "atlandı"; then
			fail "eklenti testi atlandı: $js"
		fi
	done
}

# run_examples örnekleri geçici bir klasörde, depodaki düzenle (examples/, tests/data/, docs/) çalıştırır.
run_examples() {
	local workdir f name started output code limit
	workdir=$(mktemp -d)
	git ls-files -z -- examples tests/data docs | while IFS= read -r -d '' f; do
		mkdir -p "$workdir/$(dirname "$f")" && cp "$f" "$workdir/$f"
	done
	limit="${ORNEK_ZAMAN_ASIMI:-300}"
	(
		cd "$workdir" || exit 1
		# Uyarılar Türkçe aranır (aşağıdaki 'Uyar'); ortamın dil ayarı bunu körleştirmesin.
		export TR_DIL=tr
		if output=$("$TR" test examples 2>&1); then
			echo "$output" | tail -1
		else
			echo "$output"
			echo "!! KALDI: tr test examples"
		fi
		for f in examples/*.tr; do
			name=$(basename "$f")
			case " ${ATLANACAK_ORNEKLER:-} " in *" $name "*)
				echo "↷ $name atlandı"
				continue
				;;
			esac
			started=$SECONDS
			# Standart çıktı atılır; standart hata (uyarılar, hata mesajları) denetlenir.
			output=$(timeout "$limit" "$TR" "$f" </dev/null 2>&1 >/dev/null)
			code=$?
			if [ $code -eq 124 ]; then
				echo "!! KALDI: $f $limit saniyede bitmedi"
			elif [ $code -ne 0 ]; then
				echo "!! KALDI: $f çıkış kodu $code: $(echo "$output" | head -20)"
			elif echo "$output" | grep -q 'Uyar'; then
				echo "!! KALDI: $f uyarı verdi: $(echo "$output" | grep 'Uyar' | head -3)"
			else
				echo "✓ $name ($((SECONDS - started)) sn)"
			fi
		done
	) | while IFS= read -r line; do
		case "$line" in "!! KALDI: "*) echo "${line#!! KALDI: }" >>"$workdir.failures" ;; esac
		echo "$line"
	done
	if [ -f "$workdir.failures" ]; then
		while IFS= read -r line; do failures+=("$line"); done <"$workdir.failures"
		[ "${GITHUB_ACTIONS:-}" != "true" ] || while IFS= read -r line; do echo "::error title=Tam denetim::$line"; done <"$workdir.failures"
		failed=1
	fi
	rm -rf "$workdir" "$workdir.failures"
}

for mode in $modes; do
	mode_env "$mode"
	case "$mode" in
	dagitim | v3)
		[ "$mode" = v3 ] || static_checks
		step "Derleme (go build, GOAMD64=$GOAMD64)"
		go build ./... || fail "go build"
		go_tests "$mode" -short
		step "TR testleri ve örnekler ($mode)"
		if build_tr; then
			summarize "tr test tests" "$TR" test tests
			summarize "tr test lib" "$TR" test lib
			run_examples
			[ "$mode" = v3 ] || extension_tests
		else
			fail "tr derlemesi"
		fi
		;;
	go-esleri)
		step "Derleme (go build, GOAMD64=$GOAMD64)"
		go build ./... || fail "go build"
		test_paketleri=("${esleri_paketleri[@]}")
		# -count=1: TR_CPU_AVX2 paket başlatılırken okunur ve Go test önbelleğinin anahtarına
		# girmez; derleme de dagitim'le aynı olduğundan önbellek dagitim'in sonucunu verirdi
		# (Go eşleri hiç koşmazdı).
		go_tests go-esleri -short -count=1
		test_paketleri=(./...)
		step "Dil tanımı testleri (go-esleri)"
		if build_tr; then summarize "tr test tests (go-esleri)" "$TR" test tests; else fail "tr derlemesi"; fi
		;;
	yaris)
		# Eşzamanlı kod: yorumlayıcı (görevler), motorlar, veri hattı (önden yükleme), tensör
		# (işçi havuzu, atomik sayaçlar), parçalayıcı (paralel toplu kodlama), GPU (eşzamansız
		# benzetici).
		step "Yarış dedektörü (go test -race -short)"
		CGO_ENABLED=1 go test "${p_flag[@]}" -race -short ./internal/lang/interp ./internal/stdlib/... \
			./internal/compute/dataload ./internal/compute/tensor ./internal/compute/tokenize \
			./internal/compute/gpu 2>&1 | grep -v '^ok\|no test files'
		[ "${PIPESTATUS[0]}" -eq 0 ] || fail "go test -race"
		;;
	uzun)
		go_tests uzun
		;;
	esac
done
step_end

if [ "$(repo_state)" != "$initial_state" ]; then
	fail "denetim depoda iz bıraktı (git status değişti): $(diff <(echo "$initial_state") <(repo_state) | head -10)"
fi

echo
echo "Adım süreleri:"
printf '%s\n' "${durations[@]}"
if [ $failed -eq 0 ]; then
	echo "✅ Tam denetim geçti"
else
	echo "❌ Tam denetimde kalan var:"
	printf '  - %s\n' "${failures[@]}"
fi
exit $failed
