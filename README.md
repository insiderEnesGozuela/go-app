# go-app

Bu repo, sıfırdan Go öğrenirken yazdığım bir **dijital cüzdan / ödeme sistemi** projesi. Amacım sadece "çalışan bir kod" değil; finansal sistemlerin nasıl güvenli, tutarlı ve ölçeklenebilir yazıldığını anlamak.

- **Ay 1 (devam ediyor):** Temel altyapı — config, logger, graceful shutdown, PostgreSQL, migration
- **Ay 2:** Domain logic ve concurrency (worker pool, thread-safe balance)
- **Ay 3:** HTTP API, JWT auth, middleware'ler
- **Ay 4:** Docker, Prometheus, Grafana
- **Ay 5:** Event sourcing, Redis cache, scheduled jobs

Hafta 1 notlarım burada: [docs/hafta-01-notlar.md](docs/hafta-01-notlar.md)

## Çalıştırmak için

```bash
cp .env.example .env
# .env içindeki DB_PASSWORD'ü düzenle
go run ./cmd/server
```

Sağlık kontrolü:
```bash
curl http://localhost:8080/health
```

## Testler

```bash
go test ./...
```

## Stack

- **Go 1.26**
- **zerolog** — structured logging
- **cleanenv** — config yönetimi
- **pgx/v5** — PostgreSQL (yakında)
- **golang-migrate** — migration (yakında)