# Saf hesap TR ile Go (S13/dongu, S13/fib, S13/matris_dongu), dizüstü: tools/bench/tr/<program>.tr ile
# tools/bench/go'nun aynı programı; sonuç değeri iki tarafta aynı (uyuşma), ABBA 10 koşu. Satır:
# docs/measurements/tensor-and-runtime.md, "Dizüstü i7-13700H". Taraflar: TR ve Go, ikisi işlemcide; model yok.
# Tek program için: bash compute-vs-go.sh dongu   (öntanımlı üçü, her biri ayrı kayıt)
#   bash tools/bench/baseline/machines/laptop/compute-vs-go.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
olcu_makine compute-vs-go
rc=0
for prog in ${1:-dongu fib matris_dongu}; do
  olcu_kos "compute-vs-go-$prog" S13 --key "S13/$prog" || rc=$?
done
exit $rc
