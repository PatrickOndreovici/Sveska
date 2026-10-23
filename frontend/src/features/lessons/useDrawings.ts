import { useCallback, useEffect, useRef, useState } from "react";
import { errorMessage } from "../../lib/api/client";
import {
  createStroke,
  createTextBox,
  deleteStroke,
  deleteTextBox,
  listStrokes,
  listTextBoxes,
  updateTextBox,
} from "../../lib/api/drawings";
import type { InkColor, Point, Stroke, TextBox, TextBoxInput } from "../../lib/api/types";

/** What one sweep of the eraser removed, by id. */
export type Erased = {
  strokes: readonly string[];
  textBoxes: readonly string[];
};

const fieldsOf = (b: TextBox): TextBoxInput => ({
  x: b.x,
  y: b.y,
  text: b.text,
  color: b.color,
});

/**
 * The strokes and text on a document. A change shows at once and is saved
 * in the background, one request after another, so a stroke is saved
 * before it can be erased. Every change can be undone.
 *
 * Ids here belong to the screen: a new stroke has one before the API
 * answers, and a stroke brought back by undo keeps its id although the API
 * gives it a new one. serverIds maps them to the API's. When a save fails,
 * the drawing is loaded again, so the screen never shows what the API does
 * not have.
 */
export function useDrawings(documentId: string) {
  const [strokes, setStrokes] = useState<Stroke[] | null>(null);
  const [textBoxes, setTextBoxes] = useState<TextBox[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [version, setVersion] = useState(0);
  const [undoCount, setUndoCount] = useState(0);
  const undos = useRef<(() => void)[]>([]);
  const serverIds = useRef(new Map<string, string>());
  const queue = useRef(Promise.resolve());
  // Bumped on every load, so saves queued against an older one are dropped.
  const generation = useRef(0);
  const lastLocalId = useRef(0);

  useEffect(() => {
    const controller = new AbortController();
    generation.current++;
    serverIds.current.clear();
    Promise.all([listStrokes(documentId, controller.signal), listTextBoxes(documentId, controller.signal)]).then(
      ([loadedStrokes, loadedTextBoxes]) => {
        setStrokes(loadedStrokes);
        setTextBoxes(loadedTextBoxes);
        setLoadError(null);
      },
      (error: unknown) => {
        if (!controller.signal.aborted) setLoadError(errorMessage(error, "Could not load your drawing. Try again."));
      },
    );
    return () => {
      controller.abort();
    };
  }, [documentId, version]);

  const reload = useCallback(() => {
    undos.current = [];
    setUndoCount(0);
    setVersion((v) => v + 1);
  }, []);

  const save = useCallback(
    (request: () => Promise<void>) => {
      const queuedIn = generation.current;
      queue.current = queue.current.then(async () => {
        if (queuedIn !== generation.current) return;
        try {
          await request();
        } catch (error) {
          if (queuedIn !== generation.current) return;
          generation.current++;
          setSaveError(`Your last change was not saved. ${errorMessage(error, "Try again.")}`);
          reload();
        }
      });
    },
    [reload],
  );

  const serverId = (id: string) => serverIds.current.get(id) ?? id;

  const pushUndo = (undo: () => void) => {
    undos.current.push(undo);
    setUndoCount(undos.current.length);
    setSaveError(null);
  };

  const undo = useCallback(() => {
    const last = undos.current.pop();
    setUndoCount(undos.current.length);
    last?.();
  }, []);

  function addStrokes(added: readonly Stroke[]) {
    setStrokes((list) => [...(list ?? []), ...added]);
    for (const s of added) {
      save(async () => {
        const saved = await createStroke(documentId, {
          pageNumber: s.pageNumber,
          color: s.color,
          points: s.points,
        });
        serverIds.current.set(s.id, saved.id);
      });
    }
  }

  function removeStrokes(ids: readonly string[]) {
    setStrokes((list) => list?.filter((s) => !ids.includes(s.id)) ?? null);
    for (const id of ids) save(() => deleteStroke(serverId(id)));
  }

  function addTextBoxes(added: readonly TextBox[]) {
    setTextBoxes((list) => [...(list ?? []), ...added]);
    for (const b of added) {
      save(async () => {
        const saved = await createTextBox(documentId, {
          pageNumber: b.pageNumber,
          ...fieldsOf(b),
        });
        serverIds.current.set(b.id, saved.id);
      });
    }
  }

  function removeTextBoxes(ids: readonly string[]) {
    setTextBoxes((list) => list?.filter((b) => !ids.includes(b.id)) ?? null);
    for (const id of ids) save(() => deleteTextBox(serverId(id)));
  }

  function changeTextBox(id: string, fields: TextBoxInput) {
    setTextBoxes((list) => list?.map((b) => (b.id === id ? { ...b, ...fields } : b)) ?? null);
    save(async () => {
      await updateTextBox(serverId(id), fields);
    });
  }

  function localId(kind: string) {
    lastLocalId.current++;
    return `local-${kind}-${String(lastLocalId.current)}`;
  }

  return {
    /** null while loading. */
    strokes,
    textBoxes,
    loadError,
    saveError,
    canUndo: undoCount > 0,
    undo,
    reload,

    draw: (pageNumber: number, color: InkColor, points: Point[]) => {
      const stroke: Stroke = {
        id: localId("stroke"),
        documentId,
        pageNumber,
        color,
        points,
        createdAt: new Date().toISOString(),
      };
      addStrokes([stroke]);
      pushUndo(() => {
        removeStrokes([stroke.id]);
      });
    },

    erase: (erased: Erased) => {
      const goneStrokes = (strokes ?? []).filter((s) => erased.strokes.includes(s.id));
      const goneTextBoxes = (textBoxes ?? []).filter((b) => erased.textBoxes.includes(b.id));
      removeStrokes(goneStrokes.map((s) => s.id));
      removeTextBoxes(goneTextBoxes.map((b) => b.id));
      pushUndo(() => {
        addStrokes(goneStrokes);
        addTextBoxes(goneTextBoxes);
      });
    },

    addTextBox: (pageNumber: number, fields: TextBoxInput) => {
      const now = new Date().toISOString();
      const box: TextBox = {
        id: localId("text"),
        documentId,
        pageNumber,
        ...fields,
        createdAt: now,
        updatedAt: now,
      };
      addTextBoxes([box]);
      pushUndo(() => {
        removeTextBoxes([box.id]);
      });
    },

    updateTextBox: (id: string, fields: TextBoxInput) => {
      const before = textBoxes?.find((b) => b.id === id);
      if (!before) return;
      changeTextBox(id, fields);
      pushUndo(() => {
        changeTextBox(id, fieldsOf(before));
      });
    },
  };
}
