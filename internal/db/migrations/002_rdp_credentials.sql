-- Учётные данные RDP для локальных туннелей. Пароль зашифрован мастер-ключом
-- хранилища (AES-256-GCM), как и приватные SSH-ключи.
ALTER TABLE tunnels ADD COLUMN rdp_username TEXT NOT NULL DEFAULT '';
ALTER TABLE tunnels ADD COLUMN rdp_domain TEXT NOT NULL DEFAULT '';
ALTER TABLE tunnels ADD COLUMN rdp_password_enc BLOB;
