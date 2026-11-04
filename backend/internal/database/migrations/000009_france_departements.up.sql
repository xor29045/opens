-- France instead of Belgium: the 18 French regions and their 101
-- départements replace the Belgian regions and cities. Départements use the
-- CITY kind (second level of the tree); their name carries the code, e.g.
-- "Paris (75)". Establishments located in Belgium lose their location and
-- must pick a département again.
UPDATE establishments SET region_id = NULL, city_id = NULL
 WHERE region_id IN (SELECT id FROM regions WHERE country_code = 'BE')
    OR city_id IN (SELECT id FROM regions WHERE country_code = 'BE');
DELETE FROM regions WHERE country_code = 'BE' AND kind = 'CITY';
DELETE FROM regions WHERE country_code = 'BE';
ALTER TABLE regions ALTER COLUMN country_code SET DEFAULT 'FR';

INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'Auvergne-Rhône-Alpes', 'auvergne-rhone-alpes', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'Ain (01)', '01-ain', 'CITY', (SELECT id FROM regions WHERE slug = 'auvergne-rhone-alpes')),
 ('FR', 'Allier (03)', '03-allier', 'CITY', (SELECT id FROM regions WHERE slug = 'auvergne-rhone-alpes')),
 ('FR', 'Ardèche (07)', '07-ardeche', 'CITY', (SELECT id FROM regions WHERE slug = 'auvergne-rhone-alpes')),
 ('FR', 'Cantal (15)', '15-cantal', 'CITY', (SELECT id FROM regions WHERE slug = 'auvergne-rhone-alpes')),
 ('FR', 'Drôme (26)', '26-drome', 'CITY', (SELECT id FROM regions WHERE slug = 'auvergne-rhone-alpes')),
 ('FR', 'Isère (38)', '38-isere', 'CITY', (SELECT id FROM regions WHERE slug = 'auvergne-rhone-alpes')),
 ('FR', 'Loire (42)', '42-loire', 'CITY', (SELECT id FROM regions WHERE slug = 'auvergne-rhone-alpes')),
 ('FR', 'Haute-Loire (43)', '43-haute-loire', 'CITY', (SELECT id FROM regions WHERE slug = 'auvergne-rhone-alpes')),
 ('FR', 'Puy-de-Dôme (63)', '63-puy-de-dome', 'CITY', (SELECT id FROM regions WHERE slug = 'auvergne-rhone-alpes')),
 ('FR', 'Rhône (69)', '69-rhone', 'CITY', (SELECT id FROM regions WHERE slug = 'auvergne-rhone-alpes')),
 ('FR', 'Savoie (73)', '73-savoie', 'CITY', (SELECT id FROM regions WHERE slug = 'auvergne-rhone-alpes')),
 ('FR', 'Haute-Savoie (74)', '74-haute-savoie', 'CITY', (SELECT id FROM regions WHERE slug = 'auvergne-rhone-alpes'));
INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'Bourgogne-Franche-Comté', 'bourgogne-franche-comte', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'Côte-d''Or (21)', '21-cote-d-or', 'CITY', (SELECT id FROM regions WHERE slug = 'bourgogne-franche-comte')),
 ('FR', 'Doubs (25)', '25-doubs', 'CITY', (SELECT id FROM regions WHERE slug = 'bourgogne-franche-comte')),
 ('FR', 'Jura (39)', '39-jura', 'CITY', (SELECT id FROM regions WHERE slug = 'bourgogne-franche-comte')),
 ('FR', 'Nièvre (58)', '58-nievre', 'CITY', (SELECT id FROM regions WHERE slug = 'bourgogne-franche-comte')),
 ('FR', 'Haute-Saône (70)', '70-haute-saone', 'CITY', (SELECT id FROM regions WHERE slug = 'bourgogne-franche-comte')),
 ('FR', 'Saône-et-Loire (71)', '71-saone-et-loire', 'CITY', (SELECT id FROM regions WHERE slug = 'bourgogne-franche-comte')),
 ('FR', 'Yonne (89)', '89-yonne', 'CITY', (SELECT id FROM regions WHERE slug = 'bourgogne-franche-comte')),
 ('FR', 'Territoire de Belfort (90)', '90-territoire-de-belfort', 'CITY', (SELECT id FROM regions WHERE slug = 'bourgogne-franche-comte'));
INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'Bretagne', 'bretagne', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'Côtes-d''Armor (22)', '22-cotes-d-armor', 'CITY', (SELECT id FROM regions WHERE slug = 'bretagne')),
 ('FR', 'Finistère (29)', '29-finistere', 'CITY', (SELECT id FROM regions WHERE slug = 'bretagne')),
 ('FR', 'Ille-et-Vilaine (35)', '35-ille-et-vilaine', 'CITY', (SELECT id FROM regions WHERE slug = 'bretagne')),
 ('FR', 'Morbihan (56)', '56-morbihan', 'CITY', (SELECT id FROM regions WHERE slug = 'bretagne'));
INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'Centre-Val de Loire', 'centre-val-de-loire', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'Cher (18)', '18-cher', 'CITY', (SELECT id FROM regions WHERE slug = 'centre-val-de-loire')),
 ('FR', 'Eure-et-Loir (28)', '28-eure-et-loir', 'CITY', (SELECT id FROM regions WHERE slug = 'centre-val-de-loire')),
 ('FR', 'Indre (36)', '36-indre', 'CITY', (SELECT id FROM regions WHERE slug = 'centre-val-de-loire')),
 ('FR', 'Indre-et-Loire (37)', '37-indre-et-loire', 'CITY', (SELECT id FROM regions WHERE slug = 'centre-val-de-loire')),
 ('FR', 'Loir-et-Cher (41)', '41-loir-et-cher', 'CITY', (SELECT id FROM regions WHERE slug = 'centre-val-de-loire')),
 ('FR', 'Loiret (45)', '45-loiret', 'CITY', (SELECT id FROM regions WHERE slug = 'centre-val-de-loire'));
INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'Corse', 'corse', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'Corse-du-Sud (2A)', '2a-corse-du-sud', 'CITY', (SELECT id FROM regions WHERE slug = 'corse')),
 ('FR', 'Haute-Corse (2B)', '2b-haute-corse', 'CITY', (SELECT id FROM regions WHERE slug = 'corse'));
INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'Grand Est', 'grand-est', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'Ardennes (08)', '08-ardennes', 'CITY', (SELECT id FROM regions WHERE slug = 'grand-est')),
 ('FR', 'Aube (10)', '10-aube', 'CITY', (SELECT id FROM regions WHERE slug = 'grand-est')),
 ('FR', 'Marne (51)', '51-marne', 'CITY', (SELECT id FROM regions WHERE slug = 'grand-est')),
 ('FR', 'Haute-Marne (52)', '52-haute-marne', 'CITY', (SELECT id FROM regions WHERE slug = 'grand-est')),
 ('FR', 'Meurthe-et-Moselle (54)', '54-meurthe-et-moselle', 'CITY', (SELECT id FROM regions WHERE slug = 'grand-est')),
 ('FR', 'Meuse (55)', '55-meuse', 'CITY', (SELECT id FROM regions WHERE slug = 'grand-est')),
 ('FR', 'Moselle (57)', '57-moselle', 'CITY', (SELECT id FROM regions WHERE slug = 'grand-est')),
 ('FR', 'Bas-Rhin (67)', '67-bas-rhin', 'CITY', (SELECT id FROM regions WHERE slug = 'grand-est')),
 ('FR', 'Haut-Rhin (68)', '68-haut-rhin', 'CITY', (SELECT id FROM regions WHERE slug = 'grand-est')),
 ('FR', 'Vosges (88)', '88-vosges', 'CITY', (SELECT id FROM regions WHERE slug = 'grand-est'));
INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'Hauts-de-France', 'hauts-de-france', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'Aisne (02)', '02-aisne', 'CITY', (SELECT id FROM regions WHERE slug = 'hauts-de-france')),
 ('FR', 'Nord (59)', '59-nord', 'CITY', (SELECT id FROM regions WHERE slug = 'hauts-de-france')),
 ('FR', 'Oise (60)', '60-oise', 'CITY', (SELECT id FROM regions WHERE slug = 'hauts-de-france')),
 ('FR', 'Pas-de-Calais (62)', '62-pas-de-calais', 'CITY', (SELECT id FROM regions WHERE slug = 'hauts-de-france')),
 ('FR', 'Somme (80)', '80-somme', 'CITY', (SELECT id FROM regions WHERE slug = 'hauts-de-france'));
INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'Île-de-France', 'ile-de-france', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'Paris (75)', '75-paris', 'CITY', (SELECT id FROM regions WHERE slug = 'ile-de-france')),
 ('FR', 'Seine-et-Marne (77)', '77-seine-et-marne', 'CITY', (SELECT id FROM regions WHERE slug = 'ile-de-france')),
 ('FR', 'Yvelines (78)', '78-yvelines', 'CITY', (SELECT id FROM regions WHERE slug = 'ile-de-france')),
 ('FR', 'Essonne (91)', '91-essonne', 'CITY', (SELECT id FROM regions WHERE slug = 'ile-de-france')),
 ('FR', 'Hauts-de-Seine (92)', '92-hauts-de-seine', 'CITY', (SELECT id FROM regions WHERE slug = 'ile-de-france')),
 ('FR', 'Seine-Saint-Denis (93)', '93-seine-saint-denis', 'CITY', (SELECT id FROM regions WHERE slug = 'ile-de-france')),
 ('FR', 'Val-de-Marne (94)', '94-val-de-marne', 'CITY', (SELECT id FROM regions WHERE slug = 'ile-de-france')),
 ('FR', 'Val-d''Oise (95)', '95-val-d-oise', 'CITY', (SELECT id FROM regions WHERE slug = 'ile-de-france'));
INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'Normandie', 'normandie', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'Calvados (14)', '14-calvados', 'CITY', (SELECT id FROM regions WHERE slug = 'normandie')),
 ('FR', 'Eure (27)', '27-eure', 'CITY', (SELECT id FROM regions WHERE slug = 'normandie')),
 ('FR', 'Manche (50)', '50-manche', 'CITY', (SELECT id FROM regions WHERE slug = 'normandie')),
 ('FR', 'Orne (61)', '61-orne', 'CITY', (SELECT id FROM regions WHERE slug = 'normandie')),
 ('FR', 'Seine-Maritime (76)', '76-seine-maritime', 'CITY', (SELECT id FROM regions WHERE slug = 'normandie'));
INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'Nouvelle-Aquitaine', 'nouvelle-aquitaine', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'Charente (16)', '16-charente', 'CITY', (SELECT id FROM regions WHERE slug = 'nouvelle-aquitaine')),
 ('FR', 'Charente-Maritime (17)', '17-charente-maritime', 'CITY', (SELECT id FROM regions WHERE slug = 'nouvelle-aquitaine')),
 ('FR', 'Corrèze (19)', '19-correze', 'CITY', (SELECT id FROM regions WHERE slug = 'nouvelle-aquitaine')),
 ('FR', 'Creuse (23)', '23-creuse', 'CITY', (SELECT id FROM regions WHERE slug = 'nouvelle-aquitaine')),
 ('FR', 'Dordogne (24)', '24-dordogne', 'CITY', (SELECT id FROM regions WHERE slug = 'nouvelle-aquitaine')),
 ('FR', 'Gironde (33)', '33-gironde', 'CITY', (SELECT id FROM regions WHERE slug = 'nouvelle-aquitaine')),
 ('FR', 'Landes (40)', '40-landes', 'CITY', (SELECT id FROM regions WHERE slug = 'nouvelle-aquitaine')),
 ('FR', 'Lot-et-Garonne (47)', '47-lot-et-garonne', 'CITY', (SELECT id FROM regions WHERE slug = 'nouvelle-aquitaine')),
 ('FR', 'Pyrénées-Atlantiques (64)', '64-pyrenees-atlantiques', 'CITY', (SELECT id FROM regions WHERE slug = 'nouvelle-aquitaine')),
 ('FR', 'Deux-Sèvres (79)', '79-deux-sevres', 'CITY', (SELECT id FROM regions WHERE slug = 'nouvelle-aquitaine')),
 ('FR', 'Vienne (86)', '86-vienne', 'CITY', (SELECT id FROM regions WHERE slug = 'nouvelle-aquitaine')),
 ('FR', 'Haute-Vienne (87)', '87-haute-vienne', 'CITY', (SELECT id FROM regions WHERE slug = 'nouvelle-aquitaine'));
INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'Occitanie', 'occitanie', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'Ariège (09)', '09-ariege', 'CITY', (SELECT id FROM regions WHERE slug = 'occitanie')),
 ('FR', 'Aude (11)', '11-aude', 'CITY', (SELECT id FROM regions WHERE slug = 'occitanie')),
 ('FR', 'Aveyron (12)', '12-aveyron', 'CITY', (SELECT id FROM regions WHERE slug = 'occitanie')),
 ('FR', 'Gard (30)', '30-gard', 'CITY', (SELECT id FROM regions WHERE slug = 'occitanie')),
 ('FR', 'Haute-Garonne (31)', '31-haute-garonne', 'CITY', (SELECT id FROM regions WHERE slug = 'occitanie')),
 ('FR', 'Gers (32)', '32-gers', 'CITY', (SELECT id FROM regions WHERE slug = 'occitanie')),
 ('FR', 'Hérault (34)', '34-herault', 'CITY', (SELECT id FROM regions WHERE slug = 'occitanie')),
 ('FR', 'Lot (46)', '46-lot', 'CITY', (SELECT id FROM regions WHERE slug = 'occitanie')),
 ('FR', 'Lozère (48)', '48-lozere', 'CITY', (SELECT id FROM regions WHERE slug = 'occitanie')),
 ('FR', 'Hautes-Pyrénées (65)', '65-hautes-pyrenees', 'CITY', (SELECT id FROM regions WHERE slug = 'occitanie')),
 ('FR', 'Pyrénées-Orientales (66)', '66-pyrenees-orientales', 'CITY', (SELECT id FROM regions WHERE slug = 'occitanie')),
 ('FR', 'Tarn (81)', '81-tarn', 'CITY', (SELECT id FROM regions WHERE slug = 'occitanie')),
 ('FR', 'Tarn-et-Garonne (82)', '82-tarn-et-garonne', 'CITY', (SELECT id FROM regions WHERE slug = 'occitanie'));
INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'Pays de la Loire', 'pays-de-la-loire', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'Loire-Atlantique (44)', '44-loire-atlantique', 'CITY', (SELECT id FROM regions WHERE slug = 'pays-de-la-loire')),
 ('FR', 'Maine-et-Loire (49)', '49-maine-et-loire', 'CITY', (SELECT id FROM regions WHERE slug = 'pays-de-la-loire')),
 ('FR', 'Mayenne (53)', '53-mayenne', 'CITY', (SELECT id FROM regions WHERE slug = 'pays-de-la-loire')),
 ('FR', 'Sarthe (72)', '72-sarthe', 'CITY', (SELECT id FROM regions WHERE slug = 'pays-de-la-loire')),
 ('FR', 'Vendée (85)', '85-vendee', 'CITY', (SELECT id FROM regions WHERE slug = 'pays-de-la-loire'));
INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'Provence-Alpes-Côte d''Azur', 'provence-alpes-cote-d-azur', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'Alpes-de-Haute-Provence (04)', '04-alpes-de-haute-provence', 'CITY', (SELECT id FROM regions WHERE slug = 'provence-alpes-cote-d-azur')),
 ('FR', 'Hautes-Alpes (05)', '05-hautes-alpes', 'CITY', (SELECT id FROM regions WHERE slug = 'provence-alpes-cote-d-azur')),
 ('FR', 'Alpes-Maritimes (06)', '06-alpes-maritimes', 'CITY', (SELECT id FROM regions WHERE slug = 'provence-alpes-cote-d-azur')),
 ('FR', 'Bouches-du-Rhône (13)', '13-bouches-du-rhone', 'CITY', (SELECT id FROM regions WHERE slug = 'provence-alpes-cote-d-azur')),
 ('FR', 'Var (83)', '83-var', 'CITY', (SELECT id FROM regions WHERE slug = 'provence-alpes-cote-d-azur')),
 ('FR', 'Vaucluse (84)', '84-vaucluse', 'CITY', (SELECT id FROM regions WHERE slug = 'provence-alpes-cote-d-azur'));
INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'Guadeloupe', 'region-guadeloupe', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'Guadeloupe (971)', '971-guadeloupe', 'CITY', (SELECT id FROM regions WHERE slug = 'region-guadeloupe'));
INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'Martinique', 'region-martinique', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'Martinique (972)', '972-martinique', 'CITY', (SELECT id FROM regions WHERE slug = 'region-martinique'));
INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'Guyane', 'region-guyane', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'Guyane (973)', '973-guyane', 'CITY', (SELECT id FROM regions WHERE slug = 'region-guyane'));
INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'La Réunion', 'region-la-reunion', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'La Réunion (974)', '974-la-reunion', 'CITY', (SELECT id FROM regions WHERE slug = 'region-la-reunion'));
INSERT INTO regions (country_code, name, slug, kind) VALUES ('FR', 'Mayotte', 'region-mayotte', 'REGION');
INSERT INTO regions (country_code, name, slug, kind, parent_id) VALUES
 ('FR', 'Mayotte (976)', '976-mayotte', 'CITY', (SELECT id FROM regions WHERE slug = 'region-mayotte'));
