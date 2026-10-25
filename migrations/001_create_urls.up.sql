CREATE TABLE urls (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code TEXT NOT NULL,
    original_url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ,

    CONSTRAINT urls_code_unique UNIQUE (code),
    CONSTRAINT urls_code_not_empty CHECK (code <> ''),
    CONSTRAINT urls_original_url_not_empty CHECK (original_url <> '')
);