# Hafta 1 — Go Çalışma Notları

> Bu not, Hafta 1'de wallet projesinde yazdığımız HER ŞEYİ sıfırdan anlatır.
> Sırasıyla oku; her bölüm bir öncekine yaslanır. Sonunda projede neyin neden
> böyle yazıldığını cümle cümle açıklayabiliyor olmalısın.

## İçindekiler

1. [Go Nedir? Temel Felsefe](#1-go-nedir-temel-felsefe)
2. [Go Modülü ve `go.mod`](#2-go-modülü-ve-gomod)
3. [Paket (Package) Kavramı](#3-paket-package-kavramı)
4. [Proje Klasör Yapısı — Standard Go Layout](#4-proje-klasör-yapısı--standard-go-layout)
5. [Değişkenler, Tipler, Sabitler](#5-değişkenler-tipler-sabitler)
6. [Struct ve Metotlar](#6-struct-ve-metotlar)
7. [Pointer vs Value](#7-pointer-vs-value)
8. [Interface — Go'nun Kalbi](#8-interface--gonun-kalbi)
9. [Error Handling](#9-error-handling)
10. [`context.Context`](#10-contextcontext)
11. [Goroutine, Channel, `select`](#11-goroutine-channel-select)
12. [Config Yönetimi — `cleanenv`](#12-config-yönetimi--cleanenv)
13. [Structured Logging — `zerolog`](#13-structured-logging--zerolog)
14. [HTTP Server ve Graceful Shutdown](#14-http-server-ve-graceful-shutdown)
15. [Clean Architecture Katmanları](#15-clean-architecture-katmanları)
16. [Domain Modelleri (Money, User, Wallet, Transaction)](#16-domain-modelleri)
17. [Sentinel Errors ve `errors.Is`](#17-sentinel-errors-ve-errorsis)
18. [Table-Driven Tests](#18-table-driven-tests)
19. [`go build`, `go vet`, `go test`, `go mod tidy`](#19-go-build-go-vet-go-test-go-mod-tidy)

---

## 1. Go Nedir? Temel Felsefe

Go (Golang), Google'da 2009'da çıkmış, derlenen, statik tipli bir dildir. Üç şeyi çok iyi yapar:

- **Hız:** C'ye yakın performans, GC (garbage collector) var ama düşük gecikmeli.
- **Eşzamanlılık (Concurrency):** `goroutine` ve `channel` ile bin onlarca paralel iş kolayca yönetilir.
- **Basitlik:** Az anahtar kelime (25 tane), generics dışında "magic" yok. Bir dosyayı açtığında ne olduğunu görürsün.

### Hello World

```go
package main

import "fmt"

func main() {
    fmt.Println("Merhaba dünya")
}
```

- `package main` → bu dosya çalıştırılabilir bir programın parçasıdır.
- `import "fmt"` → standart kütüphaneden formatlama paketi.
- `func main()` → programın giriş noktası. Her Go çalıştırılabilir programının tam olarak bir tane `main` fonksiyonu vardır.

### Alıştırma

1. `fmt.Println` yerine `fmt.Printf("%d + %d = %d\n", 2, 3, 5)` yazsan çıktı ne olur?  
   *Cevap: `2 + 3 = 5` (newline ile).*
2. `package main` yerine `package wallet` yazsam ne olur?  
   *Cevap: Artık çalıştırılabilir değil, kütüphane olur. `go run` hata verir.*

---

## 2. Go Modülü ve `go.mod`

Bir Go projesi = bir **modül**dür. Modülün adı genellikle git repo URL'sidir.

`go.mod` dosyamız:

```
module github.com/insiderEnesGozuela/go-app

go 1.26.3

require github.com/rs/zerolog v1.35.1
```

- `module ...` → bu modülün adı. Kod içinde `import "github.com/insiderEnesGozuela/go-app/internal/domain"` yazdığında Go bu prefix'i bu klasör olarak çözer.
- `go 1.26.3` → minimum Go versiyonu.
- `require ...` → kullandığımız dış paketler.

### Yararlı komutlar

| Komut | Ne yapar? |
|---|---|
| `go mod init github.com/.../app` | Yeni modül oluşturur, `go.mod` yazar. |
| `go get github.com/foo/bar` | Bağımlılık ekler. |
| `go mod tidy` | Kullanılmayanları siler, eksikleri ekler. |
| `go mod download` | Bağımlılıkları cache'e indirir. |

`go.sum` dosyası bağımlılıkların kriptografik hash'lerini tutar — supply chain güvenliği için.

### Alıştırma

1. `go.mod` içindeki module adının `import path` ile alakası nedir?  
   *Cevap: İçeri aktarırken yazdığın yolun başlangıcıdır. Modül `github.com/x/y` ise `internal/domain` paketini `github.com/x/y/internal/domain` olarak import edersin.*
2. `go get foo` ile `go mod tidy` farkı nedir?  
   *Cevap: `get` belirli paketi ekler. `tidy` tüm projeyi tarayıp eksikleri ekler + kullanılmayanları çıkarır.*

---

## 3. Paket (Package) Kavramı

Bir **paket** = aynı klasördeki, aynı `package X` satırıyla başlayan Go dosyalarının topluluğu.

```
internal/domain/
├── errors.go        // package domain
├── money.go         // package domain
├── user.go          // package domain
├── wallet.go        // package domain
└── transaction.go   // package domain
```

Hepsi `package domain` der. Aralarında **import yapmazlar**, doğrudan birbirinin fonksiyonlarını/tiplerini görürler. Bu önemli: Go'da aynı paket içindeki kodlar tek bir birimdir.

### Görünürlük (Export Kuralı)

Go'da `public`/`private` anahtar kelimesi yoktur. **Büyük harfle başlıyorsa dışarıdan görünür**, küçük harfle başlıyorsa paket içinde kalır:

```go
package domain

type User struct { ... }     // dışarıdan görünür (exported)
type passwordHash string     // sadece domain paketi içinde görünür
func NewUser(...) {}         // exported
func validateEmail(...) {}   // unexported (paket içi)
```

`domain.User` diye dışarıdan çağırabilirsin. `domain.passwordHash` diye çağıramazsın.

### Alıştırma

1. `internal/domain/user.go` içindeki `emailRegex` (küçük e) neden başka paketten görünmez?  
   *Cevap: Küçük harfle başlıyor → unexported. Bu kasıtlı; düzenli ifade implementation detayıdır.*
2. Aynı klasörde iki dosya `package foo` ve `package bar` derse ne olur?  
   *Cevap: Derleme hatası. Bir klasörde tek paket olabilir.*

---

## 4. Proje Klasör Yapısı — Standard Go Layout

Bizim layout:

```
go-app/
├── cmd/
│   └── server/
│       └── main.go              # uygulamanın giriş noktası
├── internal/                    # SADECE bu modülden import edilebilir
│   ├── config/                  # config okuma + validation
│   ├── logger/                  # zerolog wrapper
│   ├── domain/                  # iş kuralları, entity'ler (saf)
│   ├── repository/              # persistence interface'leri
│   ├── service/                 # use-case interface'leri
│   └── handler/                 # HTTP delivery (Month 3'te dolacak)
├── go.mod
└── go.sum
```

Üç altın kural:

1. **`cmd/<app-name>/main.go`**: Her çalıştırılabilir program için bir klasör. Bizde `cmd/server`. İleride `cmd/worker`, `cmd/migrate` ekleyebiliriz.
2. **`internal/`**: Go compiler'ı bu klasörü özel kabul eder. Başka bir modül `github.com/insiderEnesGozuela/go-app/internal/domain` import edemez. Yani gizli kalır.
3. **`pkg/`** (bizde yok): Başka projelerin de import edebileceği kütüphaneler buraya konur. Şimdilik ihtiyacımız yok.

### Bağımlılık Yönü (Clean Architecture)

```
handler → service → repository → domain
   ↑          ↑          ↑          ↑
   └──── DIŞARI         İÇERİ ────┘
```

Oklar her zaman **içeri**ye doğru gider. Yani `domain`, hiçbir şeyi import etmez. `repository`, `domain`'i import edebilir ama `service`'i edemez. Bu sayede:

- `domain`'i hiçbir DB/HTTP kütüphanesi olmadan test edebilirsin.
- DB'yi PostgreSQL'den MongoDB'ye değiştirirsen `domain` değişmez.

### Alıştırma

1. `internal/domain/wallet.go` içine `import "net/http"` yazsam Clean Architecture'a uyar mı?  
   *Cevap: Hayır. Domain HTTP bilmemeli. Bağımlılık dışarıyı işaret ediyor.*
2. Başka bir proje `github.com/insiderEnesGozuela/go-app/internal/domain` import edebilir mi?  
   *Cevap: Hayır, `internal/` klasörü dışa kapalıdır.*

---

## 5. Değişkenler, Tipler, Sabitler

### Değişken tanımlama — üç yol

```go
var x int = 5          // klasik
var y = 5              // tip çıkarımı
z := 5                 // kısa form, SADECE fonksiyon içinde
```

`:=` sadece fonksiyon içinde çalışır. Paket seviyesinde mecburen `var` kullanırsın.

### Temel tipler

| Tip | Örnek | Not |
|---|---|---|
| `bool` | `true`, `false` | |
| `int`, `int32`, `int64` | `42` | `int` platform-bağımlı (32 veya 64 bit). |
| `float32`, `float64` | `3.14` | **Para için ASLA kullanma.** |
| `string` | `"merhaba"` | UTF-8, immutable. |
| `byte` | `'a'` | `uint8` alias. |
| `rune` | `'ç'` | `int32` alias, Unicode code point. |

### `const` — Sabitler

```go
const Pi = 3.14
const (
    CurrencyTRY Currency = "TRY"
    CurrencyUSD Currency = "USD"
)
```

`const` derleme zamanında belirlenir. Çalışma zamanında değişmez. Bizim projede `Currency` ve `TransactionType` için kullandık.

### Zero Value

Go'da hiçbir değişken "tanımsız" olmaz. Atama yapmazsan **zero value** alır:

| Tip | Zero value |
|---|---|
| `int`, `float` | `0` |
| `bool` | `false` |
| `string` | `""` |
| pointer, slice, map, function, interface | `nil` |
| struct | her alanının zero value'su |

```go
var w Wallet            // w.Balance.Amount == 0, w.Version == 0, ...
```

### Alıştırma

1. `:=` operatörünü paket seviyesinde (fonksiyon dışında) kullanmaya çalışırsan ne olur?  
   *Cevap: Derleme hatası: "syntax error: non-declaration statement outside function body".*
2. `var s string`'in zero value'su nedir? `nil` mi?  
   *Cevap: `""` (boş string). String pointer değildir, `nil` olamaz.*

---

## 6. Struct ve Metotlar

### Struct = Veri taşıyıcı

```go
type User struct {
    ID           uuid.UUID
    Email        string
    PasswordHash string
    FullName     string
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

Türkçesi: "User adında bir tip tanımladım; ID, Email, ... alanları var."

### Struct oluşturma

```go
u := User{ID: id, Email: "a@b.com"}           // alan adlarıyla (önerilen)
u := User{id, "a@b.com", "", "", t, t}        // sırayla (kırılgan, kullanma)
u := &User{Email: "a@b.com"}                  // pointer dönüyor
```

### Metot ekleme

Metot, "üzerinde çalıştığı tipi" parantez içinde alır:

```go
func (w Wallet) IsEmpty() bool {
    return w.Balance.IsZero()
}
```

`(w Wallet)` kısmı **receiver**'dır. Türkçesi: "Wallet üzerinde `IsEmpty` adında bir metot tanımlıyorum, içeride o Wallet'a `w` adıyla erişirim."

### Value receiver vs Pointer receiver

```go
func (w Wallet) IsEmpty() bool { ... }     // value receiver — kopya alır
func (w *Wallet) Credit(amount Money) error { ... }  // pointer receiver — orijinali değiştirir
```

- **`Credit`'i değer alıcı yapsaydık**, fonksiyon Wallet'ın bir kopyasını alırdı. `w.Balance = next` yaptığında kopyanın bakiyesi değişirdi, gerçek wallet değişmezdi. Bu çok yaygın bir hata!
- **Kural:** Bir metot tipi değiştiriyorsa pointer receiver kullan.

### Alıştırma

1. `func (w Wallet) Credit(...)` (pointer'sız) yazsan ne olur?  
   *Cevap: `w.Balance = next` satırı çalışır ama dışarıdaki wallet etkilenmez. Sessiz bug.*
2. `Wallet{}` ifadesi geçerli mi?  
   *Cevap: Evet, tüm alanları zero value olan bir Wallet üretir.*

---

## 7. Pointer vs Value

### Pointer nedir?

Bir pointer, bir değerin **bellek adresini** tutar.

```go
x := 5
p := &x       // p, x'in adresi
fmt.Println(*p)   // 5  — yıldız "adresteki değeri getir" demek
*p = 10       // x artık 10
fmt.Println(x)    // 10
```

| Sembol | Anlam |
|---|---|
| `&x` | "x'in adresini al" |
| `*p` | "p'nin gösterdiği adresteki değeri al" |
| `*Wallet` | "Wallet pointer'ı" (tip) |

### Ne zaman pointer kullanırım?

1. **Bir fonksiyon orijinal değeri değiştirecekse** → pointer.
2. **Tip çok büyükse** (kopya pahalı) → pointer.
3. **`nil` olabilecekse** (var yok'u temsil ediyorsa) → pointer.

Bizim domain'de:

```go
SourceWalletID *uuid.UUID    // nil olabilir (deposit'te kaynak yok)
TargetWalletID *uuid.UUID    // nil olabilir (withdrawal'da hedef yok)
```

Burada pointer kullanma sebebimiz **"yok olabilir"** olması. `uuid.UUID` direkt yazsaydık, zero value (`00000000-...`) ile "yok" arasını ayıramazdık.

### Alıştırma

1. `var p *int`'in değeri nedir? `*p` yazsan ne olur?  
   *Cevap: `p == nil`. `*p` panik üretir (nil pointer dereference).*
2. Neden `Wallet`'ı çok yerde `*Wallet` olarak geçiriyoruz?  
   *Cevap: (a) Mutasyon (Credit/Debit değiştirir), (b) kopya maliyetinden kaçınma.*

---

## 8. Interface — Go'nun Kalbi

Interface, "şu metotları olan herhangi bir tipi kabul ederim" demektir. Sınıf hiyerarşisi yoktur.

### Tanım

```go
type UserRepository interface {
    Create(ctx context.Context, u *domain.User) error
    GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
    GetByEmail(ctx context.Context, email string) (*domain.User, error)
}
```

"UserRepository, bu üç metodu olan herhangi bir tiptir."

### Implicit Satisfaction (örtük uyma)

Java/C#'ta `class PostgresUserRepo implements UserRepository` yazarsın. Go'da **yazmaz**sın. Eğer tipinde o metotlar varsa, otomatik olarak interface'i karşılar:

```go
type PostgresUserRepo struct { db *pgxpool.Pool }

func (r *PostgresUserRepo) Create(ctx context.Context, u *domain.User) error { ... }
func (r *PostgresUserRepo) GetByID(...) (*domain.User, error) { ... }
func (r *PostgresUserRepo) GetByEmail(...) (*domain.User, error) { ... }

// Hiç "implements" demedik! Ama:
var repo UserRepository = &PostgresUserRepo{db: pool}  // çalışır
```

Bu çok güçlü çünkü:

- Interface'i tüketici (`service`) tanımlar, uygulayıcı (`repository/postgres`) sadece kodu yazar.
- Aynı interface'i bir de `MemoryUserRepo` ile sağlayıp test edebilirsin — gerçek DB'ye ihtiyaç yok.

### Bizim projemizdeki örnek

`internal/service/service.go` UserService interface'ini tanımlar. Service implementation'ı `UserRepository`'yi alır:

```go
type userService struct { repo repository.UserRepository }

func (s *userService) Register(ctx context.Context, in RegisterUserInput) (*domain.User, error) {
    // s.repo'yu çağırır — gerçekte PG mi, in-memory mi bilmez
}
```

Test ederken `repo`'ya mock geçer, gerçek DB başlatmazsın. Buna **Dependency Injection** denir.

### Empty interface — `interface{}` veya `any`

```go
func Print(v any) { fmt.Println(v) }
```

Her şey her interface'i karşılamaya yakın olmaz ama hiçbir metodu olmayan `any`'i her tip karşılar. `logger.With` fonksiyonumuzda `kv ...any` aldık çünkü değerler farklı tiplerde olabilir.

### Alıştırma

1. `PostgresUserRepo`'da `Create` metodunun parametreleri interface'dekiyle farklı yazılsa ne olur?  
   *Cevap: Tip interface'i karşılamaz. `var x UserRepository = &PostgresUserRepo{}` derleme hatası verir.*
2. `interface{}` yerine artık `any` yazmamızın sebebi nedir?  
   *Cevap: Go 1.18'de `any` eklendi, `interface{}` için takma ad. Daha okunaklı.*

---

## 9. Error Handling

Go'da exception yoktur. Hatalar **dönüş değeri** olarak gelir.

### Klasik pattern

```go
u, err := repo.GetByID(ctx, id)
if err != nil {
    return nil, err
}
// u'yu kullan
```

`if err != nil` her yerde görürsün. Bu Go'nun "implicit'ten kaçınma" felsefesidir — hatayı görmek zorundasın.

### `error` aslında bir interface

```go
type error interface {
    Error() string
}
```

Yani `Error() string` metodu olan herhangi bir tip hata olabilir.

### `errors.New` ve `fmt.Errorf`

```go
err1 := errors.New("user not found")               // basit
err2 := fmt.Errorf("get user %s: %w", id, err1)    // wrap
```

`%w` özel: hatayı **sarmalar (wrap)**. Sarmalanan hatayı sonra `errors.Is` ile yakalayabilirsin.

### Hata sarmalama — CLAUDE.md kuralı

CLAUDE.md "errors must be propagated upwards with meaningful context added" diyor. Yani:

```go
// KÖTÜ
if err != nil { return err }                      // ne yapıyorduğun belli değil

// İYİ
if err != nil {
    return fmt.Errorf("load config: %w", err)     // bağlam ekledik, orijinali korudu
}
```

`config.Load()` fonksiyonumuzda:

```go
return nil, fmt.Errorf("read config file %q: %w", path, err)
```

Üst katmana hata zinciri ulaşır:

```
fatal: read config file "config.yaml": open config.yaml: no such file or directory
```

### `_ = err` YASAK

CLAUDE.md "errors must never be ignored" diyor. Şunu yazma:

```go
_, _ = repo.GetByID(ctx, id)   // hatayı yedik
```

### Alıştırma

1. `fmt.Errorf("...: %v", err)` ile `fmt.Errorf("...: %w", err)` farkı nedir?  
   *Cevap: `%v` sadece string olarak basar. `%w` sarmalar — sonra `errors.Is/As` ile yakalanabilir.*
2. Bir fonksiyonun en çok kaç dönüş değeri olabilir? Tipik kaç tanedir?  
   *Cevap: Sınır yok ama tipik olarak ikidir: `(sonuç, error)`.*

---

## 10. `context.Context`

`context.Context`, bir isteğin (request) yaşam döngüsünü temsil eder. İçinde üç şey taşır:

1. **Cancellation signal:** "Bu iş artık iptal, dur."
2. **Deadline:** "5 saniye sonra otomatik iptal."
3. **Request-scoped values:** request_id, user_id gibi.

### Neden her yerde `ctx context.Context`?

CLAUDE.md kuralı: I/O yapan her fonksiyonun **ilk** parametresi `ctx` olmalı.

```go
func (r *PostgresUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
```

Sebep: Çağıran taraf "vazgeçtim, iptal et" diyebilmeli. HTTP request iptal olunca DB query de iptal olur, kaynak boşa harcanmaz.

### Üretme

```go
ctx := context.Background()                              // kök context
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)   // 5 sn deadline
defer cancel()                                           // kaynak sızdırma

ctx, cancel := context.WithCancel(parent)                // manuel iptal
```

`defer cancel()` çok kritik — `cancel` çağrılmazsa goroutine sızabilir.

### `signal.NotifyContext` — main.go'da kullandık

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
```

Bu, OS sinyali (Ctrl+C, `kill`) alınca `ctx.Done()` kanalını kapatır. main.go'da:

```go
<-ctx.Done()    // sinyal gelene kadar bloklar
```

### Context'e değer koyma

Logger paketimizde:

```go
func Into(ctx context.Context, l zerolog.Logger) context.Context {
    return context.WithValue(ctx, ctxKey{}, l)
}
```

`ctxKey{}` boş struct, tek amacı **eşsiz anahtar** olmak. Başka paket aynı string'i kullansa çakışma olmasın diye `type ctxKey struct{}` (unexported) yapıyoruz.

### Alıştırma

1. `context.WithTimeout` çağırdıktan sonra `defer cancel()` unutursan ne olur?  
   *Cevap: Context internal'inde tutulan timer ve referanslar GC'lenmez. Memory/goroutine leak.*
2. `ctx.Done()` ne döndürür?  
   *Cevap: `<-chan struct{}` — context iptal olunca kapatılan kanal. `<-ctx.Done()` bekler.*

---

## 11. Goroutine, Channel, `select`

### Goroutine — ucuz thread

```go
go fonksiyonAdi()        // arka planda çalıştır, dönüşü bekleme
go func() { ... }()      // anonim
```

Bir goroutine ~2KB stack ile başlar. Yüz binlercesini rahatça başlatabilirsin. OS thread'inden farkı budur.

### Channel — goroutine'ler arası iletişim

```go
ch := make(chan int)        // unbuffered
ch := make(chan int, 10)    // buffered (kapasite 10)

ch <- 5                     // gönder
v := <-ch                   // al
close(ch)                   // kapat
```

- **Unbuffered:** gönderen, alıcı hazır olana kadar bloklar.
- **Buffered:** kapasite dolana kadar göndermek serbest.

### `select` — birden fazla kanalı dinle

main.go'mızda:

```go
select {
case err := <-serverErr:
    return fmt.Errorf("server failed: %w", err)
case <-ctx.Done():
    log.Info().Msg("shutdown signal received")
}
```

"Hangisi önce gelirse onu yap." `serverErr` kanalına bir şey düşerse server crash olmuş. `ctx.Done()` kapanırsa Ctrl+C basılmış. İkisinden biri olana kadar bekler.

### Goroutine leak — main.go neden buffered kanal kullandı?

```go
serverErr := make(chan error, 1)   // kapasite 1
go func() {
    if err := server.ListenAndServe(); ... {
        serverErr <- err           // SIZINTI RİSKİ
    }
}()
```

Eğer kanal **unbuffered** olsaydı: `select` shutdown sinyalini önce aldıysa, fonksiyon biter, kimse `<-serverErr` yapmaz. Goroutine `serverErr <- err` satırında sonsuza dek bloklanır → leak. Buffer 1 olunca gönderen blocklamaz, goroutine temiz çıkar.

### Race Condition

İki goroutine aynı değişkene yazarsa **veri yarışı** olur. Çözümler:

1. **`sync.Mutex` / `sync.RWMutex`**: kilit
2. **Channel**: paylaşma yerine iletişim
3. **`sync/atomic`**: tek değer için atomic operasyon

Wallet bakiye güncelleme bu yüzden Hafta 2'de **DB transaction + `FOR UPDATE`** ile yapılacak — uygulama seviyesi mutex tek instance'ta kalır, çoklu instance'ta yetmez.

### Alıştırma

1. `go func() { panic("x") }()` ana program bekliyorken çağrılsa ne olur?  
   *Cevap: Tüm programı çökertir. Recover etmediysen goroutine içindeki panik process'i bitirir.*
2. Kapatılmış kanaldan okumak ne döner?  
   *Cevap: Tip'in zero value'su + `ok=false`. `v, ok := <-ch` deyimi ile yakalanır.*

---

## 12. Config Yönetimi — `cleanenv`

Uygulamamızın port, DB şifresi, log seviyesi gibi ayarlarını **kod dışında** tutuyoruz — bu 12-Factor App prensibidir.

### Struct tag'leri

```go
type DatabaseConfig struct {
    Host     string `yaml:"host" env:"DB_HOST" env-default:"localhost"`
    Password string `yaml:"password" env:"DB_PASSWORD" env-required:"true"`
}
```

Backtick içindeki şeyler **struct tag**'dir. Cleanenv bunları okur:

- `yaml:"host"` → YAML dosyasındaki anahtar adı.
- `env:"DB_HOST"` → environment variable adı.
- `env-default:"localhost"` → env yoksa bu değer.
- `env-required:"true"` → env yoksa hata ver.

### `Load` fonksiyonu

```go
func Load() (*Config, error) {
    var cfg Config
    if err := cleanenv.ReadEnv(&cfg); err != nil {
        return nil, fmt.Errorf("read env: %w", err)
    }
    if err := cfg.validate(); err != nil {
        return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
    }
    return &cfg, nil
}
```

Sıra önemli: önce oku, sonra validate et. Validation domain kuralları içerir ("port boş olamaz", "max_idle ≤ max_open").

### `.env` dosyası

`cleanenv` çalıştığında otomatik olarak proje kökündeki `.env` dosyasını okur (sebep: içinde `joho/godotenv` kullanıyor). `.env` dosyası **commit edilmez** (.gitignore'da). `.env.example` ise commit edilir — şablondur.

### Alıştırma

1. `env-required:"true"` olan bir field için env var konulmazsa ne olur?  
   *Cevap: `cleanenv.ReadEnv` hata döner. Bizim `Load`'da `fmt.Errorf("read env: %w", err)` ile sarmalanır.*
2. Neden `Load() *Config` yerine `Load() (*Config, error)` yazdık?  
   *Cevap: Config hatasında programı `panic` etmek yerine main.go'nun düzgün çıkış yapmasını istiyoruz.*

---

## 13. Structured Logging — `zerolog`

### Klasik vs Structured

Klasik:
```
2026-05-25 INFO User created: id=abc
```

Structured (JSON):
```json
{"level":"info","time":"2026-05-25T11:59:52Z","user_id":"abc","msg":"user created"}
```

Avantaj: log aggregator'da (Loki, ELK) `user_id="abc"` diye filtreleyebilirsin.

### Builder pattern

```go
log.Info().
    Str("env", cfg.App.Env).
    Str("port", cfg.HTTP.Port).
    Msg("starting server")
```

- `log.Info()` → yeni event başlat (level=info).
- `.Str("key", "value")` → string alan ekle.
- `.Int("count", 5)` → int alan.
- `.Err(err)` → error alan (otomatik `error` key'i).
- `.Msg("...")` → event'i flush et (en sonda mutlaka çağrılır).

### Pretty mode vs JSON mode

```go
if opts.Pretty {
    w = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
}
```

Dev modunda renkli/insan okuyabilir; production'da düz JSON. Konfigürasyondan kontrol ediliyor (`LOG_PRETTY=true`).

### Context propagation — Bizim eklediğimiz

```go
func With(ctx context.Context, kv ...any) context.Context {
    l := From(ctx)
    c := l.With()
    for i := 0; i+1 < len(kv); i += 2 {
        key, _ := kv[i].(string)
        c = c.Interface(key, kv[i+1])
    }
    return Into(ctx, c.Logger())
}
```

Kullanımı:
```go
ctx = logger.With(ctx, "request_id", "abc-123", "user_id", uid)
// artık bu ctx'i alan her log satırına otomatik request_id ve user_id eklenir
```

Bu çok güçlü çünkü servis/repository fonksiyonlarına ekstra parametre geçmiyorsun — context her zaten taşınıyor.

### Alıştırma

1. `.Msg(...)` çağırmadan log oluşur mu?  
   *Cevap: Hayır, `Msg`/`Msgf`/`Send` flush eder. Çağırmazsan log düşmez.*
2. `logger.From(ctx)` context'te logger yoksa ne döner?  
   *Cevap: `zerolog.Nop()` — hiçbir şey basmayan logger. Sessiz fallback, panik yok.*

---

## 14. HTTP Server ve Graceful Shutdown

### Basit HTTP server

```go
mux := http.NewServeMux()
mux.HandleFunc("/health", handler.Health)

server := &http.Server{
    Addr:         ":8080",
    Handler:      mux,
    ReadTimeout:  5*time.Second,
    WriteTimeout: 10*time.Second,
}
server.ListenAndServe()
```

`http.Server`'da `Read/WriteTimeout` çok kritik — yoksa yavaş istemci connection'ı sonsuza kadar açık tutabilir (Slowloris saldırısı).

### Graceful shutdown ne demek?

Server'ı sertçe kapatma ("kill -9"):
- Yarım giden istekler patlak.
- DB transaction'lar yarıda kalır.
- Müşteri "Para çekildi" cevabı alamadan kuyruğa düşer.

Graceful shutdown:
- Yeni istek alma.
- Mevcut istekleri bitmesini bekle (timeout ile).
- DB bağlantılarını kapat.
- Sonra çık.

### Bizim implementasyon

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

serverErr := make(chan error, 1)
go func() {
    if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        serverErr <- err
        return
    }
    serverErr <- nil
}()

select {
case err := <-serverErr:
    return fmt.Errorf("server failed: %w", err)
case <-ctx.Done():
    log.Info().Msg("shutdown signal received")
}

shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
defer cancel()

if err := server.Shutdown(shutdownCtx); err != nil {
    _ = server.Close()
    return fmt.Errorf("graceful shutdown failed: %w", err)
}
```

Adım adım:
1. `signal.NotifyContext`: SIGINT/SIGTERM gelince `ctx` iptal olur.
2. Server'ı goroutine'de başlat — main thread bloklamasın.
3. `select`: ya server hata verdi ya da kapanma sinyali geldi.
4. `server.Shutdown(shutdownCtx)`: yeni istek alma, mevcut istekleri 15s'ye kadar bekle.
5. 15s yetmezse `server.Close()` ile zorla kapat.

### `ListenAndServe` ve `http.ErrServerClosed`

`server.Shutdown` çağrıldığında `ListenAndServe` `http.ErrServerClosed` döner. Bu **normal kapanma**dır, hata değil:

```go
if err != nil && !errors.Is(err, http.ErrServerClosed) {
    serverErr <- err   // gerçek hata
}
```

### `run() error` pattern

```go
func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
        os.Exit(1)
    }
}

func run() error { ... }
```

Neden `main` içinde her şeyi yapmıyoruz?
- `log.Fatal` ve `os.Exit` **defer'leri atlamaz**. DB pool'u kapatmayı `defer pool.Close()` ile yaptıysan, `log.Fatal` çağırınca pool kapanmaz.
- `run() error` ile defer'ler düzgün çalışır.
- Test edilebilir: testten `run()` çağırırsın.

### Alıştırma

1. `server.Shutdown(ctx)` 15s içinde bitmezse ne olur?  
   *Cevap: Context timeout olur, `Shutdown` `context.DeadlineExceeded` döner. Biz bunu yakalayıp `server.Close()` ile zorla kapatıyoruz.*
2. `serverErr` buffered olmasaydı ve önce SIGTERM gelirse ne olur?  
   *Cevap: Goroutine `serverErr <- err` satırında sonsuza dek bloklanır → goroutine leak.*

---

## 15. Clean Architecture Katmanları

### 4 katman

| Katman | Klasör | İş |
|---|---|---|
| **Domain** | `internal/domain` | Saf iş kuralları, entity'ler. Hiçbir şey import etmez. |
| **Repository** | `internal/repository` | Persistence interface'leri (PG, Redis, vs.). Domain'i import eder. |
| **Service** | `internal/service` | Use-case orkestrasyon (Deposit, Transfer). Repository interface'lerini kullanır. |
| **Handler** | `internal/handler` | HTTP delivery — JSON parse, status code. Service'i çağırır. |

### Bağımlılık akışı

```
HTTP request
  ↓
Handler (parse JSON)
  ↓
Service (iş mantığı: validate, koordine et)
  ↓
Repository interface (use it through interface!)
  ↓
PostgreSQL implementation
  ↓
Database
```

### Neden böyle?

1. **Test:** Service'i test ederken DB başlatmazsın. Mock Repository verirsin.
2. **Değiştirilebilirlik:** PostgreSQL → MongoDB geçişinde sadece `repository/postgres` → `repository/mongo` yazarsın; service ve domain değişmez.
3. **Sınırları net çiz:** Domain'in HTTP'den haberi olamaz. Repository iş mantığı yapmaz.

### `UnitOfWork` — transaction sınırı

```go
type UnitOfWork interface {
    Do(ctx context.Context, fn func(ctx context.Context, repos Repositories) error) error
}
```

Transfer örneği (Hafta 2'de yazacağız):

```go
err := uow.Do(ctx, func(ctx context.Context, repos repository.Repositories) error {
    src, _ := repos.Wallets.GetForUpdate(ctx, srcID)   // FOR UPDATE lock
    dst, _ := repos.Wallets.GetForUpdate(ctx, dstID)
    if err := src.Debit(amount); err != nil { return err }
    if err := dst.Credit(amount); err != nil { return err }
    repos.Wallets.Update(ctx, src)
    repos.Wallets.Update(ctx, dst)
    return nil
})
// uow.Do otomatik olarak: fn nil dönerse COMMIT, error dönerse ROLLBACK
```

Bu pattern callback şeklinde olduğu için tx'i unutman imkânsız.

### Alıştırma

1. `domain` paketi `repository` paketini import edebilir mi?  
   *Cevap: Hayır. Ok içeriye gider. Edersen iki taraflı bağımlılık (cycle) oluşur, Go derleme hatası verir.*
2. UnitOfWork olmadan transfer yapsak hangi felaket olur?  
   *Cevap: Source'tan düşürdün, target'a eklemeden program crash. Müşterinin parası buharlaşır.*

---

## 16. Domain Modelleri

### Money — Para neden `int64`?

```go
type Money struct {
    Amount   int64
    Currency Currency
}
```

`float64` ile para tutarsan:
```go
0.1 + 0.2 == 0.30000000000000004    // IEEE 754
```

Milyonlarca işlemde bu yuvarlamalar birikir, denetim raporları tutmaz. Endüstri standardı: **minor units** (kuruş, cent) olarak `int64` tut.

100 TL = `Money{Amount: 10000, Currency: "TRY"}` (kuruş).

### Currency — tip-güvenli enum

```go
type Currency string

const (
    CurrencyTRY Currency = "TRY"
    CurrencyUSD Currency = "USD"
    CurrencyEUR Currency = "EUR"
)
```

`Currency` ayrı bir tip olduğu için, fonksiyon imzasında `Currency` istiyorsan bir `string` veremezsin (cast etmen gerekir). Bu compile-time güvenlik sağlar.

### Method receiver — `Add` örneği

```go
func (m Money) Add(other Money) (Money, error) {
    if m.Currency != other.Currency {
        return Money{}, fmt.Errorf("%w: %s vs %s", ErrCurrencyMismatch, m.Currency, other.Currency)
    }
    return Money{Amount: m.Amount + other.Amount, Currency: m.Currency}, nil
}
```

Value receiver kullandık çünkü:
- `Money` küçük (16 byte).
- `Add` yeni `Money` döner, orijinali değiştirmez (immutable pattern).
- Goroutine'ler arası güvenli paylaşılır.

### Wallet — invariant'lar entity'de

```go
func (w *Wallet) Debit(amount Money) error {
    if !amount.IsPositive() {
        return fmt.Errorf("%w: debit amount must be > 0", ErrNonPositiveAmount)
    }
    ok, err := w.Balance.GreaterThanOrEqual(amount)
    if err != nil { return err }
    if !ok {
        return fmt.Errorf("%w: have %s, need %s", ErrInsufficientBalance, w.Balance, amount)
    }
    // ...
}
```

Kuralları **entity'nin metodu** içinde yazdık. Servis seviyesinde yazsaydık başka servisler bypass edebilirdi. Bu "Tell, Don't Ask" prensibidir.

### Version — optimistic locking için hazırlık

```go
type Wallet struct {
    // ...
    Version int64
}
```

Her `Credit`/`Debit`'te `Version++`. DB'ye yazarken `UPDATE ... WHERE id=? AND version=?`. Başka tx version'ı değiştirmişse 0 satır etkilenir, çakışma yakalanır. Hafta 2'de göreceğiz.

### Transaction — null pointer ile "yok"

```go
type Transaction struct {
    SourceWalletID *uuid.UUID
    TargetWalletID *uuid.UUID
}
```

- Deposit: `Source=nil, Target=walletID`
- Withdrawal: `Source=walletID, Target=nil`
- Transfer: ikisi de dolu

`uuid.UUID` direkt yazsaydık zero value (`00000000-...`) ile "yok" arasında ayrım yapamazdık. Pointer olunca `nil` = "yok" diyebiliyoruz.

### Alıştırma

1. Neden `Money.Add` pointer receiver değil?  
   *Cevap: Yeni `Money` döner; orijinali değiştirmez. Immutable olunca paylaşımı güvenli. Ayrıca tip küçük, kopya ucuz.*
2. `IdempotencyKey` ne işe yarar?  
   *Cevap: İstemci aynı isteği iki kez gönderirse (network retry vs.) iki kez deposit yapma. Repository (user_id, key) unique constraint'i ile ikinci isteği reddeder.*

---

## 17. Sentinel Errors ve `errors.Is`

### Sentinel = "haber veren hata"

```go
// internal/domain/errors.go
var (
    ErrInsufficientBalance = errors.New("insufficient balance")
    ErrCurrencyMismatch    = errors.New("currency mismatch between wallets")
)
```

Bunlar paket seviyesi değişken. Tek bir instance var. Üst katman tipini kontrol eder, mesajını değil.

### Sarmalama

```go
return fmt.Errorf("%w: have %s, need %s", ErrInsufficientBalance, w.Balance, amount)
```

Bu hata:
- String olarak: `insufficient balance: have 60 TRY, need 1000 TRY`
- `errors.Is(err, ErrInsufficientBalance) == true`

### `errors.Is` — tip kontrolü

```go
err := wallet.Debit(amount)
if errors.Is(err, ErrInsufficientBalance) {
    return c.JSON(400, "Yetersiz bakiye")
}
if err != nil {
    return c.JSON(500, "Sunucu hatası")
}
```

String'e bakarak değil, **tip'e** bakarak karar veriyorsun. Hata mesajı değişirse handler bozulmaz.

### `errors.As` — sarmalamayı çöz

```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == "23505" {
    return ErrAlreadyExists
}
```

Bu, sarmalanmış hata zinciri içinde belirli tipte hata var mı diye bakar; varsa pointer'a koyar.

### Alıştırma

1. `errors.New("user not found")` ile `var ErrUserNotFound = errors.New("user not found")` farkı nedir?  
   *Cevap: İlki her seferinde yeni instance. İkincisi tek paket seviyesi instance — karşılaştırılabilir.*
2. `errors.Is(err, target)` ne yapar?  
   *Cevap: `err` zincirinde `target` ile eşit (sarmalanmış da olabilir) bir hata var mı diye bakar.*

---

## 18. Table-Driven Tests

Go'da test = `_test.go` ile biten dosyada, `func TestX(t *testing.T)` imzalı fonksiyon.

### Klasik test

```go
func TestNewMoney_TRY(t *testing.T) {
    m, err := NewMoney(100, CurrencyTRY)
    if err != nil { t.Fatal(err) }
    if m.Amount != 100 { t.Errorf("expected 100, got %d", m.Amount) }
}
```

### Table-driven (önerilen)

```go
func TestNewMoney(t *testing.T) {
    tests := []struct {
        name     string
        amount   int64
        currency Currency
        wantErr  error
    }{
        {"valid TRY", 100, CurrencyTRY, nil},
        {"valid USD zero", 0, CurrencyUSD, nil},
        {"unsupported currency", 100, Currency("XYZ"), ErrUnsupportedCurrency},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            _, err := NewMoney(tc.amount, tc.currency)
            if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
                t.Fatalf("expected %v, got %v", tc.wantErr, err)
            }
        })
    }
}
```

Faydaları:
- Yeni case eklemek tek satır.
- `t.Run(tc.name, ...)` her case'i ayrı subtest yapar; output okunaklı.
- Aynı setup'ı tekrarlamazsın.

### `t.Fatal` vs `t.Error`

- `t.Error`: hata kaydeder, test devam eder.
- `t.Fatal`: hata kaydeder, **bu subtest'i durdurur**.

Kural: Eğer bundan sonraki satırlar nil'e dereference yapacaksa `t.Fatal`. Yoksa `t.Error`.

### `t.Setenv` — config testlerimizde

```go
t.Setenv("DB_PASSWORD", "x")
t.Setenv("APP_ENV", "qa")
```

Test bitince otomatik geri alır. Manuel `os.Setenv` kullansaydın diğer testlere sızabilirdi.

### Çalıştırma

```bash
go test ./...                     # tüm paketler
go test ./internal/domain         # tek paket
go test -v ./internal/domain      # verbose (her subtest'i bas)
go test -run TestNewMoney ./...   # regex'le filtre
go test -cover ./...              # coverage
```

### Alıştırma

1. `t.Run("foo", func(t *testing.T) { ... })` ne yapar?  
   *Cevap: İç içe subtest oluşturur. Output'ta `TestX/foo` olarak görünür, filtre edilebilir.*
2. Test dosyası adı neden `_test.go` ile bitmeli?  
   *Cevap: Go convention'ı. Bu dosyalar normal build'e dahil edilmez, sadece `go test` koşulurken derlenir.*

---

## 19. `go build`, `go vet`, `go test`, `go mod tidy`

| Komut | Ne yapar? | Ne zaman? |
|---|---|---|
| `go run ./cmd/server` | Derle ve hemen çalıştır | Geliştirirken |
| `go build ./...` | Tüm paketleri derle (binary üretmez ./... ile) | CI'da, "compile ediyor mu?" |
| `go build -o bin/server ./cmd/server` | Binary üret | Deploy öncesi |
| `go vet ./...` | Statik analiz: unreachable code, format yanlışlığı, vs. | Her commit öncesi |
| `go test ./...` | Tüm testleri çalıştır | Her commit öncesi |
| `go test -race ./...` | Race detector ile çalıştır | Concurrent kod yazınca |
| `go mod tidy` | Bağımlılıkları temizle | `go get` sonrası |
| `gofmt -w .` veya `go fmt ./...` | Kodu otomatik formatla | Editör genelde yapar |

### Bizim Hafta 1 sonu komut çıktımız

```bash
$ go mod tidy
$ go build ./...      # temiz
$ go vet ./...        # temiz
$ go test ./...
ok  github.com/insiderEnesGozuela/go-app/internal/config  0.260s
ok  github.com/insiderEnesGozuela/go-app/internal/domain  0.391s
```

Hepsi `ok` → güvenle Hafta 2'ye geçebiliriz.

### Alıştırma

1. `go test -race` ne yakalar?  
   *Cevap: Race condition'ları — iki goroutine'in senkronizasyonsuz aynı değişkene yazmasını runtime'da yakalar.*
2. `go build ./...` neden binary üretmez?  
   *Cevap: Sadece "derlenebiliyor mu" kontrolü. Binary istersen `-o` ile çıktı yolu belirtirsin ve **tek** main paketi build etmelisin.*

---

## ✅ Hafta 1 Özeti — Bu noktada neyi bilmelisin

- [ ] Go modülü kurmak ve bağımlılık eklemek
- [ ] Standard Go Project Layout (`cmd/`, `internal/`, `pkg/`)
- [ ] Paket ve görünürlük (büyük/küçük harf kuralı)
- [ ] Struct tanımlama, value vs pointer receiver
- [ ] Pointer operatörleri (`&`, `*`), `nil` riski
- [ ] Interface'in implicit satisfaction prensibi → DI
- [ ] Hata sarmalama (`%w`) ve sentinel error pattern
- [ ] `context.Context` ile timeout/cancellation
- [ ] Goroutine, channel, `select` ve leak önleme
- [ ] `cleanenv` ile config + struct tag + `.env`
- [ ] `zerolog` ile structured log + context propagation
- [ ] HTTP server + graceful shutdown (`run() error` pattern)
- [ ] Clean Architecture: handler → service → repository → domain
- [ ] Domain modelleme: Money (int64), Currency, Wallet, Transaction
- [ ] Table-driven test + `t.Run` + `t.Setenv`
- [ ] `go build`, `vet`, `test`, `mod tidy`

## 📚 Hafta 2'ye Hazırlık

Hafta 2'de PostgreSQL bağlantısı, `pgx/v5`, `golang-migrate` ve docker-compose göreceğiz. Bunlar için bu hafta öğrendiğin:
- `context.Context` → her DB query'ye geçilir
- Repository interface → PG implementation yazacağız
- UnitOfWork → tx wrapper
- Sentinel errors → PG hatalarını domain hatasına çevirme

zorunlu. Bir kere daha gözden geçir, soruları cevapla, sonra Hafta 2'ye başlayalım.
