import { useState } from "react";
import { NavLink, Route, Routes, useParams } from "react-router-dom";
import { api, errorMessage, srv, type Job, type Server } from "../../api";
import { useServer } from "../../hooks";
import { PageTitle } from "../../layout";
import { joinAddress } from "../../join";
import { isModded, typeLabels } from "../../format";
import { JobBanner, jobLabel } from "../../JobView";
import { Alert, Button, CopyButton, Spinner, StatusBadge, cx } from "../../ui";
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

function Tab({ to, children }: { to: string; children: string }) {
  return (
    <NavLink
      to={to}
      end
      className={({ isActive }) =>
        cx(
          "-mb-px border-b-2 px-3 py-2 text-sm font-medium whitespace-nowrap",
          isActive ? "border-accent text-fg" : "border-transparent text-muted hover:text-fg",
        )
      }
    >
      {children}
    </NavLink>
  );
}

export function ServerPage() {
  const { name = "" } = useParams();
  const { data: s, error, isPending, refetch } = useServer(name);
  const [localJob, setLocalJob] = useState<string>();
  const [actionError, setActionError] = useState("");

  if (isPending) return <Spinner />;
  if (error || !s) {
    return (
      <>
        <PageTitle back={{ to: "/", label: "Servers" }}>{name}</PageTitle>
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
  const base = srv(s.name);

  return (
    <>
      <PageTitle
        back={{ to: "/", label: "Servers" }}
        actions={
          can(s, "operator") && (
            <>
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
                  Start
                </Button>
              )}
            </>
          )
        }
      >
        <span className="flex flex-wrap items-center gap-3">
          {s.name}
          <StatusBadge status={s.status} busy={s.job ? `${jobLabel(s.job.kind).toLowerCase()}…` : undefined} />
        </span>
      </PageTitle>

      <div className="-mt-3 mb-5 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-muted">
        <span>
          {s.modpack ? `${s.modpack.id} · ` : ""}
          {typeLabels[s.type] ?? s.type} {s.version}
        </span>
        <span className="flex items-center gap-1">
          Join: <code className="text-fg">{addr}</code>
          <CopyButton text={addr} />
        </span>
      </div>

      {actionError && (
        <div className="mb-4">
          <Alert>{actionError}</Alert>
        </div>
      )}
      {jobId && (
        <div className="mb-5">
          <JobBanner key={jobId} id={jobId} onClose={s.job ? undefined : () => setLocalJob(undefined)} />
        </div>
      )}

      <nav className="mb-5 flex overflow-x-auto border-b border-line">
        <Tab to={base}>Overview</Tab>
        <Tab to={base + "/console"}>Console</Tab>
        <Tab to={base + "/players"}>Players</Tab>
        <Tab to={base + "/backups"}>Backups</Tab>
        {can(s, "admin") && (isModded(s.type) || s.modpack) && <Tab to={base + "/mods"}>Mods</Tab>}
        {can(s, "admin") && <Tab to={base + "/settings"}>Settings</Tab>}
      </nav>

      <Routes>
        <Route index element={<Overview s={s} runJob={runJob} />} />
        <Route path="console" element={<Console s={s} runJob={runJob} />} />
        <Route path="players" element={<PlayersTab s={s} runJob={runJob} />} />
        <Route path="backups" element={<Backups s={s} runJob={runJob} />} />
        <Route path="mods" element={<Mods s={s} runJob={runJob} />} />
        <Route path="settings" element={<Settings s={s} runJob={runJob} />} />
      </Routes>
    </>
  );
}
