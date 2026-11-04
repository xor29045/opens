-- ADR-022: professionals no longer self-register; houses create them.
-- Registrations still waiting for acceptance become drafts of their house.
UPDATE professional_profiles SET status = 'DRAFT' WHERE status = 'PENDING_APPROVAL';
