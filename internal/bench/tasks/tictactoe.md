Build a tic-tac-toe engine and a small command-line tool in this directory. Use plain Node.js ES modules (the files end in .mjs), no dependencies, no build step. Create exactly these two files: `engine.mjs` and `cli.mjs`.

## engine.mjs

It exports these named functions. A game state is a plain object `{ board, turn, winner }`:
- `board`: array of 9 cells, index 0 is top-left, 2 is top-right, 6 is bottom-left, 8 is bottom-right; each cell is `null`, `"X"` or `"O"`.
- `turn`: `"X"` or `"O"`, the player to move. X always moves first.
- `winner`: `null` while the game is running, `"X"` or `"O"` when that player has three in a row, `"draw"` when the board is full with no winner.

Functions:
- `newGame()` returns a fresh state: empty board, `turn: "X"`, `winner: null`.
- `move(state, index)` returns a NEW state with the current player's mark placed at `index` and the turn switched, with `winner` updated. It must not modify the state passed in. It throws an `Error` whose message is exactly `invalid move` when `index` is not an integer from 0 to 8, when the cell is already taken, or when the game is already over.
- `winner(state)` returns `"X"`, `"O"`, `"draw"` or `null` for the given state, computed from the board.
- `legalMoves(state)` returns the free cell indices in ascending order, or an empty array when the game is over.
- `bestMove(state)` returns the index of the best move for the player to move, using minimax so the player never loses when a draw or win is possible. Among equally good moves, return the lowest index. It throws `invalid move` when the game is over.
- `render(state)` returns the board as three lines joined by `"\n"` with no trailing newline. Each line has three cells separated by single spaces; an empty cell is `.`. Example: `"X . O\n. X .\nO . X"`.

## cli.mjs

Run as `node cli.mjs <moves...>`: each argument is a cell index; moves are played alternately starting with X. After applying all moves it prints the rendered board, then a final line `winner: X`, `winner: O`, `winner: draw` or `winner: none`, and exits with code 0.
If any move is invalid it prints `error: invalid move` to standard error, prints nothing to standard output, and exits with code 1.
Run as `node cli.mjs --best <moves...>`: it applies the moves the same way and prints only the index chosen by `bestMove` for the player to move (exit code 0; the same error handling as above, which also applies when the game is already over).

## How to work

Write the two files, then run them with node to check your work (for example play a few games with the CLI and try invalid moves) and fix anything that is wrong. When both files behave exactly as described, stop and reply with a one-line summary.
