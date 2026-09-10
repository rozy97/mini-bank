-- Mini Bank initial schema.
-- Money amounts are stored as BIGINT in the smallest unit of the currency
-- (IDR has no subunit in practice, so this is whole Rupiah).

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    name          VARCHAR(100) NOT NULL,
    email         VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Case-insensitive uniqueness so "a@x.com" and "A@x.com" can't both register.
CREATE UNIQUE INDEX users_email_key ON users (LOWER(email));

CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE accounts (
    id         BIGSERIAL PRIMARY KEY,
    owner_id   BIGINT NOT NULL REFERENCES users (id),
    balance    BIGINT NOT NULL DEFAULT 0 CHECK (balance >= 0),
    currency   VARCHAR(3) NOT NULL DEFAULT 'IDR',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One account per user in this demo, so a user's account can be looked up by owner_id.
CREATE UNIQUE INDEX accounts_owner_id_key ON accounts (owner_id);

CREATE TRIGGER accounts_set_updated_at
    BEFORE UPDATE ON accounts
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE transfers (
    id              BIGSERIAL PRIMARY KEY,
    from_account_id BIGINT NOT NULL REFERENCES accounts (id),
    to_account_id   BIGINT NOT NULL REFERENCES accounts (id),
    amount          BIGINT NOT NULL CHECK (amount > 0),
    description     VARCHAR(255) NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT transfers_distinct_accounts CHECK (from_account_id <> to_account_id)
);

CREATE INDEX transfers_from_account_id_idx ON transfers (from_account_id);
CREATE INDEX transfers_to_account_id_idx ON transfers (to_account_id);

-- Append-only ledger: every balance change is a row here. This is both the
-- audit trail and the source for the paginated balance history endpoint.
CREATE TABLE entries (
    id            BIGSERIAL PRIMARY KEY,
    account_id    BIGINT NOT NULL REFERENCES accounts (id),
    transfer_id   BIGINT REFERENCES transfers (id),
    amount        BIGINT NOT NULL, -- negative for a debit, positive for a credit
    balance_after BIGINT NOT NULL CHECK (balance_after >= 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX entries_account_id_id_idx ON entries (account_id, id DESC);

-- Backs idempotent POST /transfers: one stored response per (user, key).
-- request_hash detects the same key being replayed with a different payload.
CREATE TABLE idempotency_keys (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users (id),
    idempotency_key VARCHAR(255) NOT NULL,
    request_hash    CHAR(64) NOT NULL,
    response_status SMALLINT,
    response_body   JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idempotency_keys_user_id_key_idx ON idempotency_keys (user_id, idempotency_key);
