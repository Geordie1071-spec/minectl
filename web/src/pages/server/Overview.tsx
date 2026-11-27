import { type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { Archive, Box, Cpu, MemoryStick, Puzzle, Users } from "lucide-react";
import { api, srv, type Backup, type Job } from "../../api";
import { usePlayers, useStats } from "../../hooks";
import { scopeHint, useJoinAddress } from "../../join";
import { dateTime, formatBytes, formatMB, isModded, timeAgo, typeLabels } from "../../format";
import { jobLabel } from "../../JobView";
import { Card, CopyButton, LiveDot, Meter, Sparkline, Spinner, StatusDot, cx } from "../../ui";
import type { TabProps } from "./ServerPage";

// ServiceCard is one node on the canvas, like a service in Railway.
function ServiceCard({
  icon: I,
  title,
  subtitle,
  to,
  children,
  highlight,
}: {
  icon: typeof Box;
  title: string;
  subtitle?: ReactNode;
  to?: string;
  children: ReactNode;
  highlight?: boolean;
}) {
  const body = (
    <>
      <div className="flex items-start gap-3">
        <I className="mt-0.5 h-5 w-5 shrink-0" strokeWidth={1.75} />
        <div className="min-w-0">
          <p className="truncate font-medium">{title}</p>
          {subtitle && <div className="mt-0.5 truncate text-[13px] text-muted">{subtitle}</div>}
        </div>
      </div>
      <div className="mt-7 text-[13px]">{children}</div>
    </>
  );
  const cls = cx(
    "block rounded-xl border bg-surface-2 p-5 shadow-card transition-[border-color,transform,box-shadow] duration-200",
    highlight ? "border-line-strong" : "border-line",
    to && "hover:-translate-y-0.5 hover:border-line-strong hover:shadow-pop",
  );
  return to ? (
    <Link to={to} className={cls}>
      {body}
    </Link>
  ) : (
    <div className={cls}>{body}</div>
  );
}

function Stat({ icon: I, label, value }: { icon: typeof Box; label: string; value: string }) {
  return (
    <div className="flex items-center gap-2.5">
      <I className="h-4 w-4 text-subtle" strokeWidth={1.75} />
      <span className="text-muted">{label}</span>
      <span className="ml-auto font-mono tabular-nums">{value}</span>
    </div>
  );
}

export function Overview({ s }: TabProps) {
  const running = s.status === "running" && !s.job;
  const { stats, history } = useStats(s.name, running);
  const players = usePlayers(s.name, running);
  const jobs = useQuery({ queryKey: ["jobs", s.name], queryFn: () => api.get<Job[]>(srv(s.name) + "/jobs"), refetchInterval: 15_000 });
  const backups = useQuery({ queryKey: ["backups", s.name], queryFn: () => api.get<Backup[]>(srv(s.name) + "/backups") });
  const { address, scope } = useJoinAddress(s.port);
  const cores = s.cpu_cores || 1;
  const cpuNow = stats ? stats.cpu_percent / cores : 0;
  const latest = backups.data?.[backups.data.length - 1];
  const totalBackup = backups.data?.reduce((n, b) => n + b.size_bytes, 0) ?? 0;
  const base = srv(s.name);
  const modded = !!s.modpack || s.mods.length > 0 || isModded(s.type);

  return (
    <div className="stagger space-y-6">
      <section className="dots rounded-xl border border-line p-6 sm:p-10">
        <div className="mx-auto grid max-w-3xl gap-6 sm:grid-cols-2">
          <ServiceCard
            icon={Box}
            title={s.name}
            highlight
            subtitle={
              <span className="flex items-center gap-1 font-mono" title={scopeHint[scope] || undefined}>
                {address}
                <CopyButton text={address} label="Copy address" />
              </span>
            }
          >
            <span className={cx("flex items-center gap-2", running ? "text-ok" : "text-muted")}>
              {s.job ? <Spinner className="h-3 w-3 text-warn" /> : <LiveDot status={s.status} />}
              {s.job ? `${jobLabel(s.job.kind)}…` : running ? "Online" : s.status === "stopped" ? "Stopped" : s.status}
              <span className="ml-auto text-muted">
                {typeLabels[s.type] ?? s.type} {s.version}
              </span>
            </span>
          </ServiceCard>

          <ServiceCard icon={Archive} title="World" subtitle={latest ? `Last backup ${timeAgo(latest.created_at)}` : "No backups yet"} to={base + "/backups"}>
            <span className="text-muted">
              {backups.data ? `${backups.data.length} backup${backups.data.length === 1 ? "" : "s"} · ${formatBytes(totalBackup)}` : "…"}
            </span>
          </ServiceCard>

          {modded && (
            <ServiceCard
              icon={Puzzle}
              title={s.modpack ? s.modpack.id : "Mods"}
              subtitle={s.modpack ? (s.modpack.source === "curseforge" ? "CurseForge modpack" : "Modrinth modpack") : typeLabels[s.type]}
              to={s.role === "admin" ? base + "/mods" : undefined}
            >
              <span className="text-muted">{s.modpack ? "Managed by the modpack" : `${s.mods.length} mod${s.mods.length === 1 ? "" : "s"} installed`}</span>
            </ServiceCard>
          )}

          <ServiceCard icon={Users} title="Players" subtitle={running ? "Connected now" : "Offline"} to={base + "/players"}>
            <span className="text-muted">{running && players.data ? `${players.data.online} of ${players.data.max} online` : "—"}</span>
          </ServiceCard>
        </div>
        {scopeHint[scope] && <p className="mx-auto mt-6 max-w-3xl text-center text-xs text-subtle">{scopeHint[scope]}</p>}
      </section>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card title="Metrics" description={running ? "Live, refreshed every second." : "Start the server to see live usage."}>
          <div className="space-y-5">
            <div className="rounded-lg border border-line bg-surface p-3">
              <div className="mb-1 flex items-center justify-between text-[13px]">
                <span className="text-muted">CPU, last minute</span>
                <span className="font-mono tabular-nums">{running && stats ? `${cpuNow.toFixed(0)}%` : "—"}</span>
              </div>
              <Sparkline values={history.map((v) => v / cores)} max={100} />
            </div>
            {running && stats && (
              <>
                <Meter value={cpuNow} label="CPU" detail={`${cpuNow.toFixed(0)}% of ${s.cpu_cores ? `${s.cpu_cores} cores` : "1 core"}`} />
                <Meter value={stats.memory_percent} label="Memory" detail={`${formatBytes(stats.memory_bytes)} / ${formatBytes(stats.memory_limit)}`} alarm={false} />
              </>
            )}
            <div className="space-y-2.5 border-t border-line pt-4 text-[13px]">
              <Stat icon={MemoryStick} label="Memory limit" value={formatMB(s.memory_mb)} />
              <Stat icon={Cpu} label="CPU limit" value={s.cpu_cores ? `${s.cpu_cores} cores` : "none"} />
            </div>
          </div>
        </Card>

        <Card title="Activity" description="Recent changes to this server." flush>
          {jobs.data && jobs.data.length > 0 ? (
            <ul className="divide-y divide-line border-t border-line">
              {jobs.data.slice(0, 7).map((j) => (
                <li key={j.id}>
                  <Link to={"/jobs/" + j.id} className="flex items-center gap-3 px-5 py-3 text-sm transition-colors hover:bg-hover/50">
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
