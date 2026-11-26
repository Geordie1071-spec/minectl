import { useState } from "react";
import { Link } from "react-router-dom";
import { srv, type Server } from "../api";
import { useMe, usePlayers, useServers } from "../hooks";
import { joinAddress } from "../join";
import { formatMB, timeAgo, typeLabels } from "../format";
import { jobLabel } from "../JobView";
import { Alert, Button, CopyButton, Empty, Skeleton, StatusBadge, TypeTile } from "../ui";

function ServerCard({ s }: { s: Server }) {
  const running = s.status === "running" && !s.job;
  const players = usePlayers(s.name, running);
  const addr = joinAddress(s.port);
  return (
    <Link
      to={srv(s.name)}
      className="group flex flex-col rounded-lg border border-line bg-surface p-5 shadow-card transition-[border-color,box-shadow,transform] duration-200 hover:-translate-y-0.5 hover:border-line-strong hover:shadow-pop"
    >
      <div className="flex items-start gap-3">
        <TypeTile type={s.type} className="h-8 w-8 text-[13px] transition-transform duration-300 group-hover:scale-110 group-hover:-rotate-6" />
        <div className="min-w-0 flex-1">
          <h2 className="truncate leading-tight font-semibold">{s.name}</h2>
          <div className="mt-0.5 flex min-w-0 items-center gap-1 text-[13px] text-muted">
            <span className="truncate font-mono">{addr}</span>
            <CopyButton text={addr} label="Copy address" />
          </div>
        </div>
        <span className="text-[13px] text-subtle">{s.role !== "admin" ? s.role : ""}</span>
      </div>
      <div className="mt-5 flex items-center gap-2 text-[13px] text-muted">
        <span className="truncate">
          {s.modpack ? s.modpack.id : typeLabels[s.type] ?? s.type} {s.modpack ? "" : s.version}
        </span>
      </div>
      <div className="mt-3 flex items-center justify-between gap-2 border-t border-line pt-3">
        <StatusBadge status={s.status} busy={s.job ? `${jobLabel(s.job.kind)}…` : undefined} />
        <span className="text-[13px] text-muted tabular-nums">
          {running && players.data ? `${players.data.online}/${players.data.max} online` : `started ${timeAgo(s.last_started_at)}`}
        </span>
      </div>
    </Link>
  );
}

function UsageRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between py-2 text-sm">
      <span className="text-muted">{label}</span>
      <span className="font-medium tabular-nums">{value}</span>
    </div>
  );
}

function Usage({ servers }: { servers: Server[] }) {
  const running = servers.filter((s) => s.status === "running");
  return (
    <section className="rounded-lg border border-line bg-surface p-5 shadow-card">
      <h2 className="text-sm font-semibold">Usage</h2>
      <p className="mt-0.5 text-[13px] text-muted">Across all servers you can see</p>
      <div className="mt-3 divide-y divide-line">
        <UsageRow label="Servers" value={String(servers.length)} />
        <UsageRow label="Running" value={`${running.length} of ${servers.length}`} />
        <UsageRow label="Memory reserved" value={formatMB(running.reduce((n, s) => n + s.memory_mb, 0))} />
        <UsageRow label="CPU cores reserved" value={String(running.reduce((n, s) => n + (s.cpu_cores || 0), 0))} />
      </div>
    </section>
  );
}

export function Dashboard() {
  const { data: me } = useMe();
  const { data, error, isPending } = useServers();
  const [q, setQ] = useState("");
  const isAdmin = me?.user.is_admin;
  const shown = data?.filter((s) => (s.name + " " + s.type + " " + (s.modpack?.id ?? "")).toLowerCase().includes(q.toLowerCase()));

  return (
    <>
      <div className="mb-6 flex items-center gap-3">
        <div className="relative flex-1">
          <svg viewBox="0 0 16 16" className="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-subtle" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
            <circle cx="7" cy="7" r="4.5" />
            <path d="m10.5 10.5 3 3" strokeLinecap="round" />
          </svg>
          <input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Search servers…"
            aria-label="Search servers"
            className="h-10 w-full rounded-md border border-line bg-surface pr-3 pl-9 text-sm placeholder:text-subtle hover:border-line-strong focus:border-fg/40 focus:ring-3 focus:ring-fg/8 focus:outline-none"
          />
        </div>
        {isAdmin && (
          <Link to="/new">
            <Button variant="primary" className="h-10">
              New server
            </Button>
          </Link>
        )}
      </div>

      {error && <Alert>{error.message}</Alert>}
      {isPending && (
        <div className="grid gap-6 lg:grid-cols-[17rem_minmax(0,1fr)]">
          <Skeleton className="h-52" />
          <div className="grid gap-4 sm:grid-cols-2">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-40" />
            ))}
          </div>
        </div>
      )}

      {data && data.length === 0 && (
        <Empty
          title={isAdmin ? "Create your first server" : "No servers shared with you yet"}
          icon={<img src="/mark.png" alt="" className="pixelated h-12 w-12" />}
        >
          {isAdmin ? (
            <>
              Start from a server type, a Modrinth modpack, or a CurseForge pack. It's ready to join in about a minute.
              <div className="mt-5">
                <Link to="/new">
                  <Button variant="primary">New server</Button>
                </Link>
              </div>
            </>
          ) : (
            "Ask an admin to give you access to a server."
          )}
        </Empty>
      )}

      {data && data.length > 0 && (
        <div className="grid gap-6 lg:grid-cols-[17rem_minmax(0,1fr)]">
          <aside className="order-2 animate-fade-up space-y-6 lg:order-none">
            <Usage servers={data} />
          </aside>
          <div>
            <h2 className="mb-3 text-sm font-semibold">Servers</h2>
            {shown && shown.length === 0 && <p className="text-sm text-muted">No servers match “{q}”.</p>}
            <div className="stagger grid gap-4 sm:grid-cols-2">
              {shown?.map((s) => (
                <ServerCard key={s.name} s={s} />
              ))}
            </div>
          </div>
        </div>
      )}
    </>
  );
}
