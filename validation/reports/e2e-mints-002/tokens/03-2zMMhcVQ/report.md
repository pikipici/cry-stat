# Riset Solana — 2zMMhcVQEXDtdE6vsFS7S7D5oUodfJHE8vd1gnBouauv

- **Mode:** LIVE API OBSERVATION (no execution)
- **Terakhir diamati:** 2026-11-09T21:15:15Z
- **Status riset:** `DATA_TIDAK_CUKUP` — **BUKAN sinyal beli**

## Data pasar (provider-reported, bukan executable quotes)

- Jumlah pool yang cocok: 30
- Pool referensi: `FAqh648xeeaTqL7du49sztp9nfj5PjRQrfvaMccyd9cz` / DEX `orca`
- Harga USD yang dilaporkan: 0.006328
- Likuiditas USD dilaporkan: 2880401.17
- Volume 1 jam dilaporkan: 12895.39
- Volume 24 jam dilaporkan: 351050.63

## Informasi mint dan akun token (Solana RPC)

- Program: SPL Token
- Mint authority: `NOT SET / null (not a full safety guarantee)`
- Freeze authority: `NOT SET / null (not a full safety guarantee)`
- Supply mentah (raw integer): 76722779350791459
- Top ≤20 **token accounts** sebagai % supply: 56.152698
- **Perhatian:** token account ≠ wallet pemilik ≠ pemilik manfaat. Pool vault dapat muncul di daftar teratas.

## Temuan dan kekurangan bukti

- Tidak ada temuan otomatis khusus; ini **bukan** klaim aman.
- Provider `dex_pairs`: `ok` (2026-11-09T21:15:08Z); sumber https://api.dexscreener.com/token-pairs/v1/solana/2zMMhcVQEXDtdE6vsFS7S7D5oUodfJHE8vd1gnBouauv
- Provider `rpc_mint`: `ok` (2026-11-09T21:15:09Z); sumber Solana RPC getAccountInfo
- Provider `rpc_supply_and_largest`: `ok` (2026-11-09T21:15:15Z); sumber Solana RPC getTokenSupply + getTokenLargestAccounts
- Provider `rpc_owner_mapping`: `ok` (2026-11-09T21:15:15Z); sumber Solana RPC getMultipleAccounts
- Provider `normalize_token_accounts`: `ok` (2026-11-09T21:15:15Z); sumber Derived from Solana RPC, not independent provider

## Batas validasi

- No sell quote, transfer simulation, or end-to-end exit verification
- No verified social/X event feed or whale/insider identity clustering
- Third-party market numbers are estimates, NOT execution prices
- This is not an automated trading or rug-pull detection system

## Kesimpulan

**DATA_TIDAK_CUKUP untuk keputusan trading.** Pemeriksaan exit, keamanan lengkap, narasi, dan hipotesis strategi tetap harus dilakukan secara terpisah.
