# Crystal-Core SSH Manager

Легковесный кроссплатформенный SSH-менеджер и лаунчер поверх **Windows Terminal** (`wt.exe`), **iTerm2** и терминалов Linux. Построен на **Wails v2 (Go + React/TypeScript)** в стилистике Windows 11 (Fluent 2 / Mica).

## Возможности

* **Omnibar + Smart Paste (`Ctrl+Alt+S`):** вставьте `185.12.34.56`, `ubuntu@host:2222` или `ssh -p 2222 root@host` — парсер сам разберёт параметры и подключит вас с выбранным ключом.
* **One-Click Bootstrap:** 4-шаговая настройка нового VPS по паролю — подтверждение отпечатка хоста, идемпотентная установка ключа в `~/.ssh/authorized_keys`, верификация входа без пароля и мгновенное затирание пароля из памяти.
* **Key Vault без боли NTFS:** приватные ключи шифруются **AES-256-GCM** (мастер-ключ в системном Keyring / Windows Credential Manager) и загружаются в память системного `ssh-agent` без записи на диск.
* **Управление серверами:** добавление и редактирование имени, адреса, порта, пользователя, ключа, jump-хоста и окружения прямо с карточки; работающие туннели переподключаются автоматически при смене адреса или ключа.
* **Мульти-подключение и трекинг сессий:** запуск 2+ серверов во вкладках или сплит-панелях с цветовой маркировкой окружений (`Prod`, `Stage`, `Dev`, `Infra`) и живым счётчиком активных сессий через IPC.
* **Фоновые SSH-туннели:** Local (`-L`), Remote (`-R`) и SOCKS5 (`-D`) с keepalive и автореконнектом. Раздел «Туннели»: создание и редактирование, старт/стоп, статус и число соединений, последняя ошибка, автозапуск вместе с приложением, превью эквивалентной команды `ssh`.
* **Поиск сервисов и автосоздание туннелей:** «Туннели → Найти сервисы» подключается к серверу по ключу, читает открытые порты (`ss -tln`, запасной вариант — `netstat`) и предлагает туннели к найденным сервисам: PostgreSQL, MySQL, Redis, RDP (xrdp / GNOME Remote Desktop) и т.д. Порты, которые есть на чистой Ubuntu 22.04/24.04 (SSH, systemd-resolved, CUPS, rpcbind), скрыты. Локальный порт подбирается свободным: тот же, что на сервере, привилегированные — `+10000` (80 → 10080), RDP — 13389. Сервисы, открытые на `0.0.0.0`, помечаются «открыт наружу».
* **RDP через SSH-туннель:** шаблон туннеля «Удалённый рабочий стол» (`-L 13389 → localhost:3389`) и кнопка «RDP» на карточке сервера и в списке туннелей: туннель поднимается автоматически, затем открывается `mstsc` (Windows), Windows App (macOS) или Remmina / FreeRDP (Linux). Для машины за SSH-сервером укажите её адрес вместо `localhost`, например `10.0.0.5:3389`. Логин, домен и пароль RDP можно сохранить в туннеле (пароль шифруется AES-256-GCM мастер-ключом) — тогда вход выполняется автоматически: на Windows через диспетчер учётных данных на время подключения, на Linux через FreeRDP (пароль передаётся через stdin, не в командной строке). Windows App на macOS не принимает пароль извне — подставляется только логин.
* **Копирование сохранённых паролей:** кнопки «Пароль» (RDP-туннель) и «Фраза» (ключ с парольной фразой) кладут секрет в буфер обмена на 15 секунд — затем он стирается автоматически, а кнопка «Очистить» стирает сразу. Если за это время вы скопировали что-то своё, буфер не трогается. На Windows пароль не попадает в журнал буфера (Win+V) и облачную синхронизацию.
* **VPS Setup одной кнопкой:** значок ракеты на карточке сервера открывает вкладку терминала и запускает на сервере установщик [Crystal-Core VPS Install Manager](https://github.com/itmaxk/crystal-core-vps-install-manager) (`curl … vps-setup-linux-$(dpkg --print-architecture)`, `chmod +x`, `sudo ./vps-setup`). Вопросы установщика и пароль `sudo` вводятся в терминале, после завершения сессия остаётся открытой.
* **Автообновление:** проверка новых версий на GitHub, загрузка с проверкой SHA256 и перезапуск в один клик (Настройки → Обновления).
* **Резервная копия:** экспорт и импорт серверов, туннелей и SSH-ключей (включая приватные) в файл `.ccbackup`, зашифрованный паролем (Argon2id + AES-256-GCM). Импорт объединяет данные с текущими и не создаёт дублей ключей.
* **Работа в фоне:** крестик сворачивает окно в трей, туннели продолжают работать; полный выход — «Выход» в меню значка в трее.
* **Quick SFTP Dropzone:** перетащите файл на карточку сервера для быстрой загрузки в `/tmp` с прогресс-баром.
* **Мониторинг доступности:** фоновый TCP-пинг с отображением задержки в мс.
* **Синхронизация с `~/.ssh/config`:** экспорт в изолированный блок `# BEGIN CRYSTAL-CORE SSH MANAGER` для работы в VS Code Remote-SSH, Git и терминале.
* **Проверка HostKey:** туннели и SFTP сверяют ключ сервера с `~/.ssh/known_hosts` (общим с системным `ssh`). Неизвестный хост нужно один раз подтвердить — через One-Click Bootstrap (отпечаток записывается в `known_hosts`) или обычным входом в терминале.

## Установка

Готовые сборки — на странице [Releases](../../releases):

| Платформа | Файл | Примечание |
| :--- | :--- | :--- |
| Windows 10/11 x64 | `crystal-core-ssh-manager-windows-amd64.zip` | Распакуйте `CrystalCoreSSHManager.exe` и `ssh-launcher-cli.exe` в одну папку. Нужны Windows Terminal и WebView2. |
| macOS (Intel + Apple Silicon) | `crystal-core-ssh-manager-macos-universal.zip` | Сборка не подписана: после распаковки выполните `xattr -dr com.apple.quarantine CrystalCoreSSHManager.app`. |
| Linux x64 | `crystal-core-ssh-manager-linux-amd64.tar.gz` | glibc ≥ 2.34 (Ubuntu 22.04+, Debian 12+), `libwebkit2gtk-4.1-0`, `libgtk-3-0` и Secret Service (GNOME Keyring / KWallet) для мастер-ключа. |

Контрольные суммы — в `SHA256SUMS.txt` в каждом релизе.

### Обновление

Начиная с версии 1.5.0 — из интерфейса: **Настройки → Обновления** (при запуске и раз в 6 часов программа сама проверяет [последний релиз](../../releases/latest), а в боковой панели появляется плашка «Обновление x.y.z»). «Скачать и установить» загружает архив для вашей платформы, сверяет его с `SHA256SUMS.txt` релиза, подменяет `CrystalCoreSSHManager(.exe)` и `ssh-launcher-cli.exe` и перезапускает программу. Туннели при этом останавливаются; туннели с автозапуском поднимутся в новой версии. Данные (`vault.db`, мастер-ключ) не затрагиваются.

Автоустановка работает для Windows x64 и Linux x64, если в папку программы можно писать без прав администратора (например, `%LOCALAPPDATA%`, `~/Applications`, а не `Program Files` или `/usr/local/bin`). В остальных случаях кнопка ведёт на страницу релиза.

Вручную (и для перехода с версий до 1.5.0): закройте приложение полностью («Выход» в меню значка в трее) и замените исполняемые файлы новыми — данные хранятся отдельно и сохранятся.

### Где хранятся данные

| Что | Где |
| :--- | :--- |
| База (`vault.db`): серверы, туннели, ключи; приватные ключи зашифрованы AES-256-GCM | Windows: `%APPDATA%\CrystalCoreSSHManager\`<br>Linux: `~/.config/CrystalCoreSSHManager/`<br>macOS: `~/Library/Application Support/CrystalCoreSSHManager/` |
| Публичные ключи для `ssh -i` и `IdentityFile` | `pubkeys/` рядом с `vault.db` |
| Мастер-ключ базы | Системное хранилище паролей, запись `com.crystalcore.sshmanager`: Диспетчер учётных данных Windows, Secret Service в Linux, Связка ключей macOS |

`vault.db` без мастер-ключа не расшифровать, поэтому для переноса на другой компьютер используйте **Настройки → Резервная копия**. `~/.ssh/known_hosts` в бэкап не входит.

## Требования для сборки

* **Go** 1.26+ (версия берётся из `go.mod`)
* **Node.js** 22+
* **Wails CLI** той же версии, что и в `go.mod`: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1`
* **Windows:** Windows Terminal (`wt.exe`) и включённая служба `OpenSSH Authentication Agent` (`sc config ssh-agent start=auto && sc start ssh-agent`)
* **Linux:** `libgtk-3-dev`, `libwebkit2gtk-4.1-dev`, `libayatana-appindicator3-dev`

## Быстрый старт

```bash
# Подтянуть Go-зависимости
go mod tidy

# Режим разработки (автоматически сгенерирует frontend/wailsjs)
wails dev

# Тесты
go test ./...

# Продакшен-сборка (Linux с WebKitGTK 4.1: добавьте -tags webkit2_41)
wails build -clean

# Windows: дополнительно собрать консольную CLI-обёртку рядом с основным .exe
go build -ldflags="-s -w" -o build/bin/ssh-launcher-cli.exe ./cmd/ssh-launcher-cli
```

## Горячие клавиши

| Комбинация | Действие |
| :--- | :--- |
| `Ctrl+Alt+S` | Показать окно и сфокусировать Omnibar (глобально на Windows) |
| `Enter` | Подключиться к выбранному серверу в новой вкладке |
| `Shift+Enter` | Подключиться в сплит-панели |
| `Ctrl+Enter` | Запустить One-Click Bootstrap для адреса в Omnibar |
| `Tab` | Переключить SSH-ключ в Omnibar |
| `Ctrl+Click` | Мульти-выделение карточек серверов |
| `Esc` | Снять выделение / очистить поиск |

## Релизы

Релиз собирается GitHub Actions (`.github/workflows/release.yml`) при пуше тега `v*`:

```bash
git tag v1.0.1 && git push origin v1.0.1
```

Workflow собирает Windows/macOS/Linux, прогоняет `go test` и публикует GitHub Release с архивами и `SHA256SUMS.txt`. Перед тегом обновите `info.productVersion` в `wails.json` (из него берётся версия для автообновления) и `version` в `frontend/package.json`. Автообновление ищет в релизе файлы с именами `crystal-core-ssh-manager-windows-amd64.zip`, `crystal-core-ssh-manager-linux-amd64.tar.gz` и `SHA256SUMS.txt` — не переименовывайте их.

Пересобрать существующий релиз (например, чтобы добавить сборку macOS): `gh workflow run release.yml --ref v1.4.0` — артефакты загрузятся в релиз с заменой.

### Ручная сборка macOS

Сборка под macOS требует Cocoa/WebKit из macOS SDK, поэтому кросс-компиляция с Windows/Linux не поддерживается. Если GitHub Actions недоступен, соберите на Mac (нужны Xcode Command Line Tools, Go 1.26 и Wails CLI 2.17.1) и дозагрузите архив в существующий релиз:

```bash
TAG=v1.4.0
git checkout "$TAG"
wails build -clean -platform darwin/universal
cd build/bin
ditto -c -k --keepParent CrystalCoreSSHManager.app crystal-core-ssh-manager-macos-universal.zip
gh release upload "$TAG" crystal-core-ssh-manager-macos-universal.zip --clobber

# Дописать хеш в SHA256SUMS.txt релиза
gh release download "$TAG" -p SHA256SUMS.txt --clobber
grep -v macos-universal SHA256SUMS.txt > SHA256SUMS.new || true
shasum -a 256 crystal-core-ssh-manager-macos-universal.zip >> SHA256SUMS.new
mv SHA256SUMS.new SHA256SUMS.txt
gh release upload "$TAG" SHA256SUMS.txt --clobber
```

Сборка не подписана сертификатом Apple: при первом запуске откройте приложение через правый клик → «Открыть» или снимите карантин командой `xattr -dr com.apple.quarantine CrystalCoreSSHManager.app`.

## Ограничения

* На macOS иконка в системном трее отключена: `systray` конфликтует с главным циклом Wails. Окно доступно из Dock, а крестик закрывает приложение (и останавливает туннели).
* Автовход RDP на Windows использует сохранённые учётные данные `mstsc`. Если политика домена запрещает их («администратор не разрешает использовать сохранённые учётные данные»), mstsc спросит пароль сам.
* В GNOME значок в трее появляется только с расширением AppIndicator. Без него свёрнутое окно возвращается повторным запуском приложения.
* Кнопка «RDP» показывается для локальных туннелей на порт `3389`.
* Глобальный хоткей `Ctrl+Alt+S` работает только на Windows; на других ОС повторный запуск приложения фокусирует уже открытое окно.

## Разработчик

Crystal-Core SSH Manager делает **[Crystal-Core](https://crystal-core.com/)** — автоматизация бизнес-процессов любой сложности, CRM и веб-приложения, чат-боты и AI-ассистенты, сайты и мобильные приложения. [Обсудить проект →](https://crystal-core.com/)
