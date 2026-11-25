import React, { useMemo, useState } from 'react';
import { AlertTriangle, Loader2, X } from 'lucide-react';
import { EnvTag, Host, HostDraft, SSHKey } from '../types/ssh';
import { ENV_TAB_COLOR, errorMessage } from '../store/useSSHStore';
import { inputClass } from './TunnelManager';
import { useEscape } from '../hooks/useEscape';

const ENV_OPTIONS: { value: EnvTag; label: string }[] = [
  { value: 'prod', label: 'prod' },
  { value: 'stage', label: 'stage' },
  { value: 'dev', label: 'dev' },
  { value: 'infra', label: 'infra' },
];

const HOSTNAME_RE = /^[A-Za-z0-9._:%\[\]-]+$/;
// Те же правила, что в internal/ssh/validate.go.
const USERNAME_BAD_RE = /[\s:"'\u0000-\u001f\u007f]/;

const labelClass = 'mb-1 block text-xs font-medium text-neutral-600 dark:text-neutral-400';
const selectClass =
  'w-full rounded-lg border border-black/10 bg-black/[0.03] px-2.5 py-1.5 text-sm outline-none focus:border-blue-500 dark:border-white/10 dark:bg-neutral-800 dark:text-white';

export const newHostDraft = (keys: SSHKey[]): HostDraft => ({
  name: '',
  hostname: '',
  port: 22,
  username: 'root',
  key_id: keys.find((k) => k.is_default)?.id ?? '',
  jump_host_id: '',
  env_tag: 'dev',
  tab_color: ENV_TAB_COLOR.dev,
  is_temporary: false,
});

export const hostToDraft = (h: Host): HostDraft => ({
  id: h.id,
  name: h.name,
  hostname: h.hostname,
  port: h.port,
  username: h.username,
  key_id: h.key_id ?? '',
  jump_host_id: h.jump_host_id ?? '',
  env_tag: h.env_tag,
  tab_color: h.tab_color,
  is_temporary: h.is_temporary,
});

interface HostFormProps {
  initial: HostDraft;
  hosts: Host[];
  keys: SSHKey[];
  /** Число работающих туннелей хоста: их переподключат при смене адреса или ключа. */
  runningTunnels: number;
  onCancel: () => void;
  onSubmit: (draft: HostDraft) => Promise<void>;
}

export const HostForm: React.FC<HostFormProps> = ({ initial, hosts, keys, runningTunnels, onCancel, onSubmit }) => {
  const [draft, setDraft] = useState<HostDraft>(initial);
  const [portText, setPortText] = useState(String(initial.port));
  const [touched, setTouched] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const isEdit = Boolean(initial.id);

  useEscape(onCancel);

  const set = <K extends keyof HostDraft>(key: K, value: HostDraft[K]) => setDraft((d) => ({ ...d, [key]: value }));

  const setEnv = (env: EnvTag) =>
    setDraft((d) => ({
      ...d,
      env_tag: env,
      // Цвет вкладки следует за окружением, если его не меняли вручную.
      tab_color: d.tab_color === ENV_TAB_COLOR[d.env_tag] ? ENV_TAB_COLOR[env] : d.tab_color,
    }));

  const hostname = draft.hostname.trim();
  const username = draft.username.trim();
  const port = Number(portText);

  const hostnameErr = !hostname ? 'Введите адрес'
    : hostname.startsWith('-') || !HOSTNAME_RE.test(hostname) ? 'Недопустимые символы' : null;
  const usernameErr = !username ? 'Введите пользователя'
    : username.startsWith('-') || USERNAME_BAD_RE.test(username) ? 'Недопустимые символы' : null;
  const portErr = Number.isInteger(port) && port >= 1 && port <= 65535 ? null : '1–65535';
  const invalid = Boolean(hostnameErr || usernameErr || portErr);

  // Нельзя выбрать себя и сервер, который сам ходит через этот хост (цикл).
  const jumpCandidates = useMemo(
    () => hosts.filter((h) => h.id !== draft.id && (!draft.id || h.jump_host_id !== draft.id)),
    [hosts, draft.id]
  );
  const jumpHost = hosts.find((h) => h.id === draft.jump_host_id);
  const selectedKey = keys.find((k) => k.id === draft.key_id);

  const connectionChanged =
    isEdit &&
    (hostname !== initial.hostname || port !== initial.port || username !== initial.username || draft.key_id !== initial.key_id);

  const sshPreview = useMemo(() => {
    const parts = ['ssh'];
    if (port && port !== 22) parts.push(`-p ${port}`);
    if (jumpHost) parts.push(`-J ${jumpHost.username}@${jumpHost.hostname}${jumpHost.port !== 22 ? `:${jumpHost.port}` : ''}`);
    parts.push(`${username || '…'}@${hostname || '…'}`);
    return parts.join(' ');
  }, [hostname, username, port, jumpHost]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setTouched(true);
    if (invalid) return;
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit({
        ...draft,
        name: draft.name.trim() || hostname,
        hostname,
        username,
        port,
      });
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm">
      <div role="dialog" aria-modal="true" aria-labelledby="host-form-title" className="max-h-full w-full max-w-[500px] overflow-y-auto rounded-2xl border border-black/10 bg-white/95 shadow-2xl dark:border-white/10 dark:bg-neutral-900/95">
        <div className="flex items-center justify-between border-b border-black/10 px-5 py-3.5 dark:border-white/10">
          <h2 id="host-form-title" className="text-sm font-semibold">{isEdit ? 'Изменить сервер' : 'Новый сервер'}</h2>
          <button type="button" onClick={onCancel} aria-label="Закрыть" className="rounded-lg p-1.5 text-neutral-400 hover:bg-black/5 dark:hover:bg-white/10">
            <X className="h-4 w-4" />
          </button>
        </div>

        <form onSubmit={handleSubmit} noValidate className="space-y-3.5 p-5">
          <div>
            <label htmlFor="host-name" className={labelClass}>Название</label>
            <input
              id="host-name"
              type="text"
              autoFocus
              value={draft.name}
              onChange={(e) => set('name', e.target.value)}
              placeholder={hostname || 'web-01'}
              className={inputClass(false).replace('font-mono ', '')}
            />
          </div>

          <div className="grid grid-cols-[minmax(0,2fr)_auto_minmax(0,3fr)_5rem] items-start gap-2">
            <div className="min-w-0">
              <label htmlFor="host-username" className={labelClass}>Пользователь</label>
              <input
                id="host-username"
                type="text"
                autoComplete="off"
                spellCheck={false}
                value={draft.username}
                onChange={(e) => set('username', e.target.value)}
                aria-invalid={touched && Boolean(usernameErr)}
                className={inputClass(touched && Boolean(usernameErr))}
              />
              {touched && usernameErr && <p className="mt-1 text-[11px] text-red-600 dark:text-red-400">{usernameErr}</p>}
            </div>
            <span className="mt-7 font-mono text-sm text-neutral-400" aria-hidden>@</span>
            <div className="min-w-0">
              <label htmlFor="host-hostname" className={labelClass}>Адрес</label>
              <input
                id="host-hostname"
                type="text"
                inputMode="url"
                autoComplete="off"
                spellCheck={false}
                value={draft.hostname}
                onChange={(e) => set('hostname', e.target.value)}
                placeholder="10.0.0.5"
                aria-invalid={touched && Boolean(hostnameErr)}
                className={inputClass(touched && Boolean(hostnameErr))}
              />
              {touched && hostnameErr && <p className="mt-1 text-[11px] text-red-600 dark:text-red-400">{hostnameErr}</p>}
            </div>
            <div className="min-w-0">
              <label htmlFor="host-port" className={labelClass}>Порт</label>
              <input
                id="host-port"
                type="text"
                inputMode="numeric"
                value={portText}
                onChange={(e) => setPortText(e.target.value.replace(/\D/g, '').slice(0, 5))}
                aria-invalid={touched && Boolean(portErr)}
                className={inputClass(touched && Boolean(portErr))}
              />
              {touched && portErr && <p className="mt-1 text-[11px] text-red-600 dark:text-red-400">{portErr}</p>}
            </div>
          </div>

          <div>
            <span className={labelClass}>Окружение</span>
            <div role="radiogroup" aria-label="Окружение" className="grid grid-cols-4 gap-1 rounded-lg bg-black/5 p-1 dark:bg-white/5">
              {ENV_OPTIONS.map((o) => (
                <button
                  key={o.value}
                  type="button"
                  role="radio"
                  aria-checked={draft.env_tag === o.value}
                  onClick={() => setEnv(o.value)}
                  className={`flex items-center justify-center gap-1.5 rounded-md px-2 py-1.5 text-xs font-medium transition ${
                    draft.env_tag === o.value
                      ? 'bg-white text-neutral-900 shadow-sm dark:bg-neutral-700 dark:text-white'
                      : 'text-neutral-600 hover:text-neutral-900 dark:text-neutral-400 dark:hover:text-white'
                  }`}
                >
                  <span className="h-2 w-2 rounded-full" style={{ backgroundColor: ENV_TAB_COLOR[o.value] }} />
                  {o.label}
                </button>
              ))}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-2.5">
            <div className="min-w-0">
              <label htmlFor="host-key" className={labelClass}>SSH-ключ</label>
              <select id="host-key" value={draft.key_id} onChange={(e) => set('key_id', e.target.value)} className={selectClass}>
                <option value="">Без ключа</option>
                {keys.map((k) => <option key={k.id} value={k.id}>{k.name}{k.is_default ? ' (по умолчанию)' : ''}</option>)}
              </select>
            </div>
            <div className="min-w-0">
              <label htmlFor="host-jump" className={labelClass}>Через jump-хост</label>
              <select id="host-jump" value={draft.jump_host_id} onChange={(e) => set('jump_host_id', e.target.value)} className={selectClass}>
                <option value="">Напрямую</option>
                {jumpCandidates.map((h) => <option key={h.id} value={h.id}>{h.name}</option>)}
              </select>
            </div>
          </div>

          {!selectedKey && (
            <div className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-800 dark:text-amber-200">
              <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
              Без ключа терминал войдёт через ssh-agent или по паролю, но туннели и SFTP работать не будут.
            </div>
          )}

          <div className="rounded-lg border border-black/10 bg-black/[0.03] px-3 py-2 dark:border-white/10 dark:bg-white/5">
            <div className="mb-0.5 text-[10px] font-medium uppercase tracking-wider text-neutral-500">Эквивалент в OpenSSH</div>
            <code className="block break-all font-mono text-xs text-neutral-700 dark:text-neutral-300">{sshPreview}</code>
          </div>

          <label className="flex cursor-pointer items-center gap-2 text-xs">
            <input type="checkbox" checked={draft.is_temporary} onChange={(e) => set('is_temporary', e.target.checked)} />
            Временный — показывать во «Временных» и не экспортировать в ~/.ssh/config
          </label>

          {connectionChanged && runningTunnels > 0 && (
            <p className="text-[11px] text-neutral-500">
              Работающие туннели ({runningTunnels}) будут переподключены с новыми параметрами. Открытые терминалы не затрагиваются.
            </p>
          )}
          {error && <div role="alert" className="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-700 dark:text-red-300">{error}</div>}

          <div className="flex justify-end gap-2 pt-1">
            <button type="button" onClick={onCancel} className="rounded-lg px-3.5 py-1.5 text-xs font-medium hover:bg-black/5 dark:hover:bg-white/10">Отмена</button>
            <button type="submit" disabled={submitting || (touched && invalid)} className="flex items-center gap-1.5 rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-medium text-white hover:bg-blue-500 disabled:opacity-50">
              {submitting && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
              {isEdit ? 'Сохранить' : 'Добавить'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
