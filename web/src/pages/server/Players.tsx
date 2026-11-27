import { useState, type FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Ban, Crown, LogOut, Search, ShieldCheck, ShieldOff, UserCheck, UserMinus, UserPlus } from "lucide-react";
import { api, errorMessage, srv, type KnownPlayer } from "../../api";
import { usePlayers } from "../../hooks";
import { dateTime, timeAgo } from "../../format";
import { ActionsMenu, Alert, Button, Card, ConfirmDialog, Input, MenuItem, Spinner, cx } from "../../ui";
import { can, type TabProps } from "./ServerPage";

const validName = /^[A-Za-z0-9_]{1,16}$/;

type Filter = "all" | "online" | "op" | "whitelisted" | "banned";

interface Row extends KnownPlayer {
  online: boolean;
}

function Tag({ tone, children }: { tone: "ok" | "accent" | "danger" | "muted"; children: string }) {
  const tones = {
    ok: "bg-ok-soft text-ok",
    accent: "bg-accent-soft text-accent",
    danger: "bg-danger-soft text-danger",
    muted: "bg-surface text-muted border border-line",
  };
  return <span className={cx("inline-flex h-5 items-center rounded-full px-2 text-xs font-medium", tones[tone])}>{children}</span>;
}

// PlayersTab lists everyone who has played, not only who's online, so bans,
// ops and the whitelist can be managed for offline players too.
export function PlayersTab({ s }: TabProps) {
  const running = s.status === "running" && !s.job;
  const online = usePlayers(s.name, running);
  const known = useQuery({
    queryKey: ["known-players", s.name],
    queryFn: () => api.get<KnownPlayer[]>(srv(s.name) + "/players/known"),
    refetchInterval: running ? 15_000 : false,
  });
  const qc = useQueryClient();
  const operator = can(s, "operator");
  const [msg, setMsg] = useState<{ kind: "info" | "error"; text: string }>();
  const [filter, setFilter] = useState<Filter>("all");
  const [query, setQuery] = useState("");
  const [name, setName] = useState("");
  const [busy, setBusy] = useState("");
  const [dialog, setDialog] = useState<{ player: string; action: "kick" | "ban" }>();
  const [reason, setReason] = useState("");

  const act = async (player: string, action: string, why = "") => {
    setBusy(player + action);
    setMsg(undefined);
    try {
      const res = await api.post<{ output: string }>(`${srv(s.name)}/players/${encodeURIComponent(player)}/${action}`, why ? { reason: why } : undefined);
      setMsg({ kind: "info", text: res.output || "Done." });
      qc.invalidateQueries({ queryKey: ["players", s.name] });
      qc.invalidateQueries({ queryKey: ["known-players", s.name] });
    } catch (err) {
      setMsg({ kind: "error", text: errorMessage(err) });
      throw err;
    } finally {
      setBusy("");
    }
  };
  const quiet = (player: string, action: string) => act(player, action).catch(() => {});

  // Everyone the server knows about, plus anyone online who isn't saved yet.
  const onlineNames = new Set((online.data?.names ?? []).map((n) => n.toLowerCase()));
  const rows: Row[] = (known.data ?? []).map((p) => ({ ...p, online: onlineNames.has(p.name.toLowerCase()) }));
  for (const n of online.data?.names ?? []) {
    if (!rows.some((r) => r.name.toLowerCase() === n.toLowerCase())) rows.push({ name: n, op: false, whitelisted: false, banned: false, online: true });
  }
  rows.sort((a, b) => Number(b.online) - Number(a.online) || a.name.localeCompare(b.name, undefined, { sensitivity: "base" }));

  const counts: Record<Filter, number> = {
    all: rows.length,
    online: rows.filter((r) => r.online).length,
    op: rows.filter((r) => r.op).length,
    whitelisted: rows.filter((r) => r.whitelisted).length,
    banned: rows.filter((r) => r.banned).length,
  };
  const shown = rows.filter(
    (r) =>
      (filter === "all" || (filter === "online" ? r.online : r[filter])) && (!query || r.name.toLowerCase().includes(query.toLowerCase())),
  );
  const filters: { key: Filter; label: string }[] = [
    { key: "all", label: "Everyone" },
    { key: "online", label: "Online" },
    { key: "op", label: "Ops" },
    { key: "whitelisted", label: "Whitelisted" },
    { key: "banned", label: "Banned" },
  ];

  const submit = (action: string) => (e: FormEvent) => {
    e.preventDefault();
    if (validName.test(name)) act(name, action).then(() => setName(""), () => {});
    else setMsg({ kind: "error", text: "Player names are 1-16 letters, numbers or underscores." });
  };

  return (
    <div className="grid items-start gap-6 lg:grid-cols-3">
      <div className="space-y-4 lg:col-span-2">
        {!running && (
          <Alert kind="info">
            The server is offline. You can see everyone who has played; start it to change bans, ops or the whitelist.
          </Alert>
        )}
        <Card
          title="Players"
          description={running && online.data ? `${online.data.online} of ${online.data.max} online now` : "Everyone who has joined this server"}
          flush
        >
          <div className="flex flex-wrap items-center gap-2 border-t border-line px-5 py-3">
            <div className="flex flex-wrap gap-1" role="tablist" aria-label="Filter players">
              {filters.map((f) => (
                <button
                  key={f.key}
                  role="tab"
                  aria-selected={filter === f.key}
                  onClick={() => setFilter(f.key)}
                  className={cx(
                    "rounded-md px-2.5 py-1 text-[13px] transition-colors",
                    filter === f.key ? "bg-hover font-medium text-fg" : "text-muted hover:text-fg",
                  )}
                >
                  {f.label} <span className="text-subtle tabular-nums">{counts[f.key]}</span>
                </button>
              ))}
            </div>
            <label className="ml-auto flex h-8 w-full items-center gap-2 rounded-md border border-line bg-surface px-2.5 focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-accent sm:w-48">
              <Search className="h-3.5 w-3.5 text-subtle" strokeWidth={1.75} />
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Find a player…"
                aria-label="Find a player"
                className="min-w-0 flex-1 bg-transparent text-[13px] placeholder:text-subtle focus:outline-none"
              />
            </label>
          </div>

          {known.isPending && (
            <div className="border-t border-line px-5 py-4">
              <Spinner />
            </div>
          )}
          {known.error && (
            <div className="border-t border-line px-5 py-4">
              <Alert>{known.error.message}</Alert>
            </div>
          )}
          {known.data && shown.length === 0 && (
            <p className="border-t border-line px-5 py-6 text-center text-sm text-muted">
              {rows.length === 0 ? "Nobody has joined yet." : "No players match."}
            </p>
          )}
          <ul className="divide-y divide-line border-t border-line">
            {shown.map((p) => (
              <li key={p.name} className="flex items-center gap-3 px-5 py-3">
                <img
                  src={`https://mc-heads.net/avatar/${encodeURIComponent(p.uuid || p.name)}/32`}
                  alt=""
                  className={cx("h-8 w-8 rounded", !p.online && "opacity-60 grayscale-[40%]")}
                  loading="lazy"
                />
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <span className="truncate font-medium">{p.name}</span>
                    {p.online && <Tag tone="ok">Online</Tag>}
                    {p.op && <Tag tone="accent">Op</Tag>}
                    {p.whitelisted && <Tag tone="muted">Whitelisted</Tag>}
                    {p.banned && <Tag tone="danger">Banned</Tag>}
                  </div>
                  <p className="mt-0.5 truncate text-[13px] text-muted" title={p.last_seen ? dateTime(p.last_seen) : undefined}>
                    {p.online ? "Playing now" : p.last_seen ? `Last seen ${timeAgo(p.last_seen)}` : "Never joined"}
                    {p.banned && p.ban_reason && ` · Banned for: ${p.ban_reason}`}
                  </p>
                </div>
                {operator && running && (
                  <div className="flex shrink-0 items-center gap-1">
                    {p.banned ? (
                      <Button size="sm" busy={busy === p.name + "pardon"} onClick={() => quiet(p.name, "pardon")}>
                        <UserCheck className="h-3.5 w-3.5" strokeWidth={1.75} /> Unban
                      </Button>
                    ) : (
                      p.online && (
                        <Button size="sm" variant="ghost" onClick={() => setDialog({ player: p.name, action: "kick" })}>
                          <LogOut className="h-3.5 w-3.5" strokeWidth={1.75} /> Kick
                        </Button>
                      )
                    )}
                    <ActionsMenu label={`Actions for ${p.name}`}>
                      <MenuItem icon={p.op ? ShieldOff : Crown} onClick={() => quiet(p.name, p.op ? "deop" : "op")}>
                        {p.op ? "Remove op" : "Make op"}
                      </MenuItem>
                      <MenuItem icon={p.whitelisted ? UserMinus : ShieldCheck} onClick={() => quiet(p.name, p.whitelisted ? "unwhitelist" : "whitelist")}>
                        {p.whitelisted ? "Remove from whitelist" : "Add to whitelist"}
                      </MenuItem>
                      {!p.banned && (
                        <>
                          <div className="my-1 h-px bg-line" />
                          <MenuItem icon={Ban} danger onClick={() => setDialog({ player: p.name, action: "ban" })}>
                            Ban
                          </MenuItem>
                        </>
                      )}
                    </ActionsMenu>
                  </div>
                )}
              </li>
            ))}
          </ul>
        </Card>
        {msg && <Alert kind={msg.kind}>{msg.text}</Alert>}
      </div>

      {operator && (
        <Card title="Add a player" description="For someone who hasn't joined yet, or to unban by name.">
          <form className="space-y-3" onSubmit={submit("whitelist")}>
            <Input value={name} onChange={(e) => setName(e.target.value.trim())} placeholder="Minecraft username" aria-label="Player name" disabled={!running} />
            <div className="grid grid-cols-2 gap-2">
              <Button type="submit" disabled={!name || !running}>
                <UserPlus className="h-4 w-4" strokeWidth={1.75} /> Whitelist
              </Button>
              <Button type="button" disabled={!name || !running} onClick={submit("op")}>
                <Crown className="h-4 w-4" strokeWidth={1.75} /> Make op
              </Button>
              <Button type="button" disabled={!name || !running} onClick={submit("pardon")} className="col-span-2">
                <UserCheck className="h-4 w-4" strokeWidth={1.75} /> Unban
              </Button>
            </div>
            <p className="text-xs text-muted">
              The whitelist only applies when it's on: turn on <span className="font-medium text-fg">Whitelist</span> under Settings → Game settings.
            </p>
          </form>
        </Card>
      )}

      <ConfirmDialog
        open={!!dialog}
        onClose={() => {
          setDialog(undefined);
          setReason("");
        }}
        title={dialog ? `${dialog.action === "ban" ? "Ban" : "Kick"} ${dialog.player}?` : ""}
        confirmLabel={dialog?.action === "ban" ? "Ban" : "Kick"}
        danger
        onConfirm={() => (dialog ? act(dialog.player, dialog.action, reason.trim()) : undefined)}
      >
        <p>
          {dialog?.action === "ban"
            ? "They're disconnected and can't join again until you unban them. You'll find them under Banned."
            : "They're disconnected, but can join again straight away."}
        </p>
        <Input
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          maxLength={100}
          placeholder="Reason (optional, shown to the player)"
          aria-label="Reason"
        />
      </ConfirmDialog>
    </div>
  );
}
