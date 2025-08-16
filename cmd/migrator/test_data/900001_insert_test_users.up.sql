INSERT INTO users (id, balance)
VALUES (1,0),(2,0),(3,0)
ON CONFLICT (id) DO NOTHING;