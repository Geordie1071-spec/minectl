import { type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api, srv, type Job } from "../../api";
import { usePlayers, useStats } from "../../hooks";
import { joinAddress } from "../../join";
import { dateTime, formatBytes, formatMB, timeAgo, typeLabels } from "../../format";
import { jobLabel } from "../../JobView";
import { Card, CopyButton, Meter, Sparkline, StatusBadge, StatusDot } from "../../ui";
import type { TabProps } from "./ServerPage";

function Detail({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="min-w-0">
      <dt className="text-[13px] text-muted">{label}</dt>
      <dd className="mt-1 truncate text-sm font-medium">{children}</dd>
    </div>
  );
}

export function Overview({ s }: TabProps) {
  const running = s.status === "running" && !s.job;
  const { stats, history } = useStats(s.name, running);
  const players = usePlayers(s.name, running);
  const jobs = useQuery({ queryKey: ["jobs", s.name], queryFn: () => api.get<Job[]>(srv(s.name) + "/jobs"), refetchInterval: 15_000 });
  const cores = s.cpu_cores || 1;
  const addr = joinAddress(s.port);
  const cpuNow = stats ? stats.cpu_percent / cores : 0;

  return (
    <div className="space-y-6">
      <section className="overflow-hidden rounded-lg border border-line bg-surface shadow-card">
        <div className="grid md:grid-cols-[minmax(0,5fr)_minmax(0,6fr)]">
          <div className="flex min-h-48 flex-col justify-between border-b border-line bg-surface-2 p-5 md:border-r md:border-b-0">
            <div className="flex items-center justify-between">
              <span className="text-[13px] text-muted">CPU, last minute</span>
              <span className="font-mono text-[13px] tabular-nums">{running && stats ? `${cpuNow.toFixed(0)}%` : "—"}</span>
            </div>
            {running ? (
              <Sparkline values={history.map((v) => v / cores)} max={100} />
            ) : (
              <div className="flex flex-1 items-center justify-center text-sm text-muted">Start the server to see live usage</div>
            )}
          </div>
          <dl className="grid grid-cols-2 gap-x-6 gap-y-5 p-5 sm:grid-cols-3">
            <Detail label="Status">
              <StatusBadge status={s.status} busy={s.job ? `${jobLabel(s.job.kind)}…` : undefined} />
            </Detail>
            <Detail label="Players">{running && players.data ? `${players.data.online} / ${players.data.max}` : "—"}</Detail>
            <Detail label="Started">{timeAgo(s.last_started_at)}</Detail>
            <div className="col-span-2 min-w-0 sm:col-span-3">
              <dt className="text-[13px] text-muted">Address</dt>
              <dd className="mt-1 flex items-center gap-1 font-mono text-sm">
                {addr}
                <CopyButton text={addr} label="Copy address" />
              </dd>
            </div>
            <Detail label="Software">
              {typeLabels[s.type] ?? s.type} {s.version}
            </Detail>
            <Detail label="Memory">{formatMB(s.memory_mb)}</Detail>
            <Detail label="CPU">{s.cpu_cores ? `${s.cpu_cores} cores` : "No limit"}</Detail>
            {s.modpack && (
              <div className="col-span-2 min-w-0 sm:col-span-3">
                <Detail label={s.modpack.source === "curseforge" ? "CurseForge pack" : "Modrinth modpack"}>{s.modpack.id}</Detail>
              </div>
            )}
          </dl>
        </div>
      </section>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <Card title="Resources" description="Live usage against this server's limits.">
          {running && stats ? (
            <div className="space-y-5">
              <Meter value={cpuNow} label="CPU" detail={`${cpuNow.toFixed(0)}% of ${s.cpu_cores ? `${s.cpu_cores} cores` : "1 core"}`} />
              <Meter
                value={stats.memory_percent}
                label="Memory"
                detail={`${formatBytes(stats.memory_bytes)} / ${formatBytes(stats.memory_limit)}`}
                alarm={false}
              />
              <p className="text-xs text-subtle">Java reserves most of its memory up front, so high memory use is normal.</p>
            </div>
          ) : (
            <p className="text-sm text-muted">{running ? "Waiting for stats…" : "Offline."}</p>
          )}
        </Card>

        <Card title="Activity" description="Recent changes to this server." flush>
          {jobs.data && jobs.data.length > 0 ? (
            <ul className="divide-y divide-line border-t border-line">
              {jobs.data.slice(0, 6).map((j) => (
                <li key={j.id}>
                  <Link to={"/jobs/" + j.id} className="flex items-center gap-3 px-5 py-3 text-sm hover:bg-hover">
                    <StatusDot status={j.status === "running" ? "creating" : j.status === "failed" ? "error" : "running"} />
                    <span className="flex-1 truncate">{jobLabel(j.kind)}</span>
                    <span className={j.status === "failed" ? "text-[13px] text-danger" : "text-[13px] text-muted"} title={dateTime(j.created_at)}>
                      {j.status === "failed" ? "Failed · " : ""}
                      {timeAgo(j.created_at)}
                    </span>
                  </Link>
                </li>
              ))}
            </ul>
          ) : (
            <p className="border-t border-line px-5 py-4 text-sm text-muted">Nothing yet.</p>
          )}
        </Card>
      </div>

      {s.data_dir && (
        <p className="text-[13px] text-subtle">
          Files: <code className="font-mono break-all">{s.data_dir}</code> · Created {dateTime(s.created_at)}
        </p>
      )}
    </div>
  );
}
