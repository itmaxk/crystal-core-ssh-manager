import React, { useEffect, useMemo, useState } from 'react';
import { Check, Copy, KeyRound, Lock, Server, ShieldCheck, UserPlus, X } from 'lucide-react';
import {
  GetAgentAccess, GrantAgentAccess, RevokeAgentAccess, SetupAgentUser,
} from '../../wailsjs/go/app/App';
import { app } from '../../wailsjs/go/models';
import { Host } from '../types/ssh';

interface Props {
  hosts: Host[];
}

const SHELL_MODE = 'shell';
const TUNNEL_MODE = 'tunnel';

export const AgentAccessPanel: React.FC<Props> = ({ hosts }) => {
  const [state, setState] = useState<app.AgentAccessState | null>(null);
  const [open, setOpen] = useState(false);
  const [username, setUsername] = useState('cc-agent');
  const [closeHome, setCloseHome] = useState(false);
  const [setup, setSetup] = useState<app.AgentSetupResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

  const [hostID, setHostID] = useState('');
  const [mode, setMode] = useState(SHELL_MODE);
  const [sourceIP, setSourceIP] = useState('');
  const [permitOpen, setPermitOpen] = useState('127.0.0.1:5432');

  const keyHosts = useMemo(() => hosts.filter((h) => h.key_id), [hosts]);
  const selectedHostID = hostID || keyHosts[0]?.id || '';

  const load = async () => {
    try {
      const st = await GetAgentAccess();
      setState(st);
      if (st.username) setUsername(st.username);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  useEffect(() => { load(); }, []);

  useEffect(() => {
    if (!hostID && keyHosts.length > 0) setHostID(keyHosts[0].id);
  }, [keyHosts, hostID]);

  const copy = async (text: string, tag: string) => {
    await navigator.clipboard.writeText(text);
    setCopied(tag);
    setTimeout(() => setCopied((c) => (c === tag ? null : c)), 2000);
  };

  const runSetup = async () => {
    setBusy(true); setError(null); setNotice(null); setSetup(null);
    try {
      const res = await SetupAgentUser({ username: username.trim(), close_home: closeHome });
      setSetup(res);
      setState(res.state);
      if (!res.needs_elevation) {
        setNotice('Пользователь и ключ готовы');
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const checkAgain = async () => {
    setBusy(true); setError(null);
    try {
      await load();
      const st = await GetAgentAccess();
      if (st.user_exists) {
        setSetup(null);
        setNotice('Пользователь найден, можно выдавать доступ');
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const grant = async () => {
    const targetID = selectedHostID;
    if (!targetID) { setError('Нет сервера с SSH-ключом'); return; }
    setBusy(true); setError(null); setNotice(null);
    try {
      const g = await GrantAgentAccess({
        host_id: targetID,
        mode,
        source_ip: sourceIP.trim(),
        permit_open: mode === TUNNEL_MODE ? permitOpen.trim() : '',
      });
      setNotice(`Доступ выдан: ${g.host_name}`);
      await load();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const revoke = async (id: string) => {
    setBusy(true); setError(null); setNotice(null);
    try {
      await RevokeAgentAccess(id);
      setNotice('Доступ отозван');
      await load();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const ready = Boolean(state?.username && state?.user_exists);

  return (
    <>
      <div className="rounded-xl border border-black/10 bg-white/65 p-4 backdrop-blur-md dark:border-white/10 dark:bg-neutral-900/60">
        <div className="flex items-center gap-2 text-sm font-medium">
          <ShieldCheck className="h-4 w-4 text-emerald-500" /> Доступ для ИИ-агентов
        </div>
        <p className="mt-1 text-xs text-neutral-500">
          Отдельный пользователь Linux без sudo и docker, свой ключ Ed25519 и доступ только к выбранным серверам.
          Сам агент запускается под этим пользователем и не видит ваше хранилище ключей.
        </p>

        {!state?.supported ? (
          <p className="mt-3 text-xs text-neutral-500">Настройка доступна только в Linux.</p>
        ) : state?.username ? (
          <div className="mt-3 space-y-2">
            <div className="flex items-center gap-2 text-xs">
              <UserPlus className="h-3.5 w-3.5" />
              <span className="font-mono">{state.username}</span>
              <span className={`rounded px-1.5 py-0.5 text-[10px] ${state.user_exists ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300' : 'bg-red-500/15 text-red-700 dark:text-red-300'}`}>
                {state.user_exists ? 'создан' : 'не найден'}
              </span>
            </div>
            {state.public_key && (
              <div className="flex items-start gap-2">
                <code className="min-w-0 flex-1 truncate rounded bg-black/5 px-2 py-1 font-mono text-[10px] dark:bg-white/10">{state.public_key}</code>
                <button type="button" onClick={() => copy(state.public_key, 'pub')} className="shrink-0 rounded-lg border border-black/10 px-2 py-1 text-[11px] dark:border-white/10">
                  {copied === 'pub' ? <Check className="h-3 w-3 text-emerald-500" /> : <Copy className="h-3 w-3" />}
                </button>
              </div>
            )}
            <div className="flex flex-wrap items-center gap-2">
              <button type="button" onClick={() => { setOpen(true); setSetup(null); }} className="rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-500">
                Выдать доступ
              </button>
              <button type="button" onClick={() => { setOpen(true); setSetup(null); }} className="rounded-lg border border-black/10 px-3 py-1.5 text-xs font-medium hover:bg-black/5 dark:border-white/10 dark:hover:bg-white/10">
                Настроить заново
              </button>
            </div>
            {state.grants.length > 0 && (
              <div className="space-y-1.5 pt-1">
                {state.grants.map((g) => (
                  <div key={g.host_id} className="rounded-lg border border-black/10 px-2.5 py-2 text-[11px] dark:border-white/10">
                    <div className="flex items-center justify-between gap-2">
                      <span className="font-medium">{g.host_name}</span>
                      <button type="button" onClick={() => revoke(g.host_id)} disabled={busy} className="text-red-600 hover:underline disabled:opacity-50">Отозвать</button>
                    </div>
                    <div className="mt-1 flex items-center gap-2 text-neutral-500">
                      <span>{g.mode === TUNNEL_MODE ? `туннель ${g.permit_open}` : 'shell'}</span>
                      {g.source_ip && <span>· from {g.source_ip}</span>}
                      <button type="button" onClick={() => copy(g.ssh_command, `cmd-${g.host_id}`)} className="ml-auto text-blue-600 hover:underline">
                        {copied === `cmd-${g.host_id}` ? 'скопировано' : 'команда'}
                      </button>
                    </div>
                    <code className="mt-1 block truncate font-mono text-[10px] text-neutral-500">{g.ssh_command}</code>
                  </div>
                ))}
              </div>
            )}
          </div>
        ) : (
          <button type="button" onClick={() => setOpen(true)} className="mt-3 rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-500">
            Настроить агента
          </button>
        )}
      </div>

      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm">
          <div className="max-h-[90vh] w-full max-w-[560px] overflow-y-auto rounded-2xl border border-black/10 bg-white/95 shadow-2xl dark:border-white/10 dark:bg-neutral-900/95">
            <div className="sticky top-0 flex items-center justify-between border-b border-black/10 bg-inherit px-5 py-3.5 dark:border-white/10">
              <h2 className="text-sm font-semibold">Доступ для ИИ-агента</h2>
              <button type="button" onClick={() => setOpen(false)} className="rounded-lg p-1.5 text-neutral-400 hover:bg-black/5"><X className="h-4 w-4" /></button>
            </div>

            <div className="space-y-4 p-5">
              {error && <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-700 dark:text-red-300">{error}</div>}
              {notice && <div className="rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3 py-2 text-xs text-emerald-700 dark:text-emerald-300">{notice}</div>}

              <section>
                <h3 className="mb-2 flex items-center gap-1.5 text-xs font-semibold"><Lock className="h-3.5 w-3.5" /> 1. Локальный пользователь</h3>
                {!ready ? (
                  <div className="space-y-2.5">
                    <div>
                      <label className="mb-1 block text-xs text-neutral-500">Имя пользователя</label>
                      <input value={username} onChange={(e) => setUsername(e.target.value)} className="w-full rounded-lg border border-black/10 bg-black/[0.03] px-3 py-1.5 font-mono text-sm outline-none focus:border-blue-500 dark:border-white/10 dark:bg-white/5 dark:text-white" />
                    </div>
                    <label className="flex items-start gap-2 text-xs text-neutral-600 dark:text-neutral-300">
                      <input type="checkbox" checked={closeHome} onChange={(e) => setCloseHome(e.target.checked)} className="mt-0.5" />
                      Закрыть мой домашний каталог от других пользователей (chmod 750)
                    </label>
                    <button type="button" onClick={runSetup} disabled={busy} className="rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-500 disabled:opacity-50">
                      {busy ? 'Создание…' : 'Создать пользователя и ключ'}
                    </button>
                  </div>
                ) : (
                  <div className="text-xs text-neutral-500">
                    Пользователь <span className="font-mono text-neutral-700 dark:text-neutral-200">{state?.username}</span> готов. Ключ: <span className="font-mono text-[10px]">{state?.home}/.ssh/id_ed25519</span>
                  </div>
                )}

                {setup?.needs_elevation && (
                  <div className="mt-3 space-y-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3">
                    <p className="text-[11px] text-amber-800 dark:text-amber-200">
                      Нужны права root, а графический запрос пароля недоступен. Выполните команду в терминале:
                    </p>
                    <code className="block select-all break-all rounded bg-black/5 p-2 font-mono text-[10px] dark:bg-white/10">{setup.command}</code>
                    <div className="flex gap-2">
                      <button type="button" onClick={() => copy(setup.command || '', 'sudo')} className="rounded-lg border border-black/10 px-2.5 py-1 text-[11px] dark:border-white/10">
                        {copied === 'sudo' ? 'Скопировано' : 'Скопировать команду'}
                      </button>
                      <button type="button" onClick={checkAgain} disabled={busy} className="rounded-lg bg-blue-600 px-2.5 py-1 text-[11px] font-medium text-white hover:bg-blue-500 disabled:opacity-50">
                        Проверить снова
                      </button>
                    </div>
                  </div>
                )}
              </section>

              {ready && (
                <section className="border-t border-black/10 pt-4 dark:border-white/10">
                  <h3 className="mb-2 flex items-center gap-1.5 text-xs font-semibold"><Server className="h-3.5 w-3.5" /> 2. Доступ к серверу</h3>
                  <div className="space-y-2.5">
                    <select value={selectedHostID} onChange={(e) => setHostID(e.target.value)} className="w-full rounded-lg border border-black/10 bg-black/[0.03] px-3 py-1.5 text-sm dark:border-white/10 dark:bg-white/5 dark:text-white">
                      {keyHosts.length === 0 && <option value="">Нет серверов с ключом</option>}
                      {keyHosts.map((h) => <option key={h.id} value={h.id}>{h.name} — {h.username}@{h.hostname}</option>)}
                    </select>

                    <div className="flex gap-3 text-xs">
                      <label className="flex items-center gap-1.5"><input type="radio" checked={mode === SHELL_MODE} onChange={() => setMode(SHELL_MODE)} /> Shell (работа на сервере)</label>
                      <label className="flex items-center gap-1.5"><input type="radio" checked={mode === TUNNEL_MODE} onChange={() => setMode(TUNNEL_MODE)} /> Только туннель</label>
                    </div>

                    {mode === TUNNEL_MODE && (
                      <div>
                        <label className="mb-1 block text-xs text-neutral-500">Разрешённый адрес для -L (host:port)</label>
                        <input value={permitOpen} onChange={(e) => setPermitOpen(e.target.value)} className="w-full rounded-lg border border-black/10 bg-black/[0.03] px-3 py-1.5 font-mono text-xs dark:border-white/10 dark:bg-white/5 dark:text-white" />
                      </div>
                    )}

                    <div>
                      <label className="mb-1 block text-xs text-neutral-500">Ограничить по IP агента (необязательно)</label>
                      <input value={sourceIP} onChange={(e) => setSourceIP(e.target.value)} placeholder="203.0.113.10 или 10.0.0.0/8" className="w-full rounded-lg border border-black/10 bg-black/[0.03] px-3 py-1.5 font-mono text-xs dark:border-white/10 dark:bg-white/5 dark:text-white" />
                    </div>

                    <p className="text-[11px] text-neutral-500">
                      {mode === SHELL_MODE
                        ? 'Ключ разрешит shell, но запретит проброс портов и agent forwarding. Пользователь на сервере — тот, что указан у хоста.'
                        : 'Ключ разрешит только проброс указанного адреса; shell, pty и agent forwarding будут запрещены.'}
                    </p>

                    <button type="button" onClick={grant} disabled={busy || !selectedHostID} className="rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-500 disabled:opacity-50">
                      {busy ? 'Выдача…' : 'Выдать доступ'}
                    </button>
                  </div>
                </section>
              )}

              <p className="border-t border-black/10 pt-3 text-[11px] leading-relaxed text-neutral-500 dark:border-white/10">
                Агент запускается под пользователем <span className="font-mono">{state?.username || username}</span> и работает со своим ключом.
                Ваш ssh-agent, keyring и <span className="font-mono">~/.config/CrystalCoreSSHManager</span> ему недоступны, если каталог закрыт.
                Отзыв доступа удаляет строку из <span className="font-mono">authorized_keys</span> на сервере.
              </p>
            </div>
          </div>
        </div>
      )}
    </>
  );
};
