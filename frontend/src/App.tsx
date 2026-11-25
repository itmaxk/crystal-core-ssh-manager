import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  ArrowLeftRight, Clock, Columns2, KeyRound, Minus, Plus, RefreshCw, Server, Settings, Square, Terminal, X,
} from 'lucide-react';
import {
  EventsOn, OnFileDrop, OnFileDropOff, WindowMinimise, WindowToggleMaximise,
} from '../wailsjs/runtime/runtime';
import {
  ExitApp, HideToTray, PreviewImportSSHConfig, SyncSSHConfig, TrayAvailable,
} from '../wailsjs/go/app/App';
import { errorMessage, useSSHStore } from './store/useSSHStore';
import { CrystalLogo } from './components/CrystalLogo';
import { Omnibar } from './components/Omnibar';
import { HostCard } from './components/HostCard';
import { BootstrapModal } from './components/BootstrapModal';
import { KeyVault } from './components/KeyVault';
import { TunnelManager } from './components/TunnelManager';
import { HostForm, hostToDraft, newHostDraft } from './components/HostForm';
import { BackupPanel } from './components/BackupPanel';
import { CloseToTrayDialog, isCloseHintHidden, setCloseHintHidden } from './components/CloseToTrayDialog';
import { HostDraft, ParsedTarget } from './types/ssh';

type Section = 'hosts' | 'temporary' | 'tunnels' | 'keys' | 'settings';

const DRAG_STYLE = { '--wails-draggable': 'drag' } as React.CSSProperties;
const NO_DRAG_STYLE = { '--wails-draggable': 'no-drag' } as React.CSSProperties;

export function App() {
  const {
    hosts, keys, pings, sessions, tunnelsByHost, uploads, selectedHostIds, agentAvailable, envFilter,
    loadInitial, refreshPings, setEnvFilter, toggleSelectHost, clearSelection,
    connectHosts, quickConnect, saveAndConnectHost, saveHost, pinHost, deleteHost, uploadFiles,
    saveTunnel, deleteTunnel, startTunnel, stopTunnel, openRDP, createTunnels,
    notice, notify, dismissNotice,
    onPingEvent, onSessionEvent, onTunnelStatusEvent, onUploadProgressEvent, setAgentAvailable,
  } = useSSHStore();

  const [activeSection, setActiveSection] = useState<Section>('hosts');
  const [tunnelHostFilter, setTunnelHostFilter] = useState('');
  const [bootstrapOpen, setBootstrapOpen] = useState(false);
  const [bootstrapTarget, setBootstrapTarget] = useState<ParsedTarget | null>(null);
  const [bootstrapKeyId, setBootstrapKeyId] = useState<string>('');
  const [syncStatus, setSyncStatus] = useState<string | null>(null);
  const [importCount, setImportCount] = useState<number | null>(null);
  const [hostDraft, setHostDraft] = useState<HostDraft | null>(null);
  const [closeDialogOpen, setCloseDialogOpen] = useState(false);
  const [trayAvailable, setTrayAvailable] = useState(false);
  const [closeHintHidden, setCloseHintHiddenState] = useState(isCloseHintHidden);

  // Крестик и Alt+F4 (через BeforeClose → ui:close_requested) прячут окно в трей.
  const requestClose = useCallback(async () => {
    let tray = false;
    try { tray = await TrayAvailable(); } catch { /* backend недоступен — выходим */ }
    if (!tray) { ExitApp(); return; }
    if (isCloseHintHidden()) { HideToTray(); return; }
    setCloseDialogOpen(true);
  }, []);

  useEffect(() => {
    loadInitial();
    const offPing = EventsOn('host:ping', (res: any) => onPingEvent(res.host_id, res.online, res.latency_ms));
    const offSession = EventsOn('session:update', (msg: any) => onSessionEvent(msg.type, msg.session_id, msg.host_id));
    const offTunnel = EventsOn('tunnel:status', (st: any) => onTunnelStatusEvent(st.tunnel_id, st.state, st.active_conns, st.last_error));
    const offSftp = EventsOn('sftp:progress', (p: any) => onUploadProgressEvent(p.host_id, p.percent, p.done, Boolean(p.error)));
    const offAgent = EventsOn('agent:status', (st: any) => setAgentAvailable(Boolean(st?.available)));
    const offClose = EventsOn('ui:close_requested', () => { requestClose(); });
    TrayAvailable().then(setTrayAvailable).catch(() => setTrayAvailable(false));

    // Quick SFTP Dropzone: Wails отдаёт реальные пути файлов и координаты drop.
    OnFileDrop((x, y, paths) => {
      const card = document.elementFromPoint(x, y)?.closest<HTMLElement>('[data-host-id]');
      const hostId = card?.dataset.hostId;
      if (hostId && paths.length > 0) uploadFiles(hostId, paths);
    }, true);

    const handleKeyDown = (e: KeyboardEvent) => { if (e.key === 'Escape') clearSelection(); };
    window.addEventListener('keydown', handleKeyDown);

    return () => {
      offPing(); offSession(); offTunnel(); offSftp(); offAgent(); offClose();
      OnFileDropOff();
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, []);

  useEffect(() => {
    if (!notice) return;
    const t = setTimeout(dismissNotice, notice.kind === 'error' ? 8000 : 4000);
    return () => clearTimeout(t);
  }, [notice]);

  const runSync = async () => {
    try {
      await SyncSSHConfig();
      setSyncStatus('Конфиг успешно обновлён');
      setTimeout(() => setSyncStatus(null), 3000);
    } catch (err) {
      notify('error', `Экспорт в ~/.ssh/config: ${errorMessage(err)}`);
    }
  };

  const runImportPreview = async () => {
    try {
      const found = await PreviewImportSSHConfig();
      setImportCount(found?.length ?? 0);
    } catch (err) {
      notify('error', `Чтение ~/.ssh/config: ${errorMessage(err)}`);
    }
  };

  const sessionCountByHost = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const hostId of Object.values(sessions)) counts[hostId] = (counts[hostId] || 0) + 1;
    return counts;
  }, [sessions]);

  const tunnelsCount = useMemo(() => Object.values(tunnelsByHost).flat().length, [tunnelsByHost]);

  const activeTunnelsCount = useMemo(
    () => Object.values(tunnelsByHost).flat().filter((t) => t.state === 'active').length,
    [tunnelsByHost]
  );

  const visibleHosts = useMemo(() => {
    let list = hosts.filter((h) => (activeSection === 'temporary' ? h.is_temporary : !h.is_temporary));
    if (envFilter !== 'all') list = list.filter((h) => h.env_tag === envFilter);
    return list;
  }, [hosts, envFilter, activeSection]);

  const keyNameMap = useMemo(() => {
    const map: Record<string, string> = {};
    for (const k of keys) map[k.id] = k.name;
    return map;
  }, [keys]);

  return (
    <div className="flex h-screen w-screen flex-col overflow-hidden bg-transparent text-neutral-900 dark:text-neutral-100">
      <header style={DRAG_STYLE} className="flex h-12 shrink-0 items-center justify-between gap-4 border-b border-black/10 px-4 dark:border-white/10">
        <div className="flex items-center gap-2 text-xs font-semibold tracking-tight">
          <CrystalLogo className="h-5 w-5 shrink-0" />
          <span>Crystal-Core SSH Manager</span>
        </div>

        <div style={NO_DRAG_STYLE} className="flex flex-1 justify-center">
          <Omnibar
            hosts={hosts}
            keys={keys}
            onConnectHost={(id, mode) => connectHosts([id], mode)}
            onQuickConnect={(target, keyId, mode) => quickConnect(target, keyId, mode)}
            onStartBootstrap={(target, keyId) => {
              setBootstrapTarget(target);
              setBootstrapKeyId(keyId);
              setBootstrapOpen(true);
            }}
          />
        </div>

        <div style={NO_DRAG_STYLE} className="flex items-center gap-1">
          <button onClick={WindowMinimise} className="rounded-md p-1.5 text-neutral-500 hover:bg-black/5 dark:hover:bg-white/10"><Minus className="h-3.5 w-3.5" /></button>
          <button onClick={WindowToggleMaximise} className="rounded-md p-1.5 text-neutral-500 hover:bg-black/5 dark:hover:bg-white/10"><Square className="h-3 w-3" /></button>
          <button onClick={requestClose} title={trayAvailable ? 'Свернуть в трей' : 'Закрыть'} aria-label={trayAvailable ? 'Свернуть в трей' : 'Закрыть'} className="rounded-md p-1.5 text-neutral-500 hover:bg-red-500 hover:text-white"><X className="h-3.5 w-3.5" /></button>
        </div>
      </header>

      <div className="flex min-h-0 flex-1">
        <aside className="flex w-48 shrink-0 flex-col justify-between border-r border-black/10 p-2.5 dark:border-white/10">
          <nav className="space-y-1 text-xs font-medium">
            <SidebarItem icon={<Server className="h-4 w-4" />} label="Все серверы" active={activeSection === 'hosts'} count={hosts.filter((h) => !h.is_temporary).length} onClick={() => setActiveSection('hosts')} />
            <SidebarItem icon={<Clock className="h-4 w-4" />} label="Временные" active={activeSection === 'temporary'} count={hosts.filter((h) => h.is_temporary).length} onClick={() => setActiveSection('temporary')} />
            <div className="my-2 border-t border-black/10 dark:border-white/10" />
            <SidebarItem icon={<ArrowLeftRight className="h-4 w-4" />} label="Туннели" active={activeSection === 'tunnels'} count={tunnelsCount} onClick={() => { setTunnelHostFilter(''); setActiveSection('tunnels'); }} />
            <SidebarItem icon={<KeyRound className="h-4 w-4" />} label="Ключи" active={activeSection === 'keys'} count={keys.length} onClick={() => setActiveSection('keys')} />
          </nav>
          <div className="text-xs font-medium">
            <SidebarItem icon={<Settings className="h-4 w-4" />} label="Настройки" active={activeSection === 'settings'} onClick={() => setActiveSection('settings')} />
          </div>
        </aside>

        <main className="relative flex min-w-0 flex-1 flex-col overflow-hidden">
          {activeSection === 'keys' && <KeyVault keys={keys} onRefresh={loadInitial} />}

          {activeSection === 'tunnels' && (
            <TunnelManager
              hosts={hosts}
              tunnelsByHost={tunnelsByHost}
              hostFilter={tunnelHostFilter}
              onHostFilterChange={setTunnelHostFilter}
              onSave={saveTunnel}
              onDelete={deleteTunnel}
              onStart={startTunnel}
              onStop={stopTunnel}
              onOpenRDP={openRDP}
              onCreateMany={createTunnels}
            />
          )}

          {activeSection === 'settings' && (
            <div className="min-h-0 max-w-xl flex-1 space-y-4 overflow-y-auto p-5">
              <h1 className="text-base font-semibold">Настройки</h1>
              <div className="rounded-xl border border-black/10 bg-white/65 p-4 backdrop-blur-md dark:border-white/10 dark:bg-neutral-900/60">
                <div className="text-sm font-medium">Синхронизация с ~/.ssh/config</div>
                <p className="mt-1 text-xs text-neutral-500">
                  Экспортирует постоянные серверы в изолированный блок # BEGIN CRYSTAL-CORE SSH MANAGER для VS Code Remote-SSH, Git и PowerShell.
                </p>
                <div className="mt-3 flex items-center gap-2">
                  <button onClick={runSync} className="rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-500">
                    Экспортировать в ~/.ssh/config
                  </button>
                  <button onClick={runImportPreview} className="rounded-lg border border-black/10 px-3 py-1.5 text-xs font-medium hover:bg-black/5 dark:border-white/10 dark:hover:bg-white/10">
                    Проверить импорт
                  </button>
                </div>
                {syncStatus && <p className="mt-2 text-xs text-emerald-600 dark:text-emerald-400">{syncStatus}</p>}
                {importCount !== null && <p className="mt-2 text-xs text-neutral-500">Найдено пользовательских хостов: {importCount}</p>}
              </div>

              <BackupPanel onImported={loadInitial} notify={notify} />

              {trayAvailable && (
                <div className="rounded-xl border border-black/10 bg-white/65 p-4 backdrop-blur-md dark:border-white/10 dark:bg-neutral-900/60">
                  <div className="text-sm font-medium">Закрытие окна</div>
                  <p className="mt-1 text-xs text-neutral-500">
                    Крестик сворачивает приложение в трей — туннели продолжают работать. Полностью выйти можно через «Выход» в меню значка в трее.
                  </p>
                  <label className="mt-3 flex cursor-pointer items-center gap-2 text-xs">
                    <input
                      type="checkbox"
                      checked={!closeHintHidden}
                      onChange={(e) => { setCloseHintHidden(!e.target.checked); setCloseHintHiddenState(!e.target.checked); }}
                    />
                    Показывать подсказку при закрытии
                  </label>
                </div>
              )}
            </div>
          )}

          {(activeSection === 'hosts' || activeSection === 'temporary') && (
            <>
              <div className="flex items-center justify-between gap-3 px-5 py-3">
                <div className="flex items-center gap-1.5">
                  {(['all', 'prod', 'stage', 'dev', 'infra'] as const).map((tag) => (
                    <button key={tag} onClick={() => setEnvFilter(tag)} className={`rounded-md px-2.5 py-1 text-xs font-medium capitalize transition ${envFilter === tag ? 'bg-blue-600 text-white' : 'bg-black/5 text-neutral-600 hover:bg-black/10 dark:bg-white/5 dark:text-neutral-300'}`}>
                      {tag === 'all' ? 'Все' : tag}
                    </button>
                  ))}
                </div>
                <div className="flex items-center gap-1.5">
                  <button onClick={refreshPings} title="Обновить пинг" aria-label="Обновить пинг" className="flex items-center gap-1.5 whitespace-nowrap rounded-md bg-black/5 px-2.5 py-1 text-xs text-neutral-600 hover:bg-black/10 dark:bg-white/5 dark:text-neutral-300">
                    <RefreshCw className="h-3.5 w-3.5" /> <span className="hidden lg:inline">Обновить пинг</span>
                  </button>
                  <button
                    onClick={() => setHostDraft({ ...newHostDraft(keys), is_temporary: activeSection === 'temporary' })}
                    className="flex items-center gap-1.5 whitespace-nowrap rounded-md bg-blue-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-blue-500"
                  >
                    <Plus className="h-3.5 w-3.5" /> Добавить сервер
                  </button>
                </div>
              </div>

              <div className="grid flex-1 grid-cols-1 content-start gap-3 overflow-y-auto px-5 pb-20 sm:grid-cols-2 lg:grid-cols-3">
                {visibleHosts.map((host) => (
                  <HostCard
                    key={host.id}
                    host={host}
                    keyName={host.key_id ? keyNameMap[host.key_id] : undefined}
                    pingMs={pings[host.id]}
                    activeSessions={sessionCountByHost[host.id] || 0}
                    tunnels={tunnelsByHost[host.id] || []}
                    isSelected={selectedHostIds.includes(host.id)}
                    uploadProgress={uploads[host.id]}
                    onSelect={toggleSelectHost}
                    onConnect={(id) => connectHosts([id], 'tabs')}
                    onPinTemporary={pinHost}
                    onOpenTunnels={(id) => { setTunnelHostFilter(id); setActiveSection('tunnels'); }}
                    onEdit={() => setHostDraft(hostToDraft(host))}
                    onOpenRDP={openRDP}
                    onDelete={deleteHost}
                  />
                ))}
              </div>

              {selectedHostIds.length >= 2 && (
                <div className="absolute bottom-4 left-1/2 z-30 flex -translate-x-1/2 items-center gap-2 rounded-xl border border-black/15 bg-white/95 px-3.5 py-2 shadow-xl backdrop-blur-xl dark:border-white/15 dark:bg-neutral-900/95">
                  <span className="text-xs font-medium text-neutral-500">Выбрано: {selectedHostIds.length}</span>
                  <button onClick={() => connectHosts(selectedHostIds, 'tabs')} className="flex items-center gap-1.5 rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-500">
                    <Terminal className="h-3.5 w-3.5" /> Во вкладках
                  </button>
                  <button onClick={() => connectHosts(selectedHostIds, 'split')} className="flex items-center gap-1.5 rounded-lg bg-black/10 px-3 py-1.5 text-xs font-medium dark:bg-white/10">
                    <Columns2 className="h-3.5 w-3.5" /> В сплит-панелях
                  </button>
                  <button onClick={clearSelection} className="rounded-lg p-1.5 text-neutral-400 hover:bg-black/5"><X className="h-3.5 w-3.5" /></button>
                </div>
              )}
            </>
          )}
        </main>
      </div>

      <footer className="flex h-7 shrink-0 items-center justify-between border-t border-black/10 px-4 text-[11px] text-neutral-500 dark:border-white/10 dark:text-neutral-400">
        <div className="flex items-center gap-2">
          <span className={`h-2 w-2 rounded-full ${agentAvailable ? 'bg-emerald-500' : 'bg-red-500'}`} />
          <span>ssh-agent: {agentAvailable ? `Активен (${keys.length} ключей)` : 'Не запущен'}</span>
        </div>
        <div className="flex items-center gap-4">
          <span>Активных сессий: {Object.keys(sessions).length}</span>
          <span>Активных туннелей: {activeTunnelsCount}</span>
        </div>
      </footer>

      {notice && (
        <div
          role="status"
          onClick={dismissNotice}
          className={`fixed bottom-10 right-4 z-[60] max-w-sm cursor-pointer rounded-lg border px-3.5 py-2.5 text-xs shadow-xl backdrop-blur-xl ${
            notice.kind === 'error'
              ? 'border-red-500/30 bg-red-50/95 text-red-800 dark:bg-red-950/90 dark:text-red-200'
              : 'border-black/10 bg-white/95 text-neutral-800 dark:border-white/10 dark:bg-neutral-900/95 dark:text-neutral-100'
          }`}
        >
          {notice.text}
        </div>
      )}

      {closeDialogOpen && (
        <CloseToTrayDialog
          onCancel={() => setCloseDialogOpen(false)}
          onExit={() => { setCloseDialogOpen(false); ExitApp(); }}
          onHide={() => { setCloseDialogOpen(false); setCloseHintHiddenState(isCloseHintHidden()); HideToTray(); }}
        />
      )}

      {hostDraft && (
        <HostForm
          initial={hostDraft}
          hosts={hosts}
          keys={keys}
          runningTunnels={(hostDraft.id ? tunnelsByHost[hostDraft.id] ?? [] : []).filter((t) => t.state !== 'stopped' && t.state !== 'error').length}
          onCancel={() => setHostDraft(null)}
          onSubmit={async (draft) => {
            await saveHost(draft);
            setHostDraft(null);
            notify('info', draft.id ? `Сервер «${draft.name}» сохранён` : `Сервер «${draft.name}» добавлен`);
          }}
        />
      )}

      <BootstrapModal
        isOpen={bootstrapOpen}
        keys={keys}
        initialTarget={bootstrapTarget}
        initialKeyId={bootstrapKeyId}
        onClose={() => setBootstrapOpen(false)}
        onSaveAndConnect={async (data) => {
          setBootstrapOpen(false);
          await saveAndConnectHost(data);
        }}
      />
    </div>
  );
}

function SidebarItem({
  icon, label, active, count, onClick,
}: {
  icon: React.ReactNode; label: string; active?: boolean; count?: number; onClick?: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex w-full items-center justify-between rounded-lg px-2.5 py-2 transition ${
        active ? 'bg-black/10 text-neutral-900 dark:bg-white/10 dark:text-white' : 'text-neutral-600 hover:bg-black/5 dark:text-neutral-400 dark:hover:bg-white/5'
      }`}
    >
      <span className="flex items-center gap-2">{icon}{label}</span>
      {count !== undefined && count > 0 && (
        <span className="rounded bg-black/5 px-1.5 py-0.5 text-[10px] dark:bg-white/10">{count}</span>
      )}
    </button>
  );
}

export default App;
