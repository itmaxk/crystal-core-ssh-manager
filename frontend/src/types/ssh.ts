export type EnvTag = 'prod' | 'stage' | 'dev' | 'infra';

export interface SSHKey {
  id: string;
  name: string;
  key_type: string;
  fingerprint: string;
  public_key?: string;
  is_default: boolean;
}

export interface Host {
  id: string;
  name: string;
  hostname: string;
  port: number;
  username: string;
  key_id?: string | null;
  jump_host_id?: string | null;
  env_tag: EnvTag;
  tab_color: string;
  is_temporary: boolean;
  last_connected_at?: string | null;
}

export interface HostDraft {
  id?: string;
  name: string;
  hostname: string;
  port: number;
  username: string;
  key_id: string;
  jump_host_id: string;
  env_tag: EnvTag;
  tab_color: string;
  is_temporary: boolean;
}

export type TunnelDirection = 'L' | 'R' | 'D';
export type TunnelState = 'stopped' | 'connecting' | 'active' | 'reconnecting' | 'error';

export interface TunnelSummary {
  id: string;
  host_id: string;
  name: string;
  direction: TunnelDirection;
  local_addr: string;
  remote_addr: string;
  auto_start: boolean;
  state: TunnelState;
  active_conns: number;
  last_error?: string;
  rdp_username: string;
  rdp_domain: string;
  has_rdp_password: boolean;
}

export interface TunnelDraft {
  id?: string;
  host_id: string;
  name: string;
  direction: TunnelDirection;
  local_addr: string;
  remote_addr: string;
  auto_start: boolean;
  /** Вход в RDP: пароль только новый (сохранённый в UI не попадает). */
  rdp_username?: string;
  rdp_domain?: string;
  rdp_password?: string;
  has_rdp_password?: boolean;
  rdp_forget_password?: boolean;
}

export interface ParsedTarget {
  username: string;
  hostname: string;
  port: number;
}
