import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, Navigate, Outlet, matchPath, useLocation, useMatch, useNavigate } from "react-router-dom";
import { api, ApiError, srv } from "./api";
import { useMe, useServer } from "./hooks";
import { isModded } from "./format";
import { Button, LiveDot, Spinner, TypeTile, cx } from "./ui";

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

interface Tab {
  to: string;
  label: string;
  end?: boolean;
}

// PillTabs shows tabs with a highlight that slides to the active one.
function PillTabs({ tabs }: { tabs: Tab[] }) {
  const { pathname } = useLocation();
  const refs = useRef<(HTMLAnchorElement | null)[]>([]);
  const [box, setBox] = useState<{ left: number; width: number } | null>(null);
  const active = tabs.findIndex((t) => matchPath({ path: t.to, end: t.end ?? false }, pathname));
  const key = tabs.map((t) => t.to).join("|");

  useLayoutEffect(() => {
    const measure = () => {
      const el = refs.current[active];
      setBox(el ? { left: el.offsetLeft, width: el.offsetWidth } : null);
    };
    measure();
    window.addEventListener("resize", measure);
    return () => window.removeEventListener("resize", measure);
  }, [active, key]);

  return (
    <div className="relative flex items-center">
      {box && (
        <span
          className="absolute inset-y-0 rounded-full bg-hover ring-1 ring-line transition-[left,width] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)]"
          style={{ left: box.left, width: box.width }}
          aria-hidden="true"
        />
      )}
      {tabs.map((t, i) => (
        <Link
          key={t.to}
          to={t.to}
          ref={(el) => {
            refs.current[i] = el;
          }}
          aria-current={i === active ? "page" : undefined}
          className={cx(
            "relative z-10 rounded-full px-3.5 py-1.5 text-sm whitespace-nowrap transition-colors duration-200",
            i === active ? "font-medium text-fg" : "text-muted hover:text-fg",
          )}
        >
          {t.label}
        </Link>
      ))}
    </div>
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
        className="flex h-8 w-8 items-center justify-center rounded-full bg-gradient-to-br from-[#4687ff] via-[#71f0f4] to-[#2c6bed] text-[13px] font-semibold text-white uppercase ring-1 ring-line transition hover:scale-105 hover:ring-line-strong active:scale-95"
      >
        {username.slice(0, 1)}
      </button>
      {open && (
        <div role="menu" className="absolute right-0 z-30 mt-3 w-60 origin-top-right animate-scale-in overflow-hidden rounded-xl border border-line bg-surface p-1.5 shadow-pop">
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

function ServerChip({ name }: { name: string }) {
  const { data } = useServer(name);
  return (
    <Link
      to={srv(name)}
      className="flex min-w-0 shrink-0 animate-fade-in items-center gap-2 rounded-full py-1 pr-2 pl-1 transition-colors hover:bg-hover sm:pr-3"
    >
      {data ? <TypeTile type={data.type} className="h-6 w-6 rounded-full text-[11px]" /> : <span className="h-6 w-6 rounded-full bg-surface-2" />}
      <span className="hidden max-w-32 truncate text-sm font-medium sm:inline">{name}</span>
      {data && <LiveDot status={data.job ? "creating" : data.status} />}
    </Link>
  );
}

function useServerTabs(name: string | undefined): Tab[] {
  const { data: s } = useServer(name ?? "");
  if (!name) return [];
  const base = srv(name);
  const admin = s?.role === "admin";
  const tabs: Tab[] = [
    { to: base, label: "Overview", end: true },
    { to: base + "/console", label: "Console" },
    { to: base + "/players", label: "Players" },
    { to: base + "/backups", label: "Backups" },
  ];
  if (admin && s && (isModded(s.type) || s.modpack)) tabs.push({ to: base + "/mods", label: "Mods" });
  if (admin) tabs.push({ to: base + "/settings", label: "Settings" });
  return tabs;
}

function Divider() {
  return <span className="mx-1 h-5 w-px shrink-0 bg-line" aria-hidden="true" />;
}

export function Layout() {
  const { data } = useMe();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const location = useLocation();
  const user = data?.user;
  const serverName = useMatch("/servers/:name/*")?.params.name;
  const serverTabs = useServerTabs(serverName);

  const globalTabs: Tab[] = [
    { to: "/", label: "Servers", end: true },
    ...(user?.is_admin ? [{ to: "/users", label: "Users" }, { to: "/audit", label: "Activity" }] : []),
  ];

  const logout = async () => {
    await api.post("/auth/logout").catch(() => {});
    qc.clear();
    navigate("/login");
  };

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-20 flex justify-center px-3 pt-4 pb-2 sm:px-4">
        <div className="flex h-14 max-w-full animate-fade-in items-center gap-1 rounded-full border border-line bg-surface/80 py-2 pr-2 pl-3 shadow-pop backdrop-blur-xl">
          <Link
            to="/"
            className={cx("shrink-0 items-center gap-2 rounded-full py-1 pr-2 pl-1 transition-colors hover:bg-hover", serverName ? "hidden sm:flex" : "flex")}
            aria-label="minectl home"
          >
            <Mark className="h-7 w-7 transition-transform duration-300 hover:-rotate-12" />
            <span className="hidden md:block">
              <Logo className="h-4" wordmarkOnly />
            </span>
          </Link>
          <span className={serverName ? "hidden sm:contents" : "contents"}>
            <Divider />
          </span>
          {serverName && (
            <>
              <Link
                to="/"
                aria-label="All servers"
                className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-muted transition-colors hover:bg-hover hover:text-fg"
              >
                <svg viewBox="0 0 16 16" className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                  <path d="M10 3.5 5.5 8l4.5 4.5" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </Link>
              <ServerChip name={serverName} />
              <Divider />
            </>
          )}
          <nav
            className="min-w-0 overflow-x-auto [scrollbar-width:none] max-sm:[mask-image:linear-gradient(to_right,black_80%,transparent)]"
            aria-label="Sections"
          >
            <PillTabs key={serverName ? "server" : "global"} tabs={serverName ? serverTabs : globalTabs} />
          </nav>
          <Divider />
          {user && <UserMenu username={user.username} isAdmin={user.is_admin} onLogout={logout} />}
        </div>
      </header>
      <main key={location.pathname.split("/").slice(0, 3).join("/")} className="mx-auto max-w-6xl animate-fade-up px-4 pt-6 pb-16 sm:px-6 sm:pt-8">
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
