-- v12: Cache mime type for reuploaded files
ALTER TABLE fluxer_file ADD COLUMN mime_type TEXT NOT NULL DEFAULT '';
-- only: postgres
ALTER TABLE fluxer_file ALTER COLUMN mime_type DROP DEFAULT;
