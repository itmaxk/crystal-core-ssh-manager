import React, { useEffect, useMemo, useRef, useState } from 'react';
import { KeyRound, Search, Sparkles, Terminal } from 'lucide-react';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { Host, ParsedTarget, SSHKey } from '../types/ssh';
import { parseSmartPaste } from '../utils/smartPaste';

interface OmnibarProps {
  hosts: Host[];
  keys: SSHKey[];
  onConnectHost: (hostId: string, mode: 'tabs' | 'split') => void;
  onQuickConnect: (target: ParsedTarget, keyId: string, mode: 'tabs' | 'split') => void;
  onStartBootstrap: (target: ParsedTarget, keyId: string) => void;
}

export const Omnibar: React.FC<OmnibarProps> = ({
  hosts,
  keys,
  onConnectHost,
  onQuickConnect,
  onStartBootstrap,
}) => {
  const inputRef = useRef<HTMLInputElement>(null);
  const [query, setQuery] = useState('');
  const [isOpen, setIsOpen] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(0);

  const defaultKey = useMemo(() => keys.find((k) => k.is_default) ?? keys[0], [keys]);
  const [selectedKeyId, setSelectedKeyId] = useState<string>(defaultKey?.id ?? '');

  useEffect(() => {
    if (!selectedKeyId && defaultKey) setSelectedKeyId(defaultKey.id);
  }, [defaultKey, selectedKeyId]);

  // Глобальный хоткей / трей / второй запуск приложения фокусируют Omnibar.
  useEffect(() => {
    const off = EventsOn('ui:focus_omnibar', () => {
      inputRef.current?.focus();
      inputRef.current?.select();
      setIsOpen(true);
    });
    return off;
  }, []);

  const parsedTarget = useMemo(() => parseSmartPaste(query), [query]);

  const filteredHosts = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return hosts.slice(0, 5);
    return hosts
      .filter(
        (h) =>
          h.name.toLowerCase().includes(q) ||
          h.hostname.toLowerCase().includes(q) ||
          h.env_tag.toLowerCase().includes(q)
      )
      .slice(0, 6);
  }, [hosts, query]);

  const cycleKey = () => {
    if (keys.length < 2) return;
    const idx = keys.findIndex((k) => k.id === selectedKeyId);
    setSelectedKeyId(keys[(idx + 1) % keys.length].id);
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Tab') {
      e.preventDefault();
      cycleKey();
      return;
    }
    if (e.key === 'Escape') {
      if (query) setQuery('');
      else {
        setIsOpen(false);
        inputRef.current?.blur();
      }
      return;
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setSelectedIndex((prev) => Math.min(prev + 1, Math.max(filteredHosts.length - 1, 0)));
      return;
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      setSelectedIndex((prev) => Math.max(prev - 1, 0));
      return;
    }
    if (e.key === 'Enter') {
      e.preventDefault();
      const mode = e.shiftKey ? 'split' : 'tabs';
      if (e.ctrlKey && parsedTarget) {
        onStartBootstrap(parsedTarget, selectedKeyId);
        setIsOpen(false);
        return;
      }
      if (parsedTarget && filteredHosts.length === 0) {
        onQuickConnect(parsedTarget, selectedKeyId, mode);
        setQuery('');
        setIsOpen(false);
        return;
      }
      const chosen = filteredHosts[selectedIndex];
      if (chosen) {
        onConnectHost(chosen.id, mode);
        setQuery('');
        setIsOpen(false);
      }
    }
  };

  const activeKeyName = keys.find((k) => k.id === selectedKeyId)?.name ?? 'Без ключа';

  return (
    <div className="relative w-full max-w-xl">
      <div className="flex items-center gap-2 rounded-lg border border-black/10 bg-white/70 px-3 py-1.5 shadow-sm backdrop-blur-md transition focus-within:border-blue-500/60 focus-within:ring-2 focus-within:ring-blue-500/20 dark:border-white/10 dark:bg-neutral-900/70">
        <Search className="h-4 w-4 shrink-0 text-neutral-500" />
        <input
          ref={inputRef}
          type="text"
          value={query}
          onFocus={() => setIsOpen(true)}
          onBlur={() => setTimeout(() => setIsOpen(false), 150)}
          onChange={(e) => {
            setQuery(e.target.value);
            setSelectedIndex(0);
          }}
          onKeyDown={handleKeyDown}
          placeholder="Поиск сервера или вставка IP / ssh -p 2222 user@host..."
          className="w-full bg-transparent text-sm text-neutral-900 placeholder-neutral-400 outline-none dark:text-neutral-100"
        />
        <button
          type="button"
          onClick={cycleKey}
          title="Переключить ключ (Tab)"
          className="flex shrink-0 items-center gap-1.5 rounded-md border border-black/10 bg-black/5 px-2 py-0.5 text-xs font-medium text-neutral-700 hover:bg-black/10 dark:border-white/10 dark:bg-white/5 dark:text-neutral-200 dark:hover:bg-white/10"
        >
          <KeyRound className="h-3 w-3 text-amber-500" />
          <span className="max-w-[96px] truncate">{activeKeyName}</span>
        </button>
      </div>

      {isOpen && (
        <div className="absolute left-0 right-0 top-full z-50 mt-1.5 overflow-hidden rounded-xl border border-black/10 bg-white/90 p-1.5 shadow-xl backdrop-blur-xl dark:border-white/10 dark:bg-neutral-900/90">
          {parsedTarget && (
            <div className="mb-1.5 rounded-lg border border-blue-500/25 bg-blue-500/10 p-2.5">
              <div className="flex items-center justify-between gap-2">
                <div className="flex flex-wrap items-center gap-1.5 font-mono text-xs">
                  <span className="rounded bg-blue-500/20 px-1.5 py-0.5 text-blue-700 dark:text-blue-300">
                    {parsedTarget.username}
                  </span>
                  <span className="text-neutral-400">@</span>
                  <span className="font-semibold text-neutral-900 dark:text-white">
                    {parsedTarget.hostname}
                  </span>
                  <span className="text-neutral-400">:</span>
                  <span className="rounded bg-black/10 px-1.5 py-0.5 dark:bg-white/10">
                    {parsedTarget.port}
                  </span>
                </div>
                <div className="flex items-center gap-1.5">
                  <button
                    type="button"
                    onMouseDown={() => onStartBootstrap(parsedTarget, selectedKeyId)}
                    className="flex items-center gap-1 rounded-md bg-amber-500/15 px-2 py-1 text-xs font-medium text-amber-700 hover:bg-amber-500/25 dark:text-amber-300"
                  >
                    <Sparkles className="h-3.5 w-3.5" />
                    Bootstrap
                  </button>
                  <button
                    type="button"
                    onMouseDown={() => {
                      onQuickConnect(parsedTarget, selectedKeyId, 'tabs');
                      setQuery('');
                    }}
                    className="flex items-center gap-1 rounded-md bg-blue-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-blue-500"
                  >
                    <Terminal className="h-3.5 w-3.5" />
                    Войти
                  </button>
                </div>
              </div>
              <div className="mt-1.5 text-[11px] text-neutral-500 dark:text-neutral-400">
                Enter — войти с ключом «{activeKeyName}» · Ctrl+Enter — One-Click Bootstrap · Tab — сменить ключ
              </div>
            </div>
          )}

          {filteredHosts.length > 0 ? (
            <ul className="space-y-0.5">
              {filteredHosts.map((host, idx) => (
                <li key={host.id}>
                  <button
                    type="button"
                    onMouseDown={() => onConnectHost(host.id, 'tabs')}
                    onMouseEnter={() => setSelectedIndex(idx)}
                    className={`flex w-full items-center justify-between rounded-lg px-2.5 py-2 text-left text-sm transition ${
                      idx === selectedIndex
                        ? 'bg-black/10 dark:bg-white/10'
                        : 'hover:bg-black/5 dark:hover:bg-white/5'
                    }`}
                  >
                    <div className="flex items-center gap-2.5">
                      <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: host.tab_color }} />
                      <div>
                        <div className="font-medium text-neutral-900 dark:text-neutral-100">{host.name}</div>
                        <div className="font-mono text-xs text-neutral-500">
                          {host.username}@{host.hostname}:{host.port}
                        </div>
                      </div>
                    </div>
                    <span className="rounded bg-black/5 px-1.5 py-0.5 text-[10px] uppercase tracking-wider text-neutral-500 dark:bg-white/10 dark:text-neutral-400">
                      {host.env_tag}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            !parsedTarget && (
              <div className="px-3 py-4 text-center text-xs text-neutral-500">
                Серверы не найдены. Вставьте IP или команду <span className="font-mono">ssh user@host</span>.
              </div>
            )
          )}
        </div>
      )}
    </div>
  );
};
