import { useEffect, useRef, useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, NavLink, Navigate, Outlet, useLocation, useNavigate } from "react-router-dom";
import { api, ApiError } from "./api";
import { useMe } from "./hooks";
import { Button, Spinner, cx } from "./ui";

export function Logo({ onDark, className = "h-7" }: { onDark?: boolean; className?: string }) {
  if (onDark) return <img src="/logo-white.png" alt="minectl" className={cx("w-auto max-w-none", className)} />;
  return (
    <>
      <img src="/logo-navy.png" alt="minectl" className={cx("logo-on-light w-auto max-w-none", className)} />
      <img src="/logo-white.png" alt="minectl" className={cx("logo-on-dark w-auto max-w-none", className)} />
    </>
  );
}

export function Mark({ className = "h-8 w-8" }: { className?: string }) {
  return <img src="/mark.png" alt="minectl" className={cx("pixelated", className)} />;
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
        cx(
          "rounded-lg px-3 py-1.5 text-sm font-medium transition-colors",
          isActive ? "bg-accent-soft text-accent" : "text-muted hover:bg-surface-2 hover:text-fg",
        )
      }
    >
      {children}
    </NavLink>
  );
}

function UserMenu({ username, isAdmin, onLogout }: { username: string; isAdmin: boolean; onLogout: () => void }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const location = useLocation();
  useEffect(() => setOpen(false), [location.pathname]);
  useEffect(() => {
    if (!open) return;
    const close = (e: MouseEvent | KeyboardEvent) => {
      if (e instanceof KeyboardEvent ? e.key === "Escape" : !ref.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", close);
    document.addEventListener("keydown", close);
    return () => {
      document.removeEventListener("mousedown", close);
      document.removeEventListener("keydown", close);
    };
  }, [open]);

  return (
    <div className="relative" ref={ref}>
      <button
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        aria-haspopup="menu"
        className="flex items-center gap-2 rounded-full py-1 pr-2.5 pl-1 text-sm font-medium hover:bg-surface-2"
      >
        <span className="flex h-7 w-7 items-center justify-center rounded-full bg-gradient-to-br from-brand to-accent text-xs font-semibold text-white uppercase">
          {username.slice(0, 1)}
        </span>
        <span className="hidden sm:inline">{username}</span>
        <svg viewBox="0 0 20 20" className="h-4 w-4 text-muted" aria-hidden="true">
          <path d="M5.5 7.5 10 12l4.5-4.5" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </button>
      {open && (
        <div role="menu" className="absolute right-0 z-20 mt-2 w-56 overflow-hidden rounded-xl border border-line bg-surface py-1 shadow-pop">
          <div className="border-b border-line px-4 py-2.5">
            <p className="text-sm font-medium">{username}</p>
            <p className="text-xs text-muted">{isAdmin ? "Admin" : "Member"}</p>
          </div>
          <Link role="menuitem" to="/account" className="block px-4 py-2 text-sm hover:bg-surface-2">
            Account and API tokens
          </Link>
          <button role="menuitem" onClick={onLogout} className="block w-full px-4 py-2 text-left text-sm text-danger hover:bg-surface-2">
            Log out
          </button>
        </div>
      )}
    </div>
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
      <header className="sticky top-0 z-10 border-b border-line bg-surface/85 backdrop-blur-md">
        <div className="mx-auto flex h-16 max-w-6xl items-center gap-3 px-4 sm:gap-6">
          <Link to="/" className="shrink-0" aria-label="minectl home">
            <span className="hidden sm:block">
              <Logo className="h-7" />
            </span>
            <Mark className="h-8 w-8 sm:hidden" />
          </Link>
          <nav className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto">
            <NavItem to="/">Servers</NavItem>
            {user?.is_admin && <NavItem to="/users">Users</NavItem>}
            {user?.is_admin && <NavItem to="/audit">Activity</NavItem>}
          </nav>
          {user && <UserMenu username={user.username} isAdmin={user.is_admin} onLogout={logout} />}
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-8">
        <Outlet />
      </main>
    </div>
  );
}

export function PageTitle({ children, actions, back }: { children: ReactNode; actions?: ReactNode; back?: { to: string; label: string } }) {
  return (
    <div className="mb-6">
      {back && (
        <Link to={back.to} className="mb-3 inline-flex items-center gap-1 text-sm font-medium text-muted hover:text-accent">
          <span aria-hidden="true">←</span> {back.label}
        </Link>
      )}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-semibold tracking-tight">{children}</h1>
        {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
      </div>
    </div>
  );
}
