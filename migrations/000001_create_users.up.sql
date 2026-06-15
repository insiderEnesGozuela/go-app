-- users: kimlik tablosu. domain.User ile birebir.
-- id'yi uygulama tarafında (uuid.New()) üretiyoruz; gen_random_uuid() yerine
-- DEFAULT koymuyoruz çünkü ID domain entity oluşturulurken zaten set ediliyor
-- ve testlerde deterministik olması işimize geliyor. Yine de DEFAULT bırakmak
-- doğrudan SQL insert'lerine karşı güvenlik ağı olur.
CREATE TABLE users (
    id            UUID PRIMARY KEY,
    email         TEXT        NOT NULL,
    password_hash TEXT        NOT NULL DEFAULT '',
    full_name     TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- E-posta ile login/lookup yapacağız → benzersiz + hızlı arama için unique index.
-- domain.NewUser e-postayı lower-case'e çektiği için burada da lower beklentisi
-- var; case-insensitive benzersizlik uygulama katmanında garanti ediliyor.
CREATE UNIQUE INDEX idx_users_email ON users (email);
