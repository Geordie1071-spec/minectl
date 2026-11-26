import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, errorMessage, srv, type Backup } from "../../api";
import { dateTime, formatBytes, timeAgo } from "../../format";
import { Alert, Button, Card, Empty, Modal, Select, Spinner } from "../../ui";
import { can, type TabProps } from "./ServerPage";

export function Backups({ s, runJob }: TabProps) {
  const backups = useQuery({ queryKey: ["backups", s.name], queryFn: () => api.get<Backup[]>(srv(s.name) + "/backups") });
  const [restore, setRestore] = useState<Backup>();
  const busy = !!s.job;
  const list = [...(backups.data ?? [])].reverse();

  return (
    <div className="stagger space-y-4">
      <BackupSchedule s={s} />
    <Card
      title="Backups"
      actions={
        can(s, "operator") && (
          <Button variant="primary" disabled={busy} onClick={() => runJob("/backups").catch(() => {})}>
            Back up now
          </Button>
        )
      }
    >
      {backups.isPending && <Spinner />}
      {backups.error && <Alert>{backups.error.message}</Alert>}
      {backups.data?.length === 0 && (
        <Empty title="No backups yet">Backups save the whole server folder. One is also made automatically before upgrades and modpack changes.</Empty>
      )}
      {list.length > 0 && (
        <ul className="divide-y divide-line">
          {list.map((b) => (
            <li key={b.id} className="flex flex-wrap items-center justify-between gap-2 py-2.5">
              <div>
                <p className="text-sm font-medium" title={dateTime(b.created_at)}>
                  {timeAgo(b.created_at)}
                </p>
                <p className="text-xs text-muted">
                  <code>{b.id}</code> · {formatBytes(b.size_bytes)}
                </p>
              </div>
              {can(s, "admin") && (
                <Button disabled={busy} onClick={() => setRestore(b)}>
                  Restore
                </Button>
              )}
            </li>
          ))}
        </ul>
      )}
      <p className="mt-4 text-xs text-muted">Older backups are pruned automatically, keeping the most recent ones.</p>
      <Modal open={!!restore} onClose={() => setRestore(undefined)} title="Restore this backup?">
        <p className="text-sm">
          {s.name} goes back to how it was {restore && timeAgo(restore.created_at)}. Anything since then is lost. The server restarts during the restore.
        </p>
        <div className="mt-4 flex justify-end gap-2">
          <Button onClick={() => setRestore(undefined)}>Cancel</Button>
          <Button
            variant="danger"
            onClick={() => {
              const id = restore!.id;
              setRestore(undefined);
              runJob(`/backups/${encodeURIComponent(id)}/restore`).catch(() => {});
            }}
          >
            Restore
          </Button>
        </div>
      </Modal>
    </Card>
    </div>
  );
}

const scheduleOptions = [
  { value: "", label: "Off" },
  { value: "hourly", label: "Every hour" },
  { value: "6h", label: "Every 6 hours" },
  { value: "daily", label: "Every day" },
];

function BackupSchedule({ s }: { s: TabProps["s"] }) {
  const qc = useQueryClient();
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const admin = can(s, "admin");

  const change = async (schedule: string) => {
    setSaving(true);
    setError("");
    try {
      await api.put(srv(s.name) + "/backup-schedule", { schedule: schedule || "off" });
      await qc.invalidateQueries({ queryKey: ["server", s.name] });
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card
      title="Automatic backups"
      description={
        s.backup_schedule && s.next_backup_at
          ? `Next backup ${timeUntil(s.next_backup_at)}. Runs while the panel is running.`
          : "Back up on a schedule while the panel is running. The oldest backups are removed to make room."
      }
      actions={
        <div className="flex items-center gap-2">
          {saving && <Spinner className="h-3.5 w-3.5 text-muted" />}
          <Select value={s.backup_schedule ?? ""} onChange={change} options={scheduleOptions} disabled={!admin || saving} className="w-40" aria-label="Backup schedule" />
        </div>
      }
    >
      {error ? <Alert>{error}</Alert> : <p className="text-[13px] text-subtle">{admin ? "Admins can change how many backups are kept in Settings." : "Only admins can change the schedule."}</p>}
    </Card>
  );
}

function timeUntil(iso: string) {
  const mins = Math.round((new Date(iso).getTime() - Date.now()) / 60000);
  if (mins <= 1) return "in a minute";
  if (mins < 60) return `in ${mins} minutes`;
  const h = Math.round(mins / 60);
  return h < 24 ? `in ${h} hour${h === 1 ? "" : "s"}` : `on ${new Date(iso).toLocaleString(undefined, { weekday: "short", hour: "2-digit", minute: "2-digit" })}`;
}
