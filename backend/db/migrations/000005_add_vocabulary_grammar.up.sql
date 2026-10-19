-- Strips diacritics for search, so "cevapi" finds "ćevapi".
CREATE EXTENSION IF NOT EXISTS unaccent;

CREATE TYPE part_of_speech AS ENUM ('noun', 'verb', 'adjective', 'other');
CREATE TYPE grammatical_gender AS ENUM ('masculine', 'feminine', 'neuter');
CREATE TYPE verb_aspect AS ENUM ('imperfective', 'perfective', 'biaspectual');

-- All grammar is optional. Enums use NULL for "not set" (an enum cannot be
-- ''); free text uses '' like notes, so Go reads a plain string. Each field
-- is only allowed for the part of speech it describes.
ALTER TABLE vocabulary_entries
    ADD COLUMN part_of_speech part_of_speech,
    ADD COLUMN example        text NOT NULL DEFAULT '',
    -- Nouns.
    ADD COLUMN gender         grammatical_gender,
    ADD COLUMN plural         text NOT NULL DEFAULT '',
    -- Verbs.
    ADD COLUMN aspect         verb_aspect,
    ADD COLUMN aspect_pair    text NOT NULL DEFAULT '',
    -- Adjectives; the term itself is the masculine form.
    ADD COLUMN feminine_form  text NOT NULL DEFAULT '',
    ADD COLUMN neuter_form    text NOT NULL DEFAULT '',

    ADD CONSTRAINT vocabulary_entries_example_length CHECK (char_length(example) <= 500),
    ADD CONSTRAINT vocabulary_entries_plural_length CHECK (char_length(plural) <= 200),
    ADD CONSTRAINT vocabulary_entries_aspect_pair_length CHECK (char_length(aspect_pair) <= 200),
    ADD CONSTRAINT vocabulary_entries_feminine_form_length CHECK (char_length(feminine_form) <= 200),
    ADD CONSTRAINT vocabulary_entries_neuter_form_length CHECK (char_length(neuter_form) <= 200),

    -- IS NOT DISTINCT FROM, not =: with part_of_speech NULL, "=" yields NULL,
    -- and a CHECK that evaluates to NULL passes.
    ADD CONSTRAINT vocabulary_entries_noun_fields CHECK (
        part_of_speech IS NOT DISTINCT FROM 'noun' OR (gender IS NULL AND plural = '')
    ),
    ADD CONSTRAINT vocabulary_entries_verb_fields CHECK (
        part_of_speech IS NOT DISTINCT FROM 'verb' OR (aspect IS NULL AND aspect_pair = '')
    ),
    ADD CONSTRAINT vocabulary_entries_adjective_fields CHECK (
        part_of_speech IS NOT DISTINCT FROM 'adjective' OR (feminine_form = '' AND neuter_form = '')
    );
