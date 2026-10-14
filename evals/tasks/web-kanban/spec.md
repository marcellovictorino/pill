Build a single-page kanban board (like a small Trello) in this directory. It is a Vite + React + TypeScript project that is already scaffolded: `src/main.tsx` renders `src/App.tsx`, and `node_modules` is already installed. Do NOT run `npm install` and do not add dependencies: use only react and react-dom (everything else must be plain CSS and TypeScript). Keep `tsconfig.json` strict. `npm run build` (which runs `tsc --noEmit && vite build`) must pass. Write your code under `src/`; you may add files and CSS there.

Run `npm run build` to check your work, and use `npm run dev` or `npm run preview` if you want to try it (a browser is not available to you, so reason carefully about behaviour and keep the code simple).

## Behaviour

The board has exactly three columns, in this order: "To do" (id `todo`), "Doing" (id `doing`), "Done" (id `done`). On first load the board is empty (no seeded cards).

- Add: each column has a text input and an Add button. Submitting (button click, or Enter in the input) appends a card with that title at the bottom of the column and clears the input. The title is trimmed; an empty or whitespace-only title creates nothing.
- Edit: each card has an Edit button. Clicking it replaces the title with a text input prefilled with the title and focused. Enter or the Save button saves, Escape cancels. A title that is empty after trimming is rejected (the old title is kept).
- Delete: each card has a Delete button that removes the card immediately (no confirm dialog).
- Move: each card has a select with the three columns; choosing another column moves the card to the end of that column.
- Drag and drop: cards are draggable (HTML5 drag and drop) and can be dropped on any column, which moves the card to the end of that column.
- Filter: a single search input above the board filters the cards of all columns by title (case-insensitive substring). Non-matching cards are hidden, not deleted; clearing the input shows them again.
- Counts: each column heading shows the number of cards currently shown in that column (so it respects the filter).
- Empty state: a column that currently shows no cards displays the text "No cards".
- Persistence: the board (cards, columns, order) survives a page reload, using `localStorage`.
- Robustness: titles may be long (they must wrap and never cause horizontal scrolling at a 390px wide viewport) or contain unicode/emoji; the page must not log console errors; all controls must be reachable and usable with the keyboard and have accessible names (labels or aria-labels); aim for no accessibility violations (sufficient colour contrast, labelled inputs, a single `h1`).

## Test contract (use exactly these `data-testid` values)

- `board`: the element wrapping the three columns.
- `column-todo`, `column-doing`, `column-done`: the column elements, in that DOM order; each contains a heading with the column name.
- `column-count-<id>`: element whose text is only the number of cards shown in that column, e.g. `column-count-todo`.
- `column-empty-<id>`: the "No cards" element, present only while that column shows no cards.
- `add-input-<id>` (the text input) and `add-submit-<id>` (the Add button) inside each column, e.g. `add-input-doing`.
- `filter-input`: the search input.
- `card`: each card element (inside its column element).
- Inside a card: `card-title` (the title text), `card-edit` (Edit button), `card-delete` (Delete button), `card-move` (the select; option values are `todo`, `doing`, `done`).
- While editing: `card-edit-input` (the text input) and `card-edit-save` (the Save button), inside the card.

When you are done, stop and reply with a one-line summary.
