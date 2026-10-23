//go:build integration

package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"testing"
)

// These tests run the vocabulary endpoints against PostgreSQL, so they cover
// what the unit tests fake: the SQL search, ordering, paging and filters,
// the owner checks, and the database constraints.

// signedUp returns a browser logged in as a new student.
func signedUp(t *testing.T, b *browser, email string) *browser {
	t.Helper()
	if resp, body := b.send(http.MethodPost, "/api/auth/signup", signUpPayload(email, "student")); resp.StatusCode != http.StatusCreated {
		t.Fatalf("signup %s = %d %v", email, resp.StatusCode, body)
	}
	return b
}

// addWord posts an entry and returns its id.
func (b *browser) addWord(fields map[string]string) string {
	b.t.Helper()
	resp, body := b.send(http.MethodPost, "/api/vocabulary", fields)
	if resp.StatusCode != http.StatusCreated {
		b.t.Fatalf("add %v = %d %v", fields, resp.StatusCode, body)
	}
	return body["entry"].(map[string]any)["id"].(string)
}

// listWords fetches one page and returns the response body.
func (b *browser) listWords(params url.Values) map[string]any {
	b.t.Helper()
	resp, body := b.send(http.MethodGet, "/api/vocabulary?"+params.Encode(), nil)
	if resp.StatusCode != http.StatusOK {
		b.t.Fatalf("list %v = %d %v", params, resp.StatusCode, body)
	}
	return body
}

func terms(body map[string]any) []string {
	var out []string
	for _, e := range body["entries"].([]any) {
		out = append(out, e.(map[string]any)["term"].(string))
	}
	return out
}

func errorCode(body map[string]any) string {
	errBody, _ := body["error"].(map[string]any)
	code, _ := errBody["code"].(string)
	return code
}

func TestVocabularyCreateListUpdateDelete(t *testing.T) {
	srv, _ := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")

	id := ana.addWord(map[string]string{
		"term": " kuća ", "translation": "house", "partOfSpeech": "noun", "gender": "feminine", "plural": "kuće",
	})

	body := ana.listWords(nil)
	entries := body["entries"].([]any)
	if len(entries) != 1 || body["total"] != 1.0 || body["unfilteredTotal"] != 1.0 || body["page"] != 1.0 || body["pageSize"] != 10.0 {
		t.Fatalf("list = %v", body)
	}
	entry := entries[0].(map[string]any)
	if entry["id"] != id || entry["term"] != "kuća" || entry["gender"] != "feminine" || entry["aspect"] != "" {
		t.Errorf("entry = %v", entry)
	}

	// PUT replaces everything: turning the noun into a verb clears its noun fields.
	resp, body := ana.send(http.MethodPut, "/api/vocabulary/"+id, map[string]string{
		"term": "čitati", "translation": "to read", "partOfSpeech": "verb", "aspect": "imperfective",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update = %d %v", resp.StatusCode, body)
	}
	updated := body["entry"].(map[string]any)
	if updated["term"] != "čitati" || updated["aspect"] != "imperfective" || updated["gender"] != "" || updated["plural"] != "" {
		t.Errorf("updated = %v", updated)
	}

	if resp, _ := ana.send(http.MethodDelete, "/api/vocabulary/"+id, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete = %d", resp.StatusCode)
	}
	if body := ana.listWords(nil); len(body["entries"].([]any)) != 0 || body["total"] != 0.0 {
		t.Errorf("list after delete = %v", body)
	}
}

func TestVocabularyIsPrivateToItsOwner(t *testing.T) {
	srv, pool := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	marko := signedUp(t, newBrowser(t, srv), "marko@example.com")
	anasID := ana.addWord(map[string]string{"term": "kuća", "translation": "house"})

	if body := marko.listWords(nil); len(body["entries"].([]any)) != 0 || body["unfilteredTotal"] != 0.0 {
		t.Errorf("Marko sees %v", body)
	}
	// Someone else's entry answers exactly like a missing one.
	resp, body := marko.send(http.MethodPut, "/api/vocabulary/"+anasID, map[string]string{"term": "x", "translation": "y"})
	if resp.StatusCode != http.StatusNotFound || errorCode(body) != "entry_not_found" {
		t.Errorf("Marko's update = %d %v", resp.StatusCode, body)
	}
	if resp, _ := marko.send(http.MethodDelete, "/api/vocabulary/"+anasID, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("Marko's delete = %d, want 404", resp.StatusCode)
	}

	var term string
	if err := pool.QueryRow(context.Background(), `SELECT term FROM vocabulary_entries WHERE id = $1`, anasID).Scan(&term); err != nil || term != "kuća" {
		t.Errorf("Ana's entry after Marko's attempts: %q, %v", term, err)
	}

	// The same word is fine for a different owner.
	marko.addWord(map[string]string{"term": "kuća", "translation": "house"})
}

func TestVocabularyRejectsDuplicates(t *testing.T) {
	srv, _ := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	ana.addWord(map[string]string{"term": "kuća", "translation": "house"})
	homeID := ana.addWord(map[string]string{"term": "kuća", "translation": "home"})

	resp, body := ana.send(http.MethodPost, "/api/vocabulary", map[string]string{"term": "Kuća", "translation": "house"})
	if resp.StatusCode != http.StatusConflict || errorCode(body) != "duplicate_entry" {
		t.Errorf("duplicate add = %d %v", resp.StatusCode, body)
	}
	resp, body = ana.send(http.MethodPut, "/api/vocabulary/"+homeID, map[string]string{"term": "kuća", "translation": "house"})
	if resp.StatusCode != http.StatusConflict || errorCode(body) != "duplicate_entry" {
		t.Errorf("duplicate update = %d %v", resp.StatusCode, body)
	}
}

func TestVocabularyRejectsInvalidInputWithoutA500(t *testing.T) {
	srv, _ := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	id := ana.addWord(map[string]string{"term": "kuća", "translation": "house"})

	tests := []struct {
		name, method, path string
		body               any
		status             int
	}{
		// The database would reject these with a CHECK; the service must answer first.
		{"gender on a verb", http.MethodPost, "/api/vocabulary",
			map[string]string{"term": "čitati", "translation": "to read", "partOfSpeech": "verb", "gender": "masculine"}, http.StatusUnprocessableEntity},
		{"unknown enum value", http.MethodPost, "/api/vocabulary",
			map[string]string{"term": "kuća", "translation": "x", "partOfSpeech": "pronoun"}, http.StatusUnprocessableEntity},
		{"malformed id on update", http.MethodPut, "/api/vocabulary/not-a-uuid",
			map[string]string{"term": "kuća", "translation": "house"}, http.StatusNotFound},
		{"malformed id on delete", http.MethodDelete, "/api/vocabulary/" + id[:len(id)-1] + "z", nil, http.StatusNotFound},
		{"negative days", http.MethodGet, "/api/vocabulary?days=-1", nil, http.StatusUnprocessableEntity},
		{"days not a number", http.MethodGet, "/api/vocabulary?days=week", nil, http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := ana.send(tt.method, tt.path, tt.body)
			if resp.StatusCode != tt.status {
				t.Errorf("status = %d, want %d (%v)", resp.StatusCode, tt.status, body)
			}
		})
	}
}

func TestVocabularySearchIgnoresCaseAndDiacritics(t *testing.T) {
	srv, _ := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	ana.addWord(map[string]string{"term": "ćevapi", "translation": "grilled minced-meat rolls"})
	ana.addWord(map[string]string{"term": "čovek", "translation": "man", "partOfSpeech": "noun", "plural": "ljudi"})
	ana.addWord(map[string]string{"term": "đak", "translation": "pupil"})
	ana.addWord(map[string]string{"term": "popust", "translation": "50% off"})

	tests := []struct {
		query string
		want  []string
	}{
		{"cevapi", []string{"ćevapi"}},
		{"ĆEVAP", []string{"ćevapi"}},
		{"LJUDI", []string{"čovek"}}, // matches the plural, not the term
		{"MAN", []string{"čovek"}},   // matches the translation
		{"dak", []string{"đak"}},     // unaccent maps đ to d
		{"%", []string{"popust"}},    // a literal %, not a wildcard
		{"_", nil},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			body := ana.listWords(url.Values{"query": {tt.query}})
			if got := terms(body); !slices.Equal(got, tt.want) {
				t.Errorf("terms = %v, want %v", got, tt.want)
			}
			if body["total"] != float64(len(tt.want)) || body["unfilteredTotal"] != 4.0 {
				t.Errorf("total = %v, unfilteredTotal = %v", body["total"], body["unfilteredTotal"])
			}
		})
	}
}

func TestVocabularySortsInSerbianAlphabeticalOrder(t *testing.T) {
	srv, _ := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	// Plain Unicode order would put "čaj" (c-a) before "cvet" (c-v); Serbian
	// treats c, č and ć as separate letters in that order.
	for _, term := range []string{"dan", "ćao", "cvet", "čuvar", "čaj"} {
		ana.addWord(map[string]string{"term": term, "translation": "x"})
	}

	if got, want := terms(ana.listWords(url.Values{"sort": {"az"}})), []string{"cvet", "čaj", "čuvar", "ćao", "dan"}; !slices.Equal(got, want) {
		t.Errorf("az = %v, want %v", got, want)
	}
	if got, want := terms(ana.listWords(nil)), []string{"čaj", "čuvar", "cvet", "ćao", "dan"}; !slices.Equal(got, want) {
		t.Errorf("newest = %v, want %v (most recently added first)", got, want)
	}
}

func TestVocabularyPagesNeverOverlap(t *testing.T) {
	srv, pool := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	for i := range 12 {
		ana.addWord(map[string]string{"term": fmt.Sprintf("reč %02d", i), "translation": "word"})
	}
	// Give every entry the same created_at so only the id tiebreaker keeps the order stable.
	if _, err := pool.Exec(context.Background(), `UPDATE vocabulary_entries SET created_at = now()`); err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	for page, wantLen := range map[string]int{"1": 10, "2": 2, "3": 0} {
		body := ana.listWords(url.Values{"page": {page}})
		got := terms(body)
		if len(got) != wantLen || body["total"] != 12.0 {
			t.Errorf("page %s: %d entries, total %v; want %d, 12", page, len(got), body["total"], wantLen)
		}
		for _, term := range got {
			if seen[term] {
				t.Errorf("%q appears on two pages", term)
			}
			seen[term] = true
		}
	}
	if len(seen) != 12 {
		t.Errorf("saw %d distinct entries across pages, want 12", len(seen))
	}
}

func TestVocabularyFiltersByDaysAdded(t *testing.T) {
	srv, pool := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	ana.addWord(map[string]string{"term": "danas", "translation": "today"})
	oldID := ana.addWord(map[string]string{"term": "juče", "translation": "yesterday"})
	if _, err := pool.Exec(context.Background(), `UPDATE vocabulary_entries SET created_at = now() - interval '10 days' WHERE id = $1`, oldID); err != nil {
		t.Fatal(err)
	}

	body := ana.listWords(url.Values{"days": {"7"}})
	if got := terms(body); !slices.Equal(got, []string{"danas"}) || body["total"] != 1.0 || body["unfilteredTotal"] != 2.0 {
		t.Errorf("last 7 days = %v (total %v, unfiltered %v)", got, body["total"], body["unfilteredTotal"])
	}
	if got := terms(ana.listWords(url.Values{"days": {"30"}})); len(got) != 2 {
		t.Errorf("last 30 days = %v, want both", got)
	}
}

func TestVocabularyWithoutSessionIs401(t *testing.T) {
	srv, _ := newStack(t)
	resp, body := newBrowser(t, srv).send(http.MethodGet, "/api/vocabulary", nil)
	if resp.StatusCode != http.StatusUnauthorized || errorCode(body) != "unauthenticated" {
		t.Errorf("list without session = %d %v", resp.StatusCode, body)
	}
}
