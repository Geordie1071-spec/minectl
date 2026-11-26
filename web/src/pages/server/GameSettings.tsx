import { useEffect, useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, srv, type Properties } from "../../api";
import { Alert, Button, Card, Field, Input, Select, Spinner } from "../../ui";
import type { TabProps } from "./ServerPage";

type Flag = "pvp" | "hardcore" | "online_mode" | "whitelist" | "allow_flight";

const flags: { key: Flag; label: string; hint: string }[] = [
  { key: "pvp", label: "PvP", hint: "Players can hurt each other." },
  { key: "online_mode", label: "Online mode", hint: "Only real Minecraft accounts can join." },
  { key: "whitelist", label: "Whitelist", hint: "Only whitelisted players can join. Add them on the Players tab first." },
  { key: "allow_flight", label: "Allow flight", hint: "Don't kick players for flying (needed by some mods)." },
  { key: "hardcore", label: "Hardcore", hint: "One life; the world is locked to hard." },
];

const triState = [
  { value: "", label: "Default" },
  { value: "true", label: "On" },
  { value: "false", label: "Off" },
];

const fromFlag = (v: boolean | undefined) => (v === undefined ? "" : String(v));
const toFlag = (v: string) => (v === "" ? undefined : v === "true");

export function GameSettings({ s, runJob }: TabProps) {
  const q = useQuery({ queryKey: ["properties", s.name], queryFn: () => api.get<Properties>(srv(s.name) + "/properties") });
  const [p, setP] = useState<Properties>({});
  const [dirty, setDirty] = useState(false);
  useEffect(() => {
    if (q.data && !dirty) setP(q.data);
  }, [q.data, dirty]);

  const set = (patch: Partial<Properties>) => {
    setP((cur) => ({ ...cur, ...patch }));
    setDirty(true);
  };
  const num = (v: string) => (v === "" ? undefined : Number(v));

  const save = (e: FormEvent) => {
    e.preventDefault();
    const body: Properties = Object.fromEntries(Object.entries(p).filter(([, v]) => v !== undefined && v !== "" && !Number.isNaN(v)));
    runJob("/properties", "put", body)
      .then(() => setDirty(false))
      .catch(() => {});
  };

  return (
    <Card title="Game settings" description="Written to server.properties. Saving restarts the server." className="lg:col-span-2">
      {q.isPending ? (
        <Spinner />
      ) : (
        <form onSubmit={save} className="space-y-6">
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="sm:col-span-2">
              <Field label="Message of the day" hint="Shown under the server name in the multiplayer list. § colour codes work.">
                <Input value={p.motd ?? ""} onChange={(e) => set({ motd: e.target.value })} maxLength={150} placeholder="A Minecraft Server" />
              </Field>
            </div>
            <Field label="Difficulty">
              <Select
                value={p.difficulty ?? ""}
                onChange={(v) => set({ difficulty: v || undefined })}
                options={[{ value: "", label: "Default (easy)" }, ...["peaceful", "easy", "normal", "hard"].map((d) => ({ value: d, label: d[0]!.toUpperCase() + d.slice(1) }))]}
              />
            </Field>
            <Field label="Game mode">
              <Select
                value={p.gamemode ?? ""}
                onChange={(v) => set({ gamemode: v || undefined })}
                options={[{ value: "", label: "Default (survival)" }, ...["survival", "creative", "adventure", "spectator"].map((d) => ({ value: d, label: d[0]!.toUpperCase() + d.slice(1) }))]}
              />
            </Field>
            <Field label="Max players" hint="1–1000. Default 20.">
              <Input type="number" min={1} max={1000} value={p.max_players ?? ""} onChange={(e) => set({ max_players: num(e.target.value) })} placeholder="20" />
            </Field>
            <Field label="View distance" hint="3–32 chunks. Lower is lighter on the server. Default 10.">
              <Input type="number" min={3} max={32} value={p.view_distance ?? ""} onChange={(e) => set({ view_distance: num(e.target.value) })} placeholder="10" />
            </Field>
          </div>
          <div className="divide-y divide-line rounded-lg border border-line">
            {flags.map((f) => (
              <div key={f.key} className="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
                <div className="min-w-0">
                  <p className="text-sm font-medium">{f.label}</p>
                  <p className="text-[13px] text-muted">{f.hint}</p>
                </div>
                <Select value={fromFlag(p[f.key])} onChange={(v) => set({ [f.key]: toFlag(v) })} options={triState} className="w-32" aria-label={f.label} />
              </div>
            ))}
          </div>
          {p.whitelist && <Alert kind="warn">With the whitelist on, only players you've whitelisted can join, including you.</Alert>}
          <div className="flex items-center justify-end gap-2">
            {dirty && (
              <Button
                type="button"
                variant="ghost"
                onClick={() => {
                  setDirty(false);
                  setP(q.data ?? {});
                }}
              >
                Discard
              </Button>
            )}
            <Button type="submit" variant="primary" disabled={!dirty || !!s.job}>
              Save and restart
            </Button>
          </div>
        </form>
      )}
    </Card>
  );
}
