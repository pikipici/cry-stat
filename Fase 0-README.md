# Crypto Station — Fase 0 scaffold (plan v0.5 bab 11.6)

Binary jalan long-poll (dev) / cek-DB (tanpa token). Webhook prod nanti.

## Env (jangan commit isi asli)

```
DATABASE_URL=postgres://cs:cs_dev_local@localhost:5432/cryptostation?sslmode=disable
BOT_TOKEN=(kosong = mode cek DB; isi = mode bot long-poll)
```

## Perintah di cbt-vps

```
cd /home/ubuntu/crypto-station
export PATH=$PATH:/usr/local/go/bin
export DATABASE_URL=postgres://cs:cs_dev_local@localhost:5432/cryptostation?sslmode=disable

# migrasi (goose)
/usr/local/go/bin/goose -dir ./migrations postgres "$DATABASE_URL" up

# build + smoke cek DB (tanpa token → exit 0 kalau settings kebaca)
go build -o /tmp/cs-bot ./cmd/bot && BOT_TOKEN= /tmp/cs-bot
```
