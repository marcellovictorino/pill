// Hidden verifier for the tic-tac-toe task. pill writes this file into the
// workspace after the agent has finished and runs it with node. It prints
// {"score": n, "total": m, "checks": [{"name", "pass", "detail"}]}.
import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

const checks = [];
function check(name, fn) {
  try {
    const detail = fn();
    checks.push({ name, pass: detail === undefined || detail === true, detail: detail === undefined || detail === true ? "" : String(detail) });
  } catch (e) {
    checks.push({ name, pass: false, detail: String((e && e.message) || e) });
  }
}
// Structural equality: array order matters, object property order does not
// (a state returned as {turn, board, winner} is as valid as {board, turn, winner}).
const eq = (a, b) => {
  if (a === b) return true;
  if (Array.isArray(a) || Array.isArray(b)) {
    // An index loop, not .every(): every() skips holes, so [1,,3] would pass for [1,2,3].
    if (!Array.isArray(a) || !Array.isArray(b) || a.length !== b.length) return false;
    for (let i = 0; i < a.length; i++) if (!eq(a[i], b[i])) return false;
    return true;
  }
  if (a && b && typeof a === "object" && typeof b === "object") {
    const ka = Object.keys(a), kb = Object.keys(b);
    return ka.length === kb.length && ka.every((k) => Object.prototype.hasOwnProperty.call(b, k) && eq(a[k], b[k]));
  }
  return false;
};
const expectEq = (got, want) => (eq(got, want) ? true : `got ${JSON.stringify(got)}, want ${JSON.stringify(want)}`);
const throwsInvalid = (fn) => {
  try { fn(); } catch (e) { return e && e.message === "invalid move" ? true : `threw ${JSON.stringify(e && e.message)}`; }
  return "did not throw";
};

let E = null;
let loadError = "engine.mjs missing";
try {
  E = await import(pathToFileURL(resolve("engine.mjs")).href);
} catch (e) {
  loadError = String((e && e.message) || e);
}

const play = (...idx) => idx.reduce((s, i) => E.move(s, i), E.newGame());

const names = [
  "newGame shape", "X moves first and turn alternates", "move does not mutate", "occupied cell throws",
  "out of range index throws", "non-integer index throws", "row win", "column win", "diagonal win",
  "draw detection", "move after game over throws", "legalMoves", "winner(state) matches state", "bestMove takes the win",
  "bestMove blocks", "bestMove lowest index tie-break", "bestMove replies to centre with a corner", "render format",
];

if (E) {
  check(names[0], () => expectEq(E.newGame(), { board: Array(9).fill(null), turn: "X", winner: null }));
  check(names[1], () => { const s = play(4); return expectEq([s.turn, s.board[4], play(4, 0).turn, play(4, 0).board[0]], ["O", "X", "X", "O"]); });
  check(names[2], () => { const s = E.newGame(); E.move(s, 0); return expectEq(s, { board: Array(9).fill(null), turn: "X", winner: null }); });
  check(names[3], () => throwsInvalid(() => play(4, 4)));
  check(names[4], () => throwsInvalid(() => E.move(E.newGame(), 9)) === true && throwsInvalid(() => E.move(E.newGame(), -1)));
  check(names[5], () => throwsInvalid(() => E.move(E.newGame(), 1.5)) === true && throwsInvalid(() => E.move(E.newGame(), "3")));
  check(names[6], () => { const s = play(0, 3, 1, 4, 2); return expectEq([s.winner, E.winner(s)], ["X", "X"]); });
  check(names[7], () => { const s = play(0, 1, 3, 4, 8, 7); return expectEq([s.winner, E.winner(s)], ["O", "O"]); });
  check(names[8], () => { const s = play(2, 0, 4, 1, 6); return expectEq([s.winner, E.winner(s)], ["X", "X"]); });
  check(names[9], () => { const s = play(0, 1, 2, 4, 3, 5, 7, 6, 8); return expectEq([s.winner, E.winner(s), E.legalMoves(s)], ["draw", "draw", []]); });
  check(names[10], () => throwsInvalid(() => E.move(play(0, 3, 1, 4, 2), 8)));
  check(names[11], () => expectEq(E.legalMoves(play(4, 0, 8)), [1, 2, 3, 5, 6, 7]));
  check(names[12], () => expectEq([E.winner(E.newGame()), E.winner(play(0, 3, 1, 4))], [null, null]));
  check(names[13], () => expectEq(E.bestMove(play(0, 3, 1, 4)), 2));
  check(names[14], () => expectEq(E.bestMove(play(0, 4, 1)), 2));
  check(names[15], () => expectEq(E.bestMove(E.newGame()), 0));
  check(names[16], () => expectEq(E.bestMove(play(4)), 0));
  check(names[17], () => expectEq(E.render(play(0, 4, 8, 2, 6)), "X . O\n. O .\nX . X"));
} else {
  for (const n of names) checks.push({ name: n, pass: false, detail: loadError });
}

function cli(...args) {
  if (!existsSync("cli.mjs")) return { status: -1, stdout: "", stderr: "cli.mjs missing" };
  return spawnSync("node", ["cli.mjs", ...args], { encoding: "utf8", timeout: 15000 });
}
check("cli plays a game to a win", () => {
  const r = cli("0", "3", "1", "4", "2");
  return expectEq([r.status, r.stdout.trimEnd()], [0, "X X X\nO O .\n. . .\nwinner: X"]);
});
check("cli reports no winner mid-game", () => {
  const r = cli("4");
  return expectEq([r.status, r.stdout.trimEnd()], [0, ". . .\n. X .\n. . .\nwinner: none"]);
});
check("cli invalid move: stderr, exit 1, no stdout", () => {
  const r = cli("4", "4");
  return expectEq([r.status, r.stdout, r.stderr.trim()], [1, "", "error: invalid move"]);
});
check("cli --best prints the chosen index", () => {
  const r = cli("--best", "0", "3", "1", "4");
  return expectEq([r.status, r.stdout.trim()], [0, "2"]);
});

const score = checks.filter((c) => c.pass).length;
console.log(JSON.stringify({ score, total: checks.length, checks }));
