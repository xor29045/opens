-- Back to the Belgian reference data. Establishments located in France lose
-- their location.
UPDATE establishments SET region_id = NULL, city_id = NULL
 WHERE region_id IN (SELECT id FROM regions WHERE country_code = 'FR')
    OR city_id IN (SELECT id FROM regions WHERE country_code = 'FR');
DELETE FROM regions WHERE country_code = 'FR' AND kind = 'CITY';
DELETE FROM regions WHERE country_code = 'FR';
ALTER TABLE regions ALTER COLUMN country_code SET DEFAULT 'BE';

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
