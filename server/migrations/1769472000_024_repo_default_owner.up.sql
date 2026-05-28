ALTER TABLE repositories ADD COLUMN default_owner_id INTEGER REFERENCES users(user_id);
CREATE INDEX idx_repositories_default_owner ON repositories(default_owner_id);
