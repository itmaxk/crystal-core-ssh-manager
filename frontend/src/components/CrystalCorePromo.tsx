import React from 'react';
import { ArrowUpRight } from 'lucide-react';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
import { version } from '../../package.json';
import { CrystalLogo } from './CrystalLogo';

export const CRYSTAL_CORE_URL = 'https://crystal-core.com/';
export const APP_VERSION = version;

// Ссылки открываем в системном браузере, а не внутри окна приложения.
const openSite = (e: React.MouseEvent) => {
  e.preventDefault();
  BrowserOpenURL(CRYSTAL_CORE_URL);
};

/** Компактная карточка разработчика внизу сайдбара. */
export const SidebarPromo: React.FC = () => (
  <a
    href={CRYSTAL_CORE_URL}
    onClick={openSite}
    title="crystal-core.com — откроется в браузере"
    className="group mb-1.5 block rounded-xl border border-blue-500/20 bg-blue-500/[0.06] p-2.5 transition hover:border-blue-500/40 hover:bg-blue-500/10"
  >
    <div className="flex items-center gap-2">
      <CrystalLogo className="h-6 w-6 shrink-0" />
      <div className="min-w-0 leading-tight">
        <div className="whitespace-nowrap text-xs font-semibold text-neutral-800 dark:text-neutral-100">Crystal-Core</div>
        <div className="whitespace-nowrap text-[10px] text-neutral-500 dark:text-neutral-400">разработчик</div>
      </div>
    </div>
    <p className="mt-1.5 text-[11px] leading-snug text-neutral-600 dark:text-neutral-300">
      Автоматизируем бизнес: CRM, чат-боты и AI-ассистенты под ваши процессы.
    </p>
    <span className="mt-1.5 inline-flex items-center gap-0.5 text-[11px] font-medium text-blue-600 group-hover:underline dark:text-blue-400">
      Обсудить проект <ArrowUpRight className="h-3 w-3" />
    </span>
  </a>
);

/** Карточка «О программе» в настройках. */
export const AboutCard: React.FC = () => (
  <div className="rounded-xl border border-black/10 bg-white/65 p-4 backdrop-blur-md dark:border-white/10 dark:bg-neutral-900/60">
    <div className="flex items-start gap-3">
      <CrystalLogo className="h-9 w-9 shrink-0" />
      <div className="min-w-0">
        <div className="text-sm font-medium">Crystal-Core SSH Manager <span className="font-normal text-neutral-500">v{APP_VERSION}</span></div>
        <p className="mt-0.5 text-xs text-neutral-500">
          Разработчик —{' '}
          <a href={CRYSTAL_CORE_URL} onClick={openSite} className="font-medium text-blue-600 hover:underline dark:text-blue-400">Crystal-Core</a>
        </p>
      </div>
    </div>
    <div className="mt-3 rounded-lg bg-blue-500/[0.06] px-3 py-2.5">
      <p className="text-xs font-medium text-neutral-800 dark:text-neutral-100">Превращаем хаос в прозрачное ядро бизнеса</p>
      <p className="mt-1 text-xs text-neutral-600 dark:text-neutral-400">
        Автоматизация процессов, CRM и веб-приложения, чат-боты и AI-ассистенты, сайты и мобильные приложения — от задачи до поддержки.
      </p>
      <div className="mt-2.5 flex flex-wrap items-center gap-2">
        <a href={CRYSTAL_CORE_URL} onClick={openSite} className="inline-flex items-center gap-1 rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-500">
          Обсудить проект <ArrowUpRight className="h-3.5 w-3.5" />
        </a>
        <a href={CRYSTAL_CORE_URL} onClick={openSite} className="text-xs text-neutral-500 hover:underline">crystal-core.com</a>
      </div>
    </div>
  </div>
);
