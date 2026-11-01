CREATE TABLE public.url_daily_clicks (
    url_id BIGINT NOT NULL,
    click_date DATE NOT NULL,
    click_count BIGINT NOT NULL DEFAULT 1,

    CONSTRAINT url_daily_clicks_pkey
        PRIMARY KEY (url_id, click_date),

    CONSTRAINT url_daily_clicks_url_id_fkey
        FOREIGN KEY (url_id)
        REFERENCES public.urls (id)
        ON DELETE CASCADE,

    CONSTRAINT url_daily_clicks_count_positive
        CHECK (click_count > 0)
);