import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api, srv, type Job } from "../../api";
import { usePlayers, useStats } from "../../hooks";
import { dateTime, formatBytes, formatMB, timeAgo, typeLabels } from "../../format";
import { jobLabel } from "../../JobView";
import { Card, Meter, Sparkline } from "../../ui";
import type { TabProps } from "./ServerPage";

function Info({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <dt className="text-xs text-muted">{label}</dt>
      <dd className="mt-0.5 text-sm">{children}</dd>
    </div>
  );
}

export function Overview({ s }: TabProps) {
  const running = s.status === "running" && !s.job;
  const { stats, history } = useStats(s.name, running);
  const players = usePlayers(s.name, running);
  const jobs = useQuery({ queryKey: ["jobs", s.name], queryFn: () => api.get<Job[]>(srv(s.name) + "/jobs"), refetchInterval: 15_000 });
  const cores = s.cpu_cores || 1;

  return (
    <div className="grid gap-4 lg:grid-cols-3">
      <div className="space-y-4 lg:col-span-2">
        <Card title="Usage">
          {running && stats ? (
            <div className="grid gap-6 sm:grid-cols-2">
              <div>
                <Meter value={stats.cpu_percent / cores} label={`CPU (${s.cpu_cores ? `${s.cpu_cores} cores` : "no limit"})`} />
                <div className="mt-3">
                  <Sparkline values={history} max={cores * 100} />
                </div>
              </div>
              <div>
                <Meter value={stats.memory_percent} label="Memory" alarm={false} />
                <p className="mt-3 text-sm text-muted">
                  {formatBytes(stats.memory_bytes)} of {formatBytes(stats.memory_limit)}
                </p>
                <p className="mt-1 text-xs text-muted">Java reserves most of its memory up front, so a high number here is normal.</p>
              </div>
            </div>
          ) : (
            <p className="text-sm text-muted">{running ? "Waiting for stats…" : "Start the server to see live usage."}</p>
          )}
        </Card>
        <Card title="Recent activity">
          {jobs.data && jobs.data.length > 0 ? (
            <ul className="divide-y divide-line text-sm">
              {jobs.data.slice(0, 8).map((j) => (
                <li key={j.id} className="flex items-center justify-between gap-2 py-2">
                  <Link to={"/jobs/" + j.id} className="hover:underline">
                    {jobLabel(j.kind)}
                  </Link>
                  <span className={j.status === "failed" ? "text-danger" : "text-muted"}>
                    {j.status === "running" ? "running" : j.status === "failed" ? "failed" : "done"} · {timeAgo(j.created_at)}
                  </span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-sm text-muted">Nothing yet.</p>
          )}
        </Card>
      </div>
      <div className="space-y-4">
        <Card title="Players">
          {running && players.data ? (
            <>
              <p className="text-2xl font-semibold">
                {players.data.online}
                <span className="text-base font-normal text-muted"> / {players.data.max}</span>
              </p>
              {players.data.names.length > 0 && <p className="mt-2 text-sm text-muted">{players.data.names.join(", ")}</p>}
            </>
          ) : (
            <p className="text-sm text-muted">{running ? "…" : "Offline"}</p>
          )}
        </Card>
        <Card title="Details">
          <dl className="grid grid-cols-2 gap-4">
            <Info label="Type">{typeLabels[s.type] ?? s.type}</Info>
            <Info label="Version">{s.version}</Info>
            <Info label="Memory">{formatMB(s.memory_mb)}</Info>
            <Info label="CPU">{s.cpu_cores ? `${s.cpu_cores} cores` : "No limit"}</Info>
            <Info label="Port">{s.port}</Info>
            <Info label="Backups">{s.backups ?? 0}</Info>
            {s.modpack && (
              <div className="col-span-2">
                <Info label={s.modpack.source === "curseforge" ? "CurseForge pack" : "Modrinth modpack"}>{s.modpack.id}</Info>
              </div>
            )}
            <Info label="Created">{dateTime(s.created_at)}</Info>
            <Info label="Last started">{timeAgo(s.last_started_at)}</Info>
            {s.data_dir && (
              <div className="col-span-2">
                <Info label="Data folder">
                  <code className="text-xs break-all">{s.data_dir}</code>
                </Info>
              </div>
            )}
          </dl>
        </Card>
      </div>
    </div>
  );
}
