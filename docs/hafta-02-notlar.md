# Hafta 2 — PostgreSQL, pgx/v5, golang-migrate ve Transaction Yönetimi

> Bu hafta domain'i (Hafta 1) gerçek bir veritabanına bağladık. Repository
> interface'lerini **somutlaştırdık** (PostgreSQL implementasyonu yazdık),
> migration altyapısı kurduk, ve finansal sistemin kalbi olan **transaction +
> satır kilidi (FOR UPDATE) + optimistic locking** mekanizmalarını oturttuk.
> Her bölüm bir öncekine yaslanır; sırayla oku.

## İçindekiler

1. [Hafta 2'nin Yeri — KPI ve Mimaride](#1-hafta-2nin-yeri)
2. [`docker-compose` ile Local PostgreSQL](#2-docker-compose-ile-local-postgresql)
3. [`golang-migrate` — Şema Versiyonlama](#3-golang-migrate--şema-versiyonlama)
4. [Migration Dosyalarımız ve Şema Tasarımı](#4-migration-dosyalarımız-ve-şema-tasarımı)
5. [`pgx/v5` ve Connection Pool](#5-pgxv5-ve-connection-pool)
6. [Repository Implementasyonu — Interface'i Somutlaştırmak](#6-repository-implementasyonu)
7. [`querier` Soyutlaması — Pool ve Tx'i Birleştirmek](#7-querier-soyutlaması)
8. [Unit of Work — Transaction Sınırı](#8-unit-of-work)
9. [`FOR UPDATE` — Satır Kilidi ve TOCTOU Yarışı](#9-for-update--satır-kilidi)
10. [Optimistic Locking — `version` Sütunu](#10-optimistic-locking)
11. [PG Hatalarını Domain Hatasına Çevirmek — `mapError`](#11-mappererror)
12. [Health vs Readiness Probe](#12-health-vs-readiness)
13. [Test Stratejisi — Unit + Build-Tag'li Integration](#13-test-stratejisi)
14. [Komutlar ve Çalıştırma](#14-komutlar-ve-çalıştırma)
15. [Hafta 2 Özeti](#15-hafta-2-özeti)

---

## 1. Hafta 2'nin Yeri

KPI roadmap'te **[Month 1] Foundation & Database** maddesinin kalan ayağındayız:

- ✅ PostgreSQL şema tasarımı, index ekleme
- ✅ `golang-migrate` ile migration yapısı
- ✅ `pgx/v5` ile DB bağlantısı

Mimaride neredeyiz? Hafta 1'de `internal/repository` paketinde sadece
**interface**'ler vardı (sözleşme). Bu hafta o sözleşmeyi yerine getiren somut
PostgreSQL kodunu `internal/storage/postgres` altına yazdık:

```
internal/
├── repository/                  # interface'ler (Hafta 1)
│   └── repository.go            #   UserRepository, WalletRepository, ...
└── storage/
    └── postgres/                # implementasyon (Hafta 2) ← YENİ
        ├── pool.go              #   pgxpool bağlantısı
        ├── migrate.go           #   golang-migrate runner
        ├── uow.go               #   UnitOfWork (tx)
        ├── errors.go            #   PG hata → domain hata çevirimi
        ├── user_repository.go
        ├── wallet_repository.go
        ├── transaction_repository.go
        └── assertions.go        #   compile-time interface kontrolü
```

**Neden `storage/postgres` ve `repository` ayrı?** Çünkü Clean Architecture'da
bağımlılık içeriye gider. `service` katmanı sadece `repository` interface'ini
import eder, `postgres` paketini **asla** görmez. PostgreSQL'i yarın MongoDB'ye
çevirsen `service` ve `domain` hiç değişmez — sadece yeni bir `storage/mongo`
yazarsın. `postgres` paketi, kod tabanında `pgx`'i import eden **tek** yerdir.

---

## 2. docker-compose ile Local PostgreSQL

Herkesin makinesinde aynı DB'nin aynı versiyonda çalışması için
`docker-compose.yml` yazdık:

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: wallet
      POSTGRES_PASSWORD: change-me
      POSTGRES_DB: wallet
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U wallet -d wallet"]
      interval: 5s
      timeout: 3s
      retries: 5
```

Kavramlar:

- **`image: postgres:16-alpine`** — Alpine tabanlı küçük imaj. `16` major
  versiyonu sabitler; `latest` kullanmak üretimde sürpriz yükseltme riski.
- **`ports: "5432:5432"`** — `host:container`. Makinende başka Postgres 5432'de
  çalışıyorsa sol tarafı `5433` yap, `DB_PORT=5433` ver.
- **`volumes: pgdata:...`** — **named volume**. Container'ı silsen bile veri
  kalır. Kasıtlı silmek için `docker compose down -v`.
- **`healthcheck`** — `pg_isready` exit 0 dönene kadar servis "healthy"
  sayılmaz. Month 4'te uygulama container'ı `depends_on: condition:
  service_healthy` ile buna bağlanacak: DB hazır olmadan migrate denenmesin.

Çalıştırma:
```bash
docker compose up -d        # arka planda başlat
docker compose ps           # healthy mi gör
docker compose logs -f      # logları izle
docker compose down         # durdur (veri kalır)
docker compose down -v      # durdur + veriyi sil
```

---

## 3. golang-migrate — Şema Versiyonlama

**Problem:** Şemayı (`CREATE TABLE ...`) elle çalıştırırsak, ekip arkadaşının
DB'si seninkinden farklı olur. Üretime çıkarken hangi tabloların eklendiğini
takip edemeyiz.

**Çözüm:** Migration = numaralandırılmış, sıralı SQL adımları. Her adımın bir
`up` (ileri) ve `down` (geri) hali var. `golang-migrate` hangi versiyonda
olduğunu DB'deki `schema_migrations` tablosunda tutar.

Dosya isimlendirme konvansiyonu:
```
000001_create_users.up.sql       ← ileri
000001_create_users.down.sql     ← geri
000002_create_wallets.up.sql
000002_create_wallets.down.sql
000003_create_transactions.up.sql
000003_create_transactions.down.sql
```

### Migration'ları binary'ye gömmek — `go:embed`

`migrations/embed.go`:
```go
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

`//go:embed *.sql` derleme zamanında tüm `.sql` dosyalarını **binary'nin içine**
gömer. Avantaj: dağıtımda ayrı `.sql` dosyası taşımana gerek yok, binary kendi
kendine yeter. Bir dosya eksikse **build patlar** — yanlışlıkla eski şemayla
deploy edemezsin.

### Migration'ı kod içinden çalıştırmak

`internal/storage/postgres/migrate.go`:
```go
func Migrate(dsn string) (err error) {
    db, err := sql.Open("pgx", dsn)            // database/sql + pgx stdlib driver
    if err != nil { return ... }
    defer func() { /* db.Close, hatayı yut(ma) */ }()

    driver, _ := migratepg.WithInstance(db, &migratepg.Config{})
    src, _ := iofs.New(appmigrations.FS, ".")  // embed FS'ten oku
    m, _ := migrate.NewWithInstance("iofs", src, "postgres", driver)

    if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
        return fmt.Errorf("apply migrations: %w", err)
    }
    return nil
}
```

İki ince nokta:

1. **`migrate.ErrNoChange` hata değildir.** DB zaten güncelse `m.Up()` bunu
   döner. Biz `errors.Is` ile yakalayıp "başarı" sayıyoruz. **Idempotent**:
   `Migrate`'i 100 kez çağır, ilkinden sonrası no-op.

2. **`database/sql` neden?** pgx'in iki yüzü var: native API (hızlı, pool için
   kullandığımız) ve `database/sql` uyumlu `stdlib` sürücüsü. `golang-migrate`
   `database/sql` konuşuyor, bu yüzden migration için `sql.Open("pgx", dsn)`
   kullanıyoruz. Bu, kod tabanında `database/sql`'e dokunan **tek** yer; geri
   kalan her şey native pgx pool.

### Uygulama açılışında otomatik migrate

`main.go` içinde pool açmadan **önce** migrate çalıştırıyoruz:
```go
if err := postgres.Migrate(cfg.Database.DSN()); err != nil {
    return fmt.Errorf("run migrations: %w", err)
}
```
Mantık: Kodun beklediği şema güncel değilse, uygulama hiç açılmasın (**fail
fast**). Eski şemaya karşı yeni kod çalıştırmak, sessiz veri bozulmasının en
yaygın sebebidir.

---

## 4. Migration Dosyalarımız ve Şema Tasarımı

Domain modelini (Hafta 1) DB tablosuna çevirdik. Kilit tasarım kararları:

### `Money` → `BIGINT`

Domain'de `Money.Amount int64` (kuruş/cent). DB'de **`BIGINT`** (= int64).
**ASLA `NUMERIC`/`FLOAT` değil.** Tip eşleşmesi tam olmalı; float yuvarlama
finansal sistemde denetim raporlarını bozar.

```sql
balance BIGINT NOT NULL DEFAULT 0
```

### Invariant'ları DB'de de kilitle — `CHECK` constraint

Domain `Debit`'te bakiyenin negatif olmasını zaten engelliyor. Ama "defense in
depth": bozuk bir migration ya da elle SQL bile bakiyeyi eksiye düşüremesin:

```sql
CONSTRAINT chk_wallets_balance_nonneg CHECK (balance >= 0),
CONSTRAINT chk_wallets_currency       CHECK (currency IN ('TRY','USD','EUR'))
```

Transaction tablosunda tipin doğru cüzdan kombinasyonuna sahip olmasını da
CHECK ile garantiledik:
```sql
CONSTRAINT chk_tx_wallets CHECK (
    (type='DEPOSIT'    AND source_wallet_id IS NULL     AND target_wallet_id IS NOT NULL) OR
    (type='WITHDRAWAL' AND source_wallet_id IS NOT NULL AND target_wallet_id IS NULL) OR
    (type='TRANSFER'   AND source_wallet_id IS NOT NULL AND target_wallet_id IS NOT NULL
                       AND source_wallet_id <> target_wallet_id)
)
```

### Unique constraint = idempotency ve iş kuralı

```sql
-- Bir kullanıcı + para birimi = tek cüzdan
CONSTRAINT uq_wallets_user_currency UNIQUE (user_id, currency)

-- Aynı idempotency key iki kez INSERT edilemez → retry double-charge'ı önler
CREATE UNIQUE INDEX idx_tx_idempotency_key ON transactions (idempotency_key);
```

### Index = sorgu hızı

```sql
-- E-posta ile login: unique + hızlı arama
CREATE UNIQUE INDEX idx_users_email ON users (email);

-- Cüzdan ekstresi: kaynak/hedef + zaman sırası
CREATE INDEX idx_tx_source_wallet ON transactions (source_wallet_id, created_at DESC);
CREATE INDEX idx_tx_target_wallet ON transactions (target_wallet_id, created_at DESC);
```

`(source_wallet_id, created_at DESC)` bileşik index'i "şu cüzdanın son
işlemleri" sorgusunu tek index taramasıyla karşılar. `created_at DESC` çünkü en
sık sorgu "en yeniler".

> **Not (gereksiz index'ten kaçınma):** `wallets`'ta ayrı bir `user_id` index'i
> eklemedik, çünkü `UNIQUE(user_id, currency)` constraint'i zaten `user_id`
> prefix'iyle başlayan bir index üretir; `user_id` üzerinden aramalar o
> index'ten karşılanır. Fazladan index = fazladan yazma maliyeti.

---

## 5. pgx/v5 ve Connection Pool

**Neden pool?** Her HTTP isteği için yeni DB bağlantısı açmak (TCP + TLS + auth)
pahalıdır. Pool, açık bağlantıları havuzda tutar, isteğe ödünç verir, geri alır.

`internal/storage/postgres/pool.go`:
```go
func NewPool(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
    poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
    if err != nil { return nil, ... }

    poolCfg.MaxConns        = cfg.MaxOpenConns      // toplam bağlantı tavanı
    poolCfg.MinConns        = cfg.MaxIdleConns      // sıcak taban (handshake'ten kaçın)
    poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime   // bağlantıyı periyodik yenile

    pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
    if err != nil { return nil, ... }

    if err := pool.Ping(ctx); err != nil {          // EAGER ping
        pool.Close()
        return nil, fmt.Errorf("ping database: %w", err)
    }
    return pool, nil
}
```

Tasarım kararları:

- **Eager `Ping`:** Pool oluşur oluşmaz bir bağlantı test ederiz. Yanlış DSN
  ilk istekte değil, **açılışta** patlasın (hızlı, açık hata).
- **`ctx` neyi yönetir?** Bağlantı *kurulumunu*, pool'un ömrünü değil. Buraya
  timeout'lu kısa context geç (main.go'da 10s verdik). Pool `Close`'a kadar
  yaşar.
- **Ping başarısızsa `pool.Close()`:** pgxpool arka planda goroutine açar; pool
  kullanılamazsa onları sızdırmadan kapat.

`main.go`'da:
```go
pool, err := postgres.NewPool(poolCtx, cfg.Database)
...
defer pool.Close()   // her çıkış yolunda bağlantıları boşalt
```
`defer pool.Close()` — `run() error` pattern'i tam da bunun için var. `log.Fatal`
defer'leri atlardı ve pool sızardı (Hafta 1 notu, bölüm 14).

---

## 6. Repository Implementasyonu

Hafta 1'de `UserRepository` bir interface'di. Şimdi onu yerine getiriyoruz:

```go
type UserRepository struct {
    pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
    return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
    const q = `
        INSERT INTO users (id, email, password_hash, full_name, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6)`
    _, err := querierFrom(ctx, r.pool).Exec(ctx, q,
        u.ID, u.Email, u.PasswordHash, u.FullName, u.CreatedAt, u.UpdatedAt)
    if err != nil {
        return fmt.Errorf("insert user: %w", mapError(err, domain.ErrUserNotFound))
    }
    return nil
}
```

İki CLAUDE.md kuralı burada görünür:

1. **Parametreli sorgu (`$1..$6`):** Kullanıcı girdisini ASLA string'e
   yapıştırmayız. SQL injection savunması bu — istisnasız her sorguda.
2. **Hata sarmalama:** `fmt.Errorf("insert user: %w", ...)` ile bağlam ekleyip
   orijinali koruyoruz (`%w`).

### `scanXxx` yardımcıları ve `rowScanner`

DB'den okurken `row.Scan(&field1, &field2, ...)` ile struct'ı dolduruyoruz.
Küçük bir interface ile bunu tekrar kullanılır yaptık:
```go
type rowScanner interface {
    Scan(dest ...any) error
}
```
`pgx.Row` bunu karşılar. Böylece `scanUser` hem `QueryRow` hem ileride `Query`
döngüsünden çağrılabilir.

### Money'yi yeniden birleştirme

DB'de `Money` iki sütuna yayıldı: `balance` (BIGINT) + `currency` (TEXT). Okurken
geri birleştiriyoruz:
```go
func scanWallet(row rowScanner) (*domain.Wallet, error) {
    var (w domain.Wallet; amount int64; currency domain.Currency)
    err := row.Scan(&w.ID, &w.UserID, &amount, &currency, &w.Version, ...)
    ...
    w.Balance = domain.Money{Amount: amount, Currency: currency}  // birleştir
    return &w, nil
}
```

---

## 7. querier Soyutlaması

**Problem:** Bir transferde aynı SQL'i bazen pool üzerinden (tek başına), bazen
transaction (tx) içinde çalıştırmak isteriz. İki ayrı repository yazmak
istemeyiz.

**Çözüm:** `*pgxpool.Pool` ve `pgx.Tx`'in **ortak** metotlarını bir interface'e
soyutla:

```go
type querier interface {
    Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
```

İmzalar pgx'inkiyle **birebir** olduğu için hem `Pool` hem `Tx` bu interface'i
örtük olarak karşılar (Hafta 1, interface bölümü — implicit satisfaction).

Sonra context'ten "şu an aktif bir tx var mı?" diye bakan bir seçici:
```go
type txKey struct{}   // çakışmasız, unexported context anahtarı

func querierFrom(ctx context.Context, pool *pgxpool.Pool) querier {
    if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
        return tx          // UnitOfWork içindeyiz → tx kullan
    }
    return pool            // tek başına → pool kullan
}
```

Her repository metodu `querierFrom(ctx, r.pool)` ile başlar. **Aynı kod**, hem
tx içinde hem dışında doğru çalışır. Çağıran taraf (UnitOfWork içinde mi değil
mi) karar verir; repository bunu bilmez bile.

---

## 8. Unit of Work

Transfer'in atomikliği finansal sistemin **olmazsa olmaz**ıdır: kaynaktan düştü,
hedefe eklenmeden program çöktü → müşterinin parası buharlaşır. Çözüm: ikisini
tek transaction'a sar, hepsi ya da hiçbiri.

`uow.go`:
```go
func (u *UnitOfWork) Do(ctx context.Context, fn func(ctx, repos) error) (err error) {
    tx, err := u.pool.Begin(ctx)
    if err != nil { return fmt.Errorf("begin tx: %w", err) }

    txCtx := context.WithValue(ctx, txKey{}, tx)   // tx'i context'e koy

    defer func() {
        if p := recover(); p != nil {
            _ = tx.Rollback(ctx)                    // panik → rollback, sonra re-panic
            panic(p)
        }
        if err != nil {
            if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
                err = errors.Join(err, fmt.Errorf("rollback: %w", rbErr))
            }
        }
    }()

    if err = fn(txCtx, u.repos); err != nil {
        return err                                  // defer rollback yapar
    }
    if err = tx.Commit(ctx); err != nil {
        return fmt.Errorf("commit tx: %w", err)
    }
    return nil
}
```

Kilit fikirler:

- **Callback pattern:** Transaction sınırı = fonksiyon sınırı. Commit/rollback'i
  unutman **imkânsız** — `Do` halleder.
- **`fn` nil dönerse COMMIT, hata dönerse ROLLBACK.** Tek kural.
- **Panik güvenliği:** `fn` panik atarsa transaction askıda kalmasın (yoksa
  kilitleri connection reap edilene kadar tutar). Rollback yapıp re-panic.
- **`errors.Join`:** Rollback'in kendisi de hata verirse, asıl hatayı kaybetme;
  ikisini birleştir.

Kullanımı (transfer):
```go
err := uow.Do(ctx, func(ctx context.Context, repos repository.Repositories) error {
    src, _ := repos.Wallets.GetForUpdate(ctx, srcID)   // tx içinde → FOR UPDATE anlamlı
    dst, _ := repos.Wallets.GetForUpdate(ctx, dstID)
    if err := src.Debit(amount); err != nil { return err }
    if err := dst.Credit(amount); err != nil { return err }
    if err := repos.Wallets.Update(ctx, src); err != nil { return err }
    return repos.Wallets.Update(ctx, dst)
})
```
`repos.Wallets` aslında pool-destekli aynı repository; ama `ctx` tx taşıdığı için
içerideki her çağrı `querierFrom` üzerinden tx'e gider.

---

## 9. FOR UPDATE — Satır Kilidi

**TOCTOU yarışı (Time Of Check To Time Of Use):**

```
Goroutine A: bakiye oku (100) → 100>=80? evet → 80 düş → yaz (20)
Goroutine B:        bakiye oku (100) → 100>=80? evet → 80 düş → yaz (20)
```

İkisi de 100 okudu, ikisi de "yeter" dedi, ikisi de 20 yazdı. 160 TL harcandı
ama bakiye 20'de kaldı → **40 TL havadan yaratıldı.** Felaket.

**Çözüm — `SELECT ... FOR UPDATE`:**
```go
func (r *WalletRepository) GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.Wallet, error) {
    const q = `SELECT ... FROM wallets WHERE id = $1 FOR UPDATE`
    ...
}
```

`FOR UPDATE` o satıra **yazma kilidi** koyar. Aynı satırı `FOR UPDATE` ile okuyan
ikinci transaction, birincisi commit/rollback yapana kadar **bloklanır**. Yani
A ve B artık sırayla çalışır, yarış biter.

> **KRİTİK:** `FOR UPDATE` sadece transaction içinde anlamlıdır. Pool'un
> autocommit'inde kilit hemen alınıp bırakılır, hiçbir işe yaramaz. Bu yüzden
> `GetForUpdate` **mutlaka** `uow.Do(...)` içinden çağrılmalı. Bu bir sözleşme;
> kodla zorlayamadığımız için dokümante ettik.

---

## 10. Optimistic Locking

`FOR UPDATE` "pesimist" kilittir (önce kilitle, sonra çalış). Alternatif:
**optimistic locking** — "çakışma olmaz herhalde, olursa yakalarım."

`wallets.version` sütunu her `Credit`/`Debit`'te artar (domain'de). Update'te:
```go
const q = `
    UPDATE wallets
    SET balance = $1, version = $2, updated_at = $3
    WHERE id = $4 AND version = $5`   // version = okuduğumuz (yani yeni-1)

tag, err := querierFrom(ctx, r.pool).Exec(ctx, q,
    w.Balance.Amount, w.Version, w.UpdatedAt, w.ID, w.Version-1)
...
if tag.RowsAffected() == 0 {
    return fmt.Errorf("update wallet %s: %w", w.ID, domain.ErrConcurrentUpdate)
}
```

Mantık: `WHERE ... AND version = $eski`. Başka bir transaction araya girip
version'ı değiştirdiyse, **0 satır etkilenir**. `RowsAffected() == 0` →
`ErrConcurrentUpdate`. Çağıran tüm read-modify-write'ı baştan deneyebilir.

**Neden ikisi birden (FOR UPDATE + version)?** `FOR UPDATE` ile çakışma zaten
serialize olur, version kontrolü teorik olarak gereksiz. Ama version kontrolü,
`Update`'in `GetForUpdate` olmadan (düz `SELECT` ile) çağrıldığı yolları da
korur. Bir predicate maliyetine "defense in depth" — bedava güvenlik.

---

## 11. mapError

`service` ve `handler` katmanları pgx bilmemeli. Ama "duplicate email" ile
"connection reset" farklı tepkiler ister (biri 409, diğeri 500). Bu çeviriyi tek
yerde yapıyoruz:

```go
func mapError(err error, notFound error) error {
    if err == nil { return nil }
    if errors.Is(err, pgx.ErrNoRows) { return notFound }   // satır yok

    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        switch pgErr.Code {
        case "23505": return domain.ErrAlreadyExists   // unique violation
        case "23503": return domain.ErrInvalidInput    // FK violation
        case "23514": return domain.ErrInvalidInput    // CHECK violation
        }
    }
    return err   // bilinmeyen → olduğu gibi (infra hatasını maskeleme!)
}
```

- **`errors.Is(err, pgx.ErrNoRows)`** — satır bulunamadı. Hangi domain hatasına
  çevrileceği tabloya göre değişir (`ErrUserNotFound` vs `ErrWalletNotFound`), o
  yüzden `notFound` parametre olarak geliyor.
- **`errors.As(err, &pgErr)`** — sarmalanmış zincirde `*pgconn.PgError` var mı
  diye bakar, varsa pointer'a koyar (Hafta 1, bölüm 17).
- **`23505` → `ErrAlreadyExists`** — duplicate email ya da tekrar eden
  idempotency key. İkincisi double-charge'ı engeller.
- **Bilinmeyen kod → `return err`** — connection reset, deadlock gibi gerçek
  altyapı hatalarını domain hatası gibi maskeleme; üst katman 500 görsün.

Bu fonksiyon **saf** (DB gerektirmez), bu yüzden en değerli unit test'imiz onu
hedefliyor — `errors_test.go`.

---

## 12. Health vs Readiness

İki farklı soru, iki farklı endpoint:

| | Soru | DB'ye dokunur mu? | Başarısızsa k8s ne yapar? |
|---|---|---|---|
| **Liveness** (`/health`) | "Process ayakta mı?" | **Hayır** | Pod'u **öldürür** (restart) |
| **Readiness** (`/readyz`) | "Trafik alabilir mi?" | **Evet** (Ping) | Pod'u **load balancer'dan çıkarır** |

```go
func Readiness(db Pinger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
        defer cancel()
        if err := db.Ping(ctx); err != nil {
            writeJSON(w, http.StatusServiceUnavailable, ...)   // 503
            return
        }
        writeJSON(w, http.StatusOK, ...)
    }
}
```

**Neden liveness DB'ye dokunmamalı?** Geçici bir DB kesintisinde liveness DB'yi
pinglerse, k8s sağlıklı pod'u öldürür → gereksiz restart fırtınası. Liveness =
"process canlı", readiness = "hazır". Ayrı sorular.

**`Pinger` interface'i:** Handler paketi DB import etmesin diye, ihtiyaç duyduğu
tek metodu (`Ping`) küçük bir interface'e soyutladık. `*pgxpool.Pool` bunu
karşılar. Bu sayede `handler` test'i sahte (fake) bir Pinger ile DB'siz
çalışıyor.

---

## 13. Test Stratejisi

İki katmanlı test yaklaşımı:

### a) Unit testler (DB gerektirmez) — her zaman çalışır

- `errors_test.go` → `mapError`'ın PG kodu → domain hata eşlemesi (table-driven).
- `handler_test.go` → `Readiness` 200/503 dallarını fake Pinger ile.

Bunlar `go test ./...` ile koşar, Docker istemez, CI'da hızlıdır.

### b) Integration testler (gerçek DB) — build tag arkasında

`integration_test.go` dosyasının başında:
```go
//go:build integration
```

Bu tag, dosyanın normal test binary'sine **derlenmemesini** sağlar. Yani
`go test ./...` onu görmez bile → Docker'sız CI hızlı ve yeşil kalır. Gerçek
DB'yle çalıştırmak için:

```bash
docker compose up -d
TEST_DATABASE_URL="host=localhost port=5432 user=wallet password=change-me dbname=wallet sslmode=disable" \
  go test -tags=integration ./internal/storage/postgres/...
```

Integration testler en kritik finansal davranışları kanıtlar:

- **`TestTransactionRepository_IdempotencyConflict`** — aynı idempotency key iki
  kez → `ErrAlreadyExists` (double-charge önlendi).
- **`TestWalletRepository_OptimisticLock`** — eski version ile Update →
  `ErrConcurrentUpdate` (lost-update önlendi).
- **`TestUnitOfWork_TransferAtomic`** — başarılı transfer iki bakiyeyi de
  değiştirir; hata olan transfer **hiçbir** bakiyeyi değiştirmez (rollback).

> **Durum (2026-06-15):** Bu makinede Docker çalışmadığı için integration
> testleri henüz **gerçek DB'ye karşı koşmadım** — sadece derlendiklerini
> doğruladım. Docker'ı açıp yukarıdaki komutla sen koşacaksın.

---

## 14. Komutlar ve Çalıştırma

```bash
# Bağımlılıkları çek/temizle
go mod tidy

# Derle / statik analiz / unit testler (DB'siz)
go build ./...
go vet ./...
go test ./...

# DB'yi ayağa kaldır
docker compose up -d
docker compose ps        # healthy bekle

# Uygulamayı çalıştır (.env dolu olmalı; .env.example'ı kopyala)
cp .env.example .env     # ilk sefer
go run ./cmd/server

# Readiness kontrolü
curl localhost:8080/readyz   # {"status":"ready"}  (DB ayaktaysa)
curl localhost:8080/health   # {"status":"ok"}

# Integration testler (DB gerekir)
TEST_DATABASE_URL="host=localhost port=5432 user=wallet password=change-me dbname=wallet sslmode=disable" \
  go test -tags=integration ./internal/storage/postgres/...
```

---

## 15. Hafta 2 Özeti — Bu noktada neyi bilmelisin

- [ ] `docker-compose` ile local Postgres (image, ports, volume, healthcheck)
- [ ] `golang-migrate`: up/down dosyaları, `go:embed`, idempotent `m.Up()`
- [ ] Şema tasarımı: `BIGINT` para, `CHECK`/`UNIQUE` constraint, bileşik index
- [ ] `pgxpool`: MaxConns/MinConns/MaxConnLifetime, eager `Ping`, `defer Close`
- [ ] Repository interface'ini PG ile somutlaştırma + parametreli sorgular
- [ ] `querier` soyutlaması: aynı kod hem pool hem tx üzerinde
- [ ] `UnitOfWork.Do`: callback pattern, commit/rollback, panik güvenliği
- [ ] `SELECT ... FOR UPDATE`: satır kilidi, TOCTOU yarışını kapatma
- [ ] Optimistic locking: `version` sütunu + `RowsAffected()==0`
- [ ] `mapError`: PG hata kodu → domain sentinel (`23505` vb.)
- [ ] Liveness vs Readiness probe ayrımı
- [ ] Unit test + build-tag'li integration test stratejisi
- [ ] Compile-time interface assertion (`var _ Iface = (*T)(nil)`)

## 📚 Hafta 3'e Hazırlık

Hafta 3 (KPI Month 2 devamı + Month 3 başı): **service katmanı
implementasyonu** (UserService, TransactionService — Deposit/Withdraw/Transfer
iş mantığı bu repository'leri kullanarak), **worker pool** (channel + goroutine
ile eşzamanlı işlem) ve ardından **chi router + middleware**'ler. Bu hafta
kurduğumuz `UnitOfWork` ve `GetForUpdate`, Transfer iş mantığının temeli olacak.
```
