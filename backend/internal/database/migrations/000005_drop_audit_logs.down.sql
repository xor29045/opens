CREATE TABLE audit_logs (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at   timestamptz NOT NULL DEFAULT now(),
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    actor_role    text,
    action        text NOT NULL,
    category      text NOT NULL CHECK (category IN ('SECURITY','ADMIN','BILLING','ESTABLISHMENT','PROFILE','AUTH')),
    severity      text NOT NULL CHECK (severity IN ('INFO','WARN','CRITICAL')),
    target_type   text,
    target_id     text,
    ip_hash       text,
    user_agent    varchar(255),
    request_id    text,
    metadata      jsonb NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX audit_logs_occurred_brin ON audit_logs USING brin (occurred_at);
CREATE INDEX audit_logs_category_idx ON audit_logs (category, id DESC);
CREATE INDEX audit_logs_target_idx ON audit_logs (target_type, target_id);
CREATE INDEX audit_logs_actor_idx ON audit_logs (actor_user_id);
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_rw') THEN
        GRANT SELECT, INSERT ON audit_logs TO app_rw;
        REVOKE UPDATE, DELETE, TRUNCATE ON audit_logs FROM app_rw;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_ro') THEN
        GRANT SELECT ON audit_logs TO app_ro;
    END IF;
END
$$;
