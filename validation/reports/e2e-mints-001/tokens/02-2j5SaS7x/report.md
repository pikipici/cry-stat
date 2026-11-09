# Riset Solana — 2j5SaS7xy776qCBpyPQbZjyQSAtKiFgrwjfErthnW2ZM

- **Mode:** LIVE API OBSERVATION (no execution)
- **Terakhir diamati:** 2026-11-09T21:14:46Z
- **Status riset:** `TIDAK_BISA_DINILAI` — **BUKAN sinyal beli**

## Data pasar (provider-reported, bukan executable quotes)

- Jumlah pool yang cocok: 24
- Pool referensi: `Fe7bpDYCs74GqiCUF14HwgUMToPKV1d9mDYVX5BTohFr` / DEX `pumpswap`
- Harga USD yang dilaporkan: 0.005586
- Likuiditas USD dilaporkan: 338796.5
- Volume 1 jam dilaporkan: 27957.24
- Volume 24 jam dilaporkan: 1351422.42

## Informasi mint dan akun token (Solana RPC)

- Program: UNKNOWN
- Mint authority: `UNKNOWN`
- Freeze authority: `UNKNOWN`
- Supply mentah (raw integer): 946217158064495
- Top ≤20 **token accounts** sebagai % supply: 28.389083
- **Perhatian:** token account ≠ wallet pemilik ≠ pemilik manfaat. Pool vault dapat muncul di daftar teratas.

## Temuan dan kekurangan bukti

- Tidak ada temuan otomatis khusus; ini **bukan** klaim aman.
- Provider `dex_pairs`: `ok` (2026-11-09T21:14:43Z); sumber https://api.dexscreener.com/token-pairs/v1/solana/2j5SaS7xy776qCBpyPQbZjyQSAtKiFgrwjfErthnW2ZM
- Provider `rpc_mint`: `unavailable` (2026-11-09T21:14:43Z); sumber Solana RPC getAccountInfo
- Provider `rpc_supply_and_largest`: `ok` (2026-11-09T21:14:45Z); sumber Solana RPC getTokenSupply + getTokenLargestAccounts
- Provider `rpc_owner_mapping`: `ok` (2026-11-09T21:14:46Z); sumber Solana RPC getMultipleAccounts
- Provider `normalize_token_accounts`: `ok` (2026-11-09T21:14:46Z); sumber Derived from Solana RPC, not independent provider

## Batas validasi

- No sell quote, transfer simulation, or end-to-end exit verification
- No verified social/X event feed or whale/insider identity clustering
- Third-party market numbers are estimates, NOT execution prices
- This is not an automated trading or rug-pull detection system

## Kesimpulan

**DATA_TIDAK_CUKUP untuk keputusan trading.** Pemeriksaan exit, keamanan lengkap, narasi, dan hipotesis strategi tetap harus dilakukan secara terpisah.
