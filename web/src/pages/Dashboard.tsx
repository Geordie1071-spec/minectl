import { Link } from "react-router-dom";
import { srv, type Server } from "../api";
import { useMe, usePlayers, useServers } from "../hooks";
import { PageTitle } from "../layout";
import { joinAddress } from "../join";
import { formatMB, typeLabels } from "../format";
import { Alert, Badge, Button, CopyButton, Empty, Spinner, StatusBadge } from "../ui";

function ServerCard({ s }: { s: Server }) {
  const running = s.status === "running" && !s.job;
  const players = usePlayers(s.name, running);
  const addr = joinAddress(s.port);
  return (
    <div className="flex flex-col rounded-lg border border-line bg-surface transition-shadow hover:shadow-md">
      <Link to={srv(s.name)} className="flex-1 p-4">
        <div className="flex items-start justify-between gap-2">
          <h2 className="truncate font-semibold">{s.name}</h2>
          <StatusBadge status={s.status} busy={s.job ? `${s.job.kind}…` : undefined} />
        </div>
        <p className="mt-1 truncate text-sm text-muted">
          {s.modpack ? s.modpack.id : `${typeLabels[s.type] ?? s.type} ${s.version}`}
          {s.modpack && <span> · {typeLabels[s.type] ?? s.type} {s.version}</span>}
        </p>
        <div className="mt-4 flex flex-wrap gap-2 text-xs">
          <Badge>{formatMB(s.memory_mb)} RAM</Badge>
          {running && players.data && (
            <Badge>
              {players.data.online}/{players.data.max} players
            </Badge>
          )}
          {s.role !== "admin" && <Badge>{s.role}</Badge>}
        </div>
      </Link>
      <div className="flex items-center justify-between gap-2 border-t border-line px-4 py-2 text-sm">
        <code className="truncate text-muted">{addr}</code>
        <CopyButton text={addr} />
      </div>
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
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {data.map((s) => (
            <ServerCard key={s.name} s={s} />
          ))}
        </div>
      )}
    </>
  );
}
