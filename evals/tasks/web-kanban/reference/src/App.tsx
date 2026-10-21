import { useEffect, useState } from "react";
import type { DragEvent, FormEvent } from "react";
import "./App.css";

type ColId = "todo" | "doing" | "done";
type Card = { id: string; title: string; col: ColId };

const COLUMNS: { id: ColId; name: string }[] = [
  { id: "todo", name: "To do" },
  { id: "doing", name: "Doing" },
  { id: "done", name: "Done" },
];
const KEY = "kanban-board-v1";

function load(): Card[] {
  try {
    const raw = localStorage.getItem(KEY);
    return raw ? (JSON.parse(raw) as Card[]) : [];
  } catch {
    return [];
  }
}

export default function App() {
  const [cards, setCards] = useState<Card[]>(load);
  const [filter, setFilter] = useState("");
  const [drafts, setDrafts] = useState<Record<ColId, string>>({ todo: "", doing: "", done: "" });
  const [editing, setEditing] = useState<{ id: string; text: string } | null>(null);

  useEffect(() => {
    localStorage.setItem(KEY, JSON.stringify(cards));
  }, [cards]);

  const moveTo = (id: string, col: ColId) =>
    setCards((cs) => {
      const c = cs.find((x) => x.id === id);
      return c ? [...cs.filter((x) => x.id !== id), { ...c, col }] : cs;
    });

  const add = (e: FormEvent, col: ColId) => {
    e.preventDefault();
    const title = drafts[col].trim();
    if (!title) return;
    setCards((cs) => [...cs, { id: crypto.randomUUID(), title, col }]);
    setDrafts((d) => ({ ...d, [col]: "" }));
  };

  const saveEdit = () => {
    if (!editing) return;
    const title = editing.text.trim();
    if (title) setCards((cs) => cs.map((c) => (c.id === editing.id ? { ...c, title } : c)));
    else return; // empty title rejected: stay in edit mode with the old value untouched
    setEditing(null);
  };

  const q = filter.trim().toLowerCase();
  const onDrop = (e: DragEvent, col: ColId) => {
    e.preventDefault();
    const id = e.dataTransfer.getData("text/plain");
    if (id) moveTo(id, col);
  };

  return (
    <main className="app">
      <h1>Kanban</h1>
      <input
        className="filter"
        data-testid="filter-input"
        type="search"
        aria-label="Filter cards"
        placeholder="Filter cards"
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
      />
      <div className="board" data-testid="board">
        {COLUMNS.map((col) => {
          const shown = cards.filter((c) => c.col === col.id && c.title.toLowerCase().includes(q));
          return (
            <section
              key={col.id}
              className="column"
              data-testid={`column-${col.id}`}
              aria-label={col.name}
              onDragOver={(e) => e.preventDefault()}
              onDrop={(e) => onDrop(e, col.id)}
            >
              <h2>
                {col.name}{" "}
                <span className="count" data-testid={`column-count-${col.id}`}>
                  {shown.length}
                </span>
              </h2>
              <form className="add" onSubmit={(e) => add(e, col.id)}>
                <input
                  data-testid={`add-input-${col.id}`}
                  aria-label={`New card title for ${col.name}`}
                  placeholder="New card"
                  value={drafts[col.id]}
                  onChange={(e) => setDrafts((d) => ({ ...d, [col.id]: e.target.value }))}
                />
                <button type="submit" data-testid={`add-submit-${col.id}`}>
                  Add
                </button>
              </form>
              {shown.length === 0 && (
                <p className="empty" data-testid={`column-empty-${col.id}`}>
                  No cards
                </p>
              )}
              <ul>
                {shown.map((c) => (
                  <li
                    key={c.id}
                    className="card"
                    data-testid="card"
                    draggable
                    onDragStart={(e) => e.dataTransfer.setData("text/plain", c.id)}
                  >
                    {editing?.id === c.id ? (
                      <div className="edit">
                        <input
                          autoFocus
                          data-testid="card-edit-input"
                          aria-label="Edit card title"
                          value={editing.text}
                          onChange={(e) => setEditing({ id: c.id, text: e.target.value })}
                          onKeyDown={(e) => {
                            if (e.key === "Enter") saveEdit();
                            if (e.key === "Escape") setEditing(null);
                          }}
                        />
                        <button type="button" data-testid="card-edit-save" onClick={saveEdit}>
                          Save
                        </button>
                      </div>
                    ) : (
                      <span className="title" data-testid="card-title">
                        {c.title}
                      </span>
                    )}
                    <div className="actions">
                      <button type="button" data-testid="card-edit" onClick={() => setEditing({ id: c.id, text: c.title })}>
                        Edit
                      </button>
                      <button type="button" data-testid="card-delete" onClick={() => setCards((cs) => cs.filter((x) => x.id !== c.id))}>
                        Delete
                      </button>
                      <select
                        data-testid="card-move"
                        aria-label={`Move "${c.title}" to column`}
                        value={c.col}
                        onChange={(e) => moveTo(c.id, e.target.value as ColId)}
                      >
                        {COLUMNS.map((o) => (
                          <option key={o.id} value={o.id}>
                            {o.name}
                          </option>
                        ))}
                      </select>
                    </div>
                  </li>
                ))}
              </ul>
            </section>
          );
        })}
      </div>
    </main>
  );
}
