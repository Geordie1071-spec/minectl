import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { srv, type JobEvent } from "./api";
import { useJobStream } from "./hooks";
import { cleanLog } from "./format";
import { Alert, Button, Spinner, cx } from "./ui";

const kindLabels: Record<string, string> = {
  create: "Creating",
  start: "Starting",
  stop: "Stopping",
  restart: "Restarting",
  delete: "Deleting",
  backup: "Backing up",
  restore: "Restoring",
  upgrade: "Upgrading",
  resize: "Resizing",
  modpack: "Changing modpack",
  "mods.add": "Adding mod",
  "mods.remove": "Removing mod",
};

export function jobLabel(kind: string) {
  return kindLabels[kind] ?? kind;
}

function EventLine({ e }: { e: JobEvent }) {
  if (e.type === "warn") return <Alert kind="warn">{e.message}</Alert>;
  if (e.type === "stage") return <p className="animate-fade-up pt-2 text-sm font-medium">{e.message}</p>;
  return <p className="animate-fade-up text-sm whitespace-pre-wrap text-muted">{e.message}</p>;
}

export function JobView({ id, compact }: { id: string; compact?: boolean }) {
  const { job, events, progress, stage, done } = useJobStream(id);
  const [showLog, setShowLog] = useState(!compact);
  const logRef = useRef<HTMLPreElement>(null);

  const logs = events.filter((e) => e.type === "log");
  const notes = events.filter((e) => e.type !== "log" && e.type !== "done" && e.type !== "error");
  const status = done ? job?.status : "running";
  const lastLog = logs.length ? cleanLog(logs[logs.length - 1]!.message ?? "") : "";

  useEffect(() => {
    const el = logRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [logs.length, showLog]);

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        {status === "running" && <Spinner className="h-5 w-5 text-accent" />}
        {status === "succeeded" && <span className="flex h-5 w-5 animate-scale-in items-center justify-center rounded-full bg-ok text-xs text-white">✓</span>}
        {status === "failed" && <span className="flex h-5 w-5 animate-scale-in items-center justify-center rounded-full bg-danger text-xs text-white">!</span>}
        <div className="min-w-0">
          <p className="font-medium">
            {job ? `${jobLabel(job.kind)} ${job.server}` : "Loading…"}
            {status === "succeeded" && " — done"}
            {status === "failed" && " — failed"}
          </p>
          {status === "running" && (stage || lastLog) && (
            <p className="truncate text-sm text-muted">
              {stage}
              {stage && lastLog && " · "}
              <span className="font-mono text-xs">{lastLog}</span>
            </p>
          )}
        </div>
      </div>

      {progress && status === "running" && (
        <div>
          <div className="mb-1 flex justify-between text-xs text-muted">
            <span>{progress.message}</span>
            {progress.percent !== undefined && <span>{progress.percent}%</span>}
          </div>
          <div className="h-2 overflow-hidden rounded-full bg-surface-2">
            <div className="h-full rounded-full bg-accent transition-[width] duration-500 ease-out" style={{ width: `${progress.percent ?? 5}%` }} />
          </div>
        </div>
      )}

      {notes.length > 0 && (
        <div className="space-y-1">
          {notes.map((e) => (
            <EventLine key={e.seq} e={e} />
          ))}
        </div>
      )}

      {status === "failed" && job?.error && <Alert>{job.error}</Alert>}
      {status === "succeeded" && (
        <Alert kind="info">{events.find((e) => e.type === "done")?.message || "Finished."}</Alert>
      )}

      {logs.length > 0 && (
        <div>
          <Button variant="ghost" className="px-0 text-xs text-muted" onClick={() => setShowLog((v) => !v)}>
            {showLog ? "Hide" : "Show"} server output ({logs.length} lines)
          </Button>
          {showLog && (
            <pre
              ref={logRef}
              className="mt-2 max-h-80 overflow-auto rounded-md bg-console p-3 font-mono text-xs leading-relaxed text-console-fg"
            >
              {logs.map((e) => cleanLog(e.message ?? "")).join("\n")}
            </pre>
          )}
        </div>
      )}

      {done && job && job.kind !== "delete" && !compact && (
        <Link to={srv(job.server)}>
          <Button variant={status === "succeeded" ? "primary" : "secondary"}>Go to {job.server}</Button>
        </Link>
      )}
    </div>
  );
}

export function JobBanner({ id, onClose }: { id: string; onClose?: () => void }) {
  const { done, job } = useJobStream(id);
  return (
    <div className={cx("rounded-xl border bg-surface-2 p-5 shadow-card", done && job?.status === "failed" ? "border-danger/40" : "border-line")}>
      <JobView id={id} compact />
      {done && onClose && (
        <div className="mt-3 flex justify-end">
          <Button variant="ghost" onClick={onClose}>
            Dismiss
          </Button>
        </div>
      )}
    </div>
  );
}
