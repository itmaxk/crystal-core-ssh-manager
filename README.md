# Crystal-Core SSH Manager

Легковесный кроссплатформенный SSH-менеджер и лаунчер поверх **Windows Terminal** (`wt.exe`), **iTerm2** и терминалов Linux. Построен на **Wails v2 (Go + React/TypeScript)** в стилистике Windows 11 (Fluent 2 / Mica).

## Возможности

* **Omnibar + Smart Paste (`Ctrl+Alt+S`):** вставьте `185.12.34.56`, `ubuntu@host:2222` или `ssh -p 2222 root@host` — парсер сам разберёт параметры и подключит вас с выбранным ключом.
* **One-Click Bootstrap:** 4-шаговая настройка нового VPS по паролю — подтверждение отпечатка хоста, идемпотентная установка ключа в `~/.ssh/authorized_keys`, верификация входа без пароля и мгновенное затирание пароля из памяти.
* **Key Vault без боли NTFS:** приватные ключи шифруются **AES-256-GCM** (мастер-ключ в системном Keyring / Windows Credential Manager) и загружаются в память системного `ssh-agent` без записи на диск.
* **Управление серверами:** добавление и редактирование имени, адреса, порта, пользователя, ключа, jump-хоста и окружения прямо с карточки; работающие туннели переподключаются автоматически при смене адреса или ключа.
* **Мульти-подключение и трекинг сессий:** запуск 2+ серверов во вкладках или сплит-панелях с цветовой маркировкой окружений (`Prod`, `Stage`, `Dev`, `Infra`) и живым счётчиком активных сессий через IPC.
* **Фоновые SSH-туннели:** Local (`-L`), Remote (`-R`) и SOCKS5 (`-D`) с keepalive и автореконнектом. Раздел «Туннели»: создание и редактирование, старт/стоп, статус и число соединений, последняя ошибка, автозапуск вместе с приложением, превью эквивалентной команды `ssh`.
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

Workflow собирает Windows/macOS/Linux, прогоняет `go test` и публикует GitHub Release с архивами и `SHA256SUMS.txt`. Перед тегом обновите `info.productVersion` в `wails.json`.

Пересобрать существующий релиз (например, чтобы добавить сборку macOS): `gh workflow run release.yml --ref v1.0.0` — артефакты загрузятся в релиз с заменой.

## Ограничения

* На macOS иконка в системном трее отключена: `systray` конфликтует с главным циклом Wails. Окно доступно из Dock.
* Глобальный хоткей `Ctrl+Alt+S` работает только на Windows; на других ОС повторный запуск приложения фокусирует уже открытое окно.
