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
#   --kip=zorlama     kararlılık (yerelde ~40 dk; otomatik sınamada haftalık, zorlama.yml): bütün
#                     Go testleri -short olmadan iki kez ve karışık sırayla (-count=2 -shuffle=on;
#                     tekrar sayısına, sıraya ve süreç geneli duruma bağlı test), sonra GOGC=1 ile
#                     (çöp toplayıcının zamanlamasına bağlı test). Kalan testin karışık sırası
#                     çıktıdaki -test.shuffle tohumuyla yeniden üretilir.
#   --hizli           --kip=dagitim ile aynı (iş sırasında; birleştirmeden önce bayraksız koşulur)
#   --kip=on          ön denetim (~1 dk): üretilmiş dosyalar, gofmt, vet, go.mod, staticcheck,
#                     ölü kod, düzen kuralları (dosya adı, satır sınırı, Türkçe metin, çıplak ad,
#                     ileti kullanımı), değişen
#                     paketlerin kısa testleri ve yeni tekrar adayları (codegraph dups --changed).
#                     Yan oturumlar devir notundan, ana oturum GPU yan dalına göndermeden önce
#                     koşar; tam denetimin yerini tutmaz.
#
# Durağan denetimler: üretilmiş dosyalar güncel mi (her üreticinin -check bayrağı olmalı),
# gofmt, go vet, go.mod/go.sum (go mod verify, go mod tidy -diff), staticcheck, ölü kod
# (deadcode), ARM (arm64) derlemesi ve
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
	--kip=zorlama) modes=zorlama ;;
	--kip=on) modes=on ;;
	*)
		echo "bilinmeyen argüman: $arg (kullanım: bash tools/full_check.sh [--hizli] [--kip=dagitim|go-esleri|v3|yaris|uzun|zorlama|on])" >&2
		exit 2
		;;
	esac
done

failed=0
failures=()
durations=()
step_names=()
step_seconds=()
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
	if [ -n "$step_name" ]; then
		durations+=("$(printf '%4d sn  %s' $((SECONDS - step_start)) "$step_name")")
		step_names+=("$step_name")
		step_seconds+=($((SECONDS - step_start)))
	fi
	step_name=""
}
# json_text metni JSON dizgesi olarak yazar (ters bölü, tırnak, satır sonu ve sekme kaçırılır).
json_text() {
	local s=${1//\\/\\\\}
	s=${s//\"/\\\"}
	s=${s//$'\n'/\\n}
	s=${s//$'\t'/\\t}
	printf '"%s"' "$s"
}
# write_metrics kayıt aracının (tools/record) koşu klasörüne kipleri, adım sürelerini, kalan
# adımları ve sonucu metrics.json olarak yazar; TR_RECORD_DIR yoksa bir şey yapmaz.
write_metrics() {
	[ -n "${TR_RECORD_DIR:-}" ] || return 0
	local i sep=""
	{
		printf '{"modes":%s,"passed":%s,"seconds":%d,"steps":[' "$(json_text "$modes")" "$([ $failed -eq 0 ] && echo true || echo false)" "$SECONDS"
		for i in "${!step_names[@]}"; do
			printf '%s{"name":%s,"seconds":%d}' "$sep" "$(json_text "${step_names[$i]}")" "${step_seconds[$i]}"
			sep=","
		done
		printf '],"failures":['
		sep=""
		for i in "${failures[@]}"; do
			printf '%s%s' "$sep" "$(json_text "$i")"
			sep=","
		done
		printf ']}\n'
	} >"$TR_RECORD_DIR/metrics.json"
}
# record_failure kalan adımı özete ve sonuca kaydeder (satır yazmaz): "!! KALDI" satırı zaten
# yazılmışsa (paralel adımın alt kabuğu) yalnızca bu çağrılır.
record_failure() {
	# Otomatik sınamada kalan adım özet sayfasında da görünsün.
	if [ "${GITHUB_ACTIONS:-}" = "true" ]; then echo "::error title=Tam denetim::$1"; fi
	failures+=("$1")
	failed=1
}

# fail kalan adımı yazar ve kaydeder.
fail() {
	echo "!! KALDI: $1"
	record_failure "$1"
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

# Çözümleme araçları sabit sürümle `go run paket@sürüm` ile koşar: go.mod'a bağımlılık olarak
# girmezler, indirilen modül sağlama veri tabanıyla (sum.golang.org) doğrulanır, ilk koşudan
# sonra modül önbelleğinden gelir. Sürüm yalnızca burada yükseltilir; yeni sürümün bulguları
# aynı commit'te düzeltilir.
STATICCHECK=honnef.co/go/tools/cmd/staticcheck@v0.8.2
DEADCODE=golang.org/x/tools/cmd/deadcode@v0.52.0

# quick_static_checks tam denetimin ve ön denetimin ortak durağan adımlarıdır.
quick_static_checks() {
	step "Üretilmiş dosyalar güncel mi"
	for tool in tools/gen/*/; do
		# -check bayrağı üreticinin kendisinde ya da ortak genmain.RunMain'dedir.
		if grep -q -e '"check"' -e 'genmain\.RunMain' "$tool"*.go 2>/dev/null; then
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

	# staticcheck'in varsayılan denetimleri (yapılandırma dosyası yok). Bilerek yazılmış bir
	# kalıp (ör. testin ürettiği panik, yalnızca assembly'nin okuduğu değişken) satırının
	# üstünde gerekçeli `//lint:ignore DENETİM gerekçe` ile işaretlenir.
	step "Durağan çözümleme (staticcheck) ve ölü kod (deadcode)"
	go run "$STATICCHECK" ./... || fail "staticcheck"
	# Hiçbir programdan ve testten ulaşılamayan fonksiyonlar silinir.
	local dead
	dead=$(go run "$DEADCODE" -test ./...) || fail "deadcode çalışmadı"
	[ -z "$dead" ] || fail "hiçbir yerden çağrılmayan fonksiyonlar (silin): $dead"
}

static_checks() {
	quick_static_checks

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
	local okfile
	okfile=$(mktemp)
	go test "${p_flag[@]}" "$@" "${test_paketleri[@]}" 2>&1 | tee "$okfile" | grep -v '^ok\|no test files'
	[ "${PIPESTATUS[0]}" -eq 0 ] || fail "go test ($name)"
	slowest_packages "$okfile"
	rm -f "$okfile"
}

# slowest_packages go test çıktısındaki "ok paket süre" satırlarından en yavaş beş paketi yazar
# (önbellekten gelenler sayılmaz): yarış kipinin ve otomatik sınamanın paket başına zaman sınırına
# (10 dk) yaklaşan paket erken görünsün.
slowest_packages() {
	local list
	list=$(awk '$1 == "ok" && $3 ~ /^[0-9.]+s$/ { sub(/s$/, "", $3); printf "%8.1f sn  %s\n", $3, $2 }' "$1" | sort -rn | head -5)
	[ -z "$list" ] || printf 'En yavaş paketler:\n%s\n' "$list"
}

# changed_packages ana dala göre (birleşme tabanından, çalışma ağacındaki ve izlenmeyen dosyalar
# dahil) değişen Go paketlerini ./yol olarak verir.
changed_packages() {
	local base
	base=$(git merge-base origin/main HEAD 2>/dev/null || git merge-base main HEAD 2>/dev/null) || return 0
	{ git diff --name-only "$base"; git ls-files -o --exclude-standard; } | grep '\.go$' | grep -v '^\.claude/' |
		xargs -r -n1 dirname | sort -u | while IFS= read -r d; do
		if ls "$d"/*.go >/dev/null 2>&1; then printf './%s\n' "$d"; fi
	done
}

# gpu_race GPU paketi değiştiyse ve bir C derleyicisi varsa (yarış dedektörü cgo ister) GPU
# paketini yerelde de yarış dedektörüyle koşar: otomatik sınamanın yarış işi PTX benzeticisini
# 10 dk sınırla koşar, yeni bir sınama bu sınırı yalnızca orada aşmasın. Süre sınırın yarısını
# geçerse uyarır. Derleyici yoksa adım atlanır ve bu yazılır.
gpu_race() {
	# Otomatik sınamada yarış işi ayrıca koşar.
	[ "${GITHUB_ACTIONS:-}" != "true" ] || return 0
	changed_packages | grep -qx './internal/compute/gpu' || return 0
	step "GPU paketi yarış dedektörüyle (değişti; go test -race -short)"
	if ! command -v "$(go env CC)" >/dev/null 2>&1; then
		echo "not: C derleyicisi ($(go env CC)) yok, adım atlandı; otomatik sınamanın yarış işi koşar"
		return 0
	fi
	local start=$SECONDS
	CGO_ENABLED=1 go test -race -short -timeout 10m ./internal/compute/gpu 2>&1 | grep -v '^ok\|no test files'
	[ "${PIPESTATUS[0]}" -eq 0 ] || fail "go test -race ./internal/compute/gpu"
	if [ $((SECONDS - start)) -gt 300 ]; then
		echo "uyarı: GPU paketi yarış dedektörüyle $((SECONDS - start)) sn sürdü; otomatik sınamanın sınırı 10 dk (büyük boyları !testing.Short() arkasına alın)"
	fi
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
		# Her örnek iki yazımıyla koşar: examples/tr (Türkçe) ve examples/en (İngilizce, kodlu saklanır).
		for f in examples/tr/*.tr examples/en/*.tr; do
			name=${f#examples/}
			case " ${ATLANACAK_ORNEKLER:-} " in *" $name "* | *" $(basename "$f") "*)
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
		while IFS= read -r line; do record_failure "$line"; done <"$workdir.failures"
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
		[ "$mode" = v3 ] || gpu_race
		step "TR testleri ve örnekler ($mode)"
		if build_tr; then
			summarize "tr test tests" "$TR" test tests
			summarize "tr test lib" "$TR" test lib
			# Örnekler öğrenme yoludur: standart biçimde olmalı (iki yazım da; kodlu dosya kodlu biçimlenir).
			# Hata örnekleri (hatalar/, errors/) bilerek bozuktur ve biçimlenmez; yalnızca numaralı örnekler ve yardımcılar.
			summarize "tr biçimle --kontrol examples" "$TR" biçimle --kontrol examples/tr/[0-9]*.tr examples/tr/helpers examples/en/[0-9]*.tr examples/en/helpers
			# Kitaplık, testler ve araç betikleri de standart biçimde olmalı; tests/negative bilerek bozuktur.
			summarize "tr biçimle --kontrol lib tests tools" "$TR" biçimle --kontrol lib tests/*.tr tests/sub tests/tr tests/coded tools/gpu tools/bench/tr
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
	zorlama)
		step "Zorlama: iki tekrar, karışık sıra (go test -count=2 -shuffle=on)"
		go test "${p_flag[@]}" -count=2 -shuffle=on -timeout 90m ./... 2>&1 | grep -v '^ok\|no test files'
		[ "${PIPESTATUS[0]}" -eq 0 ] || fail "go test -count=2 -shuffle=on"
		step "Zorlama: sık çöp toplama (GOGC=1 go test)"
		GOGC=1 go test "${p_flag[@]}" -count=1 -timeout 120m ./... 2>&1 | grep -v '^ok\|no test files'
		[ "${PIPESTATUS[0]}" -eq 0 ] || fail "GOGC=1 go test"
		;;
	on)
		quick_static_checks
		step "Düzen kuralları (dosya adı, boy sınırı, Türkçe metin, çıplak ad, ileti kullanımı)"
		go test "${p_flag[@]}" -short ./internal/ ./internal/reporoot ./tools/gen/... 2>&1 | grep -v '^ok\|no test files'
		[ "${PIPESTATUS[0]}" -eq 0 ] || fail "düzen kuralları"
		mapfile -t changed < <(changed_packages)
		if [ ${#changed[@]} -gt 0 ]; then
			step "Değişen paketlerin kısa testleri (${#changed[@]} paket)"
			go test "${p_flag[@]}" -short "${changed[@]}" 2>&1 | grep -v '^ok\|no test files'
			[ "${PIPESTATUS[0]}" -eq 0 ] || fail "değişen paketlerin testleri"
		fi
		# Taban changed_packages'takiyle aynıdır: ana dalda henüz gönderilmemiş commit'ler de
		# sayılsın diye önce origin/main.
		dups_base=main
		if git rev-parse --verify -q origin/main >/dev/null; then dups_base=origin/main; fi
		step "Tekrar adayları (codegraph dups --changed $dups_base)"
		go run ./tools/codegraph dups --changed "$dups_base" 2>&1 | head -40
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
write_metrics
if [ "$modes" = on ]; then
	# Ön denetim tam denetimin yerini tutmaz; sonuç satırı push koşulundaki satırla karışmasın.
	if [ $failed -eq 0 ]; then
		echo "✅ Ön denetim geçti (tam denetim değildir)"
	else
		echo "❌ Ön denetimde kalan var:"
		printf '  - %s\n' "${failures[@]}"
	fi
	exit $failed
fi
if [ $failed -eq 0 ]; then
	echo "✅ Tam denetim geçti"
else
	echo "❌ Tam denetimde kalan var:"
	printf '  - %s\n' "${failures[@]}"
fi
exit $failed
