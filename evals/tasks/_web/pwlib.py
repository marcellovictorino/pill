"""Shared helpers for the web-* verifiers: build the agent's app, serve it, run Node Playwright scripts."""
import contextlib
import json
import socket
import subprocess
import time
import urllib.request
from pathlib import Path

WEB = Path(__file__).resolve().parent


def build(ws):
    """Run the agent's own `npm run build` (tsc --noEmit && vite build). Returns (ok, log tail)."""
    r = subprocess.run(["npm", "run", "build"], cwd=ws, capture_output=True, text=True, timeout=300)
    return r.returncode == 0, (r.stdout + r.stderr)[-1500:]


def strict_enabled(ws):
    """The scaffold ships strict TypeScript; an agent must not switch it off."""
    try:
        text = (Path(ws) / "tsconfig.json").read_text()
        return '"strict": true' in text.replace('"strict":true', '"strict": true')
    except OSError:
        return False


def _free_port():
    with contextlib.closing(socket.socket()) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


@contextlib.contextmanager
def preview(ws):
    """Serve the built dist with `vite preview` on a free port; yields the base URL."""
    port = _free_port()
    p = subprocess.Popen(["node_modules/.bin/vite", "preview", "--host", "127.0.0.1", "--port", str(port),
                          "--strictPort"], cwd=ws, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                         start_new_session=True)
    url = f"http://127.0.0.1:{port}/"
    try:
        for _ in range(100):
            try:
                urllib.request.urlopen(url, timeout=1).read()
                break
            except Exception:
                if p.poll() is not None:
                    raise RuntimeError("vite preview exited")
                time.sleep(0.2)
        else:
            raise RuntimeError("vite preview did not start")
        yield url
    finally:
        import os
        import signal
        with contextlib.suppress(ProcessLookupError):
            os.killpg(p.pid, signal.SIGKILL)
        p.wait()


def run_node(script, url, extra=None, timeout=240, env=None):
    """Run a Playwright Node script from the shared toolchain dir; it prints one JSON line {checks:[...]}."""
    import os
    e = dict(os.environ)
    e.update(env or {})
    r = subprocess.run(["node", str(WEB / script), url, json.dumps(extra or {})], cwd=WEB,
                       capture_output=True, text=True, timeout=timeout, env=e)
    line = next((ln for ln in reversed(r.stdout.strip().splitlines()) if ln.startswith("{")), None)
    if line is None:
        raise RuntimeError("node script produced no result: " + (r.stderr or r.stdout)[-1500:])
    return json.loads(line)["checks"]
