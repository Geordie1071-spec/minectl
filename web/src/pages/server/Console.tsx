import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from "react";
import { api, errorMessage, srv } from "../../api";
import { useLogStream } from "../../hooks";
import { cleanLog } from "../../format";
import { Alert, Button, Input } from "../../ui";
import { can, type TabProps } from "./ServerPage";

function lineClass(line: string) {
  if (/\b(ERROR|FATAL|SEVERE)\b|Exception/.test(line)) return "text-red-400";
  if (/\bWARN(ING)?\b/.test(line)) return "text-amber-300";
  if (line.startsWith("> ")) return "text-green-400";
  return undefined;
}

export function Console({ s }: TabProps) {
  const hasContainer = s.status === "running" || s.status === "stopped";
  const { lines, ended, reconnect, append } = useLogStream(s.name, hasContainer);
  const [cmd, setCmd] = useState("");
  const [history, setHistory] = useState<string[]>([]);
  const [histIdx, setHistIdx] = useState(-1);
  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);
  const [follow, setFollow] = useState(true);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (el && follow) el.scrollTop = el.scrollHeight;
  }, [lines, follow]);

  // Reconnect when the server comes back up after a stop.
  const running = s.status === "running";
  useEffect(() => {
    if (running && ended) reconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [running]);

  const send = async (e: FormEvent) => {
    e.preventDefault();
    const command = cmd.trim();
    if (!command) return;
    setSending(true);
    setError("");
    try {
      const res = await api.post<{ output: string }>(srv(s.name) + "/console", { command });
      append("> " + command);
      if (res.output) res.output.split("\n").forEach(append);
      setHistory((h) => [command, ...h.filter((x) => x !== command)].slice(0, 50));
      setHistIdx(-1);
      setCmd("");
      setFollow(true);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSending(false);
    }
  };

  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowUp" && history.length) {
      e.preventDefault();
      const i = Math.min(histIdx + 1, history.length - 1);
      setHistIdx(i);
      setCmd(history[i] ?? "");
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      const i = histIdx - 1;
      setHistIdx(i);
      setCmd(i >= 0 ? (history[i] ?? "") : "");
    }
  };

  return (
    <div className="space-y-3">
      <div
        ref={ref}
        onScroll={(e) => {
          const el = e.currentTarget;
          setFollow(el.scrollHeight - el.scrollTop - el.clientHeight < 40);
        }}
        className="h-[60vh] min-h-72 overflow-auto rounded-lg bg-console p-3 font-mono text-xs leading-relaxed text-console-fg"
        role="log"
        aria-live="off"
      >
        {!hasContainer && <p className="text-console-fg/60">No logs yet; the server hasn't started.</p>}
        {lines.map((l, i) => (
          <div key={i} className={lineClass(l)} style={{ whiteSpace: "pre-wrap", wordBreak: "break-word" }}>
            {cleanLog(l)}
          </div>
        ))}
        {ended && <p className="mt-2 text-console-fg/60">— server stopped —</p>}
      </div>
      {!follow && (
        <Button variant="ghost" className="text-xs" onClick={() => setFollow(true)}>
          ↓ Jump to latest
        </Button>
      )}
      {can(s, "operator") && (
        <form onSubmit={send} className="flex gap-2">
          <Input
            value={cmd}
            onChange={(e) => setCmd(e.target.value)}
            onKeyDown={onKey}
            placeholder={running ? "Type a command, e.g. say Hello or time set day" : "Start the server to send commands"}
            disabled={!running}
            className="font-mono"
            aria-label="Console command"
            maxLength={1000}
          />
          <Button variant="primary" type="submit" busy={sending} disabled={!running || !cmd.trim()}>
            Send
          </Button>
        </form>
      )}
      {error && <Alert>{error}</Alert>}
    </div>
  );
}
