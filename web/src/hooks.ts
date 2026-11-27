import { useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, srv, type Job, type JobEvent, type Me, type Players, type Server, type Stats } from "./api";

export function useMe() {
  return useQuery({ queryKey: ["me"], queryFn: () => api.get<Me>("/auth/me"), retry: false, staleTime: 60_000 });
}

export function useServers() {
  return useQuery({ queryKey: ["servers"], queryFn: () => api.get<Server[]>("/servers"), refetchInterval: 5000 });
}

export function useServer(name: string) {
  return useQuery({ queryKey: ["server", name], queryFn: () => api.get<Server>(srv(name)), refetchInterval: 5000 });
}

export function usePlayers(name: string, enabled: boolean) {
  return useQuery({
    queryKey: ["players", name],
    queryFn: () => api.get<Players>(srv(name) + "/players"),
    enabled,
    refetchInterval: 10_000,
    retry: false,
  });
}

// useEventSource opens an SSE stream while url is set and calls onEvent for
// each named event. The browser reconnects on its own after network blips.
function useEventSource(url: string | null, events: string[], onEvent: (type: string, data: unknown) => void) {
  const handler = useRef(onEvent);
  useEffect(() => {
    handler.current = onEvent;
  });
  const eventsKey = events.join(",");
  useEffect(() => {
    if (!url) return;
    const es = new EventSource(url);
    const listeners = eventsKey.split(",").map((type) => {
      const fn = (e: MessageEvent) => {
        let data: unknown = e.data;
        try {
          data = JSON.parse(e.data);
        } catch {
          // plain text
        }
        handler.current(type, data);
        if (type === "end") es.close();
      };
      es.addEventListener(type, fn);
      return [type, fn] as const;
    });
    return () => {
      listeners.forEach(([t, fn]) => es.removeEventListener(t, fn));
      es.close();
    };
  }, [url, eventsKey]);
}

export interface JobState {
  job?: Job;
  events: JobEvent[];
  progress?: JobEvent;
  stage?: string;
  done: boolean;
}

const jobEventTypes = ["progress", "stage", "log", "info", "warn", "done", "error", "end"];

export function useJobStream(id: string | undefined): JobState {
  const qc = useQueryClient();
  const [state, setState] = useState<JobState>({ events: [], done: false });
  const { data: job } = useQuery({ queryKey: ["job", id], queryFn: () => api.get<Job>("/jobs/" + id), enabled: !!id });

  useEventSource(id ? `/api/v1/jobs/${id}/events` : null, jobEventTypes, (type, data) => {
    if (type === "end") {
      setState((s) => ({ ...s, job: data as Job, done: true, progress: undefined }));
      qc.invalidateQueries({ queryKey: ["servers"] });
      qc.invalidateQueries({ queryKey: ["server"] });
      qc.invalidateQueries({ queryKey: ["backups"] });
      qc.invalidateQueries({ queryKey: ["jobs"] });
      return;
    }
    const e = data as JobEvent;
    setState((s) => {
      if (type === "progress") return { ...s, progress: e };
      if (s.events.some((x) => x.seq === e.seq)) return s;
      return {
        ...s,
        events: [...s.events, e].slice(-400),
        stage: type === "stage" ? e.message : s.stage,
        progress: type === "stage" ? undefined : s.progress,
      };
    });
  });

  return { ...state, job: state.job ?? job };
}

const maxLogLines = 2000;

export function useLogStream(name: string, enabled: boolean) {
  const [lines, setLines] = useState<string[]>([]);
  const [ended, setEnded] = useState(false);
  const [key, setKey] = useState(0);
  useEffect(() => {
    setLines([]);
    setEnded(false);
  }, [name, enabled, key]);
  useEventSource(enabled ? `${"/api/v1" + srv(name)}/logs?tail=300&k=${key}` : null, ["log", "end", "error"], (type, data) => {
    if (type === "log") setLines((l) => (l.length >= maxLogLines ? [...l.slice(-maxLogLines + 1), String(data)] : [...l, String(data)]));
    else setEnded(true);
  });
  return { lines, ended, reconnect: () => setKey((k) => k + 1), append: (line: string) => setLines((l) => [...l, line]) };
}

export function useStats(name: string, enabled: boolean) {
  const [stats, setStats] = useState<Stats | undefined>();
  const [history, setHistory] = useState<number[]>([]);
  useEffect(() => {
    if (!enabled) {
      setStats(undefined);
      setHistory([]);
    }
  }, [enabled]);
  useEventSource(enabled ? "/api/v1" + srv(name) + "/stats" : null, ["stats"], (_, data) => {
    const s = data as Stats;
    setStats(s);
    setHistory((h) => [...h.slice(-59), s.cpu_percent]);
  });
  return { stats, history };
}
