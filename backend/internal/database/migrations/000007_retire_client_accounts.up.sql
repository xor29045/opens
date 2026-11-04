-- ADR-023: no client accounts anymore. Visitors browse anonymously; their
-- likes and passes in the "Découvrir" deck stay in their browser.
DELETE FROM users u USING user_roles ur
 WHERE ur.user_id = u.id AND ur.role_id = (SELECT id FROM roles WHERE code = 'CLIENT');

DROP TABLE IF EXISTS client_dislikes;
DROP TABLE IF EXISTS favorites;

-- "Clients only" contacts have no audience left: they are hidden rather than
-- made public without the house's consent.
UPDATE profile_contacts SET visibility = 'HIDDEN' WHERE visibility = 'CLIENTS';
UPDATE establishment_contacts SET visibility = 'HIDDEN' WHERE visibility = 'CLIENTS';
