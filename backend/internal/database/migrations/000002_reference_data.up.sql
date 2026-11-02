-- Reference data (§8.2): roles, initial plan (RM-30), Belgian regions and cities, categories.
INSERT INTO roles (id, code, description) VALUES
 (1, 'ADMIN', 'Administrateur de la plateforme (bootstrap uniquement)'),
 (2, 'MANAGER', 'Compte gérant d''un établissement'),
 (3, 'PROFESSIONAL', 'Professionnel·le salarié·e d''un établissement'),
 (4, 'CLIENT', 'Client inscrit');

INSERT INTO subscription_plans (code, name, amount_cents, currency, interval_days, trial_days, grace_days, max_pending_profiles, max_active_profiles)
VALUES ('establishment-monthly', 'Établissement — mensuel', 50000, 'EUR', 30, 0, 3, 20, NULL);

INSERT INTO regions (name, slug, kind) VALUES ('Bruxelles-Capitale', 'bruxelles-capitale', 'REGION');
INSERT INTO regions (name, slug, kind, parent_id) VALUES
 ('Bruxelles', 'bruxelles', 'CITY', (SELECT id FROM regions WHERE slug = 'bruxelles-capitale')),
 ('Ixelles', 'ixelles', 'CITY', (SELECT id FROM regions WHERE slug = 'bruxelles-capitale')),
 ('Saint-Gilles', 'saint-gilles', 'CITY', (SELECT id FROM regions WHERE slug = 'bruxelles-capitale')),
 ('Schaerbeek', 'schaerbeek', 'CITY', (SELECT id FROM regions WHERE slug = 'bruxelles-capitale')),
 ('Anderlecht', 'anderlecht', 'CITY', (SELECT id FROM regions WHERE slug = 'bruxelles-capitale')),
 ('Etterbeek', 'etterbeek', 'CITY', (SELECT id FROM regions WHERE slug = 'bruxelles-capitale')),
 ('Saint-Josse-ten-Noode', 'saint-josse-ten-noode', 'CITY', (SELECT id FROM regions WHERE slug = 'bruxelles-capitale')),
 ('Uccle', 'uccle', 'CITY', (SELECT id FROM regions WHERE slug = 'bruxelles-capitale')),
 ('Molenbeek-Saint-Jean', 'molenbeek-saint-jean', 'CITY', (SELECT id FROM regions WHERE slug = 'bruxelles-capitale'));
INSERT INTO regions (name, slug, kind) VALUES ('Flandre', 'flandre', 'REGION');
INSERT INTO regions (name, slug, kind, parent_id) VALUES
 ('Anvers', 'anvers', 'CITY', (SELECT id FROM regions WHERE slug = 'flandre')),
 ('Gand', 'gand', 'CITY', (SELECT id FROM regions WHERE slug = 'flandre')),
 ('Bruges', 'bruges', 'CITY', (SELECT id FROM regions WHERE slug = 'flandre')),
 ('Louvain', 'louvain', 'CITY', (SELECT id FROM regions WHERE slug = 'flandre')),
 ('Malines', 'malines', 'CITY', (SELECT id FROM regions WHERE slug = 'flandre')),
 ('Hasselt', 'hasselt', 'CITY', (SELECT id FROM regions WHERE slug = 'flandre')),
 ('Courtrai', 'courtrai', 'CITY', (SELECT id FROM regions WHERE slug = 'flandre')),
 ('Ostende', 'ostende', 'CITY', (SELECT id FROM regions WHERE slug = 'flandre')),
 ('Alost', 'alost', 'CITY', (SELECT id FROM regions WHERE slug = 'flandre')),
 ('Genk', 'genk', 'CITY', (SELECT id FROM regions WHERE slug = 'flandre')),
 ('Saint-Nicolas', 'saint-nicolas', 'CITY', (SELECT id FROM regions WHERE slug = 'flandre'));
INSERT INTO regions (name, slug, kind) VALUES ('Wallonie', 'wallonie', 'REGION');
INSERT INTO regions (name, slug, kind, parent_id) VALUES
 ('Liège', 'liege', 'CITY', (SELECT id FROM regions WHERE slug = 'wallonie')),
 ('Charleroi', 'charleroi', 'CITY', (SELECT id FROM regions WHERE slug = 'wallonie')),
 ('Namur', 'namur', 'CITY', (SELECT id FROM regions WHERE slug = 'wallonie')),
 ('Mons', 'mons', 'CITY', (SELECT id FROM regions WHERE slug = 'wallonie')),
 ('La Louvière', 'la-louviere', 'CITY', (SELECT id FROM regions WHERE slug = 'wallonie')),
 ('Tournai', 'tournai', 'CITY', (SELECT id FROM regions WHERE slug = 'wallonie')),
 ('Verviers', 'verviers', 'CITY', (SELECT id FROM regions WHERE slug = 'wallonie')),
 ('Seraing', 'seraing', 'CITY', (SELECT id FROM regions WHERE slug = 'wallonie')),
 ('Wavre', 'wavre', 'CITY', (SELECT id FROM regions WHERE slug = 'wallonie')),
 ('Arlon', 'arlon', 'CITY', (SELECT id FROM regions WHERE slug = 'wallonie')),
 ('Mouscron', 'mouscron', 'CITY', (SELECT id FROM regions WHERE slug = 'wallonie'));

INSERT INTO categories (slug, name, position) VALUES ('prestations', 'Prestations', 0);
INSERT INTO categories (slug, name, position, parent_id) VALUES
 ('massage', 'Massage', 0, (SELECT id FROM categories WHERE slug = 'prestations')),
 ('massage-tantrique', 'Massage tantrique', 1, (SELECT id FROM categories WHERE slug = 'prestations')),
 ('accompagnement', 'Accompagnement', 2, (SELECT id FROM categories WHERE slug = 'prestations')),
 ('domination', 'Domination', 3, (SELECT id FROM categories WHERE slug = 'prestations')),
 ('soumission', 'Soumission', 4, (SELECT id FROM categories WHERE slug = 'prestations')),
 ('fetichisme', 'Fétichisme', 5, (SELECT id FROM categories WHERE slug = 'prestations')),
 ('jeux-de-role', 'Jeux de rôle', 6, (SELECT id FROM categories WHERE slug = 'prestations')),
 ('duo', 'Duo', 7, (SELECT id FROM categories WHERE slug = 'prestations')),
 ('couples', 'Couples', 8, (SELECT id FROM categories WHERE slug = 'prestations'));
INSERT INTO categories (slug, name, position) VALUES ('lieu', 'Lieu', 1);
INSERT INTO categories (slug, name, position, parent_id) VALUES
 ('sur-place', 'Sur place', 0, (SELECT id FROM categories WHERE slug = 'lieu')),
 ('deplacement', 'Déplacement', 1, (SELECT id FROM categories WHERE slug = 'lieu'));
