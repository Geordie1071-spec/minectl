import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, srv, type Backup } from "../../api";
import { dateTime, formatBytes, timeAgo } from "../../format";
import { Alert, Button, Card, Empty, Modal, Spinner } from "../../ui";
import { can, type TabProps } from "./ServerPage";

export function Backups({ s, runJob }: TabProps) {
  const backups = useQuery({ queryKey: ["backups", s.name], queryFn: () => api.get<Backup[]>(srv(s.name) + "/backups") });
  const [restore, setRestore] = useState<Backup>();
  const busy = !!s.job;
  const list = [...(backups.data ?? [])].reverse();

  return (
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
  );
}
