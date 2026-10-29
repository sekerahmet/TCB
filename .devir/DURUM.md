# DURUM — docs/ sınıflı yazımın kimlik yazımına çevrilmesi (Faz 0, adım 3)

- Başlama: 2026-10-29 13:17 UTC
- Bitiş: —
- Başlangıç ölçüsü: `git grep -cE 'TR_(fn|mem|arg|type|kw)_' -- 'docs/*.md'` → 26 belge, 1 318 satır (main `23e7be2`).

## Biten belgeler

- `docs/guide/` (12 belge, 4 commit): bütün sınıflı yazım kimlik yazımına çevrildi; kalan yok.
  Üç ad eski bir çevirinin yanlışıydı, düzeltildi (yalnız yazım değil, anlam):
  - `05_hata_ayiklayici.md:12` `f5` tuşu hata ayıklayıcı komutudur: `TR_kw_continue` → `TR_dbg_continue` (komşu satırlar da hata ayıklayıcı komutu).
  - `10_hazir_katmanlarla_gpt.md:35` KV önbelleğinin kırpma metodu: `ö.TR_mem_strip(n)` (metin metodu) → `ö.TR_truncate(n)` (`lib/katman.tr:461`).
  - `11_hizi_izlemek.md:25` ilerleme çubuğunun ölçü anahtarı metindir: `{TR_mem_loss: kayıp}` → `{"loss": kayıp}` (örneklerdeki gibi).

## Kalanlar

`docs/` kökündeki belgeler, `docs/spec/`, `docs/archive/`, `docs/codes.md`, `docs/code_identities.md`.

## Kararsız kaldığım satırlar

(henüz yok)

## Sorunlar ve sorular

(henüz yok)
