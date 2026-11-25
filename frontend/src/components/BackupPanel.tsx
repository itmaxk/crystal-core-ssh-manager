import React, { useState } from 'react';
import { createPortal } from 'react-dom';
import { Download, Eye, EyeOff, Loader2, ShieldCheck, Upload, X } from 'lucide-react';
import { ExportBackup, ImportBackup, PickBackupFile } from '../../wailsjs/go/app/App';
import { errorMessage } from '../store/useSSHStore';
import { inputClass } from './TunnelManager';
import { useEscape } from '../hooks/useEscape';

const MIN_PASSWORD = 8;

type Mode = { kind: 'export' } | { kind: 'import'; path: string };

interface BackupPanelProps {
  onImported: () => Promise<void>;
  notify: (kind: 'error' | 'info', text: string) => void;
}

const fileName = (path: string) => path.split(/[\\/]/).pop() || path;

const plural = (n: number, one: string, few: string, many: string) => {
  const m10 = n % 10, m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return one;
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return few;
  return many;
};

export const BackupPanel: React.FC<BackupPanelProps> = ({ onImported, notify }) => {
  const [mode, setMode] = useState<Mode | null>(null);

  const startImport = async () => {
    try {
      const path = await PickBackupFile();
      if (path) setMode({ kind: 'import', path });
    } catch (err) {
      notify('error', `Выбор файла: ${errorMessage(err)}`);
    }
  };

  // Диалог рендерим вне карточки: backdrop-filter у неё делает position: fixed
  // относительным карточке, и модальное окно оказалось бы обрезано.
  return (
    <>
    <div className="rounded-xl border border-black/10 bg-white/65 p-4 backdrop-blur-md dark:border-white/10 dark:bg-neutral-900/60">
      <div className="text-sm font-medium">Резервная копия</div>
      <p className="mt-1 text-xs text-neutral-500">
        Серверы, туннели и SSH-ключи (включая приватные) в одном файле, зашифрованном вашим паролем. Подходит для переноса на другой компьютер.
      </p>
      <div className="mt-3 flex flex-wrap items-center gap-2">
        <button onClick={() => setMode({ kind: 'export' })} className="flex items-center gap-1.5 rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-500">
          <Download className="h-3.5 w-3.5" /> Экспортировать…
        </button>
        <button onClick={startImport} className="flex items-center gap-1.5 rounded-lg border border-black/10 px-3 py-1.5 text-xs font-medium hover:bg-black/5 dark:border-white/10 dark:hover:bg-white/10">
          <Upload className="h-3.5 w-3.5" /> Импортировать…
        </button>
      </div>
    </div>

      {mode && (
        <BackupPasswordDialog
          mode={mode}
          onCancel={() => setMode(null)}
          onDone={async (message) => {
            setMode(null);
            if (mode.kind === 'import') await onImported();
            notify('info', message);
          }}
        />
      )}
    </>
  );
};

const BackupPasswordDialog: React.FC<{
  mode: Mode;
  onCancel: () => void;
  onDone: (message: string) => Promise<void>;
}> = ({ mode, onCancel, onDone }) => {
  const isExport = mode.kind === 'export';
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [show, setShow] = useState(false);
  const [touched, setTouched] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEscape(onCancel, !busy);

  const passwordErr = isExport && [...password].length < MIN_PASSWORD ? `Не короче ${MIN_PASSWORD} символов` : !password ? 'Введите пароль' : null;
  const confirmErr = isExport && confirm !== password ? 'Пароли не совпадают' : null;
  const invalid = Boolean(passwordErr || confirmErr);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setTouched(true);
    if (invalid) return;
    setBusy(true);
    setError(null);
    try {
      if (mode.kind === 'export') {
        const path = await ExportBackup(password);
        if (!path) { setBusy(false); return; } // диалог сохранения отменён
        await onDone(`Резервная копия сохранена: ${path}`);
      } else {
        const s = await ImportBackup(mode.path, password);
        const parts = [
          s.keys_added && `${s.keys_added} ${plural(s.keys_added, 'ключ', 'ключа', 'ключей')}`,
          s.hosts_added && `${s.hosts_added} ${plural(s.hosts_added, 'сервер', 'сервера', 'серверов')}`,
          s.tunnels_added && `${s.tunnels_added} ${plural(s.tunnels_added, 'туннель', 'туннеля', 'туннелей')}`,
        ].filter(Boolean);
        const updated = s.hosts_updated + s.tunnels_updated + s.keys_existing;
        let msg = parts.length ? `Импортировано: ${parts.join(', ')}` : 'Новых данных нет';
        if (updated) msg += `; обновлено или уже было: ${updated}`;
        if (s.skipped) msg += `; пропущено некорректных записей: ${s.skipped}`;
        await onDone(msg);
      }
    } catch (err) {
      setError(errorMessage(err));
      setBusy(false);
    }
  };

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm">
      <div role="dialog" aria-modal="true" aria-labelledby="backup-title" className="w-full max-w-[440px] overflow-hidden rounded-2xl border border-black/10 bg-white/95 shadow-2xl dark:border-white/10 dark:bg-neutral-900/95">
        <div className="flex items-center justify-between border-b border-black/10 px-5 py-3.5 dark:border-white/10">
          <h2 id="backup-title" className="text-sm font-semibold">{isExport ? 'Экспорт резервной копии' : 'Импорт резервной копии'}</h2>
          <button type="button" onClick={onCancel} disabled={busy} aria-label="Закрыть" className="rounded-lg p-1.5 text-neutral-400 hover:bg-black/5 disabled:opacity-50 dark:hover:bg-white/10">
            <X className="h-4 w-4" />
          </button>
        </div>

        <form onSubmit={submit} noValidate className="space-y-3.5 p-5">
          {mode.kind === 'import' && (
            <div className="rounded-lg border border-black/10 bg-black/[0.03] px-3 py-2 dark:border-white/10 dark:bg-white/5">
              <div className="text-[10px] font-medium uppercase tracking-wider text-neutral-500">Файл</div>
              <div title={mode.path} className="truncate font-mono text-xs text-neutral-700 dark:text-neutral-300">{fileName(mode.path)}</div>
            </div>
          )}

          <div>
            <label htmlFor="backup-password" className="mb-1 block text-xs font-medium text-neutral-600 dark:text-neutral-400">
              {isExport ? 'Пароль для файла' : 'Пароль от резервной копии'}
            </label>
            <div className="relative">
              <input
                id="backup-password"
                type={show ? 'text' : 'password'}
                autoFocus
                autoComplete={isExport ? 'new-password' : 'current-password'}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                aria-invalid={touched && Boolean(passwordErr)}
                className={`${inputClass(touched && Boolean(passwordErr))} pr-9`}
              />
              <button type="button" onClick={() => setShow((v) => !v)} aria-label={show ? 'Скрыть пароль' : 'Показать пароль'} className="absolute right-1.5 top-1/2 -translate-y-1/2 rounded p-1 text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200">
                {show ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
              </button>
            </div>
            {touched && passwordErr && <p className="mt-1 text-[11px] text-red-600 dark:text-red-400">{passwordErr}</p>}
          </div>

          {isExport && (
            <div>
              <label htmlFor="backup-confirm" className="mb-1 block text-xs font-medium text-neutral-600 dark:text-neutral-400">Повторите пароль</label>
              <input
                id="backup-confirm"
                type={show ? 'text' : 'password'}
                autoComplete="new-password"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                aria-invalid={touched && Boolean(confirmErr)}
                className={inputClass(touched && Boolean(confirmErr))}
              />
              {touched && confirmErr && <p className="mt-1 text-[11px] text-red-600 dark:text-red-400">{confirmErr}</p>}
            </div>
          )}

          <div className="flex items-start gap-2 rounded-lg bg-black/[0.03] px-3 py-2 text-[11px] text-neutral-600 dark:bg-white/5 dark:text-neutral-400">
            <ShieldCheck className="mt-0.5 h-3.5 w-3.5 shrink-0 text-emerald-600 dark:text-emerald-400" />
            {isExport ? (
              <span>Файл шифруется AES-256-GCM, ключ выводится из пароля (Argon2id). Без пароля восстановить копию невозможно — сохраните его в менеджере паролей.</span>
            ) : (
              <span>Данные объединятся с текущими: совпадающие ключи не дублируются, серверы и туннели с теми же ID обновятся. Ничего не удаляется.</span>
            )}
          </div>

          {error && <div role="alert" className="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-700 dark:text-red-300">{error}</div>}

          <div className="flex justify-end gap-2 pt-1">
            <button type="button" onClick={onCancel} disabled={busy} className="rounded-lg px-3.5 py-1.5 text-xs font-medium hover:bg-black/5 disabled:opacity-50 dark:hover:bg-white/10">Отмена</button>
            <button type="submit" disabled={busy || (touched && invalid)} className="flex items-center gap-1.5 rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-medium text-white hover:bg-blue-500 disabled:opacity-50">
              {busy && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
              {isExport ? 'Выбрать место и сохранить' : 'Импортировать'}
            </button>
          </div>
        </form>
      </div>
    </div>,
    document.body
  );
};
