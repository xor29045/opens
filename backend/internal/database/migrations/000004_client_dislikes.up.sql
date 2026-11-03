-- Profiles a client chose to pass on in the "Découvrir" deck. Likes are
-- stored as favorites; dislikes only hide a profile from the deck.
CREATE TABLE client_dislikes (
    client_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    profile_id     uuid NOT NULL REFERENCES professional_profiles(id) ON DELETE CASCADE,
    created_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (client_user_id, profile_id)
);
CREATE INDEX client_dislikes_profile_idx ON client_dislikes (profile_id);

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_rw') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON client_dislikes TO app_rw;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_ro') THEN
        GRANT SELECT ON client_dislikes TO app_ro;
    END IF;
END
$$;
