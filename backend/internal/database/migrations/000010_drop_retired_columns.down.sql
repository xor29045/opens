-- The columns come back empty; their former values are not restored.
ALTER TABLE subscription_plans
    ADD COLUMN max_pending_profiles int NOT NULL DEFAULT 20 CHECK (max_pending_profiles >= 0);

ALTER TABLE professional_profiles
    ADD COLUMN birth_date_attested date,
    ADD COLUMN employment_reference text,
    ADD COLUMN attested_at timestamptz,
    ADD COLUMN attested_by uuid REFERENCES users(id) ON DELETE SET NULL,
    ADD CHECK ((attested_at IS NULL) = (birth_date_attested IS NULL));
CREATE INDEX profiles_pub_birth_idx ON professional_profiles (is_published, birth_date_attested);

ALTER TABLE establishments
    ADD COLUMN company_number text UNIQUE,
    ADD COLUMN approval_number text UNIQUE,
    ADD COLUMN verification_status text NOT NULL DEFAULT 'DRAFT' CHECK (verification_status IN ('DRAFT','PENDING','VERIFIED','REJECTED')),
    ADD COLUMN verification_note text,
    ADD COLUMN submitted_at timestamptz,
    ADD COLUMN verified_at timestamptz,
    ADD COLUMN verified_by uuid REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX establishments_verification_idx ON establishments (verification_status);
