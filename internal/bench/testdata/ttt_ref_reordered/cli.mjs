import { newGame, move, bestMove, render, winner } from "./engine.mjs";
let args = process.argv.slice(2); const best = args[0] === "--best"; if (best) args = args.slice(1);
try {
  let s = newGame(); for (const a of args) s = move(s, Number(a));
  if (best) console.log(bestMove(s)); else console.log(render(s) + "\nwinner: " + (winner(s) ?? "none"));
} catch (e) { console.error("error: " + e.message); process.exit(1); }
