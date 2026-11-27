import { useState, type FormEvent } from "react";
import { Alert, Button, Card, Empty, Field, Input } from "../../ui";
import type { TabProps } from "./ServerPage";

export function Mods({ s, runJob }: TabProps) {
  const [slug, setSlug] = useState("");
  const [pack, setPack] = useState("");
  const [force, setForce] = useState(false);
  const busy = !!s.job;

  const addMod = (e: FormEvent) => {
    e.preventDefault();
    runJob("/mods", "post", { slug: slug.trim() })
      .then(() => setSlug(""))
      .catch(() => {});
  };
  const setModpack = (e: FormEvent) => {
    e.preventDefault();
    runJob("/modpack", "put", { slug: pack.trim(), force })
      .then(() => setPack(""))
      .catch(() => {});
  };

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <Card title="Modpack">
        {s.modpack ? (
          <p className="text-sm">
            Running <strong>{s.modpack.id}</strong> from {s.modpack.source === "curseforge" ? "CurseForge" : "Modrinth"}.
          </p>
        ) : (
          <p className="text-sm text-muted">No modpack. Mods below are installed individually.</p>
        )}
        <form onSubmit={setModpack} className="mt-4 space-y-3">
          <Field label={s.modpack ? "Switch to a Modrinth modpack" : "Install a Modrinth modpack"} hint="The modpack's slug from its Modrinth page, e.g. fabulously-optimized. The current world is backed up first.">
            <Input value={pack} onChange={(e) => setPack(e.target.value)} placeholder="modpack-slug" />
          </Field>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={force} onChange={(e) => setForce(e.target.checked)} />
            Allow an older Minecraft version (can break the world)
          </label>
          <Button type="submit" disabled={busy || !pack.trim()}>
            {s.modpack ? "Switch modpack" : "Install modpack"}
          </Button>
        </form>
      </Card>
      <Card title={`Mods (${s.mods.length})`}>
        {s.modpack && s.mods.length > 0 && <Alert kind="warn">These mods are ignored while a modpack is set.</Alert>}
        {s.mods.length === 0 ? (
          <Empty title="No extra mods" />
        ) : (
          <ul className="divide-y divide-line">
            {s.mods.map((m) => (
              <li key={m.id} className="flex items-center justify-between gap-2 py-2">
                <span className="text-sm font-medium">{m.mod_name}</span>
                <Button variant="ghost" className="text-xs text-danger" disabled={busy} onClick={() => runJob("/mods/" + encodeURIComponent(m.mod_id), "del").catch(() => {})}>
                  Remove
                </Button>
              </li>
            ))}
          </ul>
        )}
        <form onSubmit={addMod} className="mt-4 flex gap-2">
          <Input value={slug} onChange={(e) => setSlug(e.target.value)} placeholder="Modrinth mod slug, e.g. lithium" aria-label="Mod slug" />
          <Button type="submit" disabled={busy || !slug.trim()}>
            Add
          </Button>
        </form>
        <p className="mt-2 text-xs text-muted">Required dependencies are installed automatically. The server restarts to apply changes.</p>
      </Card>
    </div>
  );
}
