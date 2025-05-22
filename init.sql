CREATE TABLE IF NOT EXISTS users (
  id INT PRIMARY KEY,
  name VARCHAR(50)
);

INSERT INTO users (id, name) VALUES (1, 'Alice'), (2, 'Bob')
  ON DUPLICATE KEY UPDATE name = VALUES(name);
