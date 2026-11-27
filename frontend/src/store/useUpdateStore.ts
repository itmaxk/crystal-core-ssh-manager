import { create } from 'zustand';
import {
  CancelUpdateDownload, CheckForUpdate, DownloadUpdate, InstallUpdate,
} from '../../wailsjs/go/app/App';
import { update } from '../../wailsjs/go/models';
import { errorMessage } from './useSSHStore';

export type UpdatePhase = 'idle' | 'checking' | 'downloading' | 'ready' | 'installing';

const AUTO_KEY = 'ccsm.autoUpdateCheck';

export const isAutoCheckEnabled = () => {
  try { return localStorage.getItem(AUTO_KEY) !== '0'; } catch { return true; }
};

export const setAutoCheckEnabled = (on: boolean) => {
  try {
    if (on) localStorage.removeItem(AUTO_KEY);
    else localStorage.setItem(AUTO_KEY, '0');
  } catch { /* недоступно — просто не запоминаем */ }
};

interface UpdateStore {
  info: update.Info | null;
  phase: UpdatePhase;
  progress: { done: number; total: number } | null;
  error: string | null;
  checkedAt: number | null;
  autoCheck: boolean;

  check: (silent?: boolean) => Promise<void>;
  download: () => Promise<void>;
  cancel: () => void;
  install: () => Promise<void>;
  setAutoCheck: (on: boolean) => void;
  onProgress: (done: number, total: number) => void;
}

export const useUpdateStore = create<UpdateStore>((set, get) => ({
  info: null,
  phase: 'idle',
  progress: null,
  error: null,
  checkedAt: null,
  autoCheck: isAutoCheckEnabled(),

  // silent: фоновая проверка — ошибки сети не показываем.
  check: async (silent = false) => {
    if (get().phase !== 'idle') return;
    set({ phase: 'checking', error: silent ? get().error : null });
    try {
      const info = await CheckForUpdate();
      set({ info, checkedAt: Date.now(), error: null });
    } catch (err) {
      if (!silent) set({ error: errorMessage(err) });
    } finally {
      set({ phase: 'idle' });
    }
  },

  download: async () => {
    set({ phase: 'downloading', progress: { done: 0, total: get().info?.asset_size ?? 0 }, error: null });
    try {
      await DownloadUpdate();
      set({ phase: 'ready', progress: null });
    } catch (err) {
      set({ phase: 'idle', progress: null, error: errorMessage(err) });
    }
  },

  cancel: () => { CancelUpdateDownload(); },

  install: async () => {
    set({ phase: 'installing', error: null });
    try {
      await InstallUpdate(); // при успехе процесс завершается и запускается новая версия
    } catch (err) {
      set({ phase: 'ready', error: errorMessage(err) });
    }
  },

  setAutoCheck: (on) => { setAutoCheckEnabled(on); set({ autoCheck: on }); },

  onProgress: (done, total) => {
    if (get().phase === 'downloading') set({ progress: { done, total } });
  },
}));
