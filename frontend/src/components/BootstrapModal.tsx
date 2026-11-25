import React, { useEffect, useState } from 'react';
import {
  AlertTriangle, CheckCircle2, Eye, EyeOff, KeyRound, Loader2, ShieldAlert, Terminal, X,
} from 'lucide-react';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { CancelBootstrap, ResolveHostKey, RunBootstrap } from '../../wailsjs/go/app/App';
import { EnvTag, ParsedTarget, SSHKey } from '../types/ssh';

type BootstrapStep = 'connect_password' | 'deploy_key' | 'verify_key' | 'done';

interface HostKeyInfo {
  hostname: string;
  remote_addr: string;
  key_type: string;
  fingerprint: string;
}

interface BootstrapModalProps {
  isOpen: boolean;
  keys: SSHKey[];
  initialTarget?: ParsedTarget | null;
  initialKeyId?: string;
  onClose: () => void;
  onSaveAndConnect: (data: {
    name: string;
    hostname: string;
    port: number;
    username: string;
    key_id: string;
    env_tag: EnvTag;
  }) => void;
}

const STEPS: { id: BootstrapStep; label: string }[] = [
  { id: 'connect_password', label: 'Подключение и проверка отпечатка хоста' },
  { id: 'deploy_key', label: 'Установка ключа в ~/.ssh/authorized_keys' },
  { id: 'verify_key', label: 'Верификация входа без пароля' },
  { id: 'done', label: 'Готово к работе' },
];

const ENV_OPTIONS: { value: EnvTag; label: string; color: string }[] = [
  { value: 'dev', label: 'Dev', color: 'bg-[#107C10]' },
  { value: 'stage', label: 'Stage', color: 'bg-[#FFB900]' },
  { value: 'prod', label: 'Prod', color: 'bg-[#E81123]' },
  { value: 'infra', label: 'Infra', color: 'bg-[#0078D4]' },
];

export const BootstrapModal: React.FC<BootstrapModalProps> = ({
  isOpen, keys, initialTarget, initialKeyId, onClose, onSaveAndConnect,
}) => {
  const [hostname, setHostname] = useState(initialTarget?.hostname ?? '');
  const [port, setPort] = useState(initialTarget?.port ?? 22);
  const [username, setUsername] = useState(initialTarget?.username ?? 'root');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [keyId, setKeyId] = useState(initialKeyId || keys.find((k) => k.is_default)?.id || keys[0]?.id || '');

  const [phase, setPhase] = useState<'form' | 'running' | 'success'>('form');
  const [currentStep, setCurrentStep] = useState<BootstrapStep | null>(null);
  const [statusMessage, setStatusMessage] = useState('');
  const [pendingHostKey, setPendingHostKey] = useState<HostKeyInfo | null>(null);
  const [error, setError] = useState<string | null>(null);

  const [serverName, setServerName] = useState('');
  const [envTag, setEnvTag] = useState<EnvTag>('dev');

  // Сбрасываем форму только при открытии окна (а не при каждом обновлении
  // списка ключей), чтобы не потерять введённые данные.
  useEffect(() => {
    if (!isOpen) return;
    setHostname(initialTarget?.hostname ?? '');
    setPort(initialTarget?.port ?? 22);
    setUsername(initialTarget?.username ?? 'root');
    setServerName(initialTarget?.hostname ?? '');
    setKeyId(initialKeyId || keys.find((k) => k.is_default)?.id || keys[0]?.id || '');
    setPassword('');
    setPhase('form');
    setCurrentStep(null);
    setPendingHostKey(null);
    setError(null);
  }, [isOpen, initialTarget, initialKeyId]);

  useEffect(() => {
    if (!isOpen) return;
    const offProgress = EventsOn('bootstrap:progress', (payload: { step: BootstrapStep; message: string }) => {
      setCurrentStep(payload.step);
      setStatusMessage(payload.message);
    });
    const offHostKey = EventsOn('bootstrap:confirm_host_key', (info: HostKeyInfo) => {
      setPendingHostKey(info);
    });
    return () => { offProgress(); offHostKey(); };
  }, [isOpen]);

  if (!isOpen) return null;

  const handleStart = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!hostname.trim() || !password || !keyId) return;
    setError(null);
    setPhase('running');
    setCurrentStep('connect_password');
    setStatusMessage('Инициализация подключения...');
    try {
      await RunBootstrap({
        hostname: hostname.trim(),
        port: Number(port) || 22,
        username: username.trim() || 'root',
        password,
        key_id: keyId,
      });
      setPassword('');
      setCurrentStep('done');
      setPhase('success');
      if (!serverName) setServerName(hostname.trim());
    } catch (err: unknown) {
      setPassword('');
      setPendingHostKey(null);
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  const handleHostKeyDecision = (approved: boolean) => {
    setPendingHostKey(null);
    ResolveHostKey(approved);
  };

  const handleClose = () => {
    if (pendingHostKey) ResolveHostKey(false);
    if (phase === 'running') CancelBootstrap();
    setPassword('');
    onClose();
  };

  const currentStepIdx = currentStep ? STEPS.findIndex((s) => s.id === currentStep) : -1;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm">
      <div className="w-full max-w-[480px] overflow-hidden rounded-2xl border border-black/10 bg-white/95 shadow-2xl backdrop-blur-xl dark:border-white/10 dark:bg-neutral-900/95">
        <div className="flex items-center justify-between border-b border-black/10 px-5 py-3.5 dark:border-white/10">
          <div>
            <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">One-Click Bootstrap</h2>
            <p className="text-xs text-neutral-500">Настройка входа по SSH-ключу на новом сервере</p>
          </div>
          <button type="button" onClick={handleClose} className="rounded-lg p-1.5 text-neutral-400 hover:bg-black/5 dark:hover:bg-white/10">
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="p-5">
          {phase === 'form' && (
            <form onSubmit={handleStart} className="space-y-3.5">
              <div className="grid grid-cols-12 gap-2.5">
                <div className="col-span-8">
                  <label className="mb-1 block text-xs font-medium text-neutral-600 dark:text-neutral-400">Хост / IP</label>
                  <input type="text" required value={hostname} onChange={(e) => setHostname(e.target.value)} placeholder="185.12.34.56" className="w-full rounded-lg border border-black/10 bg-black/[0.03] px-3 py-1.5 font-mono text-sm outline-none focus:border-blue-500 dark:border-white/10 dark:bg-white/5 dark:text-white" />
                </div>
                <div className="col-span-4">
                  <label className="mb-1 block text-xs font-medium text-neutral-600 dark:text-neutral-400">Порт</label>
                  <input type="number" min={1} max={65535} value={port} onChange={(e) => setPort(Number(e.target.value))} className="w-full rounded-lg border border-black/10 bg-black/[0.03] px-3 py-1.5 font-mono text-sm outline-none focus:border-blue-500 dark:border-white/10 dark:bg-white/5 dark:text-white" />
                </div>
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-neutral-600 dark:text-neutral-400">Пользователь</label>
                <input type="text" required value={username} onChange={(e) => setUsername(e.target.value)} className="w-full rounded-lg border border-black/10 bg-black/[0.03] px-3 py-1.5 font-mono text-sm outline-none focus:border-blue-500 dark:border-white/10 dark:bg-white/5 dark:text-white" />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-neutral-600 dark:text-neutral-400">Временный пароль</label>
                <div className="relative">
                  <input type={showPassword ? 'text' : 'password'} required autoFocus value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Пароль от VPS" className="w-full rounded-lg border border-black/10 bg-black/[0.03] py-1.5 pl-3 pr-9 text-sm outline-none focus:border-blue-500 dark:border-white/10 dark:bg-white/5 dark:text-white" />
                  <button type="button" onClick={() => setShowPassword((v) => !v)} className="absolute right-2.5 top-1/2 -translate-y-1/2 text-neutral-400">
                    {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
                <p className="mt-1 text-[11px] text-neutral-500">Не сохраняется на диск и стирается из памяти сразу после установки ключа.</p>
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-neutral-600 dark:text-neutral-400">SSH-ключ для установки</label>
                <div className="relative flex items-center">
                  <KeyRound className="pointer-events-none absolute left-3 h-3.5 w-3.5 text-amber-500" />
                  <select value={keyId} onChange={(e) => setKeyId(e.target.value)} className="w-full rounded-lg border border-black/10 bg-black/[0.03] py-1.5 pl-8 pr-3 text-sm outline-none focus:border-blue-500 dark:border-white/10 dark:bg-neutral-800 dark:text-white">
                    {keys.map((k) => <option key={k.id} value={k.id}>{k.name} ({k.key_type})</option>)}
                  </select>
                </div>
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button type="button" onClick={handleClose} className="rounded-lg px-3.5 py-1.5 text-xs font-medium text-neutral-600 hover:bg-black/5 dark:text-neutral-300 dark:hover:bg-white/10">Отмена</button>
                <button type="submit" disabled={!keyId} className="rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-medium text-white hover:bg-blue-500 disabled:opacity-50">Настроить и проверить</button>
              </div>
            </form>
          )}

          {phase !== 'form' && (
            <div className="space-y-4">
              <div className="rounded-lg bg-black/5 px-3 py-2 font-mono text-xs text-neutral-700 dark:bg-white/5 dark:text-neutral-300">
                {username}@{hostname}:{port}
              </div>
              <ol className="space-y-2.5">
                {STEPS.map((step, idx) => {
                  const isDone = phase === 'success' || (currentStepIdx > idx && !error);
                  const isActive = phase === 'running' && currentStepIdx === idx && !error;
                  const isFailed = Boolean(error) && currentStepIdx === idx;
                  return (
                    <li key={step.id} className="space-y-2">
                      <div className="flex items-center gap-2.5 text-sm">
                        {isDone ? <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-500" /> : isFailed ? <AlertTriangle className="h-4 w-4 shrink-0 text-red-500" /> : isActive ? <Loader2 className="h-4 w-4 shrink-0 animate-spin text-blue-500" /> : <span className="h-4 w-4 shrink-0 rounded-full border border-neutral-300 dark:border-neutral-700" />}
                        <span className={isDone || isActive ? 'text-neutral-900 dark:text-neutral-100' : 'text-neutral-400'}>{step.label}</span>
                      </div>
                      {step.id === 'connect_password' && pendingHostKey && (
                        <div className="ml-6 rounded-xl border border-amber-500/30 bg-amber-500/10 p-3">
                          <div className="flex items-center gap-2 text-xs font-semibold text-amber-800 dark:text-amber-300">
                            <ShieldAlert className="h-4 w-4" /> Подтвердите отпечаток нового хоста
                          </div>
                          <div className="mt-2 space-y-1 font-mono text-[11px] text-neutral-700 dark:text-neutral-300">
                            <div>Тип: {pendingHostKey.key_type}</div>
                            <div className="break-all rounded bg-black/10 p-1.5 dark:bg-black/30">{pendingHostKey.fingerprint}</div>
                          </div>
                          <div className="mt-3 flex justify-end gap-2">
                            <button type="button" onClick={() => handleHostKeyDecision(false)} className="rounded-md px-2.5 py-1 text-xs font-medium text-neutral-600 hover:bg-black/10 dark:text-neutral-300">Отмена</button>
                            <button type="button" onClick={() => handleHostKeyDecision(true)} className="rounded-md bg-amber-600 px-3 py-1 text-xs font-medium text-white hover:bg-amber-500">Доверять этому хосту</button>
                          </div>
                        </div>
                      )}
                    </li>
                  );
                })}
              </ol>

              {phase === 'running' && !error && statusMessage && <p className="text-xs text-neutral-500">{statusMessage}</p>}

              {error && (
                <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-3 text-xs text-red-700 dark:text-red-300">
                  <div className="font-semibold">Ошибка настройки</div>
                  <div className="mt-1 break-words font-mono text-[11px]">{error}</div>
                  <div className="mt-2.5 flex justify-end">
                    <button type="button" onClick={() => { setError(null); setPhase('form'); }} className="rounded-md bg-red-600 px-3 py-1 text-xs font-medium text-white hover:bg-red-500">Попробовать снова</button>
                  </div>
                </div>
              )}

              {phase === 'success' && (
                <div className="space-y-3 border-t border-black/10 pt-3.5 dark:border-white/10">
                  <div>
                    <label className="mb-1 block text-xs font-medium text-neutral-600 dark:text-neutral-400">Имя сервера</label>
                    <input type="text" value={serverName} onChange={(e) => setServerName(e.target.value)} className="w-full rounded-lg border border-black/10 bg-black/[0.03] px-3 py-1.5 text-sm outline-none focus:border-blue-500 dark:border-white/10 dark:bg-white/5 dark:text-white" />
                  </div>
                  <div className="grid grid-cols-4 gap-1.5">
                    {ENV_OPTIONS.map((opt) => (
                      <button key={opt.value} type="button" onClick={() => setEnvTag(opt.value)} className={`flex items-center justify-center gap-1.5 rounded-lg border py-1.5 text-xs font-medium ${envTag === opt.value ? 'border-blue-500 bg-blue-500/10 text-blue-700 dark:text-blue-300' : 'border-black/10 dark:border-white/10'}`}>
                        <span className={`h-2 w-2 rounded-full ${opt.color}`} /> {opt.label}
                      </button>
                    ))}
                  </div>
                  <button type="button" onClick={() => onSaveAndConnect({ name: serverName.trim() || hostname.trim(), hostname: hostname.trim(), port: Number(port) || 22, username: username.trim() || 'root', key_id: keyId, env_tag: envTag })} className="flex w-full items-center justify-center gap-2 rounded-lg bg-emerald-600 py-2 text-xs font-semibold text-white hover:bg-emerald-500">
                    <Terminal className="h-4 w-4" /> Сохранить и открыть в терминале
                  </button>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
