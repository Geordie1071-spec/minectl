import { useEffect, useRef, useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, NavLink, Navigate, Outlet, useLocation, useMatch, useNavigate } from "react-router-dom";
import { api, ApiError, srv } from "./api";
import { useMe, useServer } from "./hooks";
import { isModded } from "./format";
import { Button, Spinner, StatusDot, TypeTile, cx } from "./ui";

export function Logo({ onDark, className = "h-7", wordmarkOnly }: { onDark?: boolean; className?: string; wordmarkOnly?: boolean }) {
  if (wordmarkOnly) {
    return (
      <>
        <img src="/wordmark-navy.png" alt="minectl" className={cx("logo-on-light w-auto max-w-none", className)} />
        <img src="/wordmark-white.png" alt="minectl" className={cx("logo-on-dark w-auto max-w-none", className)} />
      </>
    );
  }
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

function Slash() {
  return (
    <svg viewBox="0 0 24 24" className="h-5 w-5 shrink-0 text-line-strong" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M16.88 3.549L7.12 20.451" strokeLinecap="round" />
    </svg>
  );
}

function TabLink({ to, end, children }: { to: string; end?: boolean; children: ReactNode }) {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) =>
        cx(
          "relative flex h-12 shrink-0 items-center px-0.5 text-sm transition-colors",
          isActive
            ? "text-fg after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:rounded-full after:bg-fg"
            : "text-muted hover:text-fg",
        )
      }
    >
      <span className="rounded-md px-2.5 py-1.5 hover:bg-hover">{children}</span>
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
        aria-label={`Account menu for ${username}`}
        className="flex h-8 w-8 items-center justify-center rounded-full bg-gradient-to-br from-[#4687ff] via-[#71f0f4] to-[#2c6bed] text-[13px] font-semibold text-white uppercase ring-1 ring-line transition hover:ring-line-strong"
      >
        {username.slice(0, 1)}
      </button>
      {open && (
        <div role="menu" className="absolute right-0 z-30 mt-2 w-60 overflow-hidden rounded-xl border border-line bg-surface p-1.5 shadow-pop">
          <div className="px-2.5 py-2">
            <p className="text-sm font-medium">{username}</p>
            <p className="text-[13px] text-muted">{isAdmin ? "Admin" : "Member"}</p>
          </div>
          <div className="my-1 h-px bg-line" />
          <Link role="menuitem" to="/account" className="flex rounded-md px-2.5 py-2 text-sm text-muted hover:bg-hover hover:text-fg">
            Account settings
          </Link>
          <Link role="menuitem" to="/account#tokens" className="flex rounded-md px-2.5 py-2 text-sm text-muted hover:bg-hover hover:text-fg">
            API tokens
          </Link>
          <div className="my-1 h-px bg-line" />
          <button role="menuitem" onClick={onLogout} className="flex w-full rounded-md px-2.5 py-2 text-left text-sm text-muted hover:bg-hover hover:text-fg">
            Log out
          </button>
        </div>
      )}
    </div>
  );
}

function ServerCrumb({ name }: { name: string }) {
  const { data } = useServer(name);
  return (
    <Link to={srv(name)} className="flex min-w-0 items-center gap-2 rounded-md px-1.5 py-1 hover:bg-hover">
      {data && <TypeTile type={data.type} className="h-5 w-5 rounded text-[10px]" />}
      <span className="truncate font-medium">{name}</span>
      {data && <StatusDot status={data.job ? "creating" : data.status} />}
    </Link>
  );
}

function ServerTabs({ name }: { name: string }) {
  const { data: s } = useServer(name);
  const base = srv(name);
  const admin = s?.role === "admin";
  return (
    <>
      <TabLink to={base} end>
        Overview
      </TabLink>
      <TabLink to={base + "/console"}>Console</TabLink>
      <TabLink to={base + "/players"}>Players</TabLink>
      <TabLink to={base + "/backups"}>Backups</TabLink>
      {admin && s && (isModded(s.type) || s.modpack) && <TabLink to={base + "/mods"}>Mods</TabLink>}
      {admin && <TabLink to={base + "/settings"}>Settings</TabLink>}
    </>
  );
}

export function Layout() {
  const { data } = useMe();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const user = data?.user;
  const serverMatch = useMatch("/servers/:name/*");
  const serverName = serverMatch?.params.name;

  const logout = async () => {
    await api.post("/auth/logout").catch(() => {});
    qc.clear();
    navigate("/login");
  };

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-20 border-b border-line bg-surface">
        <div className="mx-auto flex h-16 max-w-6xl items-center gap-2 px-4 sm:px-6">
          <Link to="/" className="flex shrink-0 items-center gap-2.5 rounded-md p-1 hover:bg-hover" aria-label="minectl home">
            <Mark className="h-7 w-7" />
            <span className="hidden sm:block">
              <Logo className="h-[18px] [&]:max-w-none" wordmarkOnly />
            </span>
          </Link>
          <Slash />
          <Link to="/" className="shrink-0 rounded-md px-1.5 py-1 font-medium hover:bg-hover">
            Servers
          </Link>
          {serverName && (
            <>
              <Slash />
              <ServerCrumb name={serverName} />
            </>
          )}
          <div className="ml-auto flex items-center gap-2">
            {user && <UserMenu username={user.username} isAdmin={user.is_admin} onLogout={logout} />}
          </div>
        </div>
        <nav className="mx-auto -mb-px flex max-w-6xl gap-1 overflow-x-auto px-2.5 sm:px-4.5" aria-label="Sections">
          {serverName ? (
            <ServerTabs name={serverName} />
          ) : (
            <>
              <TabLink to="/" end>
                Servers
              </TabLink>
              {user?.is_admin && <TabLink to="/users">Users</TabLink>}
              {user?.is_admin && <TabLink to="/audit">Activity</TabLink>}
              <TabLink to="/account">Account</TabLink>
            </>
          )}
        </nav>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-8 sm:px-6 sm:py-10">
        <Outlet />
      </main>
    </div>
  );
}

export function PageTitle({ children, actions, description }: { children: ReactNode; actions?: ReactNode; description?: ReactNode }) {
  return (
    <div className="mb-8 flex flex-wrap items-end justify-between gap-4">
      <div className="min-w-0">
        <h1 className="text-[28px] leading-tight font-semibold tracking-tight sm:text-[32px]">{children}</h1>
        {description && <p className="mt-1.5 text-sm text-muted">{description}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  );
}
