# Data model draft

Status: draft for review. **Migrated so far:** `users` and `sessions` (phase 1).
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

## Student workspace (owned by a student)

| Table | Columns | Notes |
|---|---|---|
| `lessons` | id, owner_id → users, title, description, position, created_at, updated_at, archived_at | A student's own unit of study. |
| `documents` | id, owner_id → users, lesson_id → lessons null, title, storage_key, byte_size, sha256, page_count, created_at | PDF metadata; bytes in file storage. Owner may be a teacher (assignment material). |
| `annotations` | id, document_id → documents, owner_id → users, page_number, kind (highlight, note), rects `jsonb`, quote, body, created_at, updated_at | `rects`: array of {x, y, w, h} in 0–1 page coordinates. |
| `vocabulary_entries` | id, owner_id → users, term, translation, notes, source_annotation_id → annotations null, created_at, updated_at | Unique on (owner_id, lower(term), translation). |
| `quizzes` | id, owner_id → users, lesson_id → lessons null, title, created_at, updated_at | Authored by a student for self-study or by a teacher for an assignment. |
| `quiz_questions` | id, quiz_id → quizzes, position, kind (multiple_choice, short_answer), prompt, choices `jsonb`, answer `jsonb` | Answers are never sent to a student before submission. |
| `quiz_attempts` | id, quiz_id → quizzes, user_id → users, started_at, submitted_at, score, max_score | |
| `quiz_attempt_answers` | attempt_id → quiz_attempts, question_id → quiz_questions, response `jsonb`, is_correct | PK (attempt_id, question_id). |
| `lesson_progress` | user_id → users, lesson_id → lessons, status (not_started, in_progress, completed), updated_at | PK (user_id, lesson_id). Other progress figures are derived from attempts and vocabulary, not stored. |

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
- A teacher manages classes they own, sees submissions for those classes, and
  grades them.
- A teacher sees nothing else of a student's workspace unless the answer to the
  open question on linked-teacher visibility says otherwise.
- Annotations and vocabulary are always private to their owner, even on a
  teacher's document.
