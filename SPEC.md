# Техническое задание: Crystal-Core SSH Manager

## 1. Назначение и цели проекта
**Crystal-Core SSH Manager** — легковесное кроссплатформенное десктопное приложение (приоритетная платформа — Windows 11, с поддержкой macOS и Linux) для быстрого подключения к серверам по SSH, безопасного управления ключами, мониторинга доступности и фонового порт-форвардинга.

### Ключевые цели
* Минимальное время входа на новый сервер (Zero-Friction Onboarding через Smart Paste и One-Click Bootstrap).
* Работа поверх нативных терминалов (`wt.exe`, `iTerm2`, `WezTerm`/`Kitty`) без эмуляции терминала внутри приложения.
* Отсутствие приватных ключей в открытом виде на диске.

## 2. Архитектура и технологический стек
Приложение состоит из двух компонентов:
1. **GUI-процесс (`CrystalCoreSSHManager`):** окно Wails v2 (WebView2 / WebKit), системный трей, глобальный хоткей, IPC-сервер, фоновые сервисы (мониторинг, туннели, SFTP, Bootstrap).
2. **CLI-обёртка (`ssh-launcher-cli.exe` на Windows, флаг `--connect` на macOS/Linux):** консольный процесс внутри вкладки терминала, который запускает системный `ssh`, наследует TTY и передаёт события жизненного цикла сессии в GUI по IPC.

| Слой | Технологии |
| :--- | :--- |
| **Backend** | Go 1.26+, Wails v2.17+ |
| **Frontend** | React 18, TypeScript, Tailwind CSS, Zustand, Lucide Icons |
| **База данных** | SQLite (`modernc.org/sqlite`, без CGO, режим WAL, встроенные миграции через `PRAGMA user_version`) |
| **Криптография** | Master Key в системном Keyring (`zalando/go-keyring`, сервис `com.crystalcore.sshmanager`) + AES-256-GCM |
| **SSH / SFTP** | `golang.org/x/crypto/ssh`, `golang.org/x/crypto/ssh/agent`, `github.com/pkg/sftp` |
| **IPC** | Named Pipe (`\\.\pipe\crystal-core-ssh-ipc`) на Windows; Unix Domain Socket на macOS/Linux |

## 3. Функциональные требования
* **Omnibar и Smart Paste (`Ctrl+Alt+S`):** fuzzy-поиск по хостам, парсинг `IP`, `user@host:port`, `ssh -p <port> user@host`, быстрый вход (`Enter` / `Shift+Enter`), смена ключа (`Tab`), запуск One-Click Bootstrap (`Ctrl+Enter`).
* **Мульти-подключение и трекинг сессий:** выделение 2+ серверов (`Ctrl+Click`) с запуском во вкладках или сплит-панелях (`wt.exe -w 0 new-tab / split-pane` с `--title` и `--tabColor`), подсчёт активных сессий через IPC, режим Single-Instance.
* **Key Vault и `ssh-agent`:** генерация `ed25519`, импорт RSA/ECDSA/Ed25519, загрузка приватных ключей в память системного `ssh-agent` без записи на диск (на диске только `.pub`).
* **One-Click Bootstrap:** 4-шаговая настройка нового VPS по паролю с подтверждением HostKey, идемпотентной записью в `~/.ssh/authorized_keys` через `stdin`, верификацией входа строго по ключу и затиранием пароля из памяти.
* **Менеджер SSH-туннелей:** Local (`-L`), Remote (`-R`), Dynamic SOCKS5 (`-D`), keepalive 20 сек, экспоненциальный реконнект (2–30 сек).
* **Quick SFTP Dropzone:** перетаскивание файла на карточку сервера для загрузки в `/tmp` с прогресс-баром.
* **Мониторинг доступности:** фоновый TCP-пинг порта SSH раз в 45 сек (до 10 параллельных горутин) с отображением RTT в мс.
* **Синхронизация с `~/.ssh/config`:** атомарный экспорт в изолированный блок `# BEGIN CRYSTAL-CORE SSH MANAGER ... # END CRYSTAL-CORE SSH MANAGER`.

## 4. Модель данных (SQLite)
Файл БД: `%APPDATA%\CrystalCoreSSHManager\vault.db` (Windows), `~/Library/Application Support/CrystalCoreSSHManager/vault.db` (macOS), `~/.config/CrystalCoreSSHManager/vault.db` (Linux).
* `ssh_keys` — `id`, `name`, `key_type`, `fingerprint`, `public_key`, `private_key_enc` (BLOB), `passphrase_enc` (BLOB), `is_default`.
* `hosts` — `id`, `name`, `hostname`, `port`, `username`, `key_id` (FK), `jump_host_id` (FK), `env_tag`, `tab_color`, `is_temporary`, `last_connected_at`.
* `tunnels` — `id`, `host_id` (FK), `name`, `direction` (`L`/`R`/`D`), `local_addr`, `remote_addr`, `auto_start`.
