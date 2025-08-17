CREATE TABLE transactions (
  transaction_id  TEXT PRIMARY KEY,
  user_id         BIGINT NOT NULL REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX transactions_user_id_idx
    ON transactions (user_id);

CREATE INDEX transactions_user_id_created_at_desc_idx
    ON transactions (user_id, created_at DESC);
