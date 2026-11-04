-- Dead schema of retired features: establishment verification and legal
-- numbers (ADR-017), age/contract attestation (ADR-021), pending
-- registrations quota (ADR-022). Indexes and CHECK constraints on these
-- columns go with them.
ALTER TABLE establishments
    DROP COLUMN IF EXISTS company_number,
    DROP COLUMN IF EXISTS approval_number,
    DROP COLUMN IF EXISTS verification_status,
    DROP COLUMN IF EXISTS verification_note,
    DROP COLUMN IF EXISTS submitted_at,
    DROP COLUMN IF EXISTS verified_at,
    DROP COLUMN IF EXISTS verified_by;

ALTER TABLE professional_profiles
    DROP COLUMN IF EXISTS birth_date_attested,
    DROP COLUMN IF EXISTS employment_reference,
    DROP COLUMN IF EXISTS attested_at,
    DROP COLUMN IF EXISTS attested_by;

ALTER TABLE subscription_plans DROP COLUMN IF EXISTS max_pending_profiles;
