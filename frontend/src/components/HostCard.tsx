import React, { useState } from 'react';
import { ArrowLeftRight, KeyRound, Pin, Play, Trash2, Upload } from 'lucide-react';
import { Host, TunnelSummary } from '../types/ssh';
import { portOf } from './TunnelManager';

const ENV_STRIPE: Record<Host['env_tag'], string> = {
  prod: 'bg-[#E81123]',
  stage: 'bg-[#FFB900]',
  dev: 'bg-[#107C10]',
  infra: 'bg-[#0078D4]',
};

interface HostCardProps {
  host: Host;
  keyName?: string;
  pingMs?: number | null;
  activeSessions: number;
  tunnels: TunnelSummary[];
  isSelected: boolean;
  uploadProgress?: number | null;
  confirmProd?: boolean;
  onSelect: (hostId: string, multi: boolean) => void;
  onConnect: (hostId: string) => void;
  onPinTemporary?: (hostId: string) => void;
  onOpenTunnels?: (hostId: string) => void;
  onDelete?: (hostId: string) => void;
}

// Нативный drop Wails: элемент с этим CSS-свойством становится целью, а на время
// перетаскивания получает класс .wails-drop-target-active (см. index.css).
const DROP_TARGET_STYLE = { '--wails-drop-target': 'drop' } as React.CSSProperties;

export const HostCard: React.FC<HostCardProps> = ({
  host,
  keyName,
  pingMs,
  activeSessions,
  tunnels,
  isSelected,
  uploadProgress,
  confirmProd = true,
  onSelect,
  onConnect,
  onPinTemporary,
  onOpenTunnels,
  onDelete,
}) => {
  const [prodConfirmPending, setProdConfirmPending] = useState(false);
  const [deleteConfirmPending, setDeleteConfirmPending] = useState(false);

  const isOffline = pingMs === null;
  const activeTunnel = tunnels.find((t) => t.state === 'active');
  const failedTunnel = !activeTunnel ? tunnels.find((t) => t.state === 'error') : undefined;

  const handleConnectClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (host.env_tag === 'prod' && confirmProd && !prodConfirmPending) {
      setProdConfirmPending(true);
      setTimeout(() => setProdConfirmPending(false), 3000);
      return;
    }
    setProdConfirmPending(false);
    onConnect(host.id);
  };

  const handleDeleteClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!deleteConfirmPending) {
      setDeleteConfirmPending(true);
      setTimeout(() => setDeleteConfirmPending(false), 3000);
      return;
    }
    setDeleteConfirmPending(false);
    onDelete?.(host.id);
  };

  return (
    <div
      onClick={(e) => onSelect(host.id, e.ctrlKey || e.metaKey)}
      onDoubleClick={() => onConnect(host.id)}
      data-host-id={host.id}
      style={DROP_TARGET_STYLE}
      className={`host-card group relative overflow-hidden rounded-xl border p-3.5 pl-4 transition-all select-none ${
        host.is_temporary ? 'border-dashed' : 'border-solid'
      } ${
        isSelected
          ? 'border-blue-500 bg-blue-500/10 ring-1 ring-blue-500/40'
          : activeSessions > 0
          ? 'border-black/15 bg-white/75 shadow-sm dark:border-white/20 dark:bg-neutral-900/75'
          : 'border-black/10 bg-white/60 hover:bg-white/80 dark:border-white/10 dark:bg-neutral-900/55 dark:hover:bg-neutral-900/75'
      } ${isOffline ? 'opacity-70' : ''} backdrop-blur-md`}
    >
      <div className={`absolute bottom-0 left-0 top-0 w-[3px] ${ENV_STRIPE[host.env_tag]}`} />

      <div className="drop-overlay pointer-events-none absolute inset-0 z-20 hidden flex-col items-center justify-center gap-1 rounded-xl border-2 border-dashed border-blue-500 bg-blue-500/15 backdrop-blur-sm">
        <Upload className="h-5 w-5 text-blue-600 dark:text-blue-400" />
        <span className="text-xs font-medium text-blue-700 dark:text-blue-200">
          {host.key_id ? 'Загрузить по SFTP в /tmp' : 'Назначьте ключ для SFTP'}
        </span>
      </div>

      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <h3 className="truncate text-sm font-semibold text-neutral-900 dark:text-neutral-100">{host.name}</h3>
          <p className="truncate font-mono text-xs text-neutral-500 dark:text-neutral-400">
            {host.username}@{host.hostname}:{host.port}
          </p>
        </div>

        <div className="flex shrink-0 items-center gap-1.5 font-mono text-[11px] tabular-nums">
          <span
            className={`h-2 w-2 rounded-full ${
              isOffline ? 'bg-neutral-400 dark:bg-neutral-600' : pingMs !== undefined ? 'bg-emerald-500' : 'animate-pulse bg-amber-400'
            }`}
          />
          <span className="text-neutral-500 dark:text-neutral-400">
            {isOffline ? 'offline' : pingMs !== undefined ? `${pingMs} ms` : '...'}
          </span>
        </div>
      </div>

      <div className="mt-3 flex items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-1.5">
          {keyName && (
            <span className="inline-flex items-center gap-1 rounded-md bg-black/5 px-1.5 py-0.5 text-[11px] text-neutral-600 dark:bg-white/10 dark:text-neutral-300">
              <KeyRound className="h-3 w-3 text-amber-500" />
              {keyName}
            </span>
          )}
          {activeSessions > 0 && (
            <span className="inline-flex items-center gap-1 rounded-md bg-emerald-500/15 px-1.5 py-0.5 text-[11px] font-medium text-emerald-700 dark:text-emerald-300">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
              {activeSessions} {activeSessions === 1 ? 'сессия' : 'сессии'}
            </span>
          )}
          {activeTunnel && (
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); onOpenTunnels?.(host.id); }}
              className="inline-flex items-center gap-1 rounded-md bg-blue-500/15 px-1.5 py-0.5 font-mono text-[11px] text-blue-700 hover:bg-blue-500/25 dark:text-blue-300"
            >
              <ArrowLeftRight className="h-3 w-3" />
              {activeTunnel.direction === 'R' ? `↑${portOf(activeTunnel.remote_addr)}` : portOf(activeTunnel.local_addr)}
              {activeTunnel.active_conns > 0 && ` (${activeTunnel.active_conns})`}
            </button>
          )}
          {failedTunnel && (
            <button
              type="button"
              title={failedTunnel.last_error}
              onClick={(e) => { e.stopPropagation(); onOpenTunnels?.(host.id); }}
              className="inline-flex items-center gap-1 rounded-md bg-red-500/15 px-1.5 py-0.5 text-[11px] font-medium text-red-700 hover:bg-red-500/25 dark:text-red-300"
            >
              <ArrowLeftRight className="h-3 w-3" />
              Ошибка туннеля
            </button>
          )}
          {host.is_temporary && (
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); onPinTemporary?.(host.id); }}
              className="inline-flex items-center gap-1 rounded-md bg-amber-500/15 px-1.5 py-0.5 text-[11px] font-medium text-amber-700 hover:bg-amber-500/25 dark:text-amber-300"
            >
              <Pin className="h-3 w-3" />
              Закрепить
            </button>
          )}
        </div>

        <div className="flex shrink-0 items-center gap-1">
        {onDelete && (
          <button
            type="button"
            onClick={handleDeleteClick}
            title="Удалить сервер"
            className={`flex items-center gap-1 rounded-md px-1.5 py-1 text-xs font-medium transition ${
              deleteConfirmPending
                ? 'bg-red-600 text-white'
                : 'text-neutral-400 opacity-0 hover:bg-red-500/15 hover:text-red-600 group-hover:opacity-100'
            }`}
          >
            <Trash2 className="h-3 w-3" />
            {deleteConfirmPending && 'Удалить?'}
          </button>
        )}
        <button
          type="button"
          onClick={handleConnectClick}
          className={`flex shrink-0 items-center gap-1 rounded-md px-2 py-1 text-xs font-medium transition ${
            prodConfirmPending
              ? 'bg-[#E81123] text-white'
              : 'bg-black/5 text-neutral-700 opacity-0 hover:bg-blue-600 hover:text-white group-hover:opacity-100 dark:bg-white/10 dark:text-neutral-200 dark:hover:bg-blue-600'
          }`}
        >
          <Play className="h-3 w-3 fill-current" />
          {prodConfirmPending ? 'Точно Prod?' : 'Войти'}
        </button>
        </div>
      </div>

      {uploadProgress !== null && uploadProgress !== undefined && (
        <div className="absolute bottom-0 left-0 right-0 h-1 bg-black/10 dark:bg-white/10">
          <div className="h-full bg-blue-500 transition-all duration-150" style={{ width: `${Math.min(100, Math.max(0, uploadProgress))}%` }} />
        </div>
      )}
    </div>
  );
};
