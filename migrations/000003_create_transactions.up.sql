-- transactions: değiştirilemez denetim kaydı (immutable audit log).
-- domain.Transaction ile birebir. source/target wallet nullable çünkü üç tipi
-- tek tabloda temsil ediyoruz:
--   DEPOSIT    : source = NULL,     target = wallet
--   WITHDRAWAL : source = wallet,   target = NULL
--   TRANSFER   : source = walletA,  target = walletB
CREATE TABLE transactions (
    id               UUID PRIMARY KEY,
    type             TEXT        NOT NULL,
    status           TEXT        NOT NULL,
    source_wallet_id UUID        REFERENCES wallets (id) ON DELETE RESTRICT,
    target_wallet_id UUID        REFERENCES wallets (id) ON DELETE RESTRICT,
    amount           BIGINT      NOT NULL,
    currency         TEXT        NOT NULL,
    idempotency_key  TEXT        NOT NULL,
    reference        TEXT        NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_tx_type CHECK (type IN ('DEPOSIT', 'WITHDRAWAL', 'TRANSFER')),
    CONSTRAINT chk_tx_status CHECK (status IN ('PENDING', 'COMPLETED', 'FAILED', 'REVERSED')),
    CONSTRAINT chk_tx_amount_positive CHECK (amount > 0),
    CONSTRAINT chk_tx_currency CHECK (currency IN ('TRY', 'USD', 'EUR')),

    -- Her tipin doğru cüzdan kombinasyonuna sahip olmasını DB garanti etsin.
    -- Uygulama (domain.NewDeposit/NewWithdrawal/NewTransfer) zaten zorluyor;
    -- bu CHECK bozuk veriyi tabloya hiç sokmaz.
    CONSTRAINT chk_tx_wallets CHECK (
        (type = 'DEPOSIT'    AND source_wallet_id IS NULL     AND target_wallet_id IS NOT NULL) OR
        (type = 'WITHDRAWAL' AND source_wallet_id IS NOT NULL AND target_wallet_id IS NULL) OR
        (type = 'TRANSFER'   AND source_wallet_id IS NOT NULL AND target_wallet_id IS NOT NULL
                             AND source_wallet_id <> target_wallet_id)
    )
);

-- Idempotency: aynı istemci aynı isteği iki kez gönderirse (network retry) ikinci
-- INSERT bu unique constraint'e takılır → 23505. Repository bunu yakalayıp
-- domain.ErrAlreadyExists'e çevirecek. Boş key'e izin yok (domain zaten reddediyor).
CREATE UNIQUE INDEX idx_tx_idempotency_key ON transactions (idempotency_key);

-- Bir cüzdanın işlem geçmişini (account statement) çekerken kaynak ya da hedef
-- olduğu kayıtları zaman sırasına göre tararız. created_at DESC ekledik çünkü
-- "son işlemler" sorgusu en sık olan; index'in sırası sorguyu karşılar.
CREATE INDEX idx_tx_source_wallet ON transactions (source_wallet_id, created_at DESC);
CREATE INDEX idx_tx_target_wallet ON transactions (target_wallet_id, created_at DESC);
