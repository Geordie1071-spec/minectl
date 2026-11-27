import { useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { api, errorMessage, srv, type Job, type UserWithGrants } from "../../api";
import { formatMB } from "../../format";
import { Alert, Button, Card, Field, Input, Select } from "../../ui";
import type { TabProps } from "./ServerPage";
import { GameSettings } from "./GameSettings";

export function Settings({ s, runJob }: TabProps) {
  const navigate = useNavigate();
  const busy = !!s.job;
  const [memGB, setMemGB] = useState(String(Math.round((s.memory_mb / 1024) * 10) / 10));
  const [cpu, setCpu] = useState(s.cpu_cores ? String(s.cpu_cores) : "none");
  const [version, setVersion] = useState("");
  const [confirmName, setConfirmName] = useState("");
  const [purge, setPurge] = useState(false);
  const [deleteError, setDeleteError] = useState("");

  const versions = useQuery({
    queryKey: ["versions", s.type],
    queryFn: () => api.get<string[]>("/versions/" + s.type),
    enabled: !s.modpack,
    staleTime: 10 * 60_000,
    retry: false,
  });
  const users = useQuery({ queryKey: ["users"], queryFn: () => api.get<UserWithGrants[]>("/users") });
  const withAccess = users.data?.flatMap((u) => {
    if (u.is_admin) return [{ name: u.username, role: "admin" }];
    const g = u.grants.find((g) => g.server === s.name);
    return g ? [{ name: u.username, role: g.role }] : [];
  });

  const resize = (e: FormEvent) => {
    e.preventDefault();
    runJob("/resize", "post", { memory: memGB === "auto" ? "auto" : `${Math.round(Number(memGB) * 1024)}M`, cpu }).catch(() => {});
  };
  const upgrade = (e: FormEvent) => {
    e.preventDefault();
    runJob("/upgrade", "post", { version }).catch(() => {});
  };
  const remove = async (e: FormEvent) => {
    e.preventDefault();
    setDeleteError("");
    try {
      const job = await api.del<Job>(srv(s.name) + (purge ? "?purge=true" : ""));
      navigate("/jobs/" + job.id);
    } catch (err) {
      setDeleteError(errorMessage(err));
    }
  };

  return (
    <div className="stagger grid gap-4 lg:grid-cols-2">
      <GameSettings s={s} runJob={runJob} />
      <Card title="Resources">
        <form onSubmit={resize} className="space-y-3">
          <p className="text-sm text-muted">
            Now: {formatMB(s.memory_mb)} RAM, {s.cpu_cores ? `${s.cpu_cores} CPU cores` : "no CPU limit"}.
          </p>
          <div className="grid grid-cols-2 gap-3">
            <Field label="Memory (GB)" hint='Or "auto" to use the recommendation.'>
              <Input value={memGB} onChange={(e) => setMemGB(e.target.value)} />
            </Field>
            <Field label="CPU cores" hint='A number, "auto", or "none".'>
              <Input value={cpu} onChange={(e) => setCpu(e.target.value)} />
            </Field>
          </div>
          <Button type="submit" disabled={busy}>
            Apply and restart
          </Button>
        </form>
      </Card>

      <Card title="Minecraft version">
        {s.modpack ? (
          <p className="text-sm text-muted">The modpack decides the version. Switch modpack versions from the Mods tab.</p>
        ) : (
          <form onSubmit={upgrade} className="space-y-3">
            <p className="text-sm text-muted">Now: {s.version}. A backup is made first. Minecraft can't move worlds to older versions.</p>
            <Field label="Upgrade to">
              {versions.data ? (
                <Select
                  value={version}
                  onChange={setVersion}
                  placeholder="Choose a version"
                  options={versions.data.map((v) => ({ value: v, label: v, disabled: v === s.version, hint: v === s.version ? "current" : undefined }))}
                />
              ) : (
                <Input value={version} onChange={(e) => setVersion(e.target.value)} placeholder="e.g. 26.3" />
              )}
            </Field>
            <Button type="submit" disabled={busy || !version}>
              Upgrade
            </Button>
          </form>
        )}
      </Card>

      <Card title="Who has access">
        {withAccess && (
          <ul className="divide-y divide-line text-sm">
            {withAccess.map((u) => (
              <li key={u.name} className="flex justify-between py-1.5">
                <span>{u.name}</span>
                <span className="text-muted">{u.role}</span>
              </li>
            ))}
          </ul>
        )}
        <p className="mt-3 text-xs text-muted">Give friends access from the People page.</p>
      </Card>

      <Card title="Delete server" className="border-danger/40">
        <form onSubmit={remove} className="space-y-3">
          <p className="text-sm text-muted">Removes the server from minectl and deletes its container.</p>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={purge} onChange={(e) => setPurge(e.target.checked)} />
            Also delete the world and all files (can't be undone)
          </label>
          <Field label={`Type ${s.name} to confirm`}>
            <Input value={confirmName} onChange={(e) => setConfirmName(e.target.value)} autoComplete="off" />
          </Field>
          {deleteError && <Alert>{deleteError}</Alert>}
          <Button variant="danger" type="submit" disabled={busy || confirmName !== s.name}>
            Delete {s.name}
          </Button>
        </form>
      </Card>
    </div>
  );
}
