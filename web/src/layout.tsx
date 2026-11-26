import { type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, NavLink, Navigate, Outlet, useLocation, useNavigate } from "react-router-dom";
import { api, ApiError } from "./api";
import { useMe } from "./hooks";
import { Button, Spinner, cx } from "./ui";

export function Logo() {
  return (
    <span className="flex items-center gap-2 font-semibold tracking-tight">
      <img src="/favicon.svg" alt="" className="h-6 w-6" />
      minectl
    </span>
  );
}

export function FullPageSpinner() {
  return (
    <div className="flex min-h-screen items-center justify-center text-muted">
      <Spinner className="h-6 w-6" />
    </div>
  );
}

export function RequireAuth({ children }: { children: ReactNode }) {
  const location = useLocation();
  const setup = useQuery({ queryKey: ["setup"], queryFn: () => api.get<{ needed: boolean }>("/setup"), staleTime: Infinity });
  const me = useMe();
  if (setup.data?.needed) return <Navigate to="/setup" replace />;
  if (me.isPending || setup.isPending) return <FullPageSpinner />;
  if (me.error instanceof ApiError && me.error.status === 401) {
    const next = location.pathname === "/" ? "" : `?next=${encodeURIComponent(location.pathname)}`;
    return <Navigate to={`/login${next}`} replace />;
  }
  if (me.error) {
    return (
      <div className="flex min-h-screen items-center justify-center p-4 text-center">
        <div>
          <p className="font-medium">Can't reach the panel</p>
          <p className="mt-1 text-sm text-muted">{me.error.message}</p>
          <Button className="mt-4" onClick={() => me.refetch()}>
            Try again
          </Button>
        </div>
      </div>
    );
  }
  return <>{children}</>;
}

function NavItem({ to, children }: { to: string; children: ReactNode }) {
  return (
    <NavLink
      to={to}
      end={to === "/"}
      className={({ isActive }) =>
        cx("rounded-md px-2.5 py-1.5 text-sm font-medium transition-colors", isActive ? "bg-surface-2 text-fg" : "text-muted hover:text-fg")
      }
    >
      {children}
    </NavLink>
  );
}

export function Layout() {
  const { data } = useMe();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const user = data?.user;

  const logout = async () => {
    await api.post("/auth/logout").catch(() => {});
    qc.clear();
    navigate("/login");
  };

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-10 border-b border-line bg-surface/90 backdrop-blur">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center gap-x-4 gap-y-2 px-4 py-2.5">
          <Link to="/">
            <Logo />
          </Link>
          <nav className="flex flex-1 flex-wrap items-center gap-1">
            <NavItem to="/">Servers</NavItem>
            {user?.is_admin && <NavItem to="/users">Users</NavItem>}
            {user?.is_admin && <NavItem to="/audit">Activity</NavItem>}
          </nav>
          <div className="flex items-center gap-1">
            <NavItem to="/account">{user?.username}</NavItem>
            <Button variant="ghost" onClick={logout}>
              Log out
            </Button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  );
}

export function PageTitle({ children, actions, back }: { children: ReactNode; actions?: ReactNode; back?: { to: string; label: string } }) {
  return (
    <div className="mb-6">
      {back && (
        <Link to={back.to} className="mb-2 inline-block text-sm text-muted hover:text-fg">
          ← {back.label}
        </Link>
      )}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold tracking-tight">{children}</h1>
        {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
      </div>
    </div>
  );
}
