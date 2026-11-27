import { useState, type FormEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate, useSearchParams } from "react-router-dom";
import { api, errorMessage, type Me } from "../api";
import { Alert, Button, Field, Input } from "../ui";
import { AuthShell } from "./AuthShell";

export function Login() {
  const [params] = useSearchParams();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const me = await api.post<Me>("/auth/login", { username, password });
      qc.setQueryData(["me"], me);
      const next = params.get("next");
      navigate(next && next.startsWith("/") && !next.startsWith("//") ? next : "/");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <AuthShell title="Log in to minectl">
      <form onSubmit={submit} className="space-y-4">
        <Field label="Username">
          <Input value={username} onChange={(e) => setUsername(e.target.value)} required autoFocus autoComplete="username" />
        </Field>
        <Field label="Password">
          <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required autoComplete="current-password" />
        </Field>
        {error && <Alert>{error}</Alert>}
        <Button variant="primary" type="submit" busy={busy} className="w-full">
          Log in
        </Button>
        <p className="text-xs text-muted">
          Forgot your password? An admin can reset it, or run <code>minectl serve reset-password &lt;user&gt;</code> on the server.
        </p>
      </form>
    </AuthShell>
  );
}
