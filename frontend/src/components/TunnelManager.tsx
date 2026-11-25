import React, { useEffect, useMemo, useState } from 'react';
import {
  AlertTriangle, ArrowLeftRight, ArrowRight, Globe, Loader2, Pencil, Play, Plus, Server, Square, Trash2, X, Zap,
} from 'lucide-react';
import { Host, TunnelDirection, TunnelDraft, TunnelSummary } from '../types/ssh';
import { errorMessage } from '../store/useSSHStore';

interface TunnelManagerProps {
  hosts: Host[];
  tunnelsByHost: Record<string, TunnelSummary[]>;
  hostFilter: string;
  onHostFilterChange: (hostId: string) => void;
  onSave: (draft: TunnelDraft) => Promise<void>;
  onDelete: (tunnelId: string) => void;
  onStart: (tunnelId: string, hostId: string) => void;
  onStop: (tunnelId: string) => void;
}

const DIRECTIONS: { value: TunnelDirection; label: string; flag: string; hint: string }[] = [
  { value: 'L', label: 'Локальный', flag: '-L', hint: 'Порт на этом компьютере → адрес, доступный с сервера' },
  { value: 'R', label: 'Удалённый', flag: '-R', hint: 'Порт на сервере → адрес на этом компьютере' },
  { value: 'D', label: 'SOCKS5', flag: '-D', hint: 'Динамический прокси: весь трафик через сервер' },
];

const STATE_META: Record<TunnelSummary['state'], { label: string; dot: string; text: string }> = {
  active: { label: 'Активен', dot: 'bg-emerald-500', text: 'text-emerald-700 dark:text-emerald-300' },
  connecting: { label: 'Подключение…', dot: 'animate-pulse bg-amber-400', text: 'text-amber-700 dark:text-amber-300' },
  reconnecting: { label: 'Переподключение…', dot: 'animate-pulse bg-amber-400', text: 'text-amber-700 dark:text-amber-300' },
  error: { label: 'Ошибка', dot: 'bg-red-500', text: 'text-red-700 dark:text-red-300' },
  stopped: { label: 'Остановлен', dot: 'bg-neutral-400 dark:bg-neutral-600', text: 'text-neutral-500 dark:text-neutral-400' },
};

// "8080" и ":8080" backend слушает на 127.0.0.1 — показываем так же.
export const displayAddr = (addr: string) => {
  const a = addr.trim();
  if (/^\d+$/.test(a)) return `127.0.0.1:${a}`;
  if (a.startsWith(':')) return `127.0.0.1${a}`;
  return a;
};

export const portOf = (addr: string) => displayAddr(addr).split(':').pop() ?? addr;

const ADDR_RE = /^(?:(?:\[[0-9A-Fa-f:.]+\]|[A-Za-z0-9._-]+)?:)?(\d{1,5})$/;
const validateAddr = (addr: string): string | null => {
  const m = addr.trim().match(ADDR_RE);
  if (!m || addr.trim().startsWith('-')) return 'Укажите порт или host:port';
  const port = Number(m[1]);
  if (port < 1 || port > 65535) return 'Порт должен быть от 1 до 65535';
  return null;
};

const isRunning = (t: TunnelSummary) => t.state !== 'stopped' && t.state !== 'error';

export const TunnelManager: React.FC<TunnelManagerProps> = ({
  hosts, tunnelsByHost, hostFilter, onHostFilterChange, onSave, onDelete, onStart, onStop,
}) => {
  const [editing, setEditing] = useState<TunnelDraft | null>(null);
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);

  const hostById = useMemo(() => new Map(hosts.map((h) => [h.id, h])), [hosts]);
  const totalCount = useMemo(() => Object.values(tunnelsByHost).flat().length, [tunnelsByHost]);

  const groups = useMemo(() => {
    return Object.entries(tunnelsByHost)
      .filter(([hostId, list]) => list.length > 0 && hostById.has(hostId) && (!hostFilter || hostId === hostFilter))
      .map(([hostId, list]) => ({ host: hostById.get(hostId)!, tunnels: list }))
      .sort((a, b) => a.host.name.localeCompare(b.host.name));
  }, [tunnelsByHost, hostById, hostFilter]);

  const hostsWithTunnels = useMemo(
    () => hosts.filter((h) => (tunnelsByHost[h.id] || []).length > 0),
    [hosts, tunnelsByHost]
  );

  const openCreate = () => {
    const defaultHost = hostFilter || hosts.find((h) => !h.is_temporary)?.id || hosts[0]?.id || '';
    setEditing({ host_id: defaultHost, name: '', direction: 'L', local_addr: '', remote_addr: '', auto_start: false });
  };

  const handleDelete = (id: string) => {
    if (confirmDeleteId !== id) {
      setConfirmDeleteId(id);
      setTimeout(() => setConfirmDeleteId((c) => (c === id ? null : c)), 3000);
      return;
    }
    setConfirmDeleteId(null);
    onDelete(id);
  };

  return (
    <div className="flex h-full flex-col overflow-hidden px-5 py-4">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-base font-semibold text-neutral-900 dark:text-neutral-100">SSH-туннели</h1>
          <p className="truncate text-xs text-neutral-500">Фоновый проброс портов с автопереподключением</p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {hostsWithTunnels.length > 1 && (
            <select
              aria-label="Фильтр по серверу"
              value={hostFilter}
              onChange={(e) => onHostFilterChange(e.target.value)}
              className="rounded-lg border border-black/10 bg-white/70 px-2.5 py-1.5 text-xs outline-none focus:border-blue-500 dark:border-white/10 dark:bg-neutral-800 dark:text-neutral-200"
            >
              <option value="">Все серверы</option>
              {hostsWithTunnels.map((h) => <option key={h.id} value={h.id}>{h.name}</option>)}
            </select>
          )}
          <button
            type="button"
            onClick={openCreate}
            disabled={hosts.length === 0}
            className="flex items-center gap-1.5 rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-500 disabled:opacity-50"
          >
            <Plus className="h-3.5 w-3.5" /> Новый туннель
          </button>
        </div>
      </div>

      {hostFilter && hostById.has(hostFilter) && (
        <div className="mb-3 flex items-center gap-2 text-xs text-neutral-500">
          <span>Показаны туннели сервера «{hostById.get(hostFilter)!.name}»</span>
          <button type="button" onClick={() => onHostFilterChange('')} className="font-medium text-blue-600 hover:underline dark:text-blue-400">
            Показать все
          </button>
        </div>
      )}

      {totalCount === 0 || groups.length === 0 ? (
        <div className="flex flex-1 flex-col items-center justify-center rounded-2xl border border-dashed border-black/15 p-8 text-center dark:border-white/15">
          <ArrowLeftRight className="mb-3 h-8 w-8 text-neutral-400" />
          <p className="text-sm font-medium text-neutral-700 dark:text-neutral-300">
            {hosts.length === 0 ? 'Сначала добавьте сервер' : hostFilter ? 'У этого сервера пока нет туннелей' : 'Туннелей пока нет'}
          </p>
          <p className="mt-1 max-w-sm text-xs text-neutral-500">
            Пробросьте базу данных на localhost, откройте локальный сервис наружу через сервер или поднимите SOCKS5-прокси.
          </p>
          {hosts.length > 0 && (
            <button type="button" onClick={openCreate} className="mt-4 flex items-center gap-1.5 rounded-lg bg-blue-600 px-3.5 py-2 text-xs font-medium text-white hover:bg-blue-500">
              <Plus className="h-3.5 w-3.5" /> Создать туннель
            </button>
          )}
        </div>
      ) : (
        <div className="flex-1 space-y-4 overflow-y-auto pb-4 pr-1">
          {groups.map(({ host, tunnels }) => (
            <section key={host.id} aria-label={`Туннели ${host.name}`}>
              <div className="mb-1.5 flex items-center gap-2 px-0.5">
                <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: host.tab_color }} />
                <span className="truncate text-xs font-semibold text-neutral-700 dark:text-neutral-200">{host.name}</span>
                <span className="truncate font-mono text-[11px] text-neutral-500">{host.username}@{host.hostname}:{host.port}</span>
                {!host.key_id && (
                  <span className="inline-flex items-center gap-1 rounded bg-amber-500/15 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:text-amber-300">
                    <AlertTriangle className="h-2.5 w-2.5" /> Нет ключа
                  </span>
                )}
              </div>
              <div className="space-y-2">
                {tunnels.map((t) => (
                  <TunnelRow
                    key={t.id}
                    tunnel={t}
                    confirmDelete={confirmDeleteId === t.id}
                    onToggle={() => (isRunning(t) ? onStop(t.id) : onStart(t.id, t.host_id))}
                    onEdit={() => setEditing({
                      id: t.id, host_id: t.host_id, name: t.name, direction: t.direction,
                      local_addr: t.local_addr, remote_addr: t.remote_addr, auto_start: t.auto_start,
                    })}
                    onDelete={() => handleDelete(t.id)}
                  />
                ))}
              </div>
            </section>
          ))}
        </div>
      )}

      {editing && (
        <TunnelForm
          initial={editing}
          hosts={hosts}
          isRunning={Boolean(editing.id && Object.values(tunnelsByHost).flat().some((t) => t.id === editing.id && isRunning(t)))}
          onCancel={() => setEditing(null)}
          onSubmit={async (draft) => {
            await onSave(draft);
            setEditing(null);
          }}
        />
      )}
    </div>
  );
};

const TunnelRow: React.FC<{
  tunnel: TunnelSummary;
  confirmDelete: boolean;
  onToggle: () => void;
  onEdit: () => void;
  onDelete: () => void;
}> = ({ tunnel: t, confirmDelete, onToggle, onEdit, onDelete }) => {
  const meta = STATE_META[t.state];
  const running = isRunning(t);
  const dir = DIRECTIONS.find((d) => d.value === t.direction);

  return (
    <div className={`rounded-xl border bg-white/65 p-3 backdrop-blur-md dark:bg-neutral-900/60 ${
      t.state === 'active' ? 'border-emerald-500/30' : t.state === 'error' ? 'border-red-500/30' : 'border-black/10 dark:border-white/10'
    }`}>
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className={`h-2 w-2 shrink-0 rounded-full ${meta.dot}`} title={meta.label} role="img" aria-label={meta.label} />
            <span
              title={dir ? `${dir.label} (${dir.flag})` : undefined}
              className="shrink-0 whitespace-nowrap rounded bg-blue-500/15 px-1.5 py-0.5 font-mono text-[10px] font-semibold text-blue-700 dark:text-blue-300"
            >
              {dir?.flag ?? t.direction}
            </span>
            <span className="truncate text-sm font-semibold text-neutral-900 dark:text-neutral-100">{t.name}</span>
            {t.auto_start && (
              <span title="Запускается вместе с приложением" className="inline-flex shrink-0 items-center gap-0.5 rounded bg-black/5 px-1.5 py-0.5 text-[10px] text-neutral-600 dark:bg-white/10 dark:text-neutral-300">
                <Zap className="h-2.5 w-2.5" /> Авто
              </span>
            )}
          </div>
          <TunnelMapping tunnel={t} />
        </div>

        <div className="flex shrink-0 items-center gap-1">
          <span className={`mr-1 hidden whitespace-nowrap text-[11px] font-medium md:inline ${meta.text}`}>
            {meta.label}
            {t.state === 'active' && t.active_conns > 0 && ` · ${t.active_conns} соед.`}
          </span>
          <button
            type="button"
            onClick={onToggle}
            aria-label={running ? 'Остановить туннель' : 'Запустить туннель'}
            className={`flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-xs font-medium transition ${
              running
                ? 'bg-black/5 text-neutral-700 hover:bg-black/10 dark:bg-white/10 dark:text-neutral-200 dark:hover:bg-white/15'
                : 'bg-blue-600 text-white hover:bg-blue-500'
            }`}
          >
            {running ? <Square className="h-3 w-3 fill-current" /> : <Play className="h-3 w-3 fill-current" />}
            {running ? 'Стоп' : 'Старт'}
          </button>
          <button type="button" onClick={onEdit} aria-label="Изменить туннель" title="Изменить" className="rounded-lg p-1.5 text-neutral-500 hover:bg-black/5 dark:hover:bg-white/10">
            <Pencil className="h-3.5 w-3.5" />
          </button>
          <button
            type="button"
            onClick={onDelete}
            aria-label="Удалить туннель"
            className={`flex items-center gap-1 rounded-lg px-1.5 py-1.5 text-xs font-medium ${
              confirmDelete ? 'bg-red-600 text-white' : 'text-neutral-400 hover:bg-red-500/10 hover:text-red-600'
            }`}
          >
            <Trash2 className="h-3.5 w-3.5" /> {confirmDelete && <span>Удалить?</span>}
          </button>
        </div>
      </div>

      {t.state === 'error' || (t.state === 'reconnecting' && t.last_error) ? (
        <p title={t.last_error} className={`mt-2 line-clamp-2 break-all rounded-md px-2 py-1 font-mono text-[11px] ${
          t.state === 'error' ? 'bg-red-500/10 text-red-700 dark:text-red-300' : 'bg-amber-500/10 text-amber-700 dark:text-amber-300'
        }`}>
          {t.last_error || 'Неизвестная ошибка'}
        </p>
      ) : null}
    </div>
  );
};

const TunnelMapping: React.FC<{ tunnel: Pick<TunnelSummary, 'direction' | 'local_addr' | 'remote_addr'> }> = ({ tunnel: t }) => {
  const chip = (icon: React.ReactNode, text: string, where: string) => (
    <span title={where} className="inline-flex min-w-0 items-center gap-1">
      {icon}<span className="break-all">{text}</span>
    </span>
  );
  const local = <Globe className="h-3 w-3 shrink-0 text-neutral-400" aria-label="этот компьютер" />;
  const server = <Server className="h-3 w-3 shrink-0 text-neutral-400" aria-label="сервер" />;
  const arrow = <ArrowRight className="h-3 w-3 shrink-0 text-neutral-400" aria-hidden />;

  return (
    <div className="mt-1 flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-0.5 font-mono text-xs text-neutral-600 dark:text-neutral-400">
      {t.direction === 'L' && <>{chip(local, displayAddr(t.local_addr), 'Слушает на этом компьютере')}{arrow}{chip(server, t.remote_addr, 'Адрес со стороны сервера')}</>}
      {t.direction === 'R' && <>{chip(server, displayAddr(t.remote_addr), 'Слушает на сервере')}{arrow}{chip(local, t.local_addr, 'Адрес на этом компьютере')}</>}
      {t.direction === 'D' && <>{chip(local, `socks5://${displayAddr(t.local_addr)}`, 'SOCKS5-прокси на этом компьютере')}</>}
    </div>
  );
};

export const inputClass = (invalid: boolean) =>
  `w-full rounded-lg border bg-black/[0.03] px-3 py-1.5 font-mono text-sm outline-none focus:border-blue-500 dark:bg-white/5 dark:text-white ${
    invalid ? 'border-red-500/60' : 'border-black/10 dark:border-white/10'
  }`;

const TunnelForm: React.FC<{
  initial: TunnelDraft;
  hosts: Host[];
  isRunning: boolean;
  onCancel: () => void;
  onSubmit: (draft: TunnelDraft) => Promise<void>;
}> = ({ initial, hosts, isRunning: running, onCancel, onSubmit }) => {
  const [draft, setDraft] = useState<TunnelDraft>(initial);
  const [touched, setTouched] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const isEdit = Boolean(initial.id);
  const host = hosts.find((h) => h.id === draft.host_id);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onCancel(); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onCancel]);

  const set = <K extends keyof TunnelDraft>(key: K, value: TunnelDraft[K]) => setDraft((d) => ({ ...d, [key]: value }));

  const localErr = validateAddr(draft.local_addr);
  const remoteErr = draft.direction === 'D' ? null : validateAddr(draft.remote_addr);
  const nameErr = draft.name.trim() ? null : 'Введите название';
  const hostErr = draft.host_id ? null : 'Выберите сервер';
  const invalid = Boolean(localErr || remoteErr || nameErr || hostErr);

  // Поля в порядке движения трафика: откуда слушаем → куда пробрасываем.
  const fields = {
    L: [
      { key: 'local_addr' as const, label: 'Локальный порт', placeholder: '15432', err: localErr },
      { key: 'remote_addr' as const, label: 'Адрес со стороны сервера', placeholder: 'localhost:5432', err: remoteErr },
    ],
    R: [
      { key: 'remote_addr' as const, label: 'Порт на сервере', placeholder: '8080', err: remoteErr },
      { key: 'local_addr' as const, label: 'Адрес на этом компьютере', placeholder: 'localhost:3000', err: localErr },
    ],
    D: [
      { key: 'local_addr' as const, label: 'Локальный порт SOCKS5', placeholder: '1080', err: localErr },
    ],
  }[draft.direction];

  const sshPreview = useMemo(() => {
    if (!host) return '';
    const target = `${host.username}@${host.hostname}${host.port !== 22 ? ` -p ${host.port}` : ''}`;
    const l = displayAddr(draft.local_addr || '…');
    const r = draft.remote_addr || '…';
    if (draft.direction === 'L') return `ssh -N -L ${l}:${r} ${target}`;
    if (draft.direction === 'R') return `ssh -N -R ${displayAddr(r)}:${draft.local_addr || '…'} ${target}`;
    return `ssh -N -D ${l} ${target}`;
  }, [draft, host]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setTouched(true);
    if (invalid) return;
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit({
        ...draft,
        name: draft.name.trim(),
        local_addr: draft.local_addr.trim(),
        remote_addr: draft.direction === 'D' ? '' : draft.remote_addr.trim(),
      });
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm">
      <div role="dialog" aria-modal="true" aria-labelledby="tunnel-form-title" className="w-full max-w-[500px] overflow-hidden rounded-2xl border border-black/10 bg-white/95 shadow-2xl dark:border-white/10 dark:bg-neutral-900/95">
        <div className="flex items-center justify-between border-b border-black/10 px-5 py-3.5 dark:border-white/10">
          <h2 id="tunnel-form-title" className="text-sm font-semibold">{isEdit ? 'Изменить туннель' : 'Новый туннель'}</h2>
          <button type="button" onClick={onCancel} aria-label="Закрыть" className="rounded-lg p-1.5 text-neutral-400 hover:bg-black/5 dark:hover:bg-white/10">
            <X className="h-4 w-4" />
          </button>
        </div>

        <form onSubmit={handleSubmit} noValidate className="space-y-3.5 p-5">
          <div className="grid grid-cols-2 gap-2.5">
            <div>
              <label htmlFor="tunnel-host" className="mb-1 block text-xs font-medium text-neutral-600 dark:text-neutral-400">Сервер</label>
              <select
                id="tunnel-host"
                value={draft.host_id}
                disabled={isEdit}
                onChange={(e) => set('host_id', e.target.value)}
                className="w-full rounded-lg border border-black/10 bg-black/[0.03] px-2.5 py-1.5 text-sm outline-none focus:border-blue-500 disabled:opacity-60 dark:border-white/10 dark:bg-neutral-800 dark:text-white"
              >
                {!draft.host_id && <option value="">—</option>}
                {hosts.map((h) => <option key={h.id} value={h.id}>{h.name}{h.is_temporary ? ' (временный)' : ''}</option>)}
              </select>
            </div>
            <div>
              <label htmlFor="tunnel-name" className="mb-1 block text-xs font-medium text-neutral-600 dark:text-neutral-400">Название</label>
              <input
                id="tunnel-name"
                type="text"
                autoFocus
                value={draft.name}
                onChange={(e) => set('name', e.target.value)}
                placeholder="Postgres prod"
                className={inputClass(touched && Boolean(nameErr)).replace('font-mono ', '')}
              />
            </div>
          </div>

          {host && !host.key_id && (
            <div className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-800 dark:text-amber-200">
              <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
              У сервера не назначен SSH-ключ — туннель не запустится. Назначьте ключ через One-Click Bootstrap.
            </div>
          )}

          <div>
            <span className="mb-1 block text-xs font-medium text-neutral-600 dark:text-neutral-400">Тип</span>
            <div role="radiogroup" aria-label="Тип туннеля" className="grid grid-cols-3 gap-1 rounded-lg bg-black/5 p-1 dark:bg-white/5">
              {DIRECTIONS.map((d) => (
                <button
                  key={d.value}
                  type="button"
                  role="radio"
                  aria-checked={draft.direction === d.value}
                  onClick={() => set('direction', d.value)}
                  className={`rounded-md px-2 py-1.5 text-xs font-medium transition ${
                    draft.direction === d.value
                      ? 'bg-white text-neutral-900 shadow-sm dark:bg-neutral-700 dark:text-white'
                      : 'text-neutral-600 hover:text-neutral-900 dark:text-neutral-400 dark:hover:text-white'
                  }`}
                >
                  {d.label} <span className="font-mono text-[10px] opacity-60">{d.flag}</span>
                </button>
              ))}
            </div>
            <p className="mt-1 text-[11px] text-neutral-500">{DIRECTIONS.find((d) => d.value === draft.direction)?.hint}</p>
          </div>

          <div className={`grid items-start gap-2 ${fields.length === 2 ? 'grid-cols-[1fr_auto_1fr]' : 'grid-cols-1'}`}>
            {fields.map((f, i) => (
              <React.Fragment key={f.key}>
                {i === 1 && <ArrowRight className="mt-7 h-4 w-4 text-neutral-400" aria-hidden />}
                <div className="min-w-0">
                  <label htmlFor={`tunnel-${f.key}`} className="mb-1 block text-xs font-medium text-neutral-600 dark:text-neutral-400">{f.label}</label>
                  <input
                    id={`tunnel-${f.key}`}
                    type="text"
                    inputMode="url"
                    value={draft[f.key]}
                    onChange={(e) => set(f.key, e.target.value)}
                    placeholder={f.placeholder}
                    aria-invalid={touched && Boolean(f.err)}
                    className={inputClass(touched && Boolean(f.err))}
                  />
                  {touched && f.err && <p className="mt-1 text-[11px] text-red-600 dark:text-red-400">{f.err}</p>}
                </div>
              </React.Fragment>
            ))}
          </div>
          <p className="-mt-1.5 text-[11px] text-neutral-500">
            {draft.direction === 'R' ? (
              <>Только порт — сервер слушает на 127.0.0.1. Для <span className="font-mono">0.0.0.0:порт</span> на сервере нужен <span className="font-mono">GatewayPorts yes</span>.</>
            ) : (
              <>Только порт — слушать на 127.0.0.1. Укажите <span className="font-mono">0.0.0.0:порт</span>, чтобы открыть доступ из сети.</>
            )}
          </p>

          {sshPreview && (
            <div className="rounded-lg border border-black/10 bg-black/[0.03] px-3 py-2 dark:border-white/10 dark:bg-white/5">
              <div className="mb-0.5 text-[10px] font-medium uppercase tracking-wider text-neutral-500">Эквивалент в OpenSSH</div>
              <code className="block break-all font-mono text-xs text-neutral-700 dark:text-neutral-300">{sshPreview}</code>
            </div>
          )}

          <label className="flex cursor-pointer items-center gap-2 text-xs">
            <input type="checkbox" checked={draft.auto_start} onChange={(e) => set('auto_start', e.target.checked)} />
            Запускать вместе с приложением
          </label>

          {running && (
            <p className="text-[11px] text-neutral-500">Туннель сейчас работает — после сохранения он будет перезапущен.</p>
          )}
          {error && <div role="alert" className="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-700 dark:text-red-300">{error}</div>}

          <div className="flex justify-end gap-2 pt-1">
            <button type="button" onClick={onCancel} className="rounded-lg px-3.5 py-1.5 text-xs font-medium hover:bg-black/5 dark:hover:bg-white/10">Отмена</button>
            <button type="submit" disabled={submitting || (touched && invalid)} className="flex items-center gap-1.5 rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-medium text-white hover:bg-blue-500 disabled:opacity-50">
              {submitting && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
              {isEdit ? 'Сохранить' : 'Создать'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
