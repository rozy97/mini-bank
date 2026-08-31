CREATE TABLE IF NOT EXISTS users (
	id bigserial PRIMARY KEY,
	name VARCHAR(100) NOT NULL,
	email VARCHAR(255) NOT NULL,
	password VARCHAR(255) NOT NULL,
	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMPTZ NULL
);

CREATE TABLE "accounts" (
  "id" bigserial PRIMARY KEY,
  "owner" bigserial NOT NULL,
  "balance" bigserial NOT NULL,
  "currency" varchar NOT NULL,
  "created_at" timestamptZ DEFAULT now(),
  "updated_at" timestamptZ DEFAULT now(),
  "deleted_at" timestamptZ DEFAULT NULL
);