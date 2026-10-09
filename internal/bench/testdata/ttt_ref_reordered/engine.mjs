const LINES = [[0,1,2],[3,4,5],[6,7,8],[0,3,6],[1,4,7],[2,5,8],[0,4,8],[2,4,6]];
export function winner(s) {
  for (const [a,b,c] of LINES) if (s.board[a] && s.board[a] === s.board[b] && s.board[a] === s.board[c]) return s.board[a];
  return s.board.every(Boolean) ? "draw" : null;
}
export function newGame() { return { winner: null, turn: "X", board: Array(9).fill(null) }; }
export function move(s, i) {
  if (!Number.isInteger(i) || i < 0 || i > 8 || s.board[i] || winner(s)) throw new Error("invalid move");
  const board = s.board.slice(); board[i] = s.turn;
  const n = { winner: null, turn: s.turn === "X" ? "O" : "X", board };
  n.winner = winner(n); return n;
}
export function legalMoves(s) { return winner(s) ? [] : s.board.flatMap((c, i) => (c ? [] : [i])); }
function score(s, me) {
  const w = winner(s);
  if (w === "draw") return 0; if (w) return w === me ? 1 : -1;
  const vals = legalMoves(s).map((i) => score(move(s, i), me));
  return s.turn === me ? Math.max(...vals) : Math.min(...vals);
}
export function bestMove(s) {
  let best = -2, idx = -1;
  for (const i of legalMoves(s)) { const v = score(move(s, i), s.turn); if (v > best) { best = v; idx = i; } }
  if (idx < 0) throw new Error("invalid move"); return idx;
}
export function render(s) { return [0,3,6].map((r) => s.board.slice(r, r+3).map((c) => c || ".").join(" ")).join("\n"); }
