-- Least-privilege grants (§13.2). Roles are created by the infrastructure
-- (deploy/postgres/init); grants are applied only when the roles exist so
-- that migrations also run on a bare database (tests, dev).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_rw') THEN
        GRANT USAGE ON SCHEMA public TO app_rw;
        GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_rw;
        GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO app_rw;
        -- audit_logs is append-only for the application.
        REVOKE UPDATE, DELETE, TRUNCATE ON audit_logs FROM app_rw;
        REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON schema_migrations FROM app_rw;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_ro') THEN
        GRANT USAGE ON SCHEMA public TO app_ro;
        GRANT SELECT ON ALL TABLES IN SCHEMA public TO app_ro;
    END IF;
END
$$;
