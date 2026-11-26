import { useState, type FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Navigate, useNavigate, useSearchParams } from "react-router-dom";
import { api, errorMessage, type Me } from "../api";
import { Alert, Button, Field, Input } from "../ui";
import { FullPageSpinner } from "../layout";
import { AuthShell } from "./AuthShell";

export function Setup() {
  const [params] = useSearchParams();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const status = useQuery({ queryKey: ["setup"], queryFn: () => api.get<{ needed: boolean }>("/setup") });
  const [token, setToken] = useState(params.get("token") ?? "");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  if (status.isPending) return <FullPageSpinner />;
  if (status.data && !status.data.needed) return <Navigate to="/" replace />;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (password !== confirm) {
      setError("The passwords don't match.");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const me = await api.post<Me>("/setup", { token: token.trim(), username, password });
      qc.setQueryData(["me"], me);
      qc.setQueryData(["setup"], { needed: false });
      navigate("/");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <AuthShell
      title="Set up your panel"
      subtitle={
        <>
          Create the admin account. The setup token is printed in the panel's log (<code>journalctl -u minectl</code>) and saved in{" "}
          <code>~/.minectl/setup-token</code>.
        </>
      }
    >
      <form onSubmit={submit} className="space-y-4">
        <Field label="Setup token">
          <Input value={token} onChange={(e) => setToken(e.target.value)} required autoComplete="off" spellCheck={false} className="font-mono" />
        </Field>
        <Field label="Username">
          <Input value={username} onChange={(e) => setUsername(e.target.value)} required autoComplete="username" />
        </Field>
        <Field label="Password" hint="At least 8 characters.">
          <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} autoComplete="new-password" />
        </Field>
        <Field label="Confirm password">
          <Input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} required autoComplete="new-password" />
        </Field>
        {error && <Alert>{error}</Alert>}
        <Button variant="primary" type="submit" busy={busy} className="w-full">
          Create admin account
        </Button>
      </form>
    </AuthShell>
  );
}
