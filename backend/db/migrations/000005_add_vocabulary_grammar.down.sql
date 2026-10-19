ALTER TABLE vocabulary_entries
    DROP COLUMN IF EXISTS part_of_speech,
    DROP COLUMN IF EXISTS example,
    DROP COLUMN IF EXISTS gender,
    DROP COLUMN IF EXISTS plural,
    DROP COLUMN IF EXISTS aspect,
    DROP COLUMN IF EXISTS aspect_pair,
    DROP COLUMN IF EXISTS feminine_form,
    DROP COLUMN IF EXISTS neuter_form;

DROP TYPE IF EXISTS verb_aspect;
DROP TYPE IF EXISTS grammatical_gender;
DROP TYPE IF EXISTS part_of_speech;

DROP EXTENSION IF EXISTS unaccent;
