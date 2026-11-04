-- Age shown on the profile and used by the "Découvrir" filters. Declared by
-- the house or the professional (no verification, ADR-021); adults only.
ALTER TABLE professional_profiles
    ADD COLUMN age smallint CHECK (age IS NULL OR age BETWEEN 18 AND 99);
CREATE INDEX profiles_pub_age_idx ON professional_profiles (is_published, age);
