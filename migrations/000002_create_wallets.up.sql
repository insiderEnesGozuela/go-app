-- wallets: domain.Wallet ile birebir. Bir kullanıcının her para birimi için
-- en fazla bir cüzdanı olur (UNIQUE(user_id, currency)).
CREATE TABLE wallets (
    id         UUID PRIMARY KEY,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    -- balance minor units (kuruş/cent) olarak BIGINT. ASLA NUMERIC/float değil;
    -- domain Money int64 ile çalışıyor, tip eşleşmesi tam olmalı. BIGINT = int64.
    balance    BIGINT      NOT NULL DEFAULT 0,
    currency   TEXT        NOT NULL,
    -- version optimistic locking için. UPDATE ... WHERE version = $old yaparak
    -- iki eşzamanlı yazımdan birini 0-satır-etkilendi ile yakalarız.
    version    BIGINT      NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Finansal invariant'ı DB seviyesinde de kilitliyoruz: bakiye negatif olamaz.
    -- Uygulama Debit'te zaten kontrol ediyor ama "defense in depth" — bozuk bir
    -- migration ya da elle SQL bile bakiyeyi eksiye düşüremesin.
    CONSTRAINT chk_wallets_balance_nonneg CHECK (balance >= 0),
    CONSTRAINT chk_wallets_currency CHECK (currency IN ('TRY', 'USD', 'EUR')),

    -- Bir kullanıcı + para birimi kombinasyonu tek cüzdan.
    CONSTRAINT uq_wallets_user_currency UNIQUE (user_id, currency)
);

-- Bir kullanıcının tüm cüzdanlarını listelerken (GetByUserAndCurrency / ileride
-- "kullanıcının cüzdanları") user_id üzerinden ararız. UNIQUE constraint zaten
-- (user_id, currency) için index üretir ve user_id prefix'i o index'ten
-- karşılanır, bu yüzden ayrı bir user_id index'i eklemiyoruz (gereksiz yazma
-- maliyeti olurdu).
