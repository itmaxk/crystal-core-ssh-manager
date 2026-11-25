import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { createPortal } from 'react-dom';
import {
  AlertTriangle, ArrowRight, ChevronDown, ChevronRight, Loader2, Monitor, Radar, RefreshCw, X,
} from 'lucide-react';
import { DiscoverServices } from '../../wailsjs/go/app/App';
import { app } from '../../wailsjs/go/models';
import { Host } from '../types/ssh';
import { errorMessage } from '../store/useSSHStore';
import { useEscape } from '../hooks/useEscape';

export interface DiscoveredTunnel {
  name: string;
  local_addr: string;
  remote_addr: string;
}

interface ServiceDiscoveryProps {
  hosts: Host[];
  initialHostId: string;
  onClose: () => void;
  onCreate: (hostId: string, tunnels: DiscoveredTunnel[], start: boolean) => Promise<{ created: number; errors: string[] }>;
}

type Svc = app.DiscoveredService;

const BIND_META: Record<string, { label: string; title: string; className: string }> = {
  loopback: {
    label: 'только на сервере',
    title: 'Слушает 127.0.0.1 / ::1 — снаружи недоступен, туннель — единственный способ подключиться',
    className: 'bg-black/5 text-neutral-600 dark:bg-white/10 dark:text-neutral-300',
  },
  all: {
    label: 'открыт наружу',
    title: 'Слушает на всех интерфейсах (0.0.0.0) — порт может быть доступен из интернета, проверьте файрвол (ufw)',
    className: 'bg-amber-500/15 text-amber-700 dark:text-amber-300',
  },
  address: {
    label: 'на адресе',
    title: 'Слушает на конкретном адресе сервера (например, docker0 или локальной сети)',
    className: 'bg-black/5 text-neutral-600 dark:bg-white/10 dark:text-neutral-300',
  },
};

const isKnown = (s: Svc) => !s.name.startsWith('Порт ');

export const ServiceDiscovery: React.FC<ServiceDiscoveryProps> = ({ hosts, initialHostId, onClose, onCreate }) => {
  const usable = useMemo(() => hosts.filter((h) => h.key_id), [hosts]);
  const [hostId, setHostId] = useState(() =>
    usable.some((h) => h.id === initialHostId) ? initialHostId : usable.find((h) => !h.is_temporary)?.id ?? usable[0]?.id ?? ''
  );
  const [result, setResult] = useState<app.DiscoveryResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<Record<number, boolean>>({});
  const [localPorts, setLocalPorts] = useState<Record<number, string>>({});
  const [showSystem, setShowSystem] = useState(false);
  const [start, setStart] = useState(true);
  const [creating, setCreating] = useState(false);
  const [createErrors, setCreateErrors] = useState<string[]>([]);

  useEscape(onClose, !creating);

  const host = hosts.find((h) => h.id === hostId);

  const scan = useCallback(async (id: string) => {
    if (!id) return;
    setLoading(true);
    setError(null);
    setResult(null);
    setCreateErrors([]);
    try {
      const res = await DiscoverServices(id);
      setResult(res);
      const sel: Record<number, boolean> = {};
      const ports: Record<number, string> = {};
      for (const s of res.services || []) {
        if (s.system || s.existing_tunnel) continue;
        sel[s.port] = isKnown(s);
        ports[s.port] = s.local_port ? String(s.local_port) : '';
      }
      setSelected(sel);
      setLocalPorts(ports);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { scan(hostId); }, [hostId, scan]);

  const services = result?.services || [];
  const regular = services.filter((s) => !s.system);
  const system = services.filter((s) => s.system);
  const chosen = regular.filter((s) => selected[s.port] && !s.existing_tunnel);

  const portErrors = useMemo(() => {
    const errs: Record<number, string> = {};
    const seen = new Map<number, number>();
    for (const s of chosen) {
      const v = Number(localPorts[s.port]);
      if (!Number.isInteger(v) || v < 1 || v > 65535) { errs[s.port] = '1–65535'; continue; }
      if (seen.has(v)) { errs[s.port] = 'Повтор'; errs[seen.get(v)!] = 'Повтор'; }
      seen.set(v, s.port);
    }
    return errs;
  }, [chosen, localPorts]);

  const canCreate = chosen.length > 0 && Object.keys(portErrors).length === 0 && !creating;

  const create = async () => {
    if (!canCreate || !hostId) return;
    setCreating(true);
    setCreateErrors([]);
    const { created, errors } = await onCreate(
      hostId,
      // Короткое имя для RDP: длинное «Удалённый рабочий стол (xrdp)» обрезается в списке.
      chosen.map((s) => ({ name: s.is_rdp ? 'RDP' : s.name, local_addr: localPorts[s.port], remote_addr: s.remote_addr })),
      start
    );
    setCreating(false);
    if (errors.length === 0) onClose();
    else {
      setCreateErrors(errors);
      if (created > 0) scan(hostId);
    }
  };

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm">
      <div role="dialog" aria-modal="true" aria-labelledby="discover-title" className="flex max-h-full w-full max-w-[640px] flex-col overflow-hidden rounded-2xl border border-black/10 bg-white/95 shadow-2xl dark:border-white/10 dark:bg-neutral-900/95">
        <div className="flex items-center justify-between border-b border-black/10 px-5 py-3.5 dark:border-white/10">
          <div className="min-w-0">
            <h2 id="discover-title" className="text-sm font-semibold">Найти сервисы на сервере</h2>
            <p className="truncate text-xs text-neutral-500">Читаем открытые порты по SSH и предлагаем туннели к ним</p>
          </div>
          <button type="button" onClick={onClose} disabled={creating} aria-label="Закрыть" className="rounded-lg p-1.5 text-neutral-400 hover:bg-black/5 disabled:opacity-50 dark:hover:bg-white/10">
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="flex items-center gap-2 border-b border-black/10 px-5 py-3 dark:border-white/10">
          <select
            aria-label="Сервер"
            value={hostId}
            onChange={(e) => setHostId(e.target.value)}
            disabled={loading || creating}
            className="min-w-0 flex-1 rounded-lg border border-black/10 bg-black/[0.03] px-2.5 py-1.5 text-sm outline-none focus:border-blue-500 disabled:opacity-60 dark:border-white/10 dark:bg-neutral-800 dark:text-white"
          >
            {!hostId && <option value="">—</option>}
            {hosts.map((h) => (
              <option key={h.id} value={h.id} disabled={!h.key_id}>
                {h.name} — {h.username}@{h.hostname}{h.key_id ? '' : ' (нет ключа)'}
              </option>
            ))}
          </select>
          <button
            type="button"
            onClick={() => scan(hostId)}
            disabled={loading || creating || !hostId}
            title="Сканировать заново"
            aria-label="Сканировать заново"
            className="rounded-lg border border-black/10 p-2 text-neutral-600 hover:bg-black/5 disabled:opacity-50 dark:border-white/10 dark:text-neutral-300 dark:hover:bg-white/10"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
          </button>
        </div>

        <div className="min-h-[180px] flex-1 overflow-y-auto px-5 py-3">
          {!hostId && (
            <p className="py-8 text-center text-xs text-neutral-500">Нет серверов с SSH-ключом. Назначьте ключ серверу через «Изменить сервер» или One-Click Bootstrap.</p>
          )}

          {loading && (
            <div className="flex flex-col items-center justify-center gap-2 py-10 text-xs text-neutral-500">
              <Loader2 className="h-5 w-5 animate-spin text-blue-600" />
              Подключение к {host?.name ?? 'серверу'} и чтение списка портов (<span className="font-mono">ss -tln</span>)…
            </div>
          )}

          {error && !loading && (
            <div role="alert" className="my-2 rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-700 dark:text-red-300">{error}</div>
          )}

          {result && !loading && (
            <>
              <div className="mb-2 flex flex-wrap items-center gap-x-2 text-[11px] text-neutral-500">
                <span>{result.os && result.os !== 'unknown' ? result.os : 'ОС не определена'}</span>
                <span aria-hidden>·</span>
                <span>сервисов: {regular.length}</span>
              </div>
              {!result.is_ubuntu && (
                <p className="mb-2 flex items-start gap-1.5 text-[11px] text-amber-700 dark:text-amber-300">
                  <AlertTriangle className="mt-0.5 h-3 w-3 shrink-0" />
                  Список системных портов рассчитан на Ubuntu — на других ОС проверьте выбор вручную.
                </p>
              )}

              {regular.length === 0 ? (
                <p className="py-6 text-center text-xs text-neutral-500">
                  Кроме стандартных портов чистой Ubuntu на сервере ничего не слушает.
                </p>
              ) : (
                <ul className="divide-y divide-black/5 dark:divide-white/5">
                  {regular.map((s) => {
                    const bind = BIND_META[s.bind] ?? BIND_META.address;
                    const disabled = Boolean(s.existing_tunnel);
                    const err = selected[s.port] ? portErrors[s.port] : undefined;
                    return (
                      <li key={s.port} className={`flex items-center gap-3 py-2 ${disabled ? 'opacity-60' : ''}`}>
                        <input
                          type="checkbox"
                          aria-label={`Создать туннель к ${s.name}`}
                          checked={!disabled && Boolean(selected[s.port])}
                          disabled={disabled || creating}
                          onChange={(e) => setSelected((m) => ({ ...m, [s.port]: e.target.checked }))}
                        />
                        <div className="min-w-0 flex-1">
                          <div className="flex min-w-0 items-center gap-1.5">
                            {s.is_rdp && <Monitor className="h-3.5 w-3.5 shrink-0 text-neutral-500" aria-hidden />}
                            <span className="truncate text-sm font-medium text-neutral-900 dark:text-neutral-100">{s.name}</span>
                            {s.process && <span className="truncate font-mono text-[11px] text-neutral-400">{s.process}</span>}
                          </div>
                          <div className="mt-0.5 flex flex-wrap items-center gap-1.5">
                            <span className="font-mono text-[11px] text-neutral-500">{s.remote_addr}</span>
                            <span title={bind.title} className={`whitespace-nowrap rounded px-1.5 py-px text-[10px] font-medium ${bind.className}`}>
                              {bind.label}
                            </span>
                          </div>
                        </div>
                        {disabled ? (
                          <span className="shrink-0 text-[11px] text-neutral-500">есть туннель «{s.existing_tunnel}»</span>
                        ) : (
                          <div className="flex shrink-0 items-center gap-1.5">
                            <ArrowRight className="h-3.5 w-3.5 text-neutral-400" aria-hidden />
                            <div className="w-[88px]">
                              <input
                                type="text"
                                inputMode="numeric"
                                aria-label={`Локальный порт для ${s.name}`}
                                title="Порт на этом компьютере (127.0.0.1)"
                                value={localPorts[s.port] ?? ''}
                                disabled={creating}
                                onChange={(e) => setLocalPorts((m) => ({ ...m, [s.port]: e.target.value.replace(/\D/g, '').slice(0, 5) }))}
                                className={`w-full rounded-md border bg-black/[0.03] px-2 py-1 font-mono text-xs outline-none focus:border-blue-500 dark:bg-white/5 dark:text-white ${
                                  err ? 'border-red-500/60' : 'border-black/10 dark:border-white/10'
                                }`}
                              />
                              {err && <p className="mt-0.5 text-[10px] text-red-600 dark:text-red-400">{err}</p>}
                            </div>
                          </div>
                        )}
                      </li>
                    );
                  })}
                </ul>
              )}

              {system.length > 0 && (
                <div className="mt-3 border-t border-black/5 pt-2 dark:border-white/5">
                  <button
                    type="button"
                    onClick={() => setShowSystem((v) => !v)}
                    aria-expanded={showSystem}
                    className="flex items-center gap-1 text-[11px] font-medium text-neutral-500 hover:text-neutral-800 dark:hover:text-neutral-200"
                  >
                    {showSystem ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
                    Системные порты, скрыты ({system.length})
                  </button>
                  {showSystem && (
                    <ul className="mt-1.5 space-y-1 pl-4">
                      {system.map((s) => (
                        <li key={s.port} className="flex gap-2 text-[11px] text-neutral-500">
                          <span className="w-12 shrink-0 font-mono">{s.port}</span>
                          <span className="min-w-0">{s.reason}{s.process ? ` · ${s.process}` : ''}</span>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              )}
            </>
          )}

          {createErrors.length > 0 && (
            <div role="alert" className="mt-3 space-y-0.5 rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-700 dark:text-red-300">
              {createErrors.map((e) => <div key={e}>{e}</div>)}
            </div>
          )}
        </div>

        <div className="flex flex-wrap items-center justify-between gap-2 border-t border-black/10 px-5 py-3 dark:border-white/10">
          <label className="flex cursor-pointer items-center gap-2 text-xs">
            <input type="checkbox" checked={start} onChange={(e) => setStart(e.target.checked)} disabled={creating} />
            Сразу запустить
          </label>
          <div className="flex gap-2">
            <button type="button" onClick={onClose} disabled={creating} className="rounded-lg px-3.5 py-1.5 text-xs font-medium hover:bg-black/5 disabled:opacity-50 dark:hover:bg-white/10">Отмена</button>
            <button
              type="button"
              onClick={create}
              disabled={!canCreate}
              className="flex items-center gap-1.5 rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-medium text-white hover:bg-blue-500 disabled:opacity-50"
            >
              {creating ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Radar className="h-3.5 w-3.5" />}
              {chosen.length > 0 ? `Создать туннели (${chosen.length})` : 'Создать туннели'}
            </button>
          </div>
        </div>
      </div>
    </div>,
    document.body
  );
};
