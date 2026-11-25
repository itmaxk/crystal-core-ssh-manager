import { create } from 'zustand';
import {
  ConnectHosts,
  DeleteHost,
  GetHostStatuses,
  GetTunnelStatuses,
  ListHosts,
  ListKeys,
  ListTunnels,
  RefreshHostsStatus,
  SaveHost,
  UploadFileToHost,
} from '../../wailsjs/go/app/App';
import { db } from '../../wailsjs/go/models';
import { EnvTag, Host, ParsedTarget, SSHKey, TunnelSummary } from '../types/ssh';

export const errorMessage = (err: unknown): string =>
  err instanceof Error ? err.message : String(err);

const ENV_TAB_COLOR: Record<EnvTag, string> = {
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
  onTunnelStatusEvent: (tunnelId: string, state: TunnelSummary['state'], activeConns: number) => void;
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
      const [hosts, keys, statuses, tunnelStatuses] = await Promise.all([
        ListHosts(),
        ListKeys(),
        GetHostStatuses(),
        GetTunnelStatuses(),
      ]);
      const tunnelState = new Map((tunnelStatuses || []).map((t) => [t.tunnel_id, t]));

      const pings: Record<string, number | null> = {};
      for (const [id, st] of Object.entries(statuses || {})) {
        pings[id] = st.online ? (st.latency_ms ?? 0) : null;
      }

      const tunnelsByHost: Record<string, TunnelSummary[]> = {};
      await Promise.all(
        (hosts || []).map(async (h) => {
          const list = await ListTunnels(h.id);
          tunnelsByHost[h.id] = (list || []).map((t) => ({
            id: t.id,
            name: t.name,
            local_addr: t.local_addr,
            state: (tunnelState.get(t.id)?.state as TunnelSummary['state']) ?? 'stopped',
            active_conns: tunnelState.get(t.id)?.active_conns ?? 0,
          }));
        })
      );

      set({
        hosts: (hosts || []) as Host[],
        keys: (keys || []) as SSHKey[],
        pings,
        tunnelsByHost,
        isLoading: false,
      });
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

  clearSelection: () => set({ selectedHostIds: [] }),

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

  onTunnelStatusEvent: (tunnelId, tunnelState, activeConns) =>
    set((state) => {
      const next = { ...state.tunnelsByHost };
      for (const hostId of Object.keys(next)) {
        next[hostId] = next[hostId].map((t) =>
          t.id === tunnelId ? { ...t, state: tunnelState, active_conns: activeConns } : t
        );
      }
      return { tunnelsByHost: next };
    }),

  onUploadProgressEvent: (hostId, percent, done, hasError) =>
    set((state) => ({
      uploads: { ...state.uploads, [hostId]: done || hasError ? null : percent },
    })),

  setAgentAvailable: (agentAvailable) => set({ agentAvailable }),
}));
