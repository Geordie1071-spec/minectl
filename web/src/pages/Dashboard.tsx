import { Link } from "react-router-dom";
import { srv, type Server } from "../api";
import { useMe, usePlayers, useServers } from "../hooks";
import { PageTitle } from "../layout";
import { joinAddress } from "../join";
import { formatMB, typeLabels } from "../format";
import { Alert, Button, CopyButton, Empty, Spinner, StatusBadge, TypeTile } from "../ui";
import { jobLabel } from "../JobView";

function ServerCard({ s }: { s: Server }) {
  const running = s.status === "running" && !s.job;
  const players = usePlayers(s.name, running);
  const addr = joinAddress(s.port);
  return (
    <div className="group flex flex-col rounded-xl border border-line bg-surface shadow-card transition hover:-translate-y-0.5 hover:border-accent/40 hover:shadow-pop">
      <Link to={srv(s.name)} className="flex-1 p-5">
        <div className="flex items-start gap-3">
          <TypeTile type={s.type} />
          <div className="min-w-0 flex-1">
            <div className="flex items-center justify-between gap-2">
              <h2 className="truncate font-semibold group-hover:text-accent">{s.name}</h2>
              <StatusBadge status={s.status} busy={s.job ? `${jobLabel(s.job.kind).toLowerCase()}…` : undefined} />
            </div>
            <p className="mt-0.5 truncate text-sm text-muted">
              {s.modpack ? `${s.modpack.id} · ` : ""}
              {typeLabels[s.type] ?? s.type} {s.version}
            </p>
          </div>
        </div>
        <dl className="mt-5 grid grid-cols-3 gap-2 text-sm">
          <div>
            <dt className="text-xs text-muted">Players</dt>
            <dd className="mt-0.5 font-medium">{running && players.data ? `${players.data.online}/${players.data.max}` : "—"}</dd>
          </div>
          <div>
            <dt className="text-xs text-muted">Memory</dt>
            <dd className="mt-0.5 font-medium">{formatMB(s.memory_mb)}</dd>
          </div>
          <div>
            <dt className="text-xs text-muted">Access</dt>
            <dd className="mt-0.5 font-medium capitalize">{s.role}</dd>
          </div>
        </dl>
      </Link>
      <div className="flex items-center justify-between gap-2 rounded-b-xl border-t border-line bg-surface-2/60 px-5 py-2.5 text-sm">
        <code className="truncate text-muted">{addr}</code>
        <CopyButton text={addr} />
      </div>
    </div>
  );
}

function Stat({ label, value, hint }: { label: string; value: string | number; hint?: string }) {
  return (
    <div className="rounded-xl border border-line bg-surface p-3 shadow-card sm:p-5">
      <p className="truncate text-xs text-muted sm:text-sm">{label}</p>
      <p className="mt-1 text-xl font-semibold tracking-tight sm:text-2xl">{value}</p>
      {hint && <p className="mt-0.5 hidden text-xs text-muted sm:block">{hint}</p>}
    </div>
  );
}

export function Dashboard() {
  const { data: me } = useMe();
  const { data, error, isPending } = useServers();
  const isAdmin = me?.user.is_admin;
  return (
    <>
      <PageTitle
        actions={
          isAdmin && (
            <Link to="/new">
              <Button variant="primary">New server</Button>
            </Link>
          )
        }
      >
        Servers
      </PageTitle>
      {error && <Alert>{error.message}</Alert>}
      {isPending && <Spinner />}
      {data && data.length === 0 && (
        <Empty title={isAdmin ? "No servers yet" : "No servers shared with you yet"}>
          {isAdmin ? (
            <>
              Create one from a server type, a Modrinth modpack, or a CurseForge pack.
              <div className="mt-4">
                <Link to="/new">
                  <Button variant="primary">Create your first server</Button>
                </Link>
              </div>
            </>
          ) : (
            "Ask an admin to give you access to a server."
          )}
        </Empty>
      )}
      {data && data.length > 0 && (
        <div className="mb-6 grid grid-cols-3 gap-3 sm:gap-4">
          <Stat label="Servers" value={data.length} />
          <Stat label="Running" value={data.filter((s) => s.status === "running").length} hint={`${data.filter((s) => s.status !== "running").length} stopped`} />
          <Stat
            label="Memory used"
            value={formatMB(data.filter((s) => s.status === "running").reduce((n, s) => n + s.memory_mb, 0))}
            hint="by running servers"
          />
        </div>
      )}
      {data && data.length > 0 && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {data.map((s) => (
            <ServerCard key={s.name} s={s} />
          ))}
        </div>
      )}
    </>
  );
}
