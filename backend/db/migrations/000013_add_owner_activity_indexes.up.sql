-- The dashboard counts and charts a student's own rows by owner and date.
-- These tables' indexes lead with the document, or for vocabulary with the
-- owner and term, so they cannot find one owner's rows of a given week.
CREATE INDEX vocabulary_entries_owner_created_idx ON vocabulary_entries (owner_id, created_at);
CREATE INDEX documents_owner_created_idx ON documents (owner_id, created_at);
CREATE INDEX annotations_owner_created_idx ON annotations (owner_id, created_at);
CREATE INDEX ink_strokes_owner_created_idx ON ink_strokes (owner_id, created_at);
CREATE INDEX text_boxes_owner_created_idx ON text_boxes (owner_id, created_at);
