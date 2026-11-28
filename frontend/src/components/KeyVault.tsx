import React, { useRef, useState } from 'react';
import { Check, Copy, Eye, EyeOff, FileUp, KeyRound, Lock, Plus, Sparkles, Star, Trash2, X } from 'lucide-react';
import { DeleteKey, GenerateKey, ImportKey, LockKeys, SetDefaultKey } from '../../wailsjs/go/app/App';
import { SSHKey } from '../types/ssh';
import { SecretCopyButton } from './SecretCopy';

interface KeyVaultProps {
  keys: SSHKey[];
  onRefresh: () => Promise<void>;
}

export const KeyVault: React.FC<KeyVaultProps> = ({ keys, onRefresh }) => {
  const [modalMode, setModalMode] = useState<null | 'generate' | 'import'>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const [name, setName] = useState('');
  const [passphrase, setPassphrase] = useState('');
  const [showPass, setShowPass] = useState(false);
  const [isDefault, setIsDefault] = useState(false);
  const [privateKeyPEM, setPrivateKeyPEM] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [locking, setLocking] = useState(false);
  const [lockMessage, setLockMessage] = useState<{ text: string; error: boolean } | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const openModal = (mode: 'generate' | 'import') => {
    setName(''); setPassphrase(''); setShowPass(false);
    setIsDefault(keys.length === 0); setPrivateKeyPEM(''); setError(null);
    setModalMode(mode);
  };

  const handleCopyPublicKey = async (key: SSHKey) => {
    if (!key.public_key) return;
    await navigator.clipboard.writeText(key.public_key);
    setCopiedId(key.id);
    setTimeout(() => setCopiedId((c) => (c === key.id ? null : c)), 2000);
  };

  const handleDelete = async (keyId: string) => {
    if (confirmDeleteId !== keyId) {
      setConfirmDeleteId(keyId);
      setTimeout(() => setConfirmDeleteId((c) => (c === keyId ? null : c)), 3000);
      return;
    }
    setConfirmDeleteId(null);
    await DeleteKey(keyId);
    await onRefresh();
  };

  const handleLock = async () => {
    setLocking(true); setLockMessage(null);
    try {
      const removed = await LockKeys();
      setLockMessage({
        text: removed > 0 ? `Ключи выгружены из ssh-agent: ${removed}` : 'В ssh-agent нет ключей из этого хранилища',
        error: false,
      });
    } catch (err: unknown) {
      setLockMessage({ text: err instanceof Error ? err.message : String(err), error: true });
    } finally {
      setLocking(false);
      setTimeout(() => setLockMessage(null), 5000);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    setSubmitting(true); setError(null);
    try {
      if (modalMode === 'generate') {
        await GenerateKey(name.trim(), passphrase, isDefault);
      } else {
        if (!privateKeyPEM.trim()) { setError('Вставьте приватный ключ или выберите файл.'); setSubmitting(false); return; }
        await ImportKey(name.trim(), privateKeyPEM.trim(), passphrase, isDefault);
      }
      setPassphrase(''); setPrivateKeyPEM(''); setModalMode(null);
      await onRefresh();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="flex h-full flex-col overflow-hidden px-5 py-4">
      <div className="mb-4 flex items-center justify-between">
        <div>
          <h1 className="text-base font-semibold text-neutral-900 dark:text-neutral-100">Хранилище ключей (Key Vault)</h1>
          <p className="text-xs text-neutral-500">Приватные ключи зашифрованы AES-256-GCM и загружаются в ssh-agent напрямую из памяти. При сворачивании в трей и выходе выгружаются автоматически</p>
        </div>
        <div className="flex items-center gap-2">
          <button type="button" onClick={handleLock} disabled={locking} title="Выгрузить ключи из системного ssh-agent" className="flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-lg border border-black/10 bg-white/70 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-white disabled:opacity-50 dark:border-white/10 dark:bg-white/5 dark:text-neutral-200">
            <Lock className="h-3.5 w-3.5" /> {locking ? 'Блокировка…' : 'Заблокировать'}
          </button>
          <button type="button" onClick={() => openModal('import')} title="Импортировать приватный ключ" className="flex shrink-0 items-center whitespace-nowrap gap-1.5 rounded-lg border border-black/10 bg-white/70 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-white dark:border-white/10 dark:bg-white/5 dark:text-neutral-200">
            <FileUp className="h-3.5 w-3.5" /> Импорт
          </button>
          <button type="button" onClick={() => openModal('generate')} title="Создать ключ Ed25519" className="flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-500">
            <Plus className="h-3.5 w-3.5" /> Создать
          </button>
        </div>
      </div>

      {lockMessage && (
        <p className={`mb-3 text-xs ${lockMessage.error ? 'text-red-600 dark:text-red-400' : 'text-emerald-600 dark:text-emerald-400'}`}>
          {lockMessage.text}
        </p>
      )}

      {keys.length === 0 ? (
        <div className="flex flex-1 flex-col items-center justify-center rounded-2xl border border-dashed border-black/15 p-8 text-center dark:border-white/15">
          <KeyRound className="mb-3 h-8 w-8 text-neutral-400" />
          <p className="text-sm font-medium text-neutral-700 dark:text-neutral-300">В хранилище пока нет SSH-ключей</p>
          <button type="button" onClick={() => openModal('generate')} className="mt-4 flex items-center gap-1.5 rounded-lg bg-blue-600 px-3.5 py-2 text-xs font-medium text-white hover:bg-blue-500">
            <Sparkles className="h-3.5 w-3.5" /> Сгенерировать первый ключ
          </button>
        </div>
      ) : (
        <div className="flex-1 space-y-2.5 overflow-y-auto pr-1">
          {keys.map((key) => (
            <div key={key.id} className="flex items-center justify-between gap-4 rounded-xl border border-black/10 bg-white/65 p-3.5 backdrop-blur-md dark:border-white/10 dark:bg-neutral-900/60">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <KeyRound className="h-4 w-4 shrink-0 text-amber-500" />
                  <span className="truncate text-sm font-semibold">{key.name}</span>
                  <span className="rounded bg-black/5 px-1.5 py-0.5 font-mono text-[10px] uppercase dark:bg-white/10">{key.key_type}</span>
                  {key.is_default && (
                    <span className="inline-flex items-center gap-1 rounded bg-amber-500/15 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:text-amber-300">
                      <Star className="h-2.5 w-2.5 fill-current" /> По умолчанию
                    </span>
                  )}
                </div>
                <div className="mt-1 truncate font-mono text-xs text-neutral-500">{key.fingerprint}</div>
              </div>
              <div className="flex shrink-0 items-center gap-1.5">
                {!key.is_default && (
                  <button type="button" onClick={async () => { await SetDefaultKey(key.id); await onRefresh(); }} className="rounded-lg px-2.5 py-1.5 text-xs text-neutral-600 hover:bg-black/5 dark:text-neutral-300 dark:hover:bg-white/10">По умолчанию</button>
                )}
                {key.has_passphrase && (
                  <SecretCopyButton source={{ kind: 'key', id: key.id }} label="Фраза" title="Скопировать парольную фразу ключа" className="border border-transparent" />
                )}
                <button type="button" onClick={() => handleCopyPublicKey(key)} disabled={!key.public_key} className="flex items-center gap-1.5 rounded-lg border border-black/10 bg-black/[0.03] px-2.5 py-1.5 text-xs font-medium dark:border-white/10 dark:bg-white/5">
                  {copiedId === key.id ? <><Check className="h-3.5 w-3.5 text-emerald-500" /> Скопировано</> : <><Copy className="h-3.5 w-3.5" /> .pub</>}
                </button>
                <button type="button" onClick={() => handleDelete(key.id)} className={`flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-xs font-medium ${confirmDeleteId === key.id ? 'bg-red-600 text-white' : 'text-neutral-400 hover:bg-red-500/10 hover:text-red-600'}`}>
                  <Trash2 className="h-3.5 w-3.5" /> {confirmDeleteId === key.id && <span>Удалить?</span>}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {modalMode && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm">
          <div className="w-full max-w-[460px] overflow-hidden rounded-2xl border border-black/10 bg-white/95 shadow-2xl dark:border-white/10 dark:bg-neutral-900/95">
            <div className="flex items-center justify-between border-b border-black/10 px-5 py-3.5 dark:border-white/10">
              <h2 className="text-sm font-semibold">{modalMode === 'generate' ? 'Новый ключ Ed25519' : 'Импорт приватного ключа'}</h2>
              <button type="button" onClick={() => setModalMode(null)} className="rounded-lg p-1.5 text-neutral-400 hover:bg-black/5"><X className="h-4 w-4" /></button>
            </div>
            <form onSubmit={handleSubmit} className="space-y-3.5 p-5">
              <div>
                <label className="mb-1 block text-xs font-medium text-neutral-600 dark:text-neutral-400">Название ключа</label>
                <input type="text" required autoFocus value={name} onChange={(e) => setName(e.target.value)} placeholder="Work Ed25519" className="w-full rounded-lg border border-black/10 bg-black/[0.03] px-3 py-1.5 text-sm outline-none focus:border-blue-500 dark:border-white/10 dark:bg-white/5 dark:text-white" />
              </div>
              {modalMode === 'import' && (
                <div>
                  <div className="mb-1 flex items-center justify-between">
                    <label className="text-xs font-medium text-neutral-600 dark:text-neutral-400">Приватный ключ (OpenSSH / PEM)</label>
                    <button type="button" onClick={() => fileInputRef.current?.click()} className="text-xs font-medium text-blue-600 hover:underline">Выбрать файл...</button>
                    <input ref={fileInputRef} type="file" onChange={(e) => {
                      const file = e.target.files?.[0];
                      if (!file) return;
                      const reader = new FileReader();
                      reader.onload = () => { setPrivateKeyPEM(typeof reader.result === 'string' ? reader.result : ''); if (!name) setName(file.name.replace(/\.(pem|key|ppk)$/i, '')); };
                      reader.readAsText(file);
                    }} className="hidden" />
                  </div>
                  <textarea rows={5} value={privateKeyPEM} onChange={(e) => setPrivateKeyPEM(e.target.value)} placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" className="w-full resize-none rounded-lg border border-black/10 bg-black/[0.03] p-2.5 font-mono text-xs outline-none focus:border-blue-500 dark:border-white/10 dark:bg-white/5 dark:text-white" />
                </div>
              )}
              <div>
                <label className="mb-1 block text-xs font-medium text-neutral-600 dark:text-neutral-400">Парольная фраза</label>
                <div className="relative">
                  <input type={showPass ? 'text' : 'password'} value={passphrase} onChange={(e) => setPassphrase(e.target.value)} className="w-full rounded-lg border border-black/10 bg-black/[0.03] py-1.5 pl-3 pr-9 text-sm outline-none focus:border-blue-500 dark:border-white/10 dark:bg-white/5 dark:text-white" />
                  <button type="button" onClick={() => setShowPass((v) => !v)} className="absolute right-2.5 top-1/2 -translate-y-1/2 text-neutral-400">
                    {showPass ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
              </div>
              <label className="flex cursor-pointer items-center gap-2 text-xs">
                <input type="checkbox" checked={isDefault} onChange={(e) => setIsDefault(e.target.checked)} /> Использовать по умолчанию
              </label>
              <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-[11px] leading-relaxed text-amber-800 dark:text-amber-200">
                {modalMode === 'import' ? 'Импортированный' : 'Созданный'} ключ сразу загружается в системный ssh-agent и будет доступен всем программам этого пользователя. При сворачивании в трей, выходе или нажатии «Заблокировать» он выгружается (не дольше 8 часов).
              </div>
              {error && <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-700 dark:text-red-300">{error}</div>}
              <div className="flex justify-end gap-2 pt-2">
                <button type="button" onClick={() => setModalMode(null)} className="rounded-lg px-3.5 py-1.5 text-xs font-medium">Отмена</button>
                <button type="submit" disabled={submitting} className="rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-medium text-white hover:bg-blue-500 disabled:opacity-50">
                  {submitting ? 'Сохранение...' : modalMode === 'generate' ? 'Сгенерировать' : 'Импортировать'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
