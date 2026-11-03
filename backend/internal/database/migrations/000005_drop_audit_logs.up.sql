-- The audit trail has been removed; security relies on rate limiting,
-- account lockout and admin alerts (webhook + logs).
DROP TABLE IF EXISTS audit_logs;
