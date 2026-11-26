import { useState, type FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, errorMessage, type APIToken } from "../api";
import { useMe } from "../hooks";
import { PageTitle } from "../layout";
import { timeAgo } from "../format";
import { Alert, Button, Card, CopyButton, Field, Input } from "../ui";
import { PanelSettings } from "./PanelSettings";

export function Account() {
  const { data: me } = useMe();
  const qc = useQueryClient();
  const tokens = useQuery({ queryKey: ["tokens"], queryFn: () => api.get<APIToken[]>("/tokens") });
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [pwMsg, setPwMsg] = useState<{ kind: "info" | "error"; text: string }>();
  const [tokenName, setTokenName] = useState("");
  const [newToken, setNewToken] = useState("");
  const [tokenError, setTokenError] = useState("");

  const changePassword = async (e: FormEvent) => {
    e.preventDefault();
    setPwMsg(undefined);
    try {
      await api.post("/auth/password", { current, new: next });
      setCurrent("");
      setNext("");
      setPwMsg({ kind: "info", text: "Password changed. Other devices were logged out." });
    } catch (err) {
      setPwMsg({ kind: "error", text: errorMessage(err) });
    }
  };

  const createToken = async (e: FormEvent) => {
    e.preventDefault();
    setTokenError("");
    try {
      const res = await api.post<{ token: string }>("/tokens", { name: tokenName });
      setNewToken(res.token);
      setTokenName("");
      qc.invalidateQueries({ queryKey: ["tokens"] });
    } catch (err) {
      setTokenError(errorMessage(err));
    }
  };

  return (
    <>
      <PageTitle description={me?.user.is_admin ? "Your account, API tokens, and panel-wide settings." : "Your password and personal API tokens."}>Settings</PageTitle>
      <div className="stagger grid gap-4 lg:grid-cols-2">
        <Card title={`Password for ${me?.user.username ?? ""}`}>
          <form onSubmit={changePassword} className="space-y-3">
            <Field label="Current password">
              <Input type="password" value={current} onChange={(e) => setCurrent(e.target.value)} required autoComplete="current-password" />
            </Field>
            <Field label="New password" hint="At least 8 characters.">
              <Input type="password" value={next} onChange={(e) => setNext(e.target.value)} required minLength={8} autoComplete="new-password" />
            </Field>
            {pwMsg && <Alert kind={pwMsg.kind}>{pwMsg.text}</Alert>}
            <Button variant="primary" type="submit">
              Change password
            </Button>
          </form>
        </Card>
        <div id="tokens" className="scroll-mt-32">
        <Card title="API tokens">
          <p className="text-sm text-muted">
            For scripts and bots. A token can do everything you can. Send it as <code>Authorization: Bearer …</code>.
          </p>
          {newToken && (
            <div className="mt-3">
              <Alert kind="info">
                Copy this token now; it won't be shown again.
                <span className="mt-2 flex items-center gap-2">
                  <code className="min-w-0 flex-1 break-all">{newToken}</code>
                  <CopyButton text={newToken} />
                </span>
              </Alert>
            </div>
          )}
          <ul className="mt-3 divide-y divide-line">
            {tokens.data?.map((t) => (
              <li key={t.id} className="flex items-center justify-between gap-2 py-2 text-sm">
                <span>
                  <span className="font-medium">{t.name}</span>
                  <span className="block text-xs text-muted">
                    created {timeAgo(t.created_at)} · last used {timeAgo(t.last_used_at)}
                  </span>
                </span>
                <Button
                  variant="ghost"
                  className="text-xs text-danger"
                  onClick={() => api.del(`/tokens/${t.id}`).then(() => qc.invalidateQueries({ queryKey: ["tokens"] }))}
                >
                  Revoke
                </Button>
              </li>
            ))}
          </ul>
          <form onSubmit={createToken} className="mt-3 flex gap-2">
            <Input value={tokenName} onChange={(e) => setTokenName(e.target.value)} placeholder="Token name, e.g. discord bot" aria-label="Token name" maxLength={64} />
            <Button type="submit" disabled={!tokenName.trim()}>
              Create
            </Button>
          </form>
          {tokenError && (
            <div className="mt-2">
              <Alert>{tokenError}</Alert>
            </div>
          )}
        </Card>
        </div>
        {me?.user.is_admin && <PanelSettings />}
      </div>
    </>
  );
}
