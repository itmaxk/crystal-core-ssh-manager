import { useEffect, useRef } from 'react';

/**
 * Вызывает handler по Escape. Подписка создаётся один раз, а актуальный
 * handler берётся из ref: иначе перерисовка, вызванная другим обработчиком
 * того же keydown, снимает слушатель до того, как он успеет сработать.
 */
export function useEscape(handler: () => void, enabled = true) {
  const ref = useRef(handler);
  ref.current = handler;
  const enabledRef = useRef(enabled);
  enabledRef.current = enabled;

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && enabledRef.current) ref.current();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);
}
