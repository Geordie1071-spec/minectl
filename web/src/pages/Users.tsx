import { useState, type FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Navigate } from "react-router-dom";
import { api, errorMessage, type Server, type UserWithGrants } from "../api";
import { useMe } from "../hooks";
import { PageTitle } from "../layout";
import { timeAgo } from "../format";
import { Alert, Badge, Button, Card, Field, Input, Modal, Select, Spinner } from "../ui";

function UserRow({ u, servers, me }: { u: UserWithGrants; servers: Server[]; me: number }) {
  const qc = useQueryClient();
  const [server, setServer] = useState("");
  const [role, setRole] = useState("viewer");
  const [error, setError] = useState("");
  const [resetOpen, setResetOpen] = useState(false);
  const [newPass, setNewPass] = useState("");
  const refresh = () => qc.invalidateQueries({ queryKey: ["users"] });

  const run = async (fn: () => Promise<unknown>) => {
    setError("");
    try {
      await fn();
      refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };
  const ungranted = servers.filter((s) => !u.grants.some((g) => g.server === s.name));

  return (
    <li className="space-y-3 py-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <span className="font-medium">{u.username}</span>
          {u.is_admin && <Badge>admin</Badge>}
          {u.id === me && <Badge>you</Badge>}
          <span className="text-xs text-muted">joined {timeAgo(u.created_at)}</span>
        </div>
        <div className="flex flex-wrap gap-1">
          {u.id !== me && (
            <Button variant="ghost" className="text-xs" onClick={() => run(() => api.patch(`/users/${u.id}`, { is_admin: !u.is_admin }))}>
              {u.is_admin ? "Remove admin" : "Make admin"}
            </Button>
          )}
          <Button variant="ghost" className="text-xs" onClick={() => setResetOpen(true)}>
            Reset password
          </Button>
          {u.id !== me && (
            <Button
              variant="ghost"
              className="text-xs text-danger"
              onClick={() => confirm(`Delete ${u.username}? They'll be logged out everywhere.`) && run(() => api.del(`/users/${u.id}`))}
            >
              Delete
            </Button>
          )}
        </div>
      </div>
      {!u.is_admin && (
        <div className="space-y-2 pl-0 sm:pl-4">
          {u.grants.length === 0 && <p className="text-sm text-muted">No servers yet.</p>}
          {u.grants.map((g) => (
            <div key={g.server} className="flex items-center gap-2 text-sm">
              <span className="min-w-0 flex-1 truncate">{g.server}</span>
              <Select
                value={g.role}
                onChange={(e) => run(() => api.put(`/users/${u.id}/grants/${encodeURIComponent(g.server)}`, { role: e.target.value }))}
                className="w-32"
                aria-label={`Role on ${g.server}`}
              >
                <option value="viewer">Viewer</option>
                <option value="operator">Operator</option>
              </Select>
              <Button variant="ghost" className="text-xs" onClick={() => run(() => api.del(`/users/${u.id}/grants/${encodeURIComponent(g.server)}`))}>
                Remove
              </Button>
            </div>
          ))}
          {ungranted.length > 0 && (
            <form
              className="flex flex-wrap items-center gap-2"
              onSubmit={(e) => {
                e.preventDefault();
                if (server) run(() => api.put(`/users/${u.id}/grants/${encodeURIComponent(server)}`, { role })).then(() => setServer(""));
              }}
            >
              <Select value={server} onChange={(e) => setServer(e.target.value)} className="w-auto min-w-40 flex-1" aria-label="Server">
                <option value="">Give access to…</option>
                {ungranted.map((s) => (
                  <option key={s.name} value={s.name}>
                    {s.name}
                  </option>
                ))}
              </Select>
              <Select value={role} onChange={(e) => setRole(e.target.value)} className="w-32" aria-label="Role">
                <option value="viewer">Viewer</option>
                <option value="operator">Operator</option>
              </Select>
              <Button type="submit" disabled={!server}>
                Add
              </Button>
            </form>
          )}
        </div>
      )}
      {error && <Alert>{error}</Alert>}
      <Modal open={resetOpen} onClose={() => setResetOpen(false)} title={`New password for ${u.username}`}>
        <form
          className="space-y-3"
          onSubmit={(e) => {
            e.preventDefault();
            run(() => api.patch(`/users/${u.id}`, { password: newPass })).then(() => {
              setResetOpen(false);
              setNewPass("");
            });
          }}
        >
          <Input type="text" value={newPass} onChange={(e) => setNewPass(e.target.value)} minLength={8} required autoComplete="off" />
          <p className="text-xs text-muted">They'll be logged out everywhere. Send them the new password yourself.</p>
          <div className="flex justify-end gap-2">
            <Button type="button" onClick={() => setResetOpen(false)}>
              Cancel
            </Button>
            <Button variant="primary" type="submit">
              Set password
            </Button>
          </div>
        </form>
      </Modal>
    </li>
  );
}

export function Users() {
  const { data: me } = useMe();
  const qc = useQueryClient();
  const users = useQuery({ queryKey: ["users"], queryFn: () => api.get<UserWithGrants[]>("/users"), enabled: !!me?.user.is_admin });
  const servers = useQuery({ queryKey: ["servers"], queryFn: () => api.get<Server[]>("/servers"), enabled: !!me?.user.is_admin });
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [isAdmin, setIsAdmin] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  if (me && !me.user.is_admin) return <Navigate to="/" replace />;

  const create = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api.post("/users", { username, password, is_admin: isAdmin });
      setUsername("");
      setPassword("");
      setIsAdmin(false);
      qc.invalidateQueries({ queryKey: ["users"] });
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <PageTitle description="Add people and choose which servers they can use.">Users</PageTitle>
      <div className="grid gap-4 lg:grid-cols-3">
        <div className="lg:col-span-2">
          <Card title="Everyone">
            {users.isPending && <Spinner />}
            <ul className="-my-4 divide-y divide-line">
              {users.data?.map((u) => (
                <UserRow key={u.id} u={u} servers={servers.data ?? []} me={me?.user.id ?? 0} />
              ))}
            </ul>
          </Card>
        </div>
        <div className="space-y-4">
          <Card title="Add a user">
            <form onSubmit={create} className="space-y-3">
              <Field label="Username">
                <Input value={username} onChange={(e) => setUsername(e.target.value)} required autoComplete="off" />
              </Field>
              <Field label="Password" hint="At least 8 characters. They can change it later.">
                <Input type="text" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} autoComplete="off" />
              </Field>
              <label className="flex items-center gap-2 text-sm">
                <input type="checkbox" checked={isAdmin} onChange={(e) => setIsAdmin(e.target.checked)} className="accent-[var(--accent)]" />
                Admin (full control of everything)
              </label>
              {error && <Alert>{error}</Alert>}
              <Button variant="primary" type="submit" busy={busy}>
                Add user
              </Button>
            </form>
          </Card>
          <Card title="Roles">
            <dl className="space-y-2 text-sm">
              <div>
                <dt className="font-medium">Admin</dt>
                <dd className="text-muted">Everything, on every server.</dd>
              </div>
              <div>
                <dt className="font-medium">Operator</dt>
                <dd className="text-muted">Start, stop, console, backups and players on their servers.</dd>
              </div>
              <div>
                <dt className="font-medium">Viewer</dt>
                <dd className="text-muted">Status, logs, stats and players on their servers.</dd>
              </div>
            </dl>
          </Card>
        </div>
      </div>
    </>
  );
}
