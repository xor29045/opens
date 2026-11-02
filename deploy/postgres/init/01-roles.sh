#!/bin/sh
# Creates the least-privilege roles (§13.2) and the application database.
#   app_migrator : owner of the schema, used by `ops migrate` only
#   app_rw       : used by the API and the worker (no UPDATE/DELETE on audit_logs)
#   app_ro       : read-only (reporting, support)
set -eu
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<SQL
CREATE ROLE app_migrator LOGIN PASSWORD '${APP_MIGRATOR_PASSWORD}';
CREATE ROLE app_rw LOGIN PASSWORD '${APP_RW_PASSWORD}';
CREATE ROLE app_ro LOGIN PASSWORD '${APP_RO_PASSWORD}';
CREATE DATABASE ${APP_DB_NAME} OWNER app_migrator;
SQL
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "${APP_DB_NAME}" <<SQL
ALTER SCHEMA public OWNER TO app_migrator;
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_rw;
ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO app_rw;
ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public GRANT SELECT ON TABLES TO app_ro;
ALTER ROLE app_rw SET statement_timeout = '5s';
ALTER ROLE app_ro SET statement_timeout = '5s';
SQL
