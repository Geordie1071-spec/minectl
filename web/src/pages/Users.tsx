import { useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, Navigate } from "react-router-dom";
import { Eye, KeyRound, MoreHorizontal, Server as ServerIcon, Shield, ShieldCheck, ShieldOff, Trash2, UserPlus, Users as UsersIcon, Wrench, X } from "lucide-react";
import { api, errorMessage, srv, type Server, type UserWithGrants } from "../api";
import { useMe } from "../hooks";
import { Avatar, PageTitle } from "../layout";
import { dateTime, timeAgo } from "../format";
import { Alert, Badge, Button, Card, Field, Input, Modal, Select, Spinner, TypeTile, cx } from "../ui";

const roleOptions = [
  { value: "viewer", label: "Viewer" },
  { value: "operator", label: "Operator" },
];

const roles = [
  { key: "admin", label: "Admin", icon: ShieldCheck, tone: "text-accent bg-accent-soft", text: "Everything, on every server, plus people and panel settings." },
  { key: "operator", label: "Operator", icon: Wrench, tone: "text-ok bg-ok-soft", text: "Start, stop, console, backups and players on their servers." },
  { key: "viewer", label: "Viewer", icon: Eye, tone: "text-muted bg-surface-2", text: "Status, logs, stats and players on their servers." },
] as const;

function RoleBadge({ role }: { role: "admin" | "operator" | "viewer" }) {
  const r = roles.find((x) => x.key === role)!;
  return (
    <span className={cx("inline-flex h-5 items-center gap-1 rounded-full px-2 text-xs font-medium", r.tone)}>
      <r.icon className="h-3 w-3" strokeWidth={2} />
      {r.label}
    </span>
  );
}

// ActionsMenu holds the less common actions for a person behind one button.
function ActionsMenu({ label, children }: { label: string; children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const close = (e: MouseEvent | KeyboardEvent) => {
      if (e instanceof KeyboardEvent ? e.key === "Escape" : !ref.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", close);
    document.addEventListener("keydown", close);
    return () => {
      document.removeEventListener("mousedown", close);
      document.removeEventListener("keydown", close);
    };
  }, [open]);
  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        aria-label={label}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
        className="rounded-md p-1.5 text-muted transition-colors hover:bg-hover hover:text-fg"
      >
        <MoreHorizontal className="h-4 w-4" strokeWidth={1.75} />
      </button>
      {open && (
        <div
          role="menu"
          onClick={() => setOpen(false)}
          className="absolute top-full right-0 z-30 mt-1 w-48 origin-top-right animate-scale-in rounded-lg border border-line bg-surface-2 p-1 shadow-pop"
        >
          {children}
        </div>
      )}
    </div>
  );
}

function MenuItem({ icon: I, children, onClick, danger }: { icon: typeof Shield; children: ReactNode; onClick: () => void; danger?: boolean }) {
  return (
    <button
      role="menuitem"
      type="button"
      onClick={onClick}
      className={cx(
        "flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm transition-colors hover:bg-hover",
        danger ? "text-danger" : "text-muted hover:text-fg",
      )}
    >
      <I className="h-4 w-4" strokeWidth={1.75} />
      {children}
    </button>
  );
}

function PersonCard({ u, servers, me }: { u: UserWithGrants; servers: Server[]; me: number }) {
  const qc = useQueryClient();
  const [server, setServer] = useState("");
  const [role, setRole] = useState("viewer");
  const [error, setError] = useState("");
  const [resetOpen, setResetOpen] = useState(false);
  const [newPass, setNewPass] = useState("");
  const refresh = () => qc.invalidateQueries({ queryKey: ["users"] });
  const isMe = u.id === me;

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
  const typeOf = (name: string) => servers.find((s) => s.name === name)?.type ?? "paper";

  return (
    <li className="overflow-hidden rounded-xl border border-line bg-surface-2 shadow-card">
      <div className="flex items-center gap-3 px-5 py-4">
        <Avatar name={u.username} className="h-10 w-10 text-sm" />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="truncate font-medium">{u.username}</span>
            {u.is_admin && <RoleBadge role="admin" />}
            {isMe && <Badge>you</Badge>}
          </div>
          <p className="mt-0.5 text-[13px] text-muted" title={dateTime(u.created_at)}>
            Joined {timeAgo(u.created_at)}
            {!u.is_admin && ` · ${u.grants.length} server${u.grants.length === 1 ? "" : "s"}`}
          </p>
        </div>
        <ActionsMenu label={`Actions for ${u.username}`}>
          {!isMe && (
            <MenuItem icon={u.is_admin ? ShieldOff : ShieldCheck} onClick={() => run(() => api.patch(`/users/${u.id}`, { is_admin: !u.is_admin }))}>
              {u.is_admin ? "Remove admin" : "Make admin"}
            </MenuItem>
          )}
          <MenuItem icon={KeyRound} onClick={() => setResetOpen(true)}>
            Reset password
          </MenuItem>
          {!isMe && (
            <>
              <div className="my-1 h-px bg-line" />
              <MenuItem
                icon={Trash2}
                danger
                onClick={() => confirm(`Delete ${u.username}? They'll be logged out everywhere.`) && run(() => api.del(`/users/${u.id}`))}
              >
                Delete
              </MenuItem>
            </>
          )}
        </ActionsMenu>
      </div>

      <div className="border-t border-line bg-surface px-5 py-4">
        <p className="mb-3 text-xs font-medium tracking-wide text-subtle uppercase">Server access</p>
        {u.is_admin ? (
          <p className="flex items-center gap-2 text-sm text-muted">
            <ShieldCheck className="h-4 w-4 text-accent" strokeWidth={1.75} />
            Full access to every server.
          </p>
        ) : (
          <>
            {u.grants.length === 0 ? (
              <p className="rounded-lg border border-dashed border-line px-3 py-3 text-sm text-muted">No servers yet. Give access below.</p>
            ) : (
              <ul className="divide-y divide-line rounded-lg border border-line bg-surface-2">
                {u.grants.map((g) => (
                  <li key={g.server} className="flex items-center gap-3 px-3 py-2">
                    <TypeTile type={typeOf(g.server)} className="h-7 w-7 text-xs" />
                    <Link to={srv(g.server)} className="min-w-0 flex-1 truncate text-sm font-medium hover:underline">
                      {g.server}
                    </Link>
                    <Select
                      value={g.role}
                      onChange={(role) => run(() => api.put(`/users/${u.id}/grants/${encodeURIComponent(g.server)}`, { role }))}
                      className="w-32"
                      aria-label={`Role on ${g.server}`}
                      options={roleOptions}
                    />
                    <button
                      type="button"
                      aria-label={`Remove access to ${g.server}`}
                      title="Remove access"
                      onClick={() => run(() => api.del(`/users/${u.id}/grants/${encodeURIComponent(g.server)}`))}
                      className="rounded-md p-1.5 text-subtle transition-colors hover:bg-danger-soft hover:text-danger"
                    >
                      <X className="h-4 w-4" strokeWidth={1.75} />
                    </button>
                  </li>
                ))}
              </ul>
            )}
            {ungranted.length > 0 && (
              <form
                className="mt-3 flex flex-wrap items-center gap-2"
                onSubmit={(e) => {
                  e.preventDefault();
                  if (server) run(() => api.put(`/users/${u.id}/grants/${encodeURIComponent(server)}`, { role })).then(() => setServer(""));
                }}
              >
                <Select
                  value={server}
                  onChange={setServer}
                  placeholder="Give access to a server…"
                  className="w-auto min-w-44 flex-1"
                  aria-label="Server"
                  options={ungranted.map((s) => ({ value: s.name, label: s.name }))}
                />
                <Select value={role} onChange={setRole} className="w-32" aria-label="Role" options={roleOptions} />
                <Button type="submit" disabled={!server}>
                  Add
                </Button>
              </form>
            )}
          </>
        )}
        {error && (
          <div className="mt-3">
            <Alert>{error}</Alert>
          </div>
        )}
      </div>

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

function Stat({ icon: I, label, value }: { icon: typeof Shield; label: string; value: number | string }) {
  return (
    <div className="flex items-center gap-3 rounded-xl border border-line bg-surface-2 px-4 py-3 shadow-card">
      <span className="flex h-9 w-9 items-center justify-center rounded-lg bg-surface text-muted">
        <I className="h-4 w-4" strokeWidth={1.75} />
      </span>
      <div>
        <p className="text-xl leading-tight font-medium tabular-nums">{value}</p>
        <p className="text-[13px] text-muted">{label}</p>
      </div>
    </div>
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

  const list = users.data ?? [];
  // Admins first, then by name, so the people with most control are on top.
  const sorted = [...list].sort((a, b) => Number(b.is_admin) - Number(a.is_admin) || a.username.localeCompare(b.username));
  const admins = list.filter((u) => u.is_admin).length;

  return (
    <>
      <PageTitle description="Add people and choose which servers they can use.">People</PageTitle>

      <div className="stagger mb-6 grid grid-cols-3 gap-3">
        <Stat icon={UsersIcon} label={list.length === 1 ? "person" : "people"} value={users.data ? list.length : "–"} />
        <Stat icon={ShieldCheck} label={admins === 1 ? "admin" : "admins"} value={users.data ? admins : "–"} />
        <Stat icon={ServerIcon} label={servers.data?.length === 1 ? "server" : "servers"} value={servers.data ? servers.data.length : "–"} />
      </div>

      <div className="stagger grid items-start gap-6 lg:grid-cols-3">
        <section className="lg:col-span-2" aria-label="Everyone">
          {users.isPending && <Spinner />}
          <ul className="space-y-4">
            {sorted.map((u) => (
              <PersonCard key={u.id} u={u} servers={servers.data ?? []} me={me?.user.id ?? 0} />
            ))}
          </ul>
        </section>

        <div className="space-y-6 lg:sticky lg:top-6">
          <Card title="Add a person" description="They log in with this name and password.">
            <form onSubmit={create} className="space-y-3">
              <Field label="Username">
                <Input value={username} onChange={(e) => setUsername(e.target.value)} required autoComplete="off" />
              </Field>
              <Field label="Password" hint="At least 8 characters. They can change it later.">
                <Input type="text" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} autoComplete="off" />
              </Field>
              <label className="flex items-start gap-2 rounded-lg border border-line px-3 py-2.5 text-sm">
                <input type="checkbox" className="mt-0.5" checked={isAdmin} onChange={(e) => setIsAdmin(e.target.checked)} />
                <span>
                  <span className="font-medium">Admin</span>
                  <span className="block text-[13px] text-muted">Full control of every server, people and settings. Only for people you'd trust with the machine.</span>
                </span>
              </label>
              {error && <Alert>{error}</Alert>}
              <Button variant="primary" type="submit" busy={busy} className="w-full justify-center">
                <UserPlus className="h-4 w-4" strokeWidth={1.75} /> Add person
              </Button>
            </form>
          </Card>

          <Card title="Roles" description="What each role can do on a server.">
            <dl className="space-y-3">
              {roles.map((r) => (
                <div key={r.key} className="flex gap-3">
                  <span className={cx("flex h-8 w-8 shrink-0 items-center justify-center rounded-lg", r.tone)}>
                    <r.icon className="h-4 w-4" strokeWidth={1.75} />
                  </span>
                  <div>
                    <dt className="text-sm font-medium">{r.label}</dt>
                    <dd className="text-[13px] text-muted">{r.text}</dd>
                  </div>
                </div>
              ))}
            </dl>
          </Card>
        </div>
      </div>
    </>
  );
}
