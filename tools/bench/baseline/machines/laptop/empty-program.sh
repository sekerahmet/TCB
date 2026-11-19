# Boş programın duvar saati (S14), dizüstü. Satır: docs/measurements/tensor-and-runtime.md, "Dizüstü i7-13700H".
# Taraflar: yalnız TR (emsal yok), işlemcide; model yok. 1 ısınma + 5 koşu, ortanca ve aralık (O-1).
#   bash tools/bench/baseline/machines/laptop/empty-program.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
olcu_makine empty-program
olcu_kos empty-program S14
