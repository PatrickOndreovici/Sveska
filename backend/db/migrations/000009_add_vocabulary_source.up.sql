-- The highlight a word was saved from, if any. Pairing it with owner_id
-- means a word can only cite its owner's own highlight. Deleting the
-- highlight keeps the word and clears only source_annotation_id: the
-- column list matters, since owner_id is part of the key and must stay.
ALTER TABLE vocabulary_entries
    ADD COLUMN source_annotation_id uuid,
    ADD CONSTRAINT vocabulary_entries_source_annotation_fkey
        FOREIGN KEY (source_annotation_id, owner_id)
        REFERENCES annotations (id, owner_id) ON DELETE SET NULL (source_annotation_id);

-- Deleting an annotation looks up the words that cite it. Most words cite
-- none, so only those that do are indexed.
CREATE INDEX vocabulary_entries_source_annotation_idx ON vocabulary_entries (source_annotation_id)
    WHERE source_annotation_id IS NOT NULL;
