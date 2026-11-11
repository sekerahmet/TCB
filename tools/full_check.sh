#!/usr/bin/env bash
# Tam denetim: bir işin "bitti" sayılması için gereken bütün sınamalar, tek komutla. Otomatik
# sınama (.github/workflows/ci.yml) da bu betiği çağırır; "bitti"nin tek tanımı burasıdır.
#
# Kullanım (depo kökünden): bash tools/full_check.sh [--hizli] [--kip=KİP] [--ilk-kalanda-dur]
#   (bayraksız)       dagitim ve go-esleri kipleri art arda (yerelde birleştirmeden önce)
#   --kip=dagitim     kullanıcıya giden yapılandırma: düz `go build` (GOAMD64=v1), AVX2
#                     çekirdekleri çalışma anında seçilir. Durağan denetimler, ARM derlemesi,
#                     Go testleri, trlang testleri (tests/, lib/, examples/) ve örneklerin hepsi.
#   --kip=go-esleri   AVX2 çekirdekleri kapalı (TR_CPU_AVX2=0): assembly'siz Go eşleri. Derleme,
#                     assembly'nin bulunduğu paketin ve onun çekirdeklerini en yoğun kullanan
#                     paketin Go testleri (esleri_paketleri) ve dil tanımı testleri (trlang test tests).
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
#   --taban=<commit>  ön denetimin "değişen"i bu commit'e göre bulur (yan dalda varsayılan yerel main,
#                     ana dalda origin/main): bir maddenin sonunda yalnız o maddenin değiştirdikleri
#   --ilk-kalanda-dur ilk kalan adımda denetimi bitirir: izlenen adımlarda (Go testleri, değişen
#                     paketlerin ve dosyaların testleri, örnekler) koşan go test ya da örnek kalan
#                     satırında durdurulur, öbür adımlar (düzen kuralları, depo taramaları, yarış,
#                     zorlama) bitince durulur; sonraki adımlar koşmaz, sonuç satırı ❌ olur. Yalnız betiğin kendi alt süreçleri
#                     durur, betiği çağıran kabuk durmaz. Kipler bu bayrakla birleşir.
#   --kip=commit      commit öncesi denetim (~35 sn; yan oturumların ve TCB'nin tek commit öncesi komutu,
#                     CLAUDE_YAN_DAL_KURALLARI.md "Sınama"): gofmt, .tr biçimi (format_checks), tekrar
#                     adayları, dil tanımı atıfları, düzen kuralları (./internal) ve G4'ü sık düşüren hafif testler (commit_tests). Bu makinede koşan
#                     tek kip; öbürleri Colab'da (ağır koşu kancası).
#   --kip=on          ön denetim (Colab'da 441–529 sn, yerelde oturumlar yarışırken ortanca ~4 dk): üretilmiş dosyalar, gofmt, vet, go.mod, staticcheck,
#                     ölü kod, düzen kuralları (dosya adı, satır sınırı, Türkçe metin, çıplak ad,
#                     ileti kullanımı), öbür paketlerdeki depo taramaları, .tr dosyalarının biçimi, dil
#                     tanımı testleri (trlang test tests), değişen
#                     paketlerin kısa testleri, değişen dosyaları koruyan testler (codegraph changed
#                     --go-tests; veri dosyası değişse de), yeni tekrar adayları (codegraph dups --changed;
#                     yeni aday denetimi düşürür, tam denetimde de) ve dil tanımı atıfları (codegraph spec-refs).
#                     Yan oturumlar devir notundan, ana oturum GPU yan dalına göndermeden önce
#                     koşar; tam denetimin yerini tutmaz.
#
# Durağan denetimler: üretilmiş dosyalar güncel mi (her üreticinin -check bayrağı olmalı),
# gofmt, go vet, go.mod/go.sum (go mod verify, go mod tidy -diff), staticcheck, ölü kod
# (deadcode), ARM (arm64) derlemesi ve
# vet'i, GPU'suz (-tags gpusuz) Linux program dosyasının statik olması, VS Code eklentisinin
# JavaScript dosyaları ve package.json. Go testleri -short ile koşar (uzun kipi hariç): dil
# tanımı testleri (tests/) Go testinin içinde yinelenmez, burada `trlang test tests` ile koşar.
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
base_override=""
stop_first=""
mode_flags=0
for arg in "$@"; do
	case "$arg" in --hizli | --kip=*) mode_flags=$((mode_flags + 1)) ;; esac
	case "$arg" in
	--taban=*) base_override=${arg#--taban=} ;;
	--ilk-kalanda-dur) stop_first=1 ;;
	--hizli | --kip=dagitim) modes=dagitim ;;
	--kip=go-esleri | --kip=v1) modes=go-esleri ;;
	--kip=v3) modes=v3 ;;
	--kip=yaris) modes=yaris ;;
	--kip=uzun) modes=uzun ;;
	--kip=zorlama) modes=zorlama ;;
	--kip=on) modes=on ;;
	--kip=commit) modes=commit ;;
	*)
		echo "bilinmeyen argüman: $arg (kullanım: bash tools/full_check.sh [--hizli] [--kip=dagitim|go-esleri|v3|yaris|uzun|zorlama|on|commit] [--taban=<commit>] [--ilk-kalanda-dur])" >&2
		exit 2
		;;
	esac
done
# Son kip bayrağı kazanır; --kip=commit başka bir kiple gelirse bu makinede ağır bir kip koşardı (ağır koşu
# kancası yalnız --kip=commit'i geçirir; DENETİM 383 Ş3).
if [ "$modes" = commit ] || printf '%s\n' "$@" | grep -qx -- '--kip=commit'; then
	if [ "$mode_flags" -ne 1 ]; then
		echo "--kip=commit başka bir kip bayrağıyla birlikte verilemez (--hizli, --kip=...)" >&2
		exit 2
	fi
fi
# FULL_CHECK_ARGS_ONLY=1: yalnız argümanlar sınanır (TestCommitModeTakesNoOtherModeAndSharesGofmt); koruma bozulsa
# da test bu makinede bir kipi başlatmaz (DENETİM 383 K-N3).
[ "${FULL_CHECK_ARGS_ONLY:-}" != 1 ] || exit 0
# --taban tam kimliğe çözülür: birden çok commit'e uyan kısa kimlik (da0457c5, 2026-11-03) ya da
# olmayan commit sessizce "ortak ata yok" diye okunup değişen dosyaların testleri ve dups atlanıyordu.
if [ -n "$base_override" ]; then
	resolved=$(git rev-parse --verify -q "$base_override^{commit}" 2>/dev/null) || {
		echo "--taban=$base_override tek bir commit'e çözülmüyor (belirsiz kısa kimlik ya da olmayan commit): daha uzun kimlik verin" >&2
		exit 2
	}
	base_override=$resolved
fi

# repo_scans düzen kurallarının paketleri dışında depoyu tarayan kural testleridir (ön denetim hepsini koşar).
repo_scans=(
	TestBuildTagsNeverSelectCPUFeatures
	TestCloseCallsEveryPackageCloser
	TestCodeListAndTRFileWalkersAreInTestsForAndChangedGoTests
	TestDocCodeBlocksShow
	TestDottedMemberSpellingsGolden
	TestEveryCodedEnvironmentVariableInAScriptIsAnEdge
	TestEveryErrorTypeIsLocalized
	TestEveryMemberSpellingInRepoTRFilesResolves
	TestEveryMultiNameValueSpecInTheRepositoryHasItsUses
	TestEveryRepositoryScanIsRecognized
	TestEveryTrackedDataFileHasATestOrAReason
	TestEveryTrackedFileIsANode
	TestExamplesCoverLibraryNames
	TestExamplesCoverNames
	TestExprWalkersMatchReferenceOnRepositoryPrograms
	TestFileSelectionIsConsistentInEveryConfiguration
	TestGoCodeDoesNotEditMessageText
	TestHeaderRulesLiveOnlyInSourceAndLexer
	TestHintMessagesBuiltOnlyHere
	TestJSONFieldsHaveExplicitNames
	TestLibraryModulesDoNotReadTheCommandLine
	TestMessageArgsAreNotMessageFragments
	TestRepoTRFilesPassCheck
	TestRepoTRFilesShow
	TestSizesMatchTheLayoutTest
	TestStatefulImplementersAreAsserted
	TestUserPathsResolvedByLanguageRule
)
# Commit öncesi denetimin (--kip=commit) testleri: düzen kurallarının paketleri (./internal, ./internal/reporoot;
# reporoot'un TestTestsListFilesThroughReporoot'u 2026-11-10'da G4'ü düşürdü) bütün olarak, öbür
# paketlerden yalnız G4 tam denetimini sık düşüren hafifler commit_tests ile. 2026-11-04–10'un 189 G4 koşusunda
# ./internal'ın testleri (TestTestFileMatchesSource 5 kez, TestTRCodeIsTyped, TestClassedIdentitiesAreLive,
# TestNoOldCommandName), TestReviewedSectionsWriteIdentities, kod listesinin iki testi ve TestRepoTRFilesPassCheck
# kaldı (DENETİM 383). Codegraph ve e2e testleri burada değil: codegraph paketi dizüstünde yarışta 30–154 sn sürdü,
# önlediği kalandan pahalı [ölçüldü: TCB, 2026-11-10]; G4'e kalırlar. Adların her biri commit_test_pkgs'in bir
# paketinde tanımlı olmalı (TestCommitTestsExist): taşınan ad "no tests to run" ile sessizce geçerdi.
commit_tests=(
	TestCodeSlotsResolveToIdentities
	TestEveryCodeHasDescription
	TestEveryRuleHasATest
	TestRepoTRFilesPassCheck
	TestReviewedSectionsWriteIdentities
)
commit_test_pkgs=(
	./cmd/trlang
	./internal/devtools/testrunner
	./internal/lang/codes
)
repo_scan_pkgs=(
	./cmd/trlang
	./cmd/trlang/e2e
	./internal/compute
	./internal/lang/hint
	./internal/lang/interp
	./internal/lang/messages
	./internal/lang/source
	./internal/stdlib
	./internal/stdlib/checkpoint
	./tools/codegraph
)

failed=0
stopped_early=0
# seen_fail izleyicinin gördüğü kalan satırları tutar; izlenen adımın sonucu ona da bakar, yalnız
# durdurulan sürecin çıkış koduna değil (kalan satırını yazıp 0 dönen süreç ✅ veriyordu; DENETİM istek 51).
seen_fail=$(mktemp)
go_okfile=""
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
		printf '{"modes":%s,"passed":%s,"stopped_early":%s,"seconds":%d,"steps":[' "$(json_text "$modes")" "$([ $failed -eq 0 ] && echo true || echo false)" "$([ $stopped_early -eq 1 ] && echo true || echo false)" "$SECONDS"
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
	if [ -n "$stop_first" ]; then
		stopped_early=1
		echo "!! İlk kalanda durdu (--ilk-kalanda-dur): sonraki adımlar koşmadı"
		finish
	fi
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

# children bir sürecin doğrudan alt süreçlerini yazar. Git Bash'in ps'i -o bilmez; sütunları
# PID PPID PGID WINPID'dir.
children() {
	if [ -n "$ext" ]; then
		ps -e | awk -v p="$1" 'NR > 1 && $2 == p { print $1 }'
	else
		ps -o pid= --ppid "$1"
	fi
}
# stop_tree bir süreci alt süreçleriyle durdurur. Windows'ta go.exe'nin açtığı test ikilileri
# Git Bash'in süreç ağacında görünmez: Windows'un ağacı taskkill /T ile durdurulur.
stop_tree() {
	local c w
	for c in $(children "$1"); do stop_tree "$c"; done
	if [ -n "$ext" ]; then
		w=$(ps -e | awk -v p="$1" 'NR > 1 && $1 == p { print $4 }')
		[ -z "$w" ] || taskkill //T //F //PID "$w" >/dev/null 2>&1
	fi
	kill "$1" 2>/dev/null
}
# stop_siblings ilk kalanda koşan adımı durdurur: betiğin ($$) çağıran dışındaki bütün alt
# süreçleri. Borudaki izleyici kendisi de betiğin alt sürecidir ($BASHPID); kendini durdurmaz.
# Betiğin kendisi ve onu çağıran kabuk bu ağacın dışındadır.
stop_siblings() {
	local c
	for c in $(children $$); do
		[ "$c" = "$BASHPID" ] || stop_tree "$c"
	done
}
# watch_failures bir adımın çıktısını olduğu gibi geçirir; --ilk-kalanda-dur'da ilk kalan
# satırında adımı durdurur. Kalan satır go test'in paket özetidir ("FAIL<sekme>paket"): Go kalan
# paketin çıktısını bir bütün olarak yazar, özet testin adından ve ayrıntısından sonra gelir;
# "--- FAIL"de durulsaydı ayrıntı kaybolurdu. Borunun son halkasıdır: önündeki grep satır satır
# yazmalı (--line-buffered), yoksa satırlar tamponda bekler ve durdurulunca kaybolur. Adımın
# kendi çıkış kodu kalanı kaydeder.
watch_failures() {
	if [ -z "$stop_first" ]; then
		cat
		return
	fi
	local line
	while IFS= read -r line; do
		printf '%s\n' "$line"
		case "$line" in
		"FAIL"$'\t'* | "!! KALDI: "*)
			printf '%s\n' "$line" >>"$seen_fail"
			stop_siblings
			return
			;;
		esac
	done
}
# watched_ok izlenen adımın sonucu: süreç 0 ile bitti ve izleyici kalan satırı görmedi.
watched_ok() {
	[ "$1" -eq 0 ] && [ ! -s "$seen_fail" ]
}
TR="$PWD/bin/trlang${ext}"
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

# gofmt_check depodaki (izlenen ve henüz eklenmemiş) bütün Go dosyalarını gofmt'tan geçirir; .claude/ altındaki
# çalışma ağaçları başka depolardır, silinmiş ama henüz commitlenmemiş dosyalar atlanır. Dosyalar xargs ile
# geçer: tek komut satırına yazılan 1 346 dosya Windows'un sınırını aşıp gofmt'u hiç koşturmuyordu (DENETİM 383 Ş1).
# Tam denetim, ön denetim ve commit öncesi denetim bu fonksiyonu çağırır.
gofmt_check() {
	local unformatted
	if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
		unformatted=$(git ls-files -co --exclude-standard -- '*.go' ':!:.claude/**' |
			while IFS= read -r f; do if [ -f "$f" ]; then printf '%s\n' "$f"; fi; done | xargs -r -d '\n' gofmt -l)
	else
		unformatted=$(gofmt -l ./cmd ./internal ./lib ./tools)
	fi
	[ $? -eq 0 ] || fail "gofmt çalışmadı"
	[ -z "$unformatted" ] || fail "gofmt ile biçimlenmesi gereken dosyalar: $unformatted"
}

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
	gofmt_check
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

# compare_base ön denetimin "değişen"i neye göre bulduğudur. Yan dalda (oturumun worktree'si) yerel
# main: dal ondan açılır, ana dalın gönderilmemiş commit'leri yan dalın değişikliği değildir (öyle
# sayılınca her oturumun ön denetimi 41 paketi koşuyordu, 416 sn). Ana dalda ve yerel main'i olmayan
# klonda (otomatik sınama) origin/main: gönderilmemiş commit'ler de sayılır. --taban=<commit> tabanı
# verir: bir işin maddesi bitince yalnız o maddenin değiştirdikleri (maddenin başındaki commit).
compare_base() {
	local branch
	if [ -n "$base_override" ]; then
		echo "$base_override"
		return
	fi
	branch=$(git symbolic-ref -q --short HEAD 2>/dev/null)
	if [ -n "$branch" ] && [ "$branch" != main ] && git rev-parse --verify -q main >/dev/null; then
		echo main
	elif git rev-parse --verify -q origin/main >/dev/null; then
		echo origin/main
	else
		echo main
	fi
}

codegraph_built=""
# dups_check ana dala göre eklenen ya da değişen satırlara dokunan tekrar adaylarını arar; yeni bir
# aday denetimi düşürür (CLAUDE.md, Sadelik 10): aynı işte giderilir ya da bilerek kalan kopya
# tools/codegraph/dups_exclude.go'ya gerekçesiyle yazılır. Taban compare_base'dir; ortak ata yoksa
# (sığ klon) adım atlanır.
dups_check() {
	local base out n
	codegraph_built=""
	base=$(compare_base)
	step "Tekrar adayları (codegraph dups --changed $base)"
	if ! git merge-base "$base" HEAD >/dev/null 2>&1; then
		echo "$base ile ortak ata yok (sığ klon): atlandı"
		return
	fi
	out=$(go run ./tools/edithook codegraph dups --changed "$base" 2>&1) || {
		printf '%s\n' "$out" | head -40
		fail "codegraph dups çalışmadı"
		return
	}
	codegraph_built=1
	printf '%s\n' "$out" | head -40
	n=$(printf '%s\n' "$out" | sed -n 's/^\([0-9][0-9]*\) duplicate candidate.*/\1/p' | head -1)
	if [ -z "$n" ]; then
		fail "codegraph dups çıktısında aday sayısı yok"
	elif [ "$n" -gt 0 ]; then
		fail "$n yeni tekrar adayı: aynı işte giderin ya da tools/codegraph/dups_exclude.go'ya gerekçesiyle yazın"
	fi
}

# spec_refs_check dil tanımı (docs/spec) madde atıflarını denetler: olmayan maddeye atıf ve
# madde numarasından sonraki ekin ses uyumsuzluğu denetimi düşürür. dups_check bu koşuda
# codegraph'ı derlediyse (.codegraph/last-codegraph) o ikiliyi doğrudan koşar; ortak ata yoksa
# ya da ikili bulunamazsa edithook üzerinden go run'a düşer.
spec_refs_check() {
	local bin="" out
	step "Dil tanımı atıfları (codegraph spec-refs)"
	if [ "${codegraph_built:-}" = 1 ] && [ -f .codegraph/last-codegraph ]; then
		IFS= read -r bin <.codegraph/last-codegraph || true
		bin=${bin%$'\r'}
	fi
	if [ -n "$bin" ] && [ -f "$bin" ]; then
		out=$("$bin" spec-refs 2>&1) || {
			printf '%s\n' "$out" | head -40
			fail "dil tanımı atıfları (codegraph spec-refs)"
			return
		}
	else
		out=$(go run ./tools/edithook codegraph spec-refs 2>&1) || {
			printf '%s\n' "$out" | head -40
			fail "dil tanımı atıfları (codegraph spec-refs)"
			return
		}
	fi
	printf '%s\n' "$out" | head -40
}

static_checks() {
	quick_static_checks
	dups_check
	spec_refs_check

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
	if GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags gpusuz -o "$static_dir/trlang" ./cmd/trlang; then
		go run ./tools/checkstatic "$static_dir/trlang" || fail "gpusuz Linux program dosyası statik değil"
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
	go_okfile=$okfile
	go test "${p_flag[@]}" "$@" "${test_paketleri[@]}" 2>&1 | tee "$okfile" | grep --line-buffered -v '^ok\|no test files' | watch_failures
	watched_ok "${PIPESTATUS[0]}" || fail "go test ($name)"
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
# dahil) değişen Go paketlerini ./yol olarak verir. testdata klasörleri paket değildir (Go'nun kuralı,
# ./... de onları atlar): içlerindeki Go dosyaları başka bir modülün ya da sınavın girdisidir.
changed_packages() {
	local base
	base=$(git merge-base "$(compare_base)" HEAD 2>/dev/null) || return 0
	{ git diff --name-only "$base"; git ls-files -o --exclude-standard; } | grep '\.go$' | grep -v -e '^\.claude/' -e '\(^\|/\)testdata/' |
		xargs -r -n1 dirname | sort -u | while IFS= read -r d; do
		if ls "$d"/*.go >/dev/null 2>&1; then printf './%s\n' "$d"; fi
	done
}

# selected_tests değişen dosyaları koruyan Go testlerini kısa kipte koşar: codegraph changed --go-tests (değişen kodu
# sınayan testler, değişen testler, alt süreç testleri ve değişen her dosyayı okuyan testler). Argümanlar tamamı zaten
# koşan paketlerdir (changed_packages), atlanır. Taban changed_packages ve dups_check'tekiyle aynıdır. Neden: yalnız
# .go dosyası değişen paketleri koşan ön denetim bir veri dosyası (planning_questions.txt, tr.json) değişince onu
# koruyan testi koşmuyordu; madde 15 adım 2'de iki kez kırık codegraph sınavıyla "geçti" dedi.
selected_tests() {
	local base out errfile dir name
	base=$(compare_base)
	git merge-base "$base" HEAD >/dev/null 2>&1 || return 0
	errfile=$(mktemp)
	# Yeni bir worktree'de graf henüz yoktur (oturumun ilk ön denetimi): önce kurulur.
	if [ ! -f .codegraph/graph.json ]; then
		go run ./tools/codegraph build >/dev/null 2>"$errfile" || {
			head -20 "$errfile"
			rm -f "$errfile"
			fail "codegraph build çalışmadı"
			return
		}
	fi
	# codegraph kancanın yolundan koşar (tools/edithook/codegraph.go): ağaç derlenmezken son derlenen sürümün
	# cevabı, uyarısıyla; uyarı gösterilir, codegraph'ın öbür hata çıktısı yalnız düşünce.
	out=$(go run ./tools/edithook codegraph changed --go-tests --base "$base" HEAD 2>"$errfile") || {
		head -20 "$errfile"
		rm -f "$errfile"
		fail "codegraph changed --go-tests çalışmadı"
		return
	}
	grep -A5 -E '^warning: codegraph does not build|rebuild failed' "$errfile"
	rm -f "$errfile"
	local -A full=() seen=()
	local dirs=() names=()
	for dir in "$@"; do full[$dir]=1; done
	while read -r dir name; do
		case $name in Test* | Fuzz*) ;; *) continue ;; esac
		[ -z "${full[./$dir]:-}" ] || continue
		[ -n "${seen[$dir]:-}" ] || { seen[$dir]=1; dirs+=("./$dir"); }
		names+=("$name")
	done <<<"$out"
	[ ${#dirs[@]} -gt 0 ] || return 0
	step "Değişen dosyaları koruyan testler (${#names[@]} test, ${#dirs[@]} paket; codegraph changed --go-tests)"
	local run
	run="^($(printf '%s\n' "${names[@]}" | sort -u | paste -sd'|'))\$"
	# Windows'ta komut satırı ~32 000 karakterdir: çok büyük seçimde (bir tr.json değişikliği 428 test, 12 500
	# karakter, 67 sn ölçüldü) seçilen paketler bütün olarak koşar.
	if [ ${#run} -gt 20000 ]; then run=.; fi
	go test "${p_flag[@]}" -short -count=1 -run "$run" "${dirs[@]}" 2>&1 | grep --line-buffered -v '^ok\|no test files' | watch_failures
	watched_ok "${PIPESTATUS[0]}" || fail "değişen dosyaları koruyan testler"
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
	mkdir -p bin && go build -o "$TR" ./cmd/trlang
}

# format_checks .tr dosyalarının standart biçimde olduğunu denetler; tam denetim ve ön denetim aynı
# listeyi kullanır (yan oturumların ön denetimi biçimsiz bir test dosyasını kaçırmıştı).
format_checks() {
	# Örnekler öğrenme yoludur: standart biçimde olmalı (kodlu dosya kodlu biçimlenir).
	# Hata örnekleri (hatalar/) bilerek bozuktur ve biçimlenmez; yalnızca numaralı örnekler ve yardımcılar.
	summarize "trlang format --check examples" "$TR" format --check examples/tr/[0-9]*.tr examples/tr/helpers
	# Kitaplık, testler ve araç betikleri de standart biçimde olmalı; tests/negative bilerek bozuktur.
	summarize "trlang format --check lib tests tools" "$TR" format --check lib tests/*.tr tests/sub tests/tr tests/coded tools/gpu tools/bench/tr
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
		export TR_LANG=tr
		if output=$("$TR" test examples 2>&1); then
			echo "$output" | tail -1
		else
			echo "$output"
			echo "!! KALDI: trlang test examples"
		fi
		# Her örnek koşar (kodlu saklanır; gösterim dilinden bağımsızlığını e2e TestExamplesSameOutputInEveryDisplayLanguage sınar).
		for f in examples/tr/*.tr; do
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
		case "$line" in "!! KALDI: "*) [ -z "$stop_first" ] || { stop_siblings; break; } ;; esac
	done
	local failed_lines=()
	if [ -f "$workdir.failures" ]; then mapfile -t failed_lines <"$workdir.failures"; fi
	rm -rf "$workdir" "$workdir.failures"
	for line in "${failed_lines[@]}"; do record_failure "$line"; done
}

# finish sonucu yazar ve çıkar: adım süreleri, metrics.json, sonuç satırı. İlk kalanda durunca da
# buradan çıkılır (o zaman depodaki iz denetlenmez: adım yarıda kesildi).
finish() {
	step_end
	if [ $stopped_early -eq 0 ] && [ "$(repo_state)" != "$initial_state" ]; then
		stop_first=""
		fail "denetim depoda iz bıraktı (git status değişti): $(diff <(echo "$initial_state") <(repo_state) | head -10)"
	elif [ $stopped_early -eq 1 ] && [ "$(repo_state)" != "$initial_state" ]; then
		# Durdurulan test izini silemeyebilir; kalan sayılmaz ama görünsün, sonraki koşunun
		# başlangıç durumuna sessizce girmesin.
		echo "bilgi: durdurulan adım depoda iz bıraktı (git status değişti): $(diff <(echo "$initial_state") <(repo_state) | head -10)"
	fi
	rm -f "$seen_fail" "$go_okfile"

	echo
	echo "Adım süreleri:"
	printf '%s\n' "${durations[@]}"
	write_metrics
	if [ "$modes" = commit ]; then
		if [ $failed -eq 0 ]; then
			echo "✅ Commit öncesi denetim geçti (tam denetim değildir)"
		else
			echo "❌ Commit öncesi denetimde kalan var:"
			printf '  - %s\n' "${failures[@]}"
		fi
		exit $failed
	fi
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
			summarize "trlang test tests" "$TR" test tests
			summarize "trlang test lib" "$TR" test lib
			summarize "trlang test tools/assistant" "$TR" test tools/assistant
			format_checks
			run_examples
			[ "$mode" = v3 ] || extension_tests
		else
			fail "trlang derlemesi"
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
		if build_tr; then summarize "trlang test tests (go-esleri)" "$TR" test tests; else fail "trlang derlemesi"; fi
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
	commit)
		step "Biçim (gofmt)"
		gofmt_check
		step "TR dosyalarının biçimi"
		if build_tr; then format_checks; else fail "trlang derlenemedi"; fi
		step "Düzen kuralları (./internal, ./internal/reporoot) ve G4'ü sık düşüren hafif testler (${#commit_tests[@]} test, ${#commit_test_pkgs[@]} paket)"
		go test "${p_flag[@]}" -short -count=1 ./internal ./internal/reporoot 2>&1 | grep --line-buffered -v '^ok\|no test files' | watch_failures
		watched_ok "${PIPESTATUS[0]}" || fail "düzen kuralları"
		go test "${p_flag[@]}" -short -count=1 -run "^($(printf '%s|' "${commit_tests[@]}" | sed 's/|$//'))\$" \
			"${commit_test_pkgs[@]}" 2>&1 | grep --line-buffered -v '^ok\|no test files' | watch_failures
		watched_ok "${PIPESTATUS[0]}" || fail "G4'ü sık düşüren hafif testler"
		dups_check
		spec_refs_check
		;;
	on)
		quick_static_checks
		step "Düzen kuralları (dosya adı, boy sınırı, Türkçe metin, çıplak ad, ileti kullanımı)"
		go test "${p_flag[@]}" -short -count=1 ./internal/ ./internal/reporoot ./tools/gen/... 2>&1 | grep -v '^ok\|no test files'
		[ "${PIPESTATUS[0]}" -eq 0 ] || fail "düzen kuralları"
		step "Öbür paketlerdeki depo taramaları (ileti, okuma, öneri, kod grafiği, yorumlayıcı, komut)"
		# Depoyu tarayan kural testleri: hangi dosya değişirse değişsin koşar (değişen paketlere bağlı
		# kalınca source.TestHeaderRulesLiveOnlyInSourceAndLexer'ın kırığı ancak bütün ağaçta çıkmıştı,
		# 9ca4b3df). Düzen kurallarının paketleri yukarıda bütün olarak koşar. Liste elle; codegraph'ın
		# tarama düğümlerinden üretilmesi tools/codegraph/gaps.md'de.
		go test "${p_flag[@]}" -short -count=1 -run "^($(printf '%s|' "${repo_scans[@]}" | sed 's/|$//'))\$" \
			"${repo_scan_pkgs[@]}" 2>&1 | grep -v '^ok\|no test files'
		[ "${PIPESTATUS[0]}" -eq 0 ] || fail "depo taramaları"
		step "TR dosyalarının biçimi (trlang format --check) ve dil tanımı testleri (trlang test tests, ~2 sn)"
		# Go'nun kısa kipi dil tanımı testlerini atlıyor; ön denetim onları burada koşar.
		if build_tr; then
			format_checks
			summarize "trlang test tests" "$TR" test tests
		else fail "trlang derlenemedi"; fi
		mapfile -t changed < <(changed_packages)
		if [ ${#changed[@]} -gt 0 ]; then
			step "Değişen paketlerin kısa testleri (${#changed[@]} paket)"
			# -count=1: depo testleri dosyaları `git ls-files` ile bulur, Go'nun önbelleği onu girdi saymaz; çalışma
			# ağacında `.git` dosyadır, commit'le değişmez, commit'ten önce koşulan testin eski sonucu yeni izlenen
			# dosyayı gizliyordu (T1, DENETİM istek 106, 2026-11-04). Ön denetimin dört Go adımı da böyle koşar.
			go test "${p_flag[@]}" -short -count=1 "${changed[@]}" 2>&1 | grep --line-buffered -v '^ok\|no test files' | watch_failures
			watched_ok "${PIPESTATUS[0]}" || fail "değişen paketlerin testleri"
		fi
		selected_tests "${changed[@]}"
		dups_check
		spec_refs_check
		;;
	esac
done
finish
