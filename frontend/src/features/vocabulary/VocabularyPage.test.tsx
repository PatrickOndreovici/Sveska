import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { VocabularyEntry } from "../../lib/api/types";
import { mockApi, student, type MockHandler } from "../../test/mockApi";
import { renderApp } from "../../test/renderApp";

const blank = {
  notes: "",
  example: "",
  partOfSpeech: "",
  gender: "",
  plural: "",
  aspect: "",
  aspectPair: "",
  feminineForm: "",
  neuterForm: "",
  source: null,
  createdAt: "2026-10-23T10:00:00Z",
  updatedAt: "2026-10-23T10:00:00Z",
} as const;

function entry(term: string, translation: string, extra: Partial<VocabularyEntry> = {}): VocabularyEntry {
  return { id: `id-${term}-${translation}`, term, translation, ...blank, ...extra };
}

const kuca = entry("kuća", "house", { id: "entry-kuca", partOfSpeech: "noun", gender: "feminine", plural: "kuće" });
const citati = entry("čitati", "to read", { partOfSpeech: "verb", aspect: "imperfective" });

const PAGE_SIZE = 10;

/**
 * A small in-memory API: the list answers with the stored words, paged, and
 * add, edit and delete change them. Every list URL is recorded so tests can
 * check what the page asked for. Filtering is the server's job and is
 * covered by the backend tests, so the fake ignores query, days and sort.
 */
function fakeVocabularyApi(initial: VocabularyEntry[], overrides: Record<string, MockHandler> = {}) {
  let words = [...initial];
  const listUrls: URL[] = [];

  const fetchSpy = mockApi({
    "GET /api/me": { status: 200, body: { user: student } },
    "GET /api/vocabulary": (_body, url) => {
      listUrls.push(url);
      const page = Number(url.searchParams.get("page") ?? "1");
      const start = (page - 1) * PAGE_SIZE;
      return {
        status: 200,
        body: {
          entries: words.slice(start, start + PAGE_SIZE),
          total: words.length,
          unfilteredTotal: words.length,
          page,
          pageSize: PAGE_SIZE,
        },
      };
    },
    "POST /api/vocabulary": (body) => {
      const created = { ...entry("", ""), ...(body as Partial<VocabularyEntry>) };
      created.id = `id-${created.term}-${created.translation}`;
      words = [created, ...words];
      return { status: 201, body: { entry: created } };
    },
    "PUT /api/vocabulary/entry-kuca": (body) => {
      const updated = { ...kuca, ...blank, ...(body as Partial<VocabularyEntry>) };
      words = words.map((w) => (w.id === kuca.id ? updated : w));
      return { status: 200, body: { entry: updated } };
    },
    "DELETE /api/vocabulary/entry-kuca": () => {
      words = words.filter((w) => w.id !== kuca.id);
      return { status: 204 };
    },
    ...overrides,
  });

  return { fetchSpy, listUrls, lastListUrl: () => listUrls.at(-1) };
}

function requestBody(fetchSpy: ReturnType<typeof mockApi>, method: string, url: string): unknown {
  const call = fetchSpy.mock.calls.find(([u, init]) => u === url && init?.method === method);
  return call ? JSON.parse(call[1]?.body as string) : undefined;
}

async function renderVocabulary() {
  renderApp("/vocabulary");
  await screen.findByRole("heading", { level: 1, name: "Rečnik" });
}

function addForm() {
  return within(screen.getByRole("region", { name: "Add a word" }));
}

describe("VocabularyPage", () => {
  it("shows the words from the API", async () => {
    fakeVocabularyApi([kuca, citati]);
    await renderVocabulary();

    expect(await screen.findByText("2 words collected so far.")).toBeInTheDocument();
    const list = screen.getByRole("list");
    expect(within(list).getAllByRole("listitem")).toHaveLength(2);
    expect(within(list).getByText("kuća")).toBeInTheDocument();
    expect(within(list).getByText("pl. kuće")).toBeInTheDocument();
    expect(screen.getByText("1–2 of 2")).toBeInTheDocument();
  });

  it("invites the first word when the vocabulary is empty", async () => {
    fakeVocabularyApi([]);
    await renderVocabulary();
    expect(await screen.findByText("No words yet. Add your first one.")).toBeInTheDocument();
    expect(screen.getByText("0 words collected so far.")).toBeInTheDocument();
  });

  it("sends the search, time range and sort to the API", async () => {
    const api = fakeVocabularyApi([kuca]);
    await renderVocabulary();
    await screen.findByText("1–1 of 1");
    // Defaults are left out of the URL.
    expect(api.lastListUrl()?.search).toBe("");

    await userEvent.type(screen.getByLabelText("Search"), " cevap ");
    await waitFor(() => {
      expect(api.lastListUrl()?.searchParams.get("query")).toBe("cevap");
    });

    await userEvent.click(screen.getByRole("button", { name: "7 days" }));
    await waitFor(() => {
      expect(api.lastListUrl()?.searchParams.get("days")).toBe("7");
    });

    await userEvent.selectOptions(screen.getByLabelText("Sort"), "az");
    await waitFor(() => {
      expect(api.lastListUrl()?.searchParams.get("sort")).toBe("az");
    });
    expect(api.lastListUrl()?.searchParams.get("query")).toBe("cevap");
  });

  it("asks the API for the next page", async () => {
    const words = Array.from({ length: 12 }, (_, i) => entry(`reč ${String(i + 1)}`, "word"));
    const api = fakeVocabularyApi(words);
    await renderVocabulary();
    expect(await screen.findByText("1–10 of 12")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Next" }));

    expect(await screen.findByText("11–12 of 12")).toBeInTheDocument();
    expect(api.lastListUrl()?.searchParams.get("page")).toBe("2");
    expect(screen.getByText("2 / 2")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();
  });

  it("adds a word and shows it in the list", async () => {
    const api = fakeVocabularyApi([citati]);
    await renderVocabulary();
    await screen.findByText("1 word collected so far.");

    await userEvent.type(addForm().getByLabelText("Serbian"), "kuća");
    await userEvent.type(addForm().getByLabelText("Translation"), "house");
    await userEvent.click(addForm().getByRole("button", { name: "Add word" }));

    expect(await addForm().findByRole("status")).toHaveTextContent("Added kuća (house).");
    expect(await screen.findByText("2 words collected so far.")).toBeInTheDocument();
    expect(within(screen.getByRole("list")).getByText("kuća")).toBeInTheDocument();
    expect(requestBody(api.fetchSpy, "POST", "/api/vocabulary")).toEqual({
      term: "kuća",
      translation: "house",
      notes: "",
      example: "",
      partOfSpeech: "",
      gender: "",
      plural: "",
      aspect: "",
      aspectPair: "",
      feminineForm: "",
      neuterForm: "",
    });
    // The form is empty again for the next word.
    expect(addForm().getByLabelText("Serbian")).toHaveValue("");
  });

  it("shows the API's message when the word already exists", async () => {
    fakeVocabularyApi([kuca], {
      "POST /api/vocabulary": {
        status: 409,
        body: { error: { code: "duplicate_entry", message: "You already have this word with this translation." } },
      },
    });
    await renderVocabulary();

    await userEvent.type(addForm().getByLabelText("Serbian"), "Kuća");
    await userEvent.type(addForm().getByLabelText("Translation"), "house");
    await userEvent.click(addForm().getByRole("button", { name: "Add word" }));

    expect(await addForm().findByRole("alert")).toHaveTextContent("You already have this word with this translation.");
    // The typed word stays, so it can be corrected.
    expect(addForm().getByLabelText("Serbian")).toHaveValue("Kuća");
  });

  it("puts the API's field problems next to their inputs", async () => {
    fakeVocabularyApi([], {
      "POST /api/vocabulary": {
        status: 422,
        body: {
          error: {
            code: "validation_failed",
            message: "Some fields are missing or invalid.",
            fields: { notes: "Use at most 2000 characters." },
          },
        },
      },
    });
    await renderVocabulary();

    await userEvent.type(addForm().getByLabelText("Serbian"), "kuća");
    await userEvent.type(addForm().getByLabelText("Translation"), "house");
    await userEvent.click(addForm().getByRole("button", { name: "Add word" }));

    await waitFor(() => {
      expect(addForm().getByLabelText("Notes")).toHaveAccessibleDescription("Use at most 2000 characters.");
    });
  });

  it("checks the required fields before calling the API", async () => {
    const api = fakeVocabularyApi([]);
    await renderVocabulary();

    await userEvent.click(addForm().getByRole("button", { name: "Add word" }));

    expect(addForm().getByLabelText("Serbian")).toHaveAccessibleDescription("Enter the Serbian word.");
    expect(requestBody(api.fetchSpy, "POST", "/api/vocabulary")).toBeUndefined();
  });

  it("edits a word in place", async () => {
    const api = fakeVocabularyApi([kuca]);
    await renderVocabulary();

    await userEvent.click(await screen.findByRole("button", { name: "Edit kuća (house)" }));
    const form = within(screen.getByRole("form", { name: "Edit kuća" }));
    await userEvent.clear(form.getByLabelText("Translation"));
    await userEvent.type(form.getByLabelText("Translation"), "home");
    await userEvent.click(form.getByRole("button", { name: "Save" }));

    expect(await screen.findByRole("button", { name: "Edit kuća (home)" })).toBeInTheDocument();
    expect(requestBody(api.fetchSpy, "PUT", "/api/vocabulary/entry-kuca")).toMatchObject({
      term: "kuća",
      translation: "home",
      partOfSpeech: "noun",
      gender: "feminine",
      plural: "kuće",
    });
  });

  it("keeps the edit form open when the API refuses the change", async () => {
    fakeVocabularyApi([kuca], {
      "PUT /api/vocabulary/entry-kuca": {
        status: 409,
        body: { error: { code: "duplicate_entry", message: "You already have this word with this translation." } },
      },
    });
    await renderVocabulary();

    await userEvent.click(await screen.findByRole("button", { name: "Edit kuća (house)" }));
    const form = within(screen.getByRole("form", { name: "Edit kuća" }));
    await userEvent.click(form.getByRole("button", { name: "Save" }));

    expect(await form.findByRole("alert")).toHaveTextContent("You already have this word with this translation.");
    expect(screen.getByRole("form", { name: "Edit kuća" })).toBeInTheDocument();
  });

  it("deletes a word after confirming", async () => {
    const api = fakeVocabularyApi([kuca]);
    await renderVocabulary();

    await userEvent.click(await screen.findByRole("button", { name: "Delete kuća (house)" }));
    const dialog = within(screen.getByRole("dialog", { name: "Delete this word?" }));
    await userEvent.click(dialog.getByRole("button", { name: "Delete" }));

    expect(await screen.findByText("No words yet. Add your first one.")).toBeInTheDocument();
    expect(api.fetchSpy).toHaveBeenCalledWith("/api/vocabulary/entry-kuca", expect.objectContaining({ method: "DELETE" }));
  });

  it("offers a retry when the list cannot be loaded", async () => {
    let failures = 1;
    fakeVocabularyApi([kuca], {
      "GET /api/vocabulary": () => {
        if (failures-- > 0) {
          return {
            status: 500,
            body: { error: { code: "internal_error", message: "Something went wrong on our side. Try again." } },
          };
        }
        return { status: 200, body: { entries: [kuca], total: 1, unfilteredTotal: 1, page: 1, pageSize: PAGE_SIZE } };
      },
    });
    await renderVocabulary();

    expect(await screen.findByRole("alert")).toHaveTextContent("Something went wrong on our side. Try again.");
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));

    expect(await screen.findByText("1–1 of 1")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
