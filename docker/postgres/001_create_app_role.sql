\set ON_ERROR_STOP on
\getenv app_password APP_DATABASE_PASSWORD

CREATE ROLE url_shortener_dev
    LOGIN
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    PASSWORD :'app_password';

ALTER DATABASE url_shortener OWNER TO url_shortener_dev;