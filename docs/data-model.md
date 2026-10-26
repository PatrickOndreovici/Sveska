# Data model draft

Status: draft for review. **Migrated so far:** `users` and `sessions` (phase 1), `vocabulary_entries`.
The other tables are created phase by phase (see implementation-plan.md), each
in its own migration.

Conventions: `uuid` primary keys from PostgreSQL 18's time-ordered `uuidv7()`, `timestamptz`
everywhere, `created_at`/`updated_at` on mutable tables, soft delete only where
history matters (`archived_at`), foreign keys always indexed, enums as
PostgreSQL enum types.

## Identity

| Table | Columns | Notes |
|---|---|---|
| `users` | id, email `citext` unique, password_hash, display_name, role `user_role` (student, teacher), created_at, updated_at | **Migrated.** Roles are exclusive and fixed at sign-up. Length checks on email and display_name. |
| `sessions` | id, user_id → users (cascade), token_hash `bytea` unique, created_at, expires_at | **Migrated.** Only the SHA-256 of the token is stored. Indexed on user_id. |
| `user_avatars` | user_id → users (cascade) primary key, image `bytea`, updated_at | **Migrated.** The profile picture, re-encoded by the API as a JPEG of at most 256×256 (a few dozen KB), so it is kept in the database rather than file storage; a CHECK caps it at 1 MB. `updated_at` versions the picture's URL. |

## Student workspace (owned by a student)

| Table | Columns | Notes |
|---|---|---|
| `lessons` | id, owner_id → users, title, description, position, created_at, updated_at, archived_at | A student's own unit of study. |
| `documents` | id, owner_id → users, lesson_id → lessons null, title, storage_key, byte_size, sha256, page_count, created_at | PDF metadata; bytes in file storage. Owner may be a teacher (assignment material). |
| `annotations` | id, owner_id, document_id, page_number, rects `jsonb`, quote, note, created_at, updated_at | **Migrated.** A highlight with an optional note: a note is an annotation whose `note` is not empty, so there is no `kind`. `rects`: array of {x, y, width, height} in 0–1 page coordinates, 1–200 of them; the service checks each lies within the page. The foreign key (document_id, owner_id) → documents (id, owner_id) keeps the owner the document's owner and cascades from documents. |
| `ink_strokes` | id, owner_id, document_id, page_number, color `ink_color`, points `jsonb`, created_at | **Migrated.** A line drawn by hand, such as a circle around an answer: [{x, y}, ...] in 0–1 page coordinates, 1–1000 points; the service checks each lies within the page. Never updated, only drawn or erased. `ink_color` is an enum (black, blue, red). Same composite foreign key to documents as `annotations`. |
| `text_boxes` | id, owner_id, document_id, page_number, x, y, body, color `ink_color`, created_at, updated_at | **Migrated.** Text typed onto a page, such as the answer to a fill-in exercise. (x, y) is its top-left corner in 0–1 page coordinates; a CHECK keeps it on the page. `body` is 1–1000 characters and may span lines. Same composite foreign key to documents as `annotations`. |
| `vocabulary_entries` | id, owner_id → users, term, translation, notes, example, part_of_speech null, gender null, plural, aspect null, aspect_pair, feminine_form, neuter_form, source_annotation_id → annotations null, created_at, updated_at | **Migrated.** `source_annotation_id` is the highlight a word was saved from: the foreign key (source_annotation_id, owner_id) → annotations (id, owner_id) allows only the owner's own, and `ON DELETE SET NULL (source_annotation_id)` keeps the word when the highlight goes. Grammar is optional: enums (`part_of_speech`, `grammatical_gender`, `verb_aspect`) are NULL when unset, text fields `''`. CHECKs allow gender and plural only on nouns, aspect and aspect pair only on verbs, feminine and neuter forms only on adjectives. Unique index on (owner_id, lower(term), translation). Search uses the `unaccent` extension. |
| `review_cards` | entry_id → vocabulary_entries primary key, owner_id, due_on `date`, interval_days, ease_percent, repetitions, lapses, last_reviewed_at, created_at | **Migrated.** A word's flashcard schedule, created at its first review: a word without one is new. `due_on` is a date in the student's time zone. `interval_days` is 0–365, so every word comes back at least once a year; `ease_percent` (from 250, at least 130) is how much a good answer stretches it. The foreign key (entry_id, owner_id) → vocabulary_entries (id, owner_id) keeps the owner the word's and cascades from it. `last_reviewed_at` lets an answer replace the schedule only if it is still the one it read. Index on (owner_id, due_on). |
| `reviews` | id, owner_id → users, entry_id null, rating `review_rating`, interval_days, reviewed_at | **Migrated.** Every flashcard answer (`review_rating`: again, hard, good, easy), saved with the schedule in one statement. Same composite foreign key to the word as `review_cards`, but `ON DELETE SET NULL (entry_id)`: a deleted word's reviews still count on the dashboard. Index on (owner_id, reviewed_at). |
| `quizzes` | id, owner_id → users, lesson_id → lessons null, title, created_at, updated_at | Authored by a student for self-study or by a teacher for an assignment. |
| `quiz_questions` | id, quiz_id → quizzes, position, kind (multiple_choice, short_answer), prompt, choices `jsonb`, answer `jsonb` | Answers are never sent to a student before submission. |
| `quiz_attempts` | id, quiz_id → quizzes, user_id → users, started_at, submitted_at, score, max_score | |
| `quiz_attempt_answers` | attempt_id → quiz_attempts, question_id → quiz_questions, response `jsonb`, is_correct | PK (attempt_id, question_id). |
| `lesson_progress` | user_id → users, lesson_id → lessons, status (not_started, in_progress, completed), updated_at | PK (user_id, lesson_id). Other progress figures are derived from attempts and vocabulary, not stored. |

> **Phase 2 as built:** `lessons` and `documents` are migrated without
> `position`, `archived_at` and `page_count`, which wait for a feature that
> needs them. `documents.lesson_id` is NOT NULL for now (teacher material
> without a lesson comes with assignments), and the composite foreign key
> `(lesson_id, owner_id) → lessons (id, owner_id)` makes the database refuse
> a document whose owner differs from its lesson's.

## Teacher relationship

| Table | Columns | Notes |
|---|---|---|
| `teacher_invitations` | id, student_id → users, email `citext`, token_hash, status (pending, accepted, revoked, expired), created_at, expires_at, accepted_by → users null | A student invites a teacher by email at any time. |
| `teacher_student_links` | teacher_id → users, student_id → users, created_at | PK (teacher_id, student_id). Created when an invitation is accepted. A teacher may only add linked students to classes. |
| `classes` | id, teacher_id → users, name, description, created_at, archived_at | |
| `class_memberships` | class_id → classes, student_id → users, joined_at | PK (class_id, student_id). |

## Assignments and grading

| Table | Columns | Notes |
|---|---|---|
| `assignments` | id, class_id → classes, created_by → users, title, instructions, document_id → documents null, quiz_id → quizzes null, due_at, published_at null, created_at, updated_at | Drafts have `published_at` null and are invisible to students. |
| `submissions` | id, assignment_id → assignments, student_id → users, status (draft, submitted, returned), body, quiz_attempt_id → quiz_attempts null, submitted_at, score, max_score, graded_by → users null, graded_at | Unique (assignment_id, student_id). |
| `feedback_comments` | id, submission_id → submissions, author_id → users, body, created_at | Written by the teacher or the student in reply. |

## Core authorization rules

- A student reads and writes only rows they own, plus documents and quizzes
  attached to published assignments in classes they belong to.
- Lessons, documents and vocabulary are for students only, for now: a
  teacher gets 403 `students_only`. Teachers will share material through
  assignments instead.
- A teacher manages classes they own, sees submissions for those classes, and
  grades them.
- A teacher sees nothing else of a student's workspace unless the answer to the
  open question on linked-teacher visibility says otherwise.
- Annotations and vocabulary are always private to their owner, even on a
  teacher's document.

## Dashboard

The student's dashboard (`GET /api/dashboard`) is computed from these tables
on each request; nothing is stored for it. Days are counted in the
student's time zone with `AT TIME ZONE`. Indexes on (owner_id, created_at)
for `vocabulary_entries`, `documents`, `annotations`, `ink_strokes` and
`text_boxes`, and on (owner_id, reviewed_at) for `reviews`, let it find one
student's rows of recent weeks.
