import { useEffect, useState, type FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, errorMessage, type PanelSettings as Settings } from "../api";
import { Alert, Button, Card, ConfirmDialog, Field, Input, Spinner, buttonClass } from "../ui";

// PanelSettings holds admin-only settings for the whole panel.
export function PanelSettings() {
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ["settings"], queryFn: () => api.get<Settings>("/settings") });
  const [hook, setHook] = useState("");
  const [retention, setRetention] = useState("");
  const [msg, setMsg] = useState<{ kind: "info" | "error"; text: string }>();
  const [busy, setBusy] = useState("");

  useEffect(() => {
    if (q.data) setRetention(String(q.data.backup_retention));
  }, [q.data]);

  const run = async (what: string, fn: () => Promise<unknown>, ok: string) => {
    setBusy(what);
    setMsg(undefined);
    try {
      await fn();
      setMsg({ kind: "info", text: ok });
      qc.invalidateQueries({ queryKey: ["settings"] });
    } catch (err) {
      setMsg({ kind: "error", text: errorMessage(err) });
    } finally {
      setBusy("");
    }
  };

  const saveHook = (e: FormEvent) => {
    e.preventDefault();
    run("hook", () => api.put("/settings", { discord_webhook: hook.trim() }), hook.trim() ? "Discord alerts are on." : "Discord alerts are off.").then(() => setHook(""));
  };

  return (
    <>
      <Card
        title="Alerts"
        description="Get a Discord message when a server crashes, keeps crashing and is stopped, or a scheduled backup fails."
      >
        <form onSubmit={saveHook} className="space-y-3">
          <p className="text-sm">
            Discord:{" "}
            {q.data?.discord_webhook_set ? <span className="font-medium text-ok">connected</span> : <span className="text-muted">not set up</span>}
          </p>
          <Field label="Webhook URL" hint="In Discord: Server Settings → Integrations → Webhooks → New Webhook → Copy Webhook URL.">
            <Input
              value={hook}
              onChange={(e) => setHook(e.target.value)}
              placeholder={q.data?.discord_webhook_set ? "Paste a new URL to replace it" : "https://discord.com/api/webhooks/…"}
              autoComplete="off"
              spellCheck={false}
            />
          </Field>
          <div className="flex flex-wrap gap-2">
            <Button type="submit" variant="primary" busy={busy === "hook"} disabled={!hook.trim()}>
              Save
            </Button>
            {q.data?.discord_webhook_set && (
              <>
                <Button type="button" busy={busy === "test"} onClick={() => run("test", () => api.post("/settings/test-notification"), "Test message sent. Check your Discord channel.")}>
                  Send a test
                </Button>
                <Button type="button" variant="ghost" busy={busy === "off"} onClick={() => run("off", () => api.put("/settings", { discord_webhook: "" }), "Discord alerts are off.")}>
                  Turn off
                </Button>
              </>
            )}
          </div>
        </form>
      </Card>

      <Playit linked={!!q.data?.playit_linked} />

      <Card title="Backups" description="How many backups each server keeps. Older ones are deleted when a new backup is made.">
        <form
          className="flex flex-wrap items-end gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            run("ret", () => api.put("/settings", { backup_retention: Number(retention) }), "Saved.");
          }}
        >
          <div className="w-32">
            <Field label="Keep">
              <Input type="number" min={1} max={100} value={retention} onChange={(e) => setRetention(e.target.value)} />
            </Field>
          </div>
          <Button type="submit" busy={busy === "ret"} disabled={!retention || String(q.data?.backup_retention) === retention}>
            Save
          </Button>
        </form>
      </Card>

      <Card
        title="Panel backup"
        description="Users, roles, settings and API tokens. A copy is saved every day and the last 7 are kept in ~/.minectl/panel-backups. Server worlds are backed up separately."
      >
        <div className="flex flex-wrap items-center gap-3">
          <a href="/api/v1/settings/backup" download className={buttonClass("secondary")}>
            Download a backup now
          </a>
          <p className="text-[13px] text-muted">It holds password hashes and saved secrets, so keep it somewhere private.</p>
        </div>
      </Card>

      {msg && (
        <div className="lg:col-span-2">
          <Alert kind={msg.kind}>{msg.text}</Alert>
        </div>
      )}
    </>
  );
}

// Playit links a playit.gg account: the user approves a claim code on
// playit.gg while the panel polls until the agent key arrives.
function Playit({ linked }: { linked: boolean }) {
  const qc = useQueryClient();
  const [claim, setClaim] = useState<{ code: string; url: string }>();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [unlinkOpen, setUnlinkOpen] = useState(false);

  useEffect(() => {
    if (!claim) return;
    let stopped = false;
    const poll = async () => {
      while (!stopped) {
        try {
          const { status } = await api.post<{ status: string }>("/settings/playit/claim/" + claim.code);
          if (status === "linked") {
            setClaim(undefined);
            qc.invalidateQueries({ queryKey: ["settings"] });
            return;
          }
          if (status === "rejected") {
            setClaim(undefined);
            setError("The link was rejected on playit.gg.");
            return;
          }
        } catch (err) {
          setError(errorMessage(err));
        }
        await new Promise((r) => setTimeout(r, 2000));
      }
    };
    poll();
    return () => {
      stopped = true;
    };
  }, [claim, qc]);

  const start = async () => {
    setError("");
    try {
      const c = await api.post<{ code: string; url: string }>("/settings/playit/claim");
      setClaim(c);
      window.open(c.url, "_blank", "noopener");
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const unlink = async () => {
    setBusy(true);
    setError("");
    try {
      await api.del("/settings/playit");
      qc.invalidateQueries({ queryKey: ["settings"] });
      qc.invalidateQueries({ queryKey: ["servers"] });
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card
      title="Tunnels"
      description="When a router can't forward ports, servers are made public through a free playit.gg tunnel. Link an account once to use it."
    >
      <div className="space-y-3">
        <p className="text-sm">
          playit.gg: {linked ? <span className="font-medium text-ok">linked</span> : <span className="text-muted">not linked</span>}
        </p>
        {claim && (
          <Alert kind="info">
            Approve minectl on playit.gg (sign up for free if you need to). If the page didn't open:{" "}
            <a href={claim.url} target="_blank" rel="noopener noreferrer" className="font-mono underline underline-offset-2">
              {claim.url}
            </a>
            <span className="mt-1 flex items-center gap-2 text-muted">
              <Spinner className="h-3 w-3" /> Waiting for approval…
            </span>
          </Alert>
        )}
        {error && <Alert>{error}</Alert>}
        <div className="flex flex-wrap gap-2">
          {linked ? (
            <Button type="button" variant="ghost" busy={busy} onClick={() => setUnlinkOpen(true)}>
              Unlink
            </Button>
          ) : (
            <Button type="button" variant="primary" disabled={!!claim} onClick={start}>
              Link playit.gg
            </Button>
          )}
          {claim && (
            <Button type="button" variant="ghost" onClick={() => setClaim(undefined)}>
              Cancel
            </Button>
          )}
        </div>
      </div>
      <ConfirmDialog open={unlinkOpen} onClose={() => setUnlinkOpen(false)} title="Unlink playit.gg?" confirmLabel="Unlink" danger onConfirm={unlink}>
        <p>Every playit.gg tunnel is closed, and servers that use one become private until you make them public again.</p>
      </ConfirmDialog>
    </Card>
  );
}
