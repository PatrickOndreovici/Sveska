DROP INDEX IF EXISTS vocabulary_entries_source_annotation_idx;
ALTER TABLE vocabulary_entries DROP COLUMN IF EXISTS source_annotation_id;
