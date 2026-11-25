import React, { useState } from 'react';
import { MinusSquare } from 'lucide-react';
import { useEscape } from '../hooks/useEscape';

const HINT_KEY = 'ccsm.closeHint.hidden';

// Флаг «больше не показывать» — личная настройка этого компьютера.
export const isCloseHintHidden = (): boolean => {
  try { return localStorage.getItem(HINT_KEY) === '1'; } catch { return false; }
};

export const setCloseHintHidden = (hidden: boolean) => {
  try {
    if (hidden) localStorage.setItem(HINT_KEY, '1');
    else localStorage.removeItem(HINT_KEY);
  } catch { /* хранилище недоступно — просто покажем подсказку снова */ }
};

const isWindows = typeof navigator !== 'undefined' && /Windows/i.test(navigator.userAgent);

interface CloseToTrayDialogProps {
  onHide: () => void;
  onExit: () => void;
  onCancel: () => void;
}

export const CloseToTrayDialog: React.FC<CloseToTrayDialogProps> = ({ onHide, onExit, onCancel }) => {
  const [dontShow, setDontShow] = useState(false);

  useEscape(onCancel);

  const hide = () => {
    if (dontShow) setCloseHintHidden(true);
    onHide();
  };

  return (
    <div className="fixed inset-0 z-[70] flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm">
      <div role="alertdialog" aria-modal="true" aria-labelledby="tray-title" aria-describedby="tray-desc" className="w-full max-w-[420px] overflow-hidden rounded-2xl border border-black/10 bg-white/95 shadow-2xl dark:border-white/10 dark:bg-neutral-900/95">
        <div className="space-y-3 p-5">
          <div className="flex items-start gap-3">
            <div className="rounded-lg bg-blue-500/15 p-2 text-blue-600 dark:text-blue-400">
              <MinusSquare className="h-4 w-4" />
            </div>
            <div className="min-w-0">
              <h2 id="tray-title" className="text-sm font-semibold">Приложение свернётся в трей</h2>
              <div id="tray-desc" className="mt-1 space-y-1.5 text-xs text-neutral-600 dark:text-neutral-400">
                <p>Туннели и проверка доступности серверов продолжат работать в фоне.</p>
                <p>
                  Чтобы закрыть программу полностью, нажмите правой кнопкой на значок Crystal-Core в трее и выберите <b className="font-medium text-neutral-800 dark:text-neutral-200">«Выход»</b>.
                </p>
                <p>
                  Вернуть окно: щелчок по значку в трее{isWindows ? ', Ctrl+Alt+S' : ''} или повторный запуск приложения.
                </p>
              </div>
            </div>
          </div>

          <label className="flex cursor-pointer items-center gap-2 pl-11 text-xs text-neutral-600 dark:text-neutral-400">
            <input type="checkbox" checked={dontShow} onChange={(e) => setDontShow(e.target.checked)} />
            Больше не показывать
          </label>
        </div>

        <div className="flex items-center justify-between gap-2 border-t border-black/10 px-5 py-3 dark:border-white/10">
          <button type="button" onClick={onExit} className="rounded-lg px-3 py-1.5 text-xs font-medium text-red-600 hover:bg-red-500/10 dark:text-red-400">
            Выйти совсем
          </button>
          <div className="flex gap-2">
            <button type="button" onClick={onCancel} className="rounded-lg px-3 py-1.5 text-xs font-medium hover:bg-black/5 dark:hover:bg-white/10">
              Отмена
            </button>
            <button type="button" autoFocus onClick={hide} className="rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-medium text-white hover:bg-blue-500">
              Свернуть в трей
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
