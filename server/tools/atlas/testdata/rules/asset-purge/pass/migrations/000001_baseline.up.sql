CREATE TABLE assets (id INTEGER PRIMARY KEY, entry_count INTEGER);
CREATE TABLE river_job (id INTEGER PRIMARY KEY, state TEXT);
CREATE TRIGGER assets_cleanup AFTER UPDATE OF entry_count ON assets
WHEN NEW.entry_count = 0
BEGIN
  DELETE FROM assets WHERE id = NEW.id;
END;
