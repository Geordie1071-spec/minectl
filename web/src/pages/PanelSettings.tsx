import { useEffect, useState, type FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, errorMessage, type PanelSettings as Settings } from "../api";
import { Alert, Button, Card, Field, Input } from "../ui";

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

      {msg && (
        <div className="lg:col-span-2">
          <Alert kind={msg.kind}>{msg.text}</Alert>
        </div>
      )}
    </>
  );
}
