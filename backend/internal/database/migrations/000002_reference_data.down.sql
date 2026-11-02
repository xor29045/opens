DELETE FROM categories WHERE parent_id IS NOT NULL;
DELETE FROM categories;
DELETE FROM regions WHERE parent_id IS NOT NULL;
DELETE FROM regions;
DELETE FROM subscription_plans WHERE code = 'establishment-monthly';
DELETE FROM roles;
