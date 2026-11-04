-- Deleted client accounts are not restored; only the tables come back.
CREATE TABLE IF NOT EXISTS favorites (
    client_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    profile_id     uuid NOT NULL REFERENCES professional_profiles(id) ON DELETE CASCADE,
    created_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (client_user_id, profile_id)
);
CREATE INDEX IF NOT EXISTS favorites_profile_idx ON favorites (profile_id);

CREATE TABLE IF NOT EXISTS client_dislikes (
    client_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    profile_id     uuid NOT NULL REFERENCES professional_profiles(id) ON DELETE CASCADE,
    created_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (client_user_id, profile_id)
);
CREATE INDEX IF NOT EXISTS client_dislikes_profile_idx ON client_dislikes (profile_id);
