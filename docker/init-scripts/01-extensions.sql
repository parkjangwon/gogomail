-- Pre-install Postgres extensions that tests rely on.
-- This runs at database creation time, before any Go migration runs.
-- Extensions are installed in the public schema so they are visible
-- regardless of the search_path used by individual test schemas.

CREATE EXTENSION IF NOT EXISTS pg_trgm SCHEMA public;
