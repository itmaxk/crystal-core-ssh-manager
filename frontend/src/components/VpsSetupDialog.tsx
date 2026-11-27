import React, { useState } from 'react';
import { createPortal } from 'react-dom';
import { AlertTriangle, Loader2, Rocket, Terminal, X } from 'lucide-react';
import { Host } from '../types/ssh';
import { useEscape } from '../hooks/useEscape';

export const VPS_SETUP_SCRIPT = 'vps-setup';

// То же, что выполняет ssh-launcher-cli (internal/cli/scripts.go); здесь только для показа.
const COMMAND = `curl -fsSLo vps-setup "https://github.com/itmaxk/crystal-core-vps-install-manager/releases/latest/download/vps-setup-linux-$(dpkg --print-architecture)"
chmod +x vps-setup
sudo ./vps-setup`;

interface VpsSetupDialogProps {
  host: Host;
  onCancel: () => void;
  onRun: () => Promise<boolean>;
}

export const VpsSetupDialog: React.FC<VpsSetupDialogProps> = ({ host, onCancel, onRun }) => {
  const [busy, setBusy] = useState(false);
  useEscape(onCancel, !busy);

  const run = async () => {
    setBusy(true);
    const ok = await onRun();
    setBusy(false);
    if (ok) onCancel();
  };

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm">
      <div role="dialog" aria-modal="true" aria-labelledby="vps-setup-title" className="w-full max-w-[520px] overflow-hidden rounded-2xl border border-black/10 bg-white/95 shadow-2xl dark:border-white/10 dark:bg-neutral-900/95">
        <div className="flex items-center justify-between border-b border-black/10 px-5 py-3.5 dark:border-white/10">
          <h2 id="vps-setup-title" className="flex min-w-0 items-center gap-2 text-sm font-semibold">
            <Rocket className="h-4 w-4 shrink-0 text-blue-600 dark:text-blue-400" />
            <span className="truncate">VPS Setup на «{host.name}»</span>
          </h2>
          <button type="button" onClick={onCancel} disabled={busy} aria-label="Закрыть" className="rounded-lg p-1.5 text-neutral-400 hover:bg-black/5 disabled:opacity-50 dark:hover:bg-white/10">
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="space-y-3.5 p-5">
          <p className="text-xs text-neutral-600 dark:text-neutral-400">
            Откроется вкладка терминала с SSH-сессией{' '}
            <span className="font-mono text-neutral-800 dark:text-neutral-200">{host.username}@{host.hostname}</span>,
            и на сервере выполнится установщик Crystal-Core VPS Install Manager:
          </p>
          <pre className="overflow-x-auto whitespace-pre rounded-lg border border-black/10 bg-black/[0.03] px-3 py-2.5 font-mono text-[11px] leading-relaxed text-neutral-800 dark:border-white/10 dark:bg-white/5 dark:text-neutral-200">{COMMAND}</pre>
          <ul className="list-disc space-y-1 pl-4 text-[11px] text-neutral-600 dark:text-neutral-400">
            <li>Нужен Ubuntu или Debian. Архитектуру (amd64 / arm64) выберет <span className="font-mono">dpkg</span>.</li>
            <li>Под root установщик запустится без <span className="font-mono">sudo</span>, иначе <span className="font-mono">sudo</span> спросит пароль в терминале.</li>
            <li>Вопросы установщика отвечаются в терминале. После завершения сессия остаётся открытой.</li>
          </ul>

          {host.env_tag === 'prod' && (
            <div className="flex items-start gap-2 rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-[11px] text-red-700 dark:text-red-300">
              <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
              <span>Это prod-сервер. Установщик меняет системные настройки.</span>
            </div>
          )}

          <div className="flex justify-end gap-2 pt-1">
            <button type="button" onClick={onCancel} disabled={busy} className="rounded-lg px-3.5 py-1.5 text-xs font-medium hover:bg-black/5 disabled:opacity-50 dark:hover:bg-white/10">Отмена</button>
            <button type="button" autoFocus onClick={run} disabled={busy} className={`flex items-center gap-1.5 rounded-lg px-4 py-1.5 text-xs font-medium text-white disabled:opacity-50 ${host.env_tag === 'prod' ? 'bg-[#E81123] hover:bg-red-500' : 'bg-blue-600 hover:bg-blue-500'}`}>
              {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Terminal className="h-3.5 w-3.5" />}
              Запустить в терминале
            </button>
          </div>
        </div>
      </div>
    </div>,
    document.body
  );
};
