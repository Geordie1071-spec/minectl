import { useState, type FormEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { api, errorMessage, srv } from "../../api";
import { usePlayers } from "../../hooks";
import { Alert, Button, Card, Empty, Input, Spinner } from "../../ui";
import { can, type TabProps } from "./ServerPage";

const validName = /^[A-Za-z0-9_]{1,16}$/;

export function PlayersTab({ s }: TabProps) {
  const running = s.status === "running" && !s.job;
  const players = usePlayers(s.name, running);
  const qc = useQueryClient();
  const [msg, setMsg] = useState<{ kind: "info" | "error"; text: string }>();
  const [name, setName] = useState("");
  const [busy, setBusy] = useState("");
  const operator = can(s, "operator");

  const act = async (player: string, action: string) => {
    setBusy(player + action);
    setMsg(undefined);
    try {
      const res = await api.post<{ output: string }>(`${srv(s.name)}/players/${encodeURIComponent(player)}/${action}`);
      setMsg({ kind: "info", text: res.output || "Done." });
      qc.invalidateQueries({ queryKey: ["players", s.name] });
    } catch (err) {
      setMsg({ kind: "error", text: errorMessage(err) });
    } finally {
      setBusy("");
    }
  };

  const submit = (action: string) => (e: FormEvent) => {
    e.preventDefault();
    if (validName.test(name)) act(name, action);
    else setMsg({ kind: "error", text: "Player names are 1-16 letters, numbers or underscores." });
  };

  if (!running) return <Empty title="The server is offline">Start it to see and manage players.</Empty>;

  return (
    <div className="grid gap-4 lg:grid-cols-3">
      <div className="lg:col-span-2">
        <Card title={players.data ? `Online (${players.data.online}/${players.data.max})` : "Online"}>
          {players.isPending && <Spinner />}
          {players.error && <Alert>{players.error.message}</Alert>}
          {players.data?.names.length === 0 && <p className="text-sm text-muted">Nobody's online right now.</p>}
          <ul className="divide-y divide-line">
            {players.data?.names.map((p) => (
              <li key={p} className="flex flex-wrap items-center justify-between gap-2 py-2">
                <span className="flex items-center gap-2 font-medium">
                  <img src={`https://mc-heads.net/avatar/${encodeURIComponent(p)}/24`} alt="" className="h-6 w-6 rounded" loading="lazy" />
                  {p}
                </span>
                {operator && (
                  <span className="flex flex-wrap gap-1">
                    <Button variant="ghost" className="text-xs" busy={busy === p + "op"} onClick={() => act(p, "op")}>
                      Op
                    </Button>
                    <Button variant="ghost" className="text-xs" busy={busy === p + "deop"} onClick={() => act(p, "deop")}>
                      De-op
                    </Button>
                    <Button variant="ghost" className="text-xs" busy={busy === p + "kick"} onClick={() => act(p, "kick")}>
                      Kick
                    </Button>
                    <Button
                      variant="ghost"
                      className="text-xs text-danger"
                      busy={busy === p + "ban"}
                      onClick={() => confirm(`Ban ${p}?`) && act(p, "ban")}
                    >
                      Ban
                    </Button>
                  </span>
                )}
              </li>
            ))}
          </ul>
        </Card>
      </div>
      {operator && (
        <Card title="Manage a player">
          <form className="space-y-3" onSubmit={submit("whitelist")}>
            <Input value={name} onChange={(e) => setName(e.target.value.trim())} placeholder="Player name" aria-label="Player name" />
            <div className="flex flex-wrap gap-2">
              <Button type="submit" disabled={!name}>
                Whitelist
              </Button>
              <Button type="button" disabled={!name} onClick={submit("unwhitelist")}>
                Un-whitelist
              </Button>
              <Button type="button" disabled={!name} onClick={submit("op")}>
                Op
              </Button>
              <Button type="button" disabled={!name} onClick={submit("pardon")}>
                Unban
              </Button>
            </div>
            <p className="text-xs text-muted">
              The whitelist only applies if it's turned on: run <code>whitelist on</code> in the console.
            </p>
          </form>
        </Card>
      )}
      {msg && (
        <div className="lg:col-span-3">
          <Alert kind={msg.kind}>{msg.text}</Alert>
        </div>
      )}
    </div>
  );
}
