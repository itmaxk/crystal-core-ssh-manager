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
  env_tag: EnvTag;
  tab_color: string;
  is_temporary: boolean;
  last_connected_at?: string | null;
}

export interface TunnelSummary {
  id: string;
  name: string;
  local_addr: string;
  state: 'stopped' | 'connecting' | 'active' | 'reconnecting' | 'error';
  active_conns: number;
}

export interface ParsedTarget {
  username: string;
  hostname: string;
  port: number;
}
