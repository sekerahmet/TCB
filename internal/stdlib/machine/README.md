# machine

`TR_fn_machine()` (dil tanımı 19.12): sürecin çalıştığı makinenin o anki görüntüsü bir `TR_type_Machine`
değeri olarak. Üyeler: `TR_mem_memory_total_bytes`, `TR_mem_memory_limit_bytes`, `TR_mem_memory_available_bytes`,
`TR_mem_usable_cpus`, `TR_mem_cpu_features`, `TR_mem_os`, `TR_mem_arch`.

## Standartlar

**Tek anlık görüntü.** Her çağrı bütün üyeleri bir kez okur (`read`); üye her okumada yeniden ölçmez.
Dilin hiçbir yerleşiği hesabını bu değere göre değiştirmez.

**Okunamayan bilgi hatadır.** Okunamayan üye `TR_none` ya da 0 olmaz, okununca H7735 verir; `TR_none`
yalnız `.TR_memory_limit_bytes`'ta "sınır yok" demektir. macOS'ta kullanılabilir bellek ve sınır bu sürümde
okunmaz (H7735, macOS cümlesiyle). Denetim: `TestMachineMembers`.

**Bellek sınırı.** Linux'ta `/proc/self/cgroup` sürecin yolunu, `/proc/self/mountinfo` cgroup dosya sisteminin
bağlandığı yeri ve kökünü verir; v1'in bellek denetleyicisi varsa (karma düzende de) `memory.limit_in_bytes`
(2^62'den büyüğü sınırsız), yoksa v2'nin `memory.max`'ı (`max` sınırsız; kök cgroup'ta dosya yoktur, sınırsız).
Windows'ta iş nesnesinin `JOB_OBJECT_LIMIT_PROCESS_MEMORY` ve `JOB_OBJECT_LIMIT_JOB_MEMORY` sınırlarının küçüğü.
Ayrıştırma `limit.go`'dadır, her işletim sisteminde testdata'yla sınanır.
Denetim: `TestMachineMemoryLimitCgroup`, `TestUnescapeMount`, `TestJobMemoryLimit`, `TestJobLimitLayout`,
`TestMemoryLimitOutsideJob`.

**Kapalı tablo.** `.TR_os` `windows`, `linux`, `macos`; `.TR_arch` `x86_64`, `arm64`; tablo dışında Go'nun
adı (`os_arch.go`). Karşılaştırmadaki yazım denetleyicide denetlenir
(`internal/lang/checker/machine_comparison.go`). Denetim: `TestMachineOSTable`, `TestMachineCPUFeaturesTable`.

## Testler

Bütün testler kaynak dosyalarının adını taşır; davranış testi yoktur.
