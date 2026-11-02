DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_rw') THEN
        REVOKE ALL ON ALL TABLES IN SCHEMA public FROM app_rw;
        REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM app_rw;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_ro') THEN
        REVOKE ALL ON ALL TABLES IN SCHEMA public FROM app_ro;
    END IF;
END
$$;
