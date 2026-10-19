// MOCK DATA: lets the page be seen before the API exists. Delete this file
// once VocabularyPage loads entries from GET /api/vocabulary.

import type { EntryFields } from "./entryFields";

export type MockVocabularyEntry = EntryFields & {
  id: string;
  createdAt: string;
};

const DAY_MS = 24 * 60 * 60 * 1000;
const now = Date.now();

const blank: Omit<EntryFields, "term" | "translation"> = {
  notes: "",
  example: "",
  partOfSpeech: "",
  gender: "",
  plural: "",
  aspect: "",
  aspectPair: "",
  feminineForm: "",
  neuterForm: "",
};

// Spread over two months so every time range and several pages have something to show.
const rows: (Partial<EntryFields> & { term: string; translation: string; daysAgo: number })[] = [
  {
    term: "ćevapi",
    translation: "grilled minced-meat rolls",
    partOfSpeech: "noun",
    gender: "masculine",
    notes: "Usually plural. Served with lepinja and kajmak.",
    daysAgo: 0.1,
  },
  {
    term: "sveska",
    translation: "notebook",
    partOfSpeech: "noun",
    gender: "feminine",
    plural: "sveske",
    example: "Pišem u svesku.",
    daysAgo: 0.3,
  },
  {
    term: "čitati",
    translation: "to read",
    partOfSpeech: "verb",
    aspect: "imperfective",
    aspectPair: "pročitati",
    example: "Čitam knjigu svako veče.",
    daysAgo: 0.6,
  },
  { term: "čovek", translation: "man, person", partOfSpeech: "noun", gender: "masculine", plural: "ljudi", daysAgo: 1.2 },
  { term: "kuća", translation: "house", partOfSpeech: "noun", gender: "feminine", plural: "kuće", daysAgo: 1.5 },
  {
    term: "kuća",
    translation: "home",
    partOfSpeech: "noun",
    gender: "feminine",
    example: "Sutra sam kod kuće.",
    notes: "Kod kuće = at home.",
    daysAgo: 1.5,
  },
  { term: "hvala", translation: "thank you", partOfSpeech: "other", notes: "Hvala lepo = thank you very much.", daysAgo: 2 },
  { term: "žena", translation: "woman", partOfSpeech: "noun", gender: "feminine", plural: "žene", notes: "Also: wife.", daysAgo: 2.5 },
  {
    term: "dobar",
    translation: "good",
    partOfSpeech: "adjective",
    feminineForm: "dobra",
    neuterForm: "dobro",
    notes: "Comparative: bolji.",
    daysAgo: 3,
  },
  { term: "dete", translation: "child", partOfSpeech: "noun", gender: "neuter", plural: "deca", daysAgo: 3.5 },
  { term: "šuma", translation: "forest", partOfSpeech: "noun", gender: "feminine", plural: "šume", daysAgo: 4 },
  { term: "đak", translation: "pupil", partOfSpeech: "noun", gender: "masculine", plural: "đaci", daysAgo: 5 },
  { term: "džep", translation: "pocket", partOfSpeech: "noun", gender: "masculine", plural: "džepovi", daysAgo: 6 },
  { term: "voda", translation: "water", partOfSpeech: "noun", gender: "feminine", daysAgo: 8 },
  {
    term: "hleb",
    translation: "bread",
    partOfSpeech: "noun",
    gender: "masculine",
    notes: "Ekavian. Ijekavian: hljeb.",
    daysAgo: 9,
  },
  { term: "ulica", translation: "street", partOfSpeech: "noun", gender: "feminine", plural: "ulice", daysAgo: 10 },
  { term: "vrata", translation: "door", partOfSpeech: "noun", gender: "neuter", notes: "Plural only.", daysAgo: 11 },
  { term: "prozor", translation: "window", partOfSpeech: "noun", gender: "masculine", plural: "prozori", daysAgo: 12 },
  {
    term: "ljubav",
    translation: "love",
    partOfSpeech: "noun",
    gender: "feminine",
    notes: "Feminine despite the consonant ending.",
    daysAgo: 13,
  },
  { term: "sto", translation: "table", partOfSpeech: "noun", gender: "masculine", plural: "stolovi", notes: "Also: hundred.", daysAgo: 15 },
  { term: "stolica", translation: "chair", partOfSpeech: "noun", gender: "feminine", plural: "stolice", daysAgo: 16 },
  {
    term: "pisati",
    translation: "to write",
    partOfSpeech: "verb",
    aspect: "imperfective",
    aspectPair: "napisati",
    notes: "Present: pišem, pišeš.",
    example: "Pišem pismo prijatelju.",
    daysAgo: 18,
  },
  {
    term: "učiti",
    translation: "to learn",
    partOfSpeech: "verb",
    aspect: "imperfective",
    aspectPair: "naučiti",
    example: "Učim srpski.",
    daysAgo: 19,
  },
  {
    term: "razumeti",
    translation: "to understand",
    partOfSpeech: "verb",
    aspect: "imperfective",
    notes: "Ne razumem = I don't understand.",
    daysAgo: 20,
  },
  { term: "ručati", translation: "to have lunch", partOfSpeech: "verb", aspect: "biaspectual", daysAgo: 20.5 },
  { term: "grad", translation: "city", partOfSpeech: "noun", gender: "masculine", plural: "gradovi", daysAgo: 21 },
  { term: "selo", translation: "village", partOfSpeech: "noun", gender: "neuter", plural: "sela", daysAgo: 22 },
  { term: "more", translation: "sea", partOfSpeech: "noun", gender: "neuter", plural: "mora", daysAgo: 23 },
  { term: "reka", translation: "river", partOfSpeech: "noun", gender: "feminine", plural: "reke", daysAgo: 24 },
  { term: "planina", translation: "mountain", partOfSpeech: "noun", gender: "feminine", plural: "planine", daysAgo: 25 },
  {
    term: "lep",
    translation: "beautiful",
    partOfSpeech: "adjective",
    feminineForm: "lepa",
    neuterForm: "lepo",
    daysAgo: 25.5,
  },
  { term: "jutro", translation: "morning", partOfSpeech: "noun", gender: "neuter", example: "Dobro jutro!", daysAgo: 26 },
  { term: "veče", translation: "evening", partOfSpeech: "noun", gender: "neuter", plural: "večeri", daysAgo: 27 },
  { term: "noć", translation: "night", partOfSpeech: "noun", gender: "feminine", plural: "noći", example: "Laku noć.", daysAgo: 28 },
  { term: "čaša", translation: "glass", partOfSpeech: "noun", gender: "feminine", plural: "čaše", daysAgo: 29 },
  { term: "kafa", translation: "coffee", partOfSpeech: "noun", gender: "feminine", notes: "Also kava in Croatian.", daysAgo: 33 },
  { term: "čaj", translation: "tea", partOfSpeech: "noun", gender: "masculine", plural: "čajevi", daysAgo: 35 },
  { term: "mleko", translation: "milk", partOfSpeech: "noun", gender: "neuter", daysAgo: 38 },
  { term: "jabuka", translation: "apple", partOfSpeech: "noun", gender: "feminine", plural: "jabuke", daysAgo: 40 },
  { term: "zdravo", translation: "hello", partOfSpeech: "other", notes: "Informal.", daysAgo: 44 },
  { term: "doviđenja", translation: "goodbye", partOfSpeech: "other", daysAgo: 47 },
  { term: "izvinite", translation: "excuse me", notes: "Formal. Informal: izvini.", daysAgo: 50 },
  { term: "molim", translation: "please", notes: "Also: you're welcome, pardon?", daysAgo: 55 },
  { term: "prijatelj", translation: "friend", partOfSpeech: "noun", gender: "masculine", plural: "prijatelji", daysAgo: 60 },
];

export const mockEntries: MockVocabularyEntry[] = rows.map(({ daysAgo, ...fields }, index) => ({
  ...blank,
  ...fields,
  id: String(index + 1),
  createdAt: new Date(now - daysAgo * DAY_MS).toISOString(),
}));
