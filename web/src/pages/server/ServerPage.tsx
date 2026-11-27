import { useState } from "react";
import { Route, Routes, useLocation, useParams } from "react-router-dom";
import { api, errorMessage, srv, type Job, type Server } from "../../api";
import { useServer } from "../../hooks";
import { PageTitle, Tabs, type Tab } from "../../layout";
import { useJoinAddress } from "../../join";
import { isModded, typeLabels } from "../../format";
import { JobBanner, jobLabel } from "../../JobView";
import { Alert, Button, CopyButton, Skeleton, StatusBadge } from "../../ui";
import { Box, ChevronRight, Play, RotateCw, Square } from "lucide-react";
import { Link } from "react-router-dom";
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

function serverTabs(s: Server): Tab[] {
  const base = srv(s.name);
  const tabs: Tab[] = [
    { to: base, label: "Overview", end: true },
    { to: base + "/console", label: "Console" },
    { to: base + "/players", label: "Players" },
    { to: base + "/backups", label: "Backups" },
  ];
  if (can(s, "admin") && (isModded(s.type) || s.modpack)) tabs.push({ to: base + "/mods", label: "Mods" });
  if (can(s, "admin")) tabs.push({ to: base + "/settings", label: "Settings" });
  return tabs;
}

export function ServerPage() {
  const { name = "" } = useParams();
  const location = useLocation();
  const { data: s, error, isPending, refetch } = useServer(name);
  const { address: addr } = useJoinAddress(s?.port ?? 25565, s?.public);
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

  return (
    <>
      <div className="mb-5 flex items-center gap-1 text-[13px] text-muted">
        <Link to="/" className="hover:text-fg">
          Servers
        </Link>
        <ChevronRight className="h-3.5 w-3.5 text-subtle" />
        <span className="text-fg">{s.name}</span>
      </div>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <div className="flex min-w-0 items-center gap-4">
          <span className="flex h-12 w-12 shrink-0 animate-scale-in items-center justify-center rounded-xl border border-line-strong bg-surface-2">
            <Box className="h-6 w-6" strokeWidth={1.5} />
          </span>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <h1 className="truncate text-[28px] leading-tight font-medium tracking-tight">{s.name}</h1>
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
                  <RotateCw className="h-4 w-4" strokeWidth={1.75} /> Restart
                </Button>
                <Button disabled={busy} onClick={() => runJob("/stop").catch(() => {})}>
                  <Square className="h-3.5 w-3.5" strokeWidth={1.75} /> Stop
                </Button>
              </>
            ) : (
              <Button variant="primary" disabled={busy} onClick={() => runJob("/start").catch(() => {})}>
                <Play className="h-4 w-4" strokeWidth={2} /> Start server
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

      <div className="mb-8">
        <Tabs tabs={serverTabs(s)} />
      </div>
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
