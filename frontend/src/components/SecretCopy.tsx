import React, { useEffect, useState } from 'react';
import { ClipboardCopy, Eraser } from 'lucide-react';
import { SecretSource, useSSHStore } from '../store/useSSHStore';

/** Секунд до автоочистки буфера (0 — пароля в буфере нет). */
export function useSecretCountdown(): number {
  const secretClip = useSSHStore((s) => s.secretClip);
  const onSecretCleared = useSSHStore((s) => s.onSecretCleared);
  const [now, setNow] = useState(Date.now());

  useEffect(() => {
    if (!secretClip) return;
    setNow(Date.now());
    const t = setInterval(() => setNow(Date.now()), 250);
    return () => clearInterval(t);
  }, [secretClip]);

  // Бэкенд сам стирает буфер и шлёт clipboard:cleared; это запасной сброс,
  // если событие не дошло.
  useEffect(() => {
    if (secretClip && now > secretClip.expiresAt + 2000) onSecretCleared();
  }, [now, secretClip, onSecretCleared]);

  if (!secretClip) return 0;
  return Math.max(0, Math.ceil((secretClip.expiresAt - now) / 1000));
}

interface SecretCopyButtonProps {
  source: SecretSource;
  /** Подпись на кнопке до копирования. */
  label: string;
  title: string;
  className?: string;
}

/**
 * Копирует сохранённый пароль в буфер на 15 секунд. Пока пароль лежит в
 * буфере, кнопка показывает обратный отсчёт и по нажатию стирает его сразу.
 */
export const SecretCopyButton: React.FC<SecretCopyButtonProps> = ({ source, label, title, className = '' }) => {
  const secretClip = useSSHStore((s) => s.secretClip);
  const copySecret = useSSHStore((s) => s.copySecret);
  const clearSecret = useSSHStore((s) => s.clearSecret);
  const seconds = useSecretCountdown();
  const [busy, setBusy] = useState(false);
  const active = secretClip?.source === `${source.kind}:${source.id}` && seconds > 0;

  const onClick = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (busy) return;
    setBusy(true);
    try {
      if (active) await clearSecret();
      else await copySecret(source);
    } finally {
      setBusy(false);
    }
  };

  return (
    <button
      type="button"
      onClick={onClick}
      onDoubleClick={(e) => e.stopPropagation()}
      disabled={busy}
      title={active ? 'Стереть пароль из буфера обмена сейчас' : `${title}. Буфер очистится через 15 секунд`}
      aria-label={active ? `Очистить буфер обмена, осталось ${seconds} с` : title}
      className={`flex items-center gap-1 whitespace-nowrap rounded-lg px-2.5 py-1.5 text-xs font-medium transition disabled:opacity-60 ${
        active
          ? 'bg-amber-500/15 text-amber-800 hover:bg-amber-500/25 dark:text-amber-200'
          : 'bg-black/5 text-neutral-700 hover:bg-black/10 dark:bg-white/10 dark:text-neutral-200 dark:hover:bg-white/15'
      } ${className}`}
    >
      {active ? <Eraser className="h-3 w-3" /> : <ClipboardCopy className="h-3 w-3" />}
      {active ? <>Очистить <span className="font-mono tabular-nums">{seconds}</span></> : label}
    </button>
  );
};

/** Индикатор в строке состояния: какой пароль в буфере и сколько осталось. */
export const SecretClipStatus: React.FC = () => {
  const secretClip = useSSHStore((s) => s.secretClip);
  const clearSecret = useSSHStore((s) => s.clearSecret);
  const seconds = useSecretCountdown();
  if (!secretClip || seconds <= 0) return null;
  return (
    <span role="status" className="flex items-center gap-1.5 text-amber-700 dark:text-amber-300">
      <ClipboardCopy className="h-3 w-3" />
      <span className="max-w-[16rem] truncate">{secretClip.label} в буфере</span>
      <span className="font-mono tabular-nums">· {seconds} с</span>
      <button type="button" onClick={clearSecret} className="rounded px-1.5 py-px font-medium hover:bg-amber-500/15">
        Очистить
      </button>
    </span>
  );
};
