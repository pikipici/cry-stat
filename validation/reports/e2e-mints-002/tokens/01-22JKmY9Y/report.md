# Riset Solana — 22JKmY9YZfnQfb3pGYb6hctL5Jyvv3zhL3nQ8oyNuoYp

- **Mode:** LIVE API OBSERVATION (no execution)
- **Terakhir diamati:** 2026-11-09T21:15:04Z
- **Status riset:** `DATA_TIDAK_CUKUP` — **BUKAN sinyal beli**

## Data pasar (provider-reported, bukan executable quotes)

- Jumlah pool yang cocok: 7
- Pool referensi: `BFVfdYwBQxgReXaKfdSxohSP1GpXcEa1emRQuLe8w52h` / DEX `raydium`
- Harga USD yang dilaporkan: 0.0005736
- Likuiditas USD dilaporkan: 184058.55
- Volume 1 jam dilaporkan: 51010.91
- Volume 24 jam dilaporkan: 2116915.97

## Informasi mint dan akun token (Solana RPC)

- Program: SPL Token
- Mint authority: `NOT SET / null (not a full safety guarantee)`
- Freeze authority: `NOT SET / null (not a full safety guarantee)`
- Supply mentah (raw integer): 999961862925634
- Top ≤20 **token accounts** sebagai % supply: 47.165426
- **Perhatian:** token account ≠ wallet pemilik ≠ pemilik manfaat. Pool vault dapat muncul di daftar teratas.

## Temuan dan kekurangan bukti

- Tidak ada temuan otomatis khusus; ini **bukan** klaim aman.
- Provider `dex_pairs`: `ok` (2026-11-09T21:15:01Z); sumber https://api.dexscreener.com/token-pairs/v1/solana/22JKmY9YZfnQfb3pGYb6hctL5Jyvv3zhL3nQ8oyNuoYp
- Provider `rpc_mint`: `ok` (2026-11-09T21:15:02Z); sumber Solana RPC getAccountInfo
- Provider `rpc_supply_and_largest`: `ok` (2026-11-09T21:15:03Z); sumber Solana RPC getTokenSupply + getTokenLargestAccounts
- Provider `rpc_owner_mapping`: `ok` (2026-11-09T21:15:04Z); sumber Solana RPC getMultipleAccounts
- Provider `normalize_token_accounts`: `ok` (2026-11-09T21:15:04Z); sumber Derived from Solana RPC, not independent provider

## Batas validasi

- No sell quote, transfer simulation, or end-to-end exit verification
- No verified social/X event feed or whale/insider identity clustering
- Third-party market numbers are estimates, NOT execution prices
- This is not an automated trading or rug-pull detection system

## Kesimpulan

**DATA_TIDAK_CUKUP untuk keputusan trading.** Pemeriksaan exit, keamanan lengkap, narasi, dan hipotesis strategi tetap harus dilakukan secara terpisah.
