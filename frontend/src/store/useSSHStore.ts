import { create } from 'zustand';
import {
  ConnectHosts,
  DeleteHost,
  DeleteTunnel,
  GetHostStatuses,
  GetTunnelStatuses,
  ListHosts,
  ListAllTunnels,
  ListKeys,
  OpenRDP,
  RefreshHostsStatus,
  SaveHost,
  SaveTunnel,
  StartTunnel,
  StopTunnel,
  UploadFileToHost,
} from '../../wailsjs/go/app/App';
import { db } from '../../wailsjs/go/models';
import {
  EnvTag, Host, HostDraft, ParsedTarget, SSHKey, TunnelDirection, TunnelDraft, TunnelState, TunnelSummary,
} from '../types/ssh';

export const errorMessage = (err: unknown): string =>
  err instanceof Error ? err.message : String(err);

export const ENV_TAB_COLOR: Record<EnvTag, string> = {
  prod: '#E81123',
  stage: '#FFB900',
  dev: '#107C10',
  infra: '#0078D4',
};

interface SSHStore {
  hosts: Host[];
  keys: SSHKey[];
  pings: Record<string, number | null>;
  sessions: Record<string, string>;
  tunnelsByHost: Record<string, TunnelSummary[]>;
  uploads: Record<string, number | null>;
  selectedHostIds: string[];
  agentAvailable: boolean;
  envFilter: 'all' | EnvTag;
  isLoading: boolean;
  notice: { kind: 'error' | 'info'; text: string } | null;

  notify: (kind: 'error' | 'info', text: string) => void;
  dismissNotice: () => void;
  pinHost: (hostId: string) => Promise<void>;
  saveHost: (draft: HostDraft) => Promise<void>;
  deleteHost: (hostId: string) => Promise<void>;
  uploadFiles: (hostId: string, paths: string[]) => Promise<void>;
  loadInitial: () => Promise<void>;
  refreshPings: () => void;
  setEnvFilter: (filter: 'all' | EnvTag) => void;
  toggleSelectHost: (hostId: string, multi: boolean) => void;
  clearSelection: () => void;
  connectHosts: (hostIds: string[], mode: 'tabs' | 'split') => Promise<void>;
  quickConnect: (target: ParsedTarget, keyId: string, mode: 'tabs' | 'split') => Promise<void>;
  saveAndConnectHost: (data: {
    name: string;
    hostname: string;
    port: number;
    username: string;
    key_id: string;
    env_tag: EnvTag;
  }) => Promise<void>;
  onPingEvent: (hostId: string, online: boolean, latencyMs?: number | null) => void;
  onSessionEvent: (type: 'session_started' | 'session_ended', sessionId: string, hostId: string) => void;
  onTunnelStatusEvent: (tunnelId: string, state: TunnelState, activeConns: number, lastError?: string) => void;
  loadTunnels: () => Promise<void>;
  saveTunnel: (draft: TunnelDraft) => Promise<void>;
  deleteTunnel: (tunnelId: string) => Promise<void>;
  startTunnel: (tunnelId: string, hostId: string) => Promise<void>;
  stopTunnel: (tunnelId: string) => Promise<void>;
  openRDP: (tunnelId: string) => Promise<void>;
  onUploadProgressEvent: (hostId: string, percent: number, done: boolean, hasError: boolean) => void;
  setAgentAvailable: (available: boolean) => void;
}

export const useSSHStore = create<SSHStore>((set, get) => ({
  hosts: [],
  keys: [],
  pings: {},
  sessions: {},
  tunnelsByHost: {},
  uploads: {},
  selectedHostIds: [],
  agentAvailable: true,
  envFilter: 'all',
  isLoading: false,
  notice: null,

  notify: (kind, text) => set({ notice: { kind, text } }),
  dismissNotice: () => set({ notice: null }),

  pinHost: async (hostId) => {
    const host = get().hosts.find((h) => h.id === hostId);
    if (!host) return;
    try {
      await SaveHost(db.Host.createFrom({ ...host, is_temporary: false }));
      await get().loadInitial();
    } catch (err) {
      get().notify('error', `Не удалось закрепить сервер: ${errorMessage(err)}`);
    }
  },

  saveHost: async (draft) => {
    // Ошибку пробрасываем в форму, чтобы показать её рядом с полями.
    await SaveHost(db.Host.createFrom({
      ...draft,
      id: draft.id ?? '',
      key_id: draft.key_id || null,
      jump_host_id: draft.jump_host_id || null,
    }));
    await get().loadInitial();
  },

  deleteHost: async (hostId) => {
    try {
      await DeleteHost(hostId);
      set((state) => ({ selectedHostIds: state.selectedHostIds.filter((id) => id !== hostId) }));
      await get().loadInitial();
    } catch (err) {
      get().notify('error', `Не удалось удалить сервер: ${errorMessage(err)}`);
    }
  },

  uploadFiles: async (hostId, paths) => {
    for (const path of paths) {
      try {
        const remote = await UploadFileToHost(hostId, path, '');
        get().notify('info', `Загружено: ${remote}`);
      } catch (err) {
        set((state) => ({ uploads: { ...state.uploads, [hostId]: null } }));
        get().notify('error', `SFTP: ${errorMessage(err)}`);
        return;
      }
    }
  },

  loadInitial: async () => {
    set({ isLoading: true });
    try {
      const [hosts, keys, statuses] = await Promise.all([
        ListHosts(),
        ListKeys(),
        GetHostStatuses(),
      ]);

      const pings: Record<string, number | null> = {};
      for (const [id, st] of Object.entries(statuses || {})) {
        pings[id] = st.online ? (st.latency_ms ?? 0) : null;
      }

      set({
        hosts: (hosts || []) as Host[],
        keys: (keys || []) as SSHKey[],
        pings,
        isLoading: false,
      });
      await get().loadTunnels();
    } catch (err) {
      set({ isLoading: false });
      get().notify('error', `Не удалось загрузить данные: ${errorMessage(err)}`);
    }
  },

  refreshPings: () => RefreshHostsStatus(),
  setEnvFilter: (envFilter) => set({ envFilter }),

  toggleSelectHost: (hostId, multi) =>
    set((state) => {
      if (!multi) {
        return { selectedHostIds: state.selectedHostIds[0] === hostId ? [] : [hostId] };
      }
      const exists = state.selectedHostIds.includes(hostId);
      return {
        selectedHostIds: exists
          ? state.selectedHostIds.filter((id) => id !== hostId)
          : [...state.selectedHostIds, hostId],
      };
    }),

  // Без лишней перерисовки: Escape вызывает это на каждое нажатие.
  clearSelection: () => { if (get().selectedHostIds.length) set({ selectedHostIds: [] }); },

  connectHosts: async (hostIds, mode) => {
    if (hostIds.length === 0) return;
    try {
      await ConnectHosts(hostIds, mode);
    } catch (err) {
      get().notify('error', `Не удалось подключиться: ${errorMessage(err)}`);
    }
  },

  quickConnect: async (target, keyId, mode) => {
    try {
      // Повторный Smart Paste того же адреса переиспользует существующий хост,
      // а не плодит дубликаты во «Временных».
      const existing = get().hosts.find(
        (h) => h.hostname === target.hostname && h.port === target.port && h.username === target.username
      );
      let hostId = existing?.id;
      // Ключ меняем только у временного хоста; постоянный хост не трогаем.
      if (!existing || (existing.is_temporary && keyId && existing.key_id !== keyId)) {
        const saved = await SaveHost(db.Host.createFrom({
          ...(existing ?? {
            id: '',
            name: target.hostname,
            hostname: target.hostname,
            port: target.port,
            username: target.username,
            env_tag: 'dev',
            tab_color: ENV_TAB_COLOR.dev,
            is_temporary: true,
          }),
          key_id: keyId || null,
        }));
        hostId = saved?.id;
        await get().loadInitial();
      }
      if (hostId) await ConnectHosts([hostId], mode);
    } catch (err) {
      get().notify('error', `Не удалось подключиться: ${errorMessage(err)}`);
    }
  },

  saveAndConnectHost: async (data) => {
    try {
      const saved = await SaveHost(db.Host.createFrom({
        id: '',
        name: data.name,
        hostname: data.hostname,
        port: data.port,
        username: data.username,
        key_id: data.key_id,
        env_tag: data.env_tag,
        tab_color: ENV_TAB_COLOR[data.env_tag],
        is_temporary: false,
      }));

      await get().loadInitial();
      if (saved?.id) {
        await ConnectHosts([saved.id], 'tabs');
      }
    } catch (err) {
      get().notify('error', `Не удалось сохранить сервер: ${errorMessage(err)}`);
    }
  },

  onPingEvent: (hostId, online, latencyMs) =>
    set((state) => ({
      pings: { ...state.pings, [hostId]: online ? (latencyMs ?? 0) : null },
    })),

  onSessionEvent: (type, sessionId, hostId) =>
    set((state) => {
      const next = { ...state.sessions };
      if (type === 'session_started') {
        next[sessionId] = hostId;
      } else {
        delete next[sessionId];
      }
      return { sessions: next };
    }),

  onTunnelStatusEvent: (tunnelId, tunnelState, activeConns, lastError) =>
    set((state) => {
      const next = { ...state.tunnelsByHost };
      for (const hostId of Object.keys(next)) {
        next[hostId] = next[hostId].map((t) =>
          t.id === tunnelId
            ? { ...t, state: tunnelState, active_conns: activeConns, last_error: lastError || undefined }
            : t
        );
      }
      return { tunnelsByHost: next };
    }),

  loadTunnels: async () => {
    try {
      const [list, statuses] = await Promise.all([ListAllTunnels(), GetTunnelStatuses()]);
      // Сохраняем последнюю ошибку остановленных туннелей, которых нет в статусах backend.
      const prev = new Map(Object.values(get().tunnelsByHost).flat().map((t) => [t.id, t]));
      const live = new Map((statuses || []).map((s) => [s.tunnel_id, s]));
      const tunnelsByHost: Record<string, TunnelSummary[]> = {};
      for (const t of list || []) {
        const st = live.get(t.id);
        (tunnelsByHost[t.host_id] ||= []).push({
          id: t.id,
          host_id: t.host_id,
          name: t.name,
          direction: t.direction as TunnelDirection,
          local_addr: t.local_addr,
          remote_addr: t.remote_addr ?? '',
          auto_start: t.auto_start,
          state: (st?.state as TunnelState) ?? (prev.get(t.id)?.state === 'error' ? 'error' : 'stopped'),
          active_conns: st?.active_conns ?? 0,
          last_error: st?.last_error || (st ? undefined : prev.get(t.id)?.last_error),
        });
      }
      set({ tunnelsByHost });
    } catch (err) {
      get().notify('error', `Не удалось загрузить туннели: ${errorMessage(err)}`);
    }
  },

  saveTunnel: async (draft) => {
    // Ошибку пробрасываем в форму, чтобы показать её рядом с полями.
    await SaveTunnel(db.Tunnel.createFrom({ ...draft, id: draft.id ?? '' }));
    await get().loadTunnels();
  },

  deleteTunnel: async (tunnelId) => {
    try {
      await DeleteTunnel(tunnelId);
      await get().loadTunnels();
    } catch (err) {
      get().notify('error', `Не удалось удалить туннель: ${errorMessage(err)}`);
    }
  },

  startTunnel: async (tunnelId, hostId) => {
    try {
      await StartTunnel(tunnelId, hostId);
    } catch (err) {
      get().onTunnelStatusEvent(tunnelId, 'error', 0, errorMessage(err));
      get().notify('error', `Туннель не запущен: ${errorMessage(err)}`);
    }
  },

  stopTunnel: async (tunnelId) => {
    try {
      await StopTunnel(tunnelId);
    } catch (err) {
      get().notify('error', `Не удалось остановить туннель: ${errorMessage(err)}`);
    }
  },

  openRDP: async (tunnelId) => {
    try {
      await OpenRDP(tunnelId);
    } catch (err) {
      get().notify('error', `RDP: ${errorMessage(err)}`);
    }
  },

  onUploadProgressEvent: (hostId, percent, done, hasError) =>
    set((state) => ({
      uploads: { ...state.uploads, [hostId]: done || hasError ? null : percent },
    })),

  setAgentAvailable: (agentAvailable) => set({ agentAvailable }),
}));
