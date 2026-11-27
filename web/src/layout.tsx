import { useEffect, useLayoutEffect, useRef, useState, type ComponentType, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, NavLink, Navigate, Outlet, matchPath, useLocation, useNavigate } from "react-router-dom";
import { Activity, ArrowUpRight, BookOpen, Bug, LayoutGrid, LogOut, Menu, MoreVertical, Plus, Settings, KeyRound, Users, X } from "lucide-react";
import { api, ApiError } from "./api";
import { useMe } from "./hooks";
import { Button, Spinner, cx } from "./ui";

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

export interface Tab {
  to: string;
  label: string;
  end?: boolean;
}

// Tabs is an underlined tab bar whose indicator slides to the active tab.
export function Tabs({ tabs }: { tabs: Tab[] }) {
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
    <nav className="relative -mb-px flex gap-1 overflow-x-auto border-b border-line [scrollbar-width:none]" aria-label="Sections">
      {tabs.map((t, i) => (
        <Link
          key={t.to}
          to={t.to}
          ref={(el) => {
            refs.current[i] = el;
          }}
          aria-current={i === active ? "page" : undefined}
          className={cx(
            "relative shrink-0 px-3 py-2.5 text-sm whitespace-nowrap transition-colors",
            i === active ? "font-medium text-fg" : "text-muted hover:text-fg",
          )}
        >
          {t.label}
        </Link>
      ))}
      {box && (
        <span
          className="absolute bottom-0 h-0.5 rounded-full bg-accent transition-[left,width] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)]"
          style={{ left: box.left + 8, width: box.width - 16 }}
          aria-hidden="true"
        />
      )}
    </nav>
  );
}

type Icon = ComponentType<{ className?: string; strokeWidth?: number }>;

function NavItem({ to, icon: I, children, end }: { to: string; icon: Icon; children: ReactNode; end?: boolean }) {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) =>
        cx(
          "group flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors",
          isActive ? "bg-hover font-medium text-fg" : "text-muted hover:bg-hover/60 hover:text-fg",
        )
      }
    >
      {({ isActive }) => (
        <>
          <I className={cx("h-4 w-4 shrink-0 transition-colors", isActive ? "text-accent" : "text-subtle group-hover:text-fg")} strokeWidth={1.75} />
          {children}
        </>
      )}
    </NavLink>
  );
}

function ExternalItem({ href, icon: I, children }: { href: string; icon: Icon; children: ReactNode }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className="group flex items-center gap-3 rounded-md px-3 py-2 text-sm text-muted transition-colors hover:bg-hover/60 hover:text-fg"
    >
      <I className="h-4 w-4 shrink-0 text-subtle group-hover:text-fg" strokeWidth={1.75} />
      {children}
      <ArrowUpRight className="-ml-1.5 h-3 w-3 text-subtle" strokeWidth={1.75} />
    </a>
  );
}

function Avatar({ name, className = "h-7 w-7 text-xs" }: { name: string; className?: string }) {
  return (
    <span
      className={cx(
        "flex shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-[#4687ff] via-[#71f0f4] to-[#2c6bed] font-semibold text-white uppercase",
        className,
      )}
      aria-hidden="true"
    >
      {name.slice(0, 1)}
    </span>
  );
}

function UserRow({ username, isAdmin, onLogout }: { username: string; isAdmin: boolean; onLogout: () => void }) {
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
      {open && (
        <div role="menu" className="absolute right-2 bottom-full left-2 z-30 mb-2 origin-bottom animate-scale-in rounded-lg border border-line bg-surface-2 p-1 shadow-pop">
          <Link role="menuitem" to="/account" className="flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm text-muted hover:bg-hover hover:text-fg">
            <Settings className="h-4 w-4" strokeWidth={1.75} /> Account settings
          </Link>
          <Link role="menuitem" to="/account#tokens" className="flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm text-muted hover:bg-hover hover:text-fg">
            <KeyRound className="h-4 w-4" strokeWidth={1.75} /> API tokens
          </Link>
          <div className="my-1 h-px bg-line" />
          <button role="menuitem" onClick={onLogout} className="flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm text-muted hover:bg-hover hover:text-fg">
            <LogOut className="h-4 w-4" strokeWidth={1.75} /> Log out
          </button>
        </div>
      )}
      <button
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        aria-haspopup="menu"
        aria-label={`Account menu for ${username}`}
        className="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-hover/60"
      >
        <Avatar name={username} />
        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-medium">{username}</span>
          <span className="block text-[11px] font-medium tracking-wide text-accent uppercase">{isAdmin ? "Admin" : "Member"}</span>
        </span>
        <MoreVertical className="h-4 w-4 text-subtle" strokeWidth={1.75} />
      </button>
    </div>
  );
}

const repo = "https://github.com/Geordie1071-spec/minectl";

function Sidebar({ isAdmin, username, onLogout }: { isAdmin: boolean; username: string; onLogout: () => void }) {
  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center gap-3 border-b border-line px-4 py-4">
        <Mark className="h-8 w-8" />
        <div className="min-w-0">
          <Logo className="h-3.5" wordmarkOnly />
          <p className="mt-1 text-[11px] font-medium tracking-wide text-subtle uppercase">Self-hosted</p>
        </div>
      </div>
      <nav className="flex-1 overflow-y-auto" aria-label="Main">
        <div className="space-y-0.5 border-b border-line p-2">
          <NavItem to="/" icon={LayoutGrid} end>
            Servers
          </NavItem>
          {isAdmin && (
            <NavItem to="/new" icon={Plus}>
              New server
            </NavItem>
          )}
        </div>
        <div className="space-y-0.5 border-b border-line p-2">
          {isAdmin && (
            <NavItem to="/users" icon={Users}>
              People
            </NavItem>
          )}
          {isAdmin && (
            <NavItem to="/audit" icon={Activity}>
              Activity
            </NavItem>
          )}
          <NavItem to="/account" icon={Settings}>
            Settings
          </NavItem>
        </div>
        <div className="space-y-0.5 p-2">
          <ExternalItem href={repo + "#web-panel"} icon={BookOpen}>
            Docs
          </ExternalItem>
          <ExternalItem href={repo + "/issues"} icon={Bug}>
            Report a problem
          </ExternalItem>
        </div>
      </nav>
      <div className="border-t border-line">
        <UserRow username={username} isAdmin={isAdmin} onLogout={onLogout} />
      </div>
    </div>
  );
}

export function Layout() {
  const { data } = useMe();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const location = useLocation();
  const user = data?.user;
  const [drawer, setDrawer] = useState(false);
  useEffect(() => setDrawer(false), [location.pathname]);

  const logout = async () => {
    await api.post("/auth/logout").catch(() => {});
    qc.clear();
    navigate("/login");
  };

  const sidebar = user && <Sidebar isAdmin={user.is_admin} username={user.username} onLogout={logout} />;

  return (
    <div className="flex h-dvh flex-col">
      <header className="flex h-12 shrink-0 items-center gap-3 px-4 lg:hidden">
        <button onClick={() => setDrawer(true)} aria-label="Open menu" className="-ml-1 rounded-md p-1.5 text-muted hover:bg-hover hover:text-fg">
          <Menu className="h-5 w-5" strokeWidth={1.75} />
        </button>
        <Link to="/" aria-label="minectl home">
          <Mark className="h-7 w-7" />
        </Link>
      </header>

      {drawer && (
        <div className="fixed inset-0 z-40 lg:hidden">
          <div className="absolute inset-0 animate-fade-in bg-black/50" onClick={() => setDrawer(false)} />
          <aside className="absolute inset-y-0 left-0 w-64 animate-slide-in border-r border-line bg-bg">
            <button onClick={() => setDrawer(false)} aria-label="Close menu" className="absolute top-4 right-3 rounded-md p-1.5 text-muted hover:bg-hover hover:text-fg">
              <X className="h-4 w-4" />
            </button>
            {sidebar}
          </aside>
        </div>
      )}

      <div className="flex min-h-0 flex-1">
        <aside className="hidden w-60 shrink-0 lg:block">{sidebar}</aside>
        <div className="min-w-0 flex-1 lg:py-2 lg:pr-2">
          <div className="h-full overflow-y-auto border-t border-line bg-surface lg:rounded-lg lg:border">
            <main key={location.pathname.split("/").slice(0, 3).join("/")} className="mx-auto max-w-5xl animate-fade-up px-4 py-8 sm:px-8 sm:py-12">
              <Outlet />
            </main>
          </div>
        </div>
      </div>
    </div>
  );
}

export function PageTitle({ children, actions, description }: { children: ReactNode; actions?: ReactNode; description?: ReactNode }) {
  return (
    <div className="mb-8 flex flex-wrap items-center justify-between gap-4">
      <div className="min-w-0">
        <h1 className="text-[28px] leading-tight font-medium tracking-tight">{children}</h1>
        {description && <p className="mt-1.5 text-sm text-muted">{description}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  );
}
