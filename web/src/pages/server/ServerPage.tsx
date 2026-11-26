import { useState } from "react";
import { Route, Routes, useLocation, useParams } from "react-router-dom";
import { api, errorMessage, srv, type Job, type Server } from "../../api";
import { useServer } from "../../hooks";
import { PageTitle } from "../../layout";
import { joinAddress } from "../../join";
import { typeLabels } from "../../format";
import { JobBanner, jobLabel } from "../../JobView";
import { Alert, Button, CopyButton, Skeleton, StatusBadge, TypeTile } from "../../ui";
import { Overview } from "./Overview";
import { Console } from "./Console";
import { PlayersTab } from "./Players";
import { Backups } from "./Backups";
import { Mods } from "./Mods";
import { Settings } from "./Settings";

export interface TabProps {
  s: Server;
  runJob: (path: string, method?: "post" | "put" | "del", body?: unknown) => Promise<void>;
}

const rank = { viewer: 1, operator: 2, admin: 3 } as const;
export const can = (s: Server, role: keyof typeof rank) => rank[s.role] >= rank[role];

export function ServerPage() {
  const { name = "" } = useParams();
  const location = useLocation();
  const { data: s, error, isPending, refetch } = useServer(name);
  const [localJob, setLocalJob] = useState<string>();
  const [actionError, setActionError] = useState("");

  if (isPending) {
    return (
      <div className="space-y-6">
        <div className="flex items-center gap-4">
          <Skeleton className="h-12 w-12" />
          <div className="space-y-2">
            <Skeleton className="h-7 w-48" />
            <Skeleton className="h-4 w-64" />
          </div>
        </div>
        <Skeleton className="h-52" />
      </div>
    );
  }
  if (error || !s) {
    return (
      <>
        <PageTitle>{name}</PageTitle>
        <Alert>{error?.message ?? "Server not found"}</Alert>
      </>
    );
  }

  const runJob: TabProps["runJob"] = async (path, method = "post", body) => {
    setActionError("");
    try {
      const url = srv(s.name) + path;
      const job = method === "del" ? await api.del<Job>(url) : await api[method]<Job>(url, body);
      setLocalJob(job.id);
      refetch();
    } catch (err) {
      setActionError(errorMessage(err));
      throw err;
    }
  };

  const jobId = s.job?.id ?? localJob;
  const busy = !!s.job;
  const running = s.status === "running";
  const addr = joinAddress(s.port);

  return (
    <>
      <div className="mb-8 flex flex-wrap items-center justify-between gap-4">
        <div className="flex min-w-0 items-center gap-4">
          <TypeTile type={s.type} className="h-12 w-12 animate-scale-in rounded-lg text-lg" />
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <h1 className="truncate text-[28px] leading-tight font-semibold tracking-tight">{s.name}</h1>
              <StatusBadge status={s.status} busy={s.job ? `${jobLabel(s.job.kind)}…` : undefined} />
            </div>
            <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted">
              <span>
                {s.modpack ? `${s.modpack.id} · ` : ""}
                {typeLabels[s.type] ?? s.type} {s.version}
              </span>
              <span className="text-line-strong" aria-hidden="true">
                •
              </span>
              <span className="flex items-center gap-1 font-mono text-[13px] text-fg">
                {addr}
                <CopyButton text={addr} label="Copy address" />
              </span>
            </div>
          </div>
        </div>
        {can(s, "operator") && (
          <div className="flex items-center gap-2">
            {running ? (
              <>
                <Button disabled={busy} onClick={() => runJob("/restart").catch(() => {})}>
                  Restart
                </Button>
                <Button disabled={busy} onClick={() => runJob("/stop").catch(() => {})}>
                  Stop
                </Button>
              </>
            ) : (
              <Button variant="primary" disabled={busy} onClick={() => runJob("/start").catch(() => {})}>
                Start server
              </Button>
            )}
          </div>
        )}
      </div>

      {actionError && (
        <div className="mb-6">
          <Alert>{actionError}</Alert>
        </div>
      )}
      {jobId && (
        <div className="mb-6">
          <JobBanner key={jobId} id={jobId} onClose={s.job ? undefined : () => setLocalJob(undefined)} />
        </div>
      )}

      <div key={location.pathname} className="animate-fade-up">
      <Routes>
        <Route index element={<Overview s={s} runJob={runJob} />} />
        <Route path="console" element={<Console s={s} runJob={runJob} />} />
        <Route path="players" element={<PlayersTab s={s} runJob={runJob} />} />
        <Route path="backups" element={<Backups s={s} runJob={runJob} />} />
        <Route path="mods" element={<Mods s={s} runJob={runJob} />} />
        <Route path="settings" element={<Settings s={s} runJob={runJob} />} />
      </Routes>
      </div>
    </>
  );
}
