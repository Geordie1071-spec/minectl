import { useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { Archive, Box, LayoutGrid, List, Plus, Puzzle, Search, Star } from "lucide-react";
import { srv, type Server } from "../api";
import { useMe, usePlayers, useServers } from "../hooks";
import { useJoinAddress } from "../join";
import { isModded, timeAgo, typeLabels } from "../format";
import { jobLabel } from "../JobView";
import { Alert, Button, Empty, LiveDot, Select, Skeleton, Spinner, cx } from "../ui";

type Sort = "recent" | "name";
type View = "grid" | "list";

// Small per-browser preferences; storage can be unavailable (private mode).
function useStored<T>(key: string, initial: T): [T, (v: T) => void] {
  const [v, setV] = useState<T>(() => {
    try {
      const raw = localStorage.getItem(key);
      return raw ? (JSON.parse(raw) as T) : initial;
    } catch {
      return initial;
    }
  });
  const set = (next: T) => {
    setV(next);
    try {
      localStorage.setItem(key, JSON.stringify(next));
    } catch {
      // not persisted
    }
  };
  return [v, set];
}

function statusLine(s: Server, online?: { online: number; max: number }) {
  if (s.job) return `${jobLabel(s.job.kind)}…`;
  if (s.status === "running") return online ? `${online.online}/${online.max} players online` : "online";
  return s.status === "stopped" ? `stopped · ${timeAgo(s.last_started_at)}` : s.status;
}

function NodeIcon({ icon: I, label }: { icon: typeof Box; label: string }) {
  return (
    <span
      title={label}
      className="flex h-10 w-10 items-center justify-center rounded-lg border border-line-strong bg-surface-2 text-fg shadow-card transition-transform duration-300 group-hover:-translate-y-0.5"
    >
      <I className="h-[18px] w-[18px]" strokeWidth={1.75} />
    </span>
  );
}

function ServerCard({ s, starred, onStar }: { s: Server; starred: boolean; onStar: () => void }) {
  const running = s.status === "running" && !s.job;
  const players = usePlayers(s.name, running);
  return (
    <Link
      to={srv(s.name)}
      className="group flex flex-col rounded-xl border border-line bg-surface-2 p-1.5 transition-[border-color,transform,box-shadow] duration-200 hover:-translate-y-0.5 hover:border-line-strong hover:shadow-pop"
    >
      <div className="flex items-center justify-between gap-2 px-2.5 pt-1.5 pb-2.5">
        <h2 className="truncate font-medium">{s.name}</h2>
        <button
          type="button"
          aria-label={starred ? `Unstar ${s.name}` : `Star ${s.name}`}
          aria-pressed={starred}
          onClick={(e) => {
            e.preventDefault();
            e.stopPropagation();
            onStar();
          }}
          className="rounded p-1 text-subtle transition-colors hover:bg-hover hover:text-fg"
        >
          <Star className={cx("h-4 w-4 transition-transform active:scale-75", starred && "fill-warn text-warn")} strokeWidth={1.75} />
        </button>
      </div>
      <div className="dots flex h-44 flex-col rounded-lg border border-line bg-surface">
        <div className="flex flex-1 items-center justify-center gap-2.5">
          <NodeIcon icon={Box} label={`${typeLabels[s.type] ?? s.type} ${s.version}`} />
          {(isModded(s.type) || s.modpack) && <NodeIcon icon={Puzzle} label={s.modpack ? s.modpack.id : `${s.mods.length} mods`} />}
          <NodeIcon icon={Archive} label="World and backups" />
        </div>
        <div className="flex items-center gap-2 px-3 pb-2.5 text-[13px] text-muted">
          {s.job ? <Spinner className="h-3 w-3 text-warn" /> : <LiveDot status={s.status} />}
          <span className="truncate">{s.modpack ? s.modpack.id : `${typeLabels[s.type] ?? s.type} ${s.version}`}</span>
          <span className="text-subtle" aria-hidden="true">
            ·
          </span>
          <span className="truncate">{statusLine(s, running ? players.data : undefined)}</span>
        </div>
      </div>
    </Link>
  );
}

function ServerRow({ s, starred, onStar }: { s: Server; starred: boolean; onStar: () => void }) {
  const running = s.status === "running" && !s.job;
  const players = usePlayers(s.name, running);
  const { address } = useJoinAddress(s.port);
  return (
    <Link to={srv(s.name)} className="group flex items-center gap-4 px-4 py-3.5 transition-colors hover:bg-hover/50">
      <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-line-strong bg-surface">
        <Box className="h-4 w-4" strokeWidth={1.75} />
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate font-medium">{s.name}</p>
        <p className="truncate font-mono text-[13px] text-muted">{address}</p>
      </div>
      <span className="hidden w-40 truncate text-[13px] text-muted md:block">
        {s.modpack ? s.modpack.id : `${typeLabels[s.type] ?? s.type} ${s.version}`}
      </span>
      <span className="hidden w-44 items-center gap-2 text-[13px] text-muted sm:flex">
        {s.job ? <Spinner className="h-3 w-3 text-warn" /> : <LiveDot status={s.status} />}
        <span className="truncate">{statusLine(s, running ? players.data : undefined)}</span>
      </span>
      <button
        type="button"
        aria-label={starred ? `Unstar ${s.name}` : `Star ${s.name}`}
        aria-pressed={starred}
        onClick={(e) => {
          e.preventDefault();
          onStar();
        }}
        className="rounded p-1 text-subtle hover:bg-hover hover:text-fg"
      >
        <Star className={cx("h-4 w-4", starred && "fill-warn text-warn")} strokeWidth={1.75} />
      </button>
    </Link>
  );
}

export function Dashboard() {
  const { data: me } = useMe();
  const { data, error, isPending } = useServers();
  const [q, setQ] = useState("");
  const [sort, setSort] = useStored<Sort>("minectl.sort", "recent");
  const [view, setView] = useStored<View>("minectl.view", "grid");
  const [stars, setStars] = useStored<string[]>("minectl.stars", []);
  const search = useRef<HTMLInputElement>(null);
  const isAdmin = me?.user.is_admin;

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        search.current?.focus();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const shown = useMemo(() => {
    const list = (data ?? []).filter((s) => (s.name + " " + s.type + " " + (s.modpack?.id ?? "")).toLowerCase().includes(q.toLowerCase()));
    const recent = (s: Server) => Math.max(new Date(s.last_started_at).getTime(), new Date(s.created_at).getTime());
    list.sort((a, b) => {
      const star = Number(stars.includes(b.name)) - Number(stars.includes(a.name));
      if (star) return star;
      return sort === "name" ? a.name.localeCompare(b.name) : recent(b) - recent(a);
    });
    return list;
  }, [data, q, sort, stars]);

  const toggleStar = (name: string) => setStars(stars.includes(name) ? stars.filter((n) => n !== name) : [...stars, name]);
  const mac = /Mac/.test(navigator.platform);

  return (
    <>
      <div className="mb-8 flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-[28px] leading-tight font-medium tracking-tight">Servers</h1>
        <div className="flex w-full items-center gap-2 sm:w-auto">
          <label className="flex h-9 flex-1 items-center gap-2 rounded-md border border-line bg-surface px-3 transition-colors focus-within:border-fg/30 hover:border-line-strong sm:w-64 sm:flex-none">
            <Search className="h-4 w-4 shrink-0 text-subtle" strokeWidth={1.75} />
            <input
              ref={search}
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Search servers…"
              aria-label="Search servers"
              className="min-w-0 flex-1 bg-transparent text-sm placeholder:text-subtle focus:outline-none"
            />
            <span className="hidden items-center gap-0.5 sm:flex" aria-hidden="true">
              <kbd className="rounded border border-line bg-surface-2 px-1 font-mono text-[11px] text-subtle">{mac ? "⌘" : "Ctrl"}</kbd>
              <kbd className="rounded border border-line bg-surface-2 px-1 font-mono text-[11px] text-subtle">K</kbd>
            </span>
          </label>
          {isAdmin && (
            <Link to="/new">
              <Button variant="primary">
                <Plus className="h-4 w-4" strokeWidth={2} /> New
              </Button>
            </Link>
          )}
        </div>
      </div>

      {error && <Alert>{error.message}</Alert>}

      {isPending && (
        <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-60 rounded-xl" />
          ))}
        </div>
      )}

      {data && data.length === 0 && (
        <Empty title={isAdmin ? "Create your first server" : "No servers shared with you yet"} icon={<img src="/mark.png" alt="" className="pixelated h-12 w-12" />}>
          {isAdmin ? (
            <>
              Start from a server type, a Modrinth modpack, or a CurseForge pack. It's ready to join in about a minute.
              <div className="mt-5">
                <Link to="/new">
                  <Button variant="primary">
                    <Plus className="h-4 w-4" strokeWidth={2} /> New server
                  </Button>
                </Link>
              </div>
            </>
          ) : (
            "Ask an admin to give you access to a server."
          )}
        </Empty>
      )}

      {data && data.length > 0 && (
        <>
          <div className="mb-4 flex items-center justify-between gap-3">
            <div className="flex items-center gap-3 text-sm">
              <span className="flex items-center gap-1.5">
                <LayoutGrid className="h-4 w-4 text-subtle" strokeWidth={1.75} />
                {shown.length} {shown.length === 1 ? "Server" : "Servers"}
              </span>
              <span className="h-4 w-px bg-line" aria-hidden="true" />
              <span className="flex items-center gap-1 text-muted">
                Sort by:
                <Select
                  variant="inline"
                  value={sort}
                  onChange={(v) => setSort(v as Sort)}
                  aria-label="Sort servers by"
                  options={[
                    { value: "recent", label: "Recent activity" },
                    { value: "name", label: "Name" },
                  ]}
                />
              </span>
            </div>
            <div className="flex rounded-md border border-line p-0.5" role="group" aria-label="View">
              {(
                [
                  ["grid", LayoutGrid],
                  ["list", List],
                ] as const
              ).map(([v, I]) => (
                <button
                  key={v}
                  type="button"
                  onClick={() => setView(v)}
                  aria-pressed={view === v}
                  aria-label={v === "grid" ? "Grid view" : "List view"}
                  className={cx("rounded p-1.5 transition-colors", view === v ? "bg-hover text-fg" : "text-subtle hover:text-fg")}
                >
                  <I className="h-4 w-4" strokeWidth={1.75} />
                </button>
              ))}
            </div>
          </div>

          {shown.length === 0 && <p className="py-8 text-center text-sm text-muted">No servers match “{q}”.</p>}

          {view === "grid" ? (
            <div key="grid" className="stagger grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
              {shown.map((s) => (
                <ServerCard key={s.name} s={s} starred={stars.includes(s.name)} onStar={() => toggleStar(s.name)} />
              ))}
            </div>
          ) : (
            shown.length > 0 && (
              <div key="list" className="stagger divide-y divide-line overflow-hidden rounded-xl border border-line bg-surface-2">
                {shown.map((s) => (
                  <ServerRow key={s.name} s={s} starred={stars.includes(s.name)} onStar={() => toggleStar(s.name)} />
                ))}
              </div>
            )
          )}
        </>
      )}
    </>
  );
}
