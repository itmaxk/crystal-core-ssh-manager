import React, { useState } from 'react';
import { ArrowUpCircle, CheckCircle2, ChevronDown, Download, ExternalLink, Loader2, RefreshCw, RotateCw, X } from 'lucide-react';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
import { useUpdateStore } from '../store/useUpdateStore';
import { APP_VERSION } from './CrystalCorePromo';

/**
 * Заметки релиза — markdown GitHub. Показываем как текст (без HTML):
 * заголовки и пункты списков превращаем в строки с отступами, убираем разметку.
 */
const NotesView: React.FC<{ text: string }> = ({ text }) => {
  const clean = (line: string) => line.replace(/\*\*(.+?)\*\*/g, '$1').replace(/`([^`]+)`/g, '$1').replace(/\[([^\]]+)\]\([^)]+\)/g, '$1');
  const lines = text.replace(/\r/g, '').split('\n');
  return (
    <div className="space-y-1">
      {lines.map((raw, i) => {
        const line = raw.trimEnd();
        if (!line.trim() || /^\s*(---|\|)/.test(line) || /^\s*>/.test(line) && !line.replace(/^\s*>\s*/, '')) return null;
        const h = line.match(/^#{1,6}\s+(.*)$/);
        if (h) return <div key={i} className="pt-1 font-semibold text-neutral-800 first:pt-0 dark:text-neutral-100">{clean(h[1])}</div>;
        const li = line.match(/^(\s*)[*-]\s+(.*)$/);
        if (li) return <div key={i} className="flex gap-1.5" style={{ paddingLeft: li[1].length * 4 }}><span aria-hidden>•</span><span>{clean(li[2])}</span></div>;
        return <div key={i}>{clean(line.replace(/^\s*>\s*/, ''))}</div>;
      })}
    </div>
  );
};

const mb = (n: number) => `${(n / 1024 / 1024).toFixed(1)} МБ`;

const openURL = (url: string) => (e: React.MouseEvent) => {
  e.preventDefault();
  BrowserOpenURL(url);
};

/** Плашка в сайдбаре, когда доступна новая версия. */
export const SidebarUpdateBadge: React.FC<{ onOpen: () => void }> = ({ onOpen }) => {
  const info = useUpdateStore((s) => s.info);
  const phase = useUpdateStore((s) => s.phase);
  if (!info?.available) return null;
  return (
    <button
      type="button"
      onClick={onOpen}
      className="mb-1.5 flex w-full items-center gap-2 rounded-lg bg-emerald-500/10 px-2.5 py-2 text-left text-[11px] font-medium text-emerald-800 transition hover:bg-emerald-500/20 dark:text-emerald-200"
    >
      {phase === 'downloading' ? <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin" /> : <ArrowUpCircle className="h-3.5 w-3.5 shrink-0" />}
      <span className="min-w-0 truncate">
        {phase === 'ready' ? 'Обновление готово' : `Обновление ${info.latest}`}
      </span>
    </button>
  );
};

/** Карточка «Обновления» в настройках. */
export const UpdateCard: React.FC = () => {
  const { info, phase, progress, error, checkedAt, autoCheck, check, download, cancel, install, setAutoCheck } = useUpdateStore();
  const [notesOpen, setNotesOpen] = useState(true);
  const available = Boolean(info?.available);
  const pct = progress && progress.total > 0 ? Math.min(100, Math.round((progress.done / progress.total) * 100)) : null;

  return (
    <div className="rounded-xl border border-black/10 bg-white/65 p-4 backdrop-blur-md dark:border-white/10 dark:bg-neutral-900/60">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="text-sm font-medium">Обновления</div>
          <p className="mt-1 text-xs text-neutral-500">
            Установлена версия <span className="font-mono">{APP_VERSION}</span>
            {checkedAt && !available && !error && (
              <span className="ml-1.5 inline-flex items-center gap-1 text-emerald-600 dark:text-emerald-400">
                <CheckCircle2 className="h-3 w-3" /> последняя
              </span>
            )}
          </p>
        </div>
        {!available && (
          <button
            type="button"
            onClick={() => check()}
            disabled={phase === 'checking'}
            className="flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-lg border border-black/10 px-3 py-1.5 text-xs font-medium hover:bg-black/5 disabled:opacity-60 dark:border-white/10 dark:hover:bg-white/10"
          >
            {phase === 'checking' ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
            Проверить
          </button>
        )}
      </div>

      {available && info && (
        <div className="mt-3 rounded-lg border border-emerald-500/25 bg-emerald-500/[0.06] p-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="text-xs font-medium text-neutral-800 dark:text-neutral-100">
              Доступна версия <span className="font-mono">{info.latest}</span>
              {info.asset_size ? <span className="ml-1.5 font-normal text-neutral-500">· {mb(info.asset_size)}</span> : null}
            </div>
            <a href={info.url} onClick={openURL(info.url)} className="inline-flex items-center gap-1 text-[11px] text-blue-600 hover:underline dark:text-blue-400">
              Страница релиза <ExternalLink className="h-3 w-3" />
            </a>
          </div>

          {info.notes && (
            <div className="mt-2">
              <button type="button" onClick={() => setNotesOpen((v) => !v)} aria-expanded={notesOpen} className="flex items-center gap-1 text-[11px] font-medium text-neutral-600 hover:text-neutral-900 dark:text-neutral-400 dark:hover:text-neutral-100">
                <ChevronDown className={`h-3 w-3 transition-transform ${notesOpen ? '' : '-rotate-90'}`} /> Что нового
              </button>
              {notesOpen && (
                <div className="mt-1.5 max-h-48 overflow-y-auto rounded-md bg-white/70 px-2.5 py-2 text-[11px] leading-relaxed text-neutral-700 dark:bg-black/20 dark:text-neutral-300">
                  <NotesView text={info.notes} />
                </div>
              )}
            </div>
          )}

          {phase === 'downloading' && (
            <div className="mt-3">
              <div className="h-1.5 overflow-hidden rounded-full bg-black/10 dark:bg-white/10" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct ?? undefined}>
                <div className={`h-full rounded-full bg-blue-600 transition-[width] duration-150 ${pct === null ? 'w-1/3 animate-pulse' : ''}`} style={pct === null ? undefined : { width: `${pct}%` }} />
              </div>
              <div className="mt-1 flex justify-between font-mono text-[10px] tabular-nums text-neutral-500">
                <span>{progress ? mb(progress.done) : ''}{progress?.total ? ` / ${mb(progress.total)}` : ''}</span>
                <span>{pct !== null ? `${pct}%` : ''}</span>
              </div>
            </div>
          )}

          <div className="mt-3 flex flex-wrap items-center gap-2">
            {!info.can_install ? (
              <>
                <a href={info.url} onClick={openURL(info.url)} className="inline-flex items-center gap-1.5 rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-500">
                  <Download className="h-3.5 w-3.5" /> Скачать вручную
                </a>
                {info.reason && <span className="text-[11px] text-neutral-500">{info.reason}</span>}
              </>
            ) : phase === 'downloading' ? (
              <button type="button" onClick={cancel} className="flex items-center gap-1.5 rounded-lg border border-black/10 px-3 py-1.5 text-xs font-medium hover:bg-black/5 dark:border-white/10 dark:hover:bg-white/10">
                <X className="h-3.5 w-3.5" /> Отменить
              </button>
            ) : phase === 'ready' || phase === 'installing' ? (
              <>
                <button type="button" onClick={install} disabled={phase === 'installing'} className="flex items-center gap-1.5 rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-emerald-500 disabled:opacity-60">
                  {phase === 'installing' ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RotateCw className="h-3.5 w-3.5" />}
                  Перезапустить и обновить
                </button>
                <span className="text-[11px] text-neutral-500">Туннели остановятся; с автозапуском — поднимутся в новой версии.</span>
              </>
            ) : (
              <>
                <button type="button" onClick={download} className="flex items-center gap-1.5 rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-500">
                  <Download className="h-3.5 w-3.5" /> Скачать и установить
                </button>
                <span className="text-[11px] text-neutral-500">Файл проверяется по SHA256 из релиза.</span>
              </>
            )}
          </div>
        </div>
      )}

      {error && <div role="alert" className="mt-3 rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-700 dark:text-red-300">{error}</div>}

      <label className="mt-3 flex cursor-pointer items-center gap-2 text-xs">
        <input type="checkbox" checked={autoCheck} onChange={(e) => setAutoCheck(e.target.checked)} />
        Проверять обновления при запуске
      </label>
    </div>
  );
};
