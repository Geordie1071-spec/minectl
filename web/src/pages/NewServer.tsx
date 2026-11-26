import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { api, errorMessage, type Job, type Modpack, type Recommendation } from "../api";
import { PageTitle } from "../layout";
import { compact, formatMB, typeLabels } from "../format";
import { Alert, Button, Card, Field, Input, Select, Spinner, cx } from "../ui";

type Source = "type" | "modrinth" | "curseforge";

const sources: { id: Source; title: string; body: string }[] = [
  { id: "type", title: "Server type", body: "Paper, Vanilla, Fabric, Forge and more" },
  { id: "modrinth", title: "Modrinth modpack", body: "Search and install in one click" },
  { id: "curseforge", title: "CurseForge pack", body: "Paste a link to the pack" },
];

const types: { id: string; body: string }[] = [
  { id: "paper", body: "Fast, supports plugins. Best for most servers." },
  { id: "vanilla", body: "The official server, unchanged." },
  { id: "fabric", body: "Lightweight mod loader." },
  { id: "forge", body: "The classic mod loader." },
  { id: "neoforge", body: "Modern fork of Forge." },
  { id: "quilt", body: "Fabric-compatible mod loader." },
  { id: "spigot", body: "Plugin server; Paper is usually better." },
];

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return v;
}

// typingName cleans a name as it's typed, keeping a trailing dash so
// "my-" can become "my-server". toServerName also trims the end.
function typingName(s: string): string {
  return s
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, "-")
    .replace(/^-+/, "")
    .replace(/-{2,}/g, "-")
    .slice(0, 32);
}

export function toServerName(s: string): string {
  return typingName(s).replace(/-+$/, "");
}

function curseforgeSlug(input: string): string {
  const m = input.match(/curseforge\.com\/minecraft\/modpacks\/([^/?#]+)/);
  return m ? m[1]! : input.trim();
}

function ModrinthPicker({ selected, onSelect }: { selected?: Modpack; onSelect: (p: Modpack | undefined) => void }) {
  const [q, setQ] = useState("");
  const query = useDebounced(q, 300);
  const results = useQuery({
    queryKey: ["modpacks", query],
    queryFn: () => api.get<Modpack[]>("/modpacks/search?q=" + encodeURIComponent(query)),
    enabled: !selected,
    staleTime: 60_000,
  });

  if (selected) {
    return (
      <div className="flex items-center gap-3 rounded-md border border-accent bg-accent-soft p-3">
        {selected.icon_url && <img src={selected.icon_url} alt="" className="h-10 w-10 rounded" />}
        <div className="min-w-0 flex-1">
          <p className="font-medium">{selected.title}</p>
          <p className="truncate text-sm text-muted">{selected.description}</p>
        </div>
        <Button variant="ghost" onClick={() => onSelect(undefined)}>
          Change
        </Button>
      </div>
    );
  }
  return (
    <div className="space-y-2">
      <Input placeholder="Search Modrinth modpacks…" value={q} onChange={(e) => setQ(e.target.value)} autoFocus />
      {results.isFetching && !results.data && <Spinner />}
      {results.error && <Alert>{results.error.message}</Alert>}
      <ul className="max-h-96 divide-y divide-line overflow-y-auto rounded-md border border-line">
        {results.data?.map((p) => (
          <li key={p.project_id}>
            <button type="button" onClick={() => onSelect(p)} className="flex w-full items-center gap-3 p-3 text-left hover:bg-surface-2">
              {p.icon_url ? <img src={p.icon_url} alt="" className="h-10 w-10 shrink-0 rounded" loading="lazy" /> : <div className="h-10 w-10 shrink-0 rounded bg-surface-2" />}
              <div className="min-w-0">
                <p className="font-medium">
                  {p.title} <span className="text-xs font-normal text-muted">by {p.author}</span>
                </p>
                <p className="truncate text-sm text-muted">{p.description}</p>
              </div>
              <span className="ml-auto shrink-0 text-xs text-muted">{compact(p.downloads)} ↓</span>
            </button>
          </li>
        ))}
        {results.data?.length === 0 && <li className="p-3 text-sm text-muted">No modpacks found.</li>}
      </ul>
    </div>
  );
}

export function NewServer() {
  const navigate = useNavigate();
  const [source, setSource] = useState<Source>("type");
  const [type, setType] = useState("paper");
  const [version, setVersion] = useState("");
  const [pack, setPack] = useState<Modpack>();
  const [cfInput, setCfInput] = useState("");
  const [name, setName] = useState("");
  const [nameTouched, setNameTouched] = useState(false);
  const [players, setPlayers] = useState(5);
  const [memoryMode, setMemoryMode] = useState<"auto" | "custom">("auto");
  const [memoryGB, setMemoryGB] = useState(4);
  const [port, setPort] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const versions = useQuery({
    queryKey: ["versions", type],
    queryFn: () => api.get<string[]>("/versions/" + type),
    enabled: source === "type",
    staleTime: 10 * 60_000,
    retry: false,
  });

  const suggested = source === "modrinth" ? pack?.slug ?? "" : source === "curseforge" ? curseforgeSlug(cfInput) : "";
  useEffect(() => {
    if (!nameTouched && suggested) setName(toServerName(suggested));
  }, [suggested, nameTouched]);

  const memory = memoryMode === "auto" ? "auto" : `${memoryGB}G`;
  const recParams = useDebounced(
    useMemo(() => {
      const p = new URLSearchParams({ players: String(players), memory });
      if (source === "type") p.set("type", type);
      if (source === "modrinth" && pack) p.set("modpack", pack.slug);
      return p.toString();
    }, [players, memory, source, type, pack]),
    250,
  );
  const recEnabled = source === "type" || (source === "modrinth" && !!pack);
  const recQuery = useQuery({
    queryKey: ["recommend", recParams],
    queryFn: () => api.get<Recommendation>("/recommend?" + recParams),
    enabled: recEnabled,
    placeholderData: (prev) => prev,
    staleTime: 60_000,
  });
  // Don't show the last server type's numbers while no modpack is picked.
  const rec = { ...recQuery, data: recEnabled && !recQuery.error ? recQuery.data : undefined };
  const hostGB = rec.data?.host.memory_mb ? Math.floor(rec.data.host.memory_mb / 1024) : 32;

  const ready = name && (source === "type" || (source === "modrinth" && pack) || (source === "curseforge" && cfInput.trim()));

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError("");
    const body: Record<string, unknown> = { name: toServerName(name), players, memory };
    if (port) body.port = Number(port);
    if (source === "type") {
      body.type = type;
      if (version) body.version = version;
    } else if (source === "modrinth") {
      body.modpack = pack?.slug;
    } else {
      body.curseforge = cfInput.trim();
    }
    try {
      const job = await api.post<Job>("/servers", body);
      navigate("/jobs/" + job.id);
    } catch (err) {
      setError(errorMessage(err));
      setBusy(false);
    }
  };

  return (
    <div className="mx-auto max-w-3xl">
      <PageTitle description="Run a server type, a Modrinth modpack, or a CurseForge pack.">New server</PageTitle>
      <form onSubmit={submit} className="space-y-6">
        <Card title="1. What do you want to run?">
          <div className="grid gap-2 sm:grid-cols-3" role="radiogroup">
            {sources.map((s) => (
              <button
                key={s.id}
                type="button"
                role="radio"
                aria-checked={source === s.id}
                onClick={() => setSource(s.id)}
                className={cx(
                  "rounded-md border p-3 text-left transition-colors",
                  source === s.id ? "border-accent bg-accent-soft" : "border-line hover:bg-surface-2",
                )}
              >
                <p className="text-sm font-medium">{s.title}</p>
                <p className="mt-0.5 text-xs text-muted">{s.body}</p>
              </button>
            ))}
          </div>

          <div className="mt-5">
            {source === "type" && (
              <div className="space-y-4">
                <div className="grid gap-2 sm:grid-cols-2">
                  {types.map((t) => (
                    <label
                      key={t.id}
                      className={cx(
                        "flex cursor-pointer items-start gap-2 rounded-md border p-2.5",
                        type === t.id ? "border-accent bg-accent-soft" : "border-line hover:bg-surface-2",
                      )}
                    >
                      <input
                        type="radio"
                        name="type"
                        className="mt-1 accent-[var(--accent)]"
                        checked={type === t.id}
                        onChange={() => {
                          setType(t.id);
                          setVersion("");
                        }}
                      />
                      <span>
                        <span className="text-sm font-medium">{typeLabels[t.id]}</span>
                        <span className="block text-xs text-muted">{t.body}</span>
                      </span>
                    </label>
                  ))}
                </div>
                <Field label="Minecraft version" hint="Players need the same version in their launcher.">
                  {versions.data ? (
                    <Select value={version} onChange={(e) => setVersion(e.target.value)}>
                      <option value="">Latest ({versions.data[0]})</option>
                      {versions.data.map((v) => (
                        <option key={v} value={v}>
                          {v}
                        </option>
                      ))}
                    </Select>
                  ) : (
                    <Input placeholder={versions.isPending ? "Loading versions…" : "Latest"} value={version} onChange={(e) => setVersion(e.target.value)} />
                  )}
                </Field>
              </div>
            )}
            {source === "modrinth" && <ModrinthPicker selected={pack} onSelect={setPack} />}
            {source === "curseforge" && (
              <Field
                label="CurseForge pack"
                hint="Paste the pack's page link, e.g. curseforge.com/minecraft/modpacks/better-mc-forge-bmc4. The pack installs before the server starts, which can take several minutes."
              >
                <Input placeholder="https://www.curseforge.com/minecraft/modpacks/…" value={cfInput} onChange={(e) => setCfInput(e.target.value)} />
              </Field>
            )}
          </div>
        </Card>

        <Card title="2. Name and players">
          <div className="space-y-4">
            <Field label="Server name" hint="Lowercase letters, numbers and dashes.">
              <Input
                value={name}
                onChange={(e) => {
                  setName(typingName(e.target.value));
                  setNameTouched(true);
                }}
                onBlur={() => setName((n) => toServerName(n))}
                placeholder="survival"
                required
                maxLength={32}
              />
            </Field>
            <Field label={`Expected players: ${players}`} hint="Used to recommend memory and CPU.">
              <input
                type="range"
                min={1}
                max={50}
                value={players}
                onChange={(e) => setPlayers(Number(e.target.value))}
                className="w-full accent-[var(--accent)]"
              />
            </Field>
          </div>
        </Card>

        <Card title="3. Resources">
          <div className="space-y-4">
            <div className="grid gap-2 sm:grid-cols-2" role="radiogroup">
              {(["auto", "custom"] as const).map((m) => (
                <button
                  key={m}
                  type="button"
                  role="radio"
                  aria-checked={memoryMode === m}
                  onClick={() => {
                    setMemoryMode(m);
                    if (m === "custom" && rec.data) setMemoryGB(Math.max(1, Math.round(rec.data.recommended.memory_mb / 1024)));
                  }}
                  className={cx("rounded-md border p-3 text-left", memoryMode === m ? "border-accent bg-accent-soft" : "border-line hover:bg-surface-2")}
                >
                  <p className="text-sm font-medium">{m === "auto" ? "Recommended" : "Choose myself"}</p>
                  <p className="mt-0.5 text-xs text-muted">
                    {m === "auto"
                      ? source === "curseforge"
                        ? "Picked after the pack is installed"
                        : rec.data
                          ? `${formatMB(rec.data.recommended.memory_mb)} RAM, ${rec.data.recommended.cpu_cores} CPU cores`
                          : "Based on players and mods"
                      : "Set the memory yourself"}
                  </p>
                </button>
              ))}
            </div>
            {memoryMode === "custom" && (
              <Field label={`Memory: ${memoryGB} GB`}>
                <input
                  type="range"
                  min={1}
                  max={Math.max(2, hostGB)}
                  value={memoryGB}
                  onChange={(e) => setMemoryGB(Number(e.target.value))}
                  className="w-full accent-[var(--accent)]"
                />
              </Field>
            )}
            {recQuery.error && recEnabled && <Alert>{recQuery.error.message}</Alert>}
            {recQuery.isFetching && source === "modrinth" && !rec.data && (
              <p className="flex items-center gap-2 text-sm text-muted">
                <Spinner /> Reading the modpack…
              </p>
            )}
            {rec.data && source !== "curseforge" && (
              <p className="text-xs text-muted">
                {rec.data.recommended.reason}
                {rec.data.modpack && ` · ${rec.data.mods} server mods, ${typeLabels[rec.data.type] ?? rec.data.type} ${rec.data.modpack.mc_version}`}
                {rec.data.host.memory_mb > 0 && ` · this machine has ${formatMB(rec.data.host.memory_mb)} and ${rec.data.host.cpus} cores`}
              </p>
            )}
            {rec.data?.warnings.map((w) => (
              <Alert key={w} kind="warn">
                {w}
              </Alert>
            ))}
            <details className="text-sm">
              <summary className="cursor-pointer text-muted">Advanced</summary>
              <div className="mt-3 max-w-xs">
                <Field label="Port" hint="Leave empty to pick the next free one from 25565.">
                  <Input type="number" min={1024} max={65535} value={port} onChange={(e) => setPort(e.target.value)} placeholder="auto" />
                </Field>
              </div>
            </details>
          </div>
        </Card>

        {error && <Alert>{error}</Alert>}
        <div className="flex justify-end gap-2">
          <Button type="button" onClick={() => navigate("/")}>
            Cancel
          </Button>
          <Button variant="primary" type="submit" busy={busy} disabled={!ready}>
            Create server
          </Button>
        </div>
      </form>
    </div>
  );
}
