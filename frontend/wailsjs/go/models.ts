export namespace app {
	
	export class AddKeyInput {
	    name: string;
	    key_type: string;
	    fingerprint: string;
	    public_key: string;
	    private_key_pem: string;
	    passphrase: string;
	    is_default: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AddKeyInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.key_type = source["key_type"];
	        this.fingerprint = source["fingerprint"];
	        this.public_key = source["public_key"];
	        this.private_key_pem = source["private_key_pem"];
	        this.passphrase = source["passphrase"];
	        this.is_default = source["is_default"];
	    }
	}
	export class AgentGrant {
	    host_id: string;
	    host_name: string;
	    mode: string;
	    source_ip?: string;
	    permit_open?: string;
	    ssh_command: string;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentGrant(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host_id = source["host_id"];
	        this.host_name = source["host_name"];
	        this.mode = source["mode"];
	        this.source_ip = source["source_ip"];
	        this.permit_open = source["permit_open"];
	        this.ssh_command = source["ssh_command"];
	        this.created_at = source["created_at"];
	    }
	}
	export class AgentAccessState {
	    supported: boolean;
	    username: string;
	    home: string;
	    public_key: string;
	    created_at: string;
	    user_exists: boolean;
	    grants: AgentGrant[];
	
	    static createFrom(source: any = {}) {
	        return new AgentAccessState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.supported = source["supported"];
	        this.username = source["username"];
	        this.home = source["home"];
	        this.public_key = source["public_key"];
	        this.created_at = source["created_at"];
	        this.user_exists = source["user_exists"];
	        this.grants = this.convertValues(source["grants"], AgentGrant);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class AgentGrantInput {
	    host_id: string;
	    mode: string;
	    source_ip: string;
	    permit_open: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentGrantInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host_id = source["host_id"];
	        this.mode = source["mode"];
	        this.source_ip = source["source_ip"];
	        this.permit_open = source["permit_open"];
	    }
	}
	export class AgentSetupInput {
	    username: string;
	    close_home: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AgentSetupInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.close_home = source["close_home"];
	    }
	}
	export class AgentSetupResult {
	    state: AgentAccessState;
	    needs_elevation: boolean;
	    script_path?: string;
	    command?: string;
	    output?: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentSetupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = this.convertValues(source["state"], AgentAccessState);
	        this.needs_elevation = source["needs_elevation"];
	        this.script_path = source["script_path"];
	        this.command = source["command"];
	        this.output = source["output"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BootstrapInput {
	    hostname: string;
	    port: number;
	    username: string;
	    password: string;
	    key_id: string;
	
	    static createFrom(source: any = {}) {
	        return new BootstrapInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hostname = source["hostname"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.key_id = source["key_id"];
	    }
	}
	export class DiscoveredService {
	    port: number;
	    name: string;
	    process?: string;
	    bind: string;
	    addresses: string[];
	    remote_addr: string;
	    local_port: number;
	    is_rdp: boolean;
	    system: boolean;
	    reason?: string;
	    existing_tunnel?: string;
	
	    static createFrom(source: any = {}) {
	        return new DiscoveredService(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.name = source["name"];
	        this.process = source["process"];
	        this.bind = source["bind"];
	        this.addresses = source["addresses"];
	        this.remote_addr = source["remote_addr"];
	        this.local_port = source["local_port"];
	        this.is_rdp = source["is_rdp"];
	        this.system = source["system"];
	        this.reason = source["reason"];
	        this.existing_tunnel = source["existing_tunnel"];
	    }
	}
	export class DiscoveryResult {
	    os: string;
	    is_ubuntu: boolean;
	    services: DiscoveredService[];
	
	    static createFrom(source: any = {}) {
	        return new DiscoveryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os = source["os"];
	        this.is_ubuntu = source["is_ubuntu"];
	        this.services = this.convertValues(source["services"], DiscoveredService);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ImportSummary {
	    keys_added: number;
	    keys_existing: number;
	    hosts_added: number;
	    hosts_updated: number;
	    tunnels_added: number;
	    tunnels_updated: number;
	    skipped: number;
	
	    static createFrom(source: any = {}) {
	        return new ImportSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.keys_added = source["keys_added"];
	        this.keys_existing = source["keys_existing"];
	        this.hosts_added = source["hosts_added"];
	        this.hosts_updated = source["hosts_updated"];
	        this.tunnels_added = source["tunnels_added"];
	        this.tunnels_updated = source["tunnels_updated"];
	        this.skipped = source["skipped"];
	    }
	}
	export class RDPCredentialsInput {
	    tunnel_id: string;
	    username: string;
	    domain: string;
	    password: string;
	    keep_password: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RDPCredentialsInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tunnel_id = source["tunnel_id"];
	        this.username = source["username"];
	        this.domain = source["domain"];
	        this.password = source["password"];
	        this.keep_password = source["keep_password"];
	    }
	}
	export class SecretCopied {
	    label: string;
	    seconds: number;
	
	    static createFrom(source: any = {}) {
	        return new SecretCopied(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.seconds = source["seconds"];
	    }
	}

}

export namespace db {
	
	export class Host {
	    id: string;
	    name: string;
	    hostname: string;
	    port: number;
	    username: string;
	    key_id?: string;
	    jump_host_id?: string;
	    env_tag: string;
	    tab_color: string;
	    is_temporary: boolean;
	    last_connected_at?: string;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new Host(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.hostname = source["hostname"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.key_id = source["key_id"];
	        this.jump_host_id = source["jump_host_id"];
	        this.env_tag = source["env_tag"];
	        this.tab_color = source["tab_color"];
	        this.is_temporary = source["is_temporary"];
	        this.last_connected_at = source["last_connected_at"];
	        this.created_at = source["created_at"];
	    }
	}
	export class SSHKey {
	    id: string;
	    name: string;
	    key_type: string;
	    fingerprint: string;
	    public_key: string;
	    has_passphrase: boolean;
	    is_default: boolean;
	    created_at: string;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new SSHKey(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.key_type = source["key_type"];
	        this.fingerprint = source["fingerprint"];
	        this.public_key = source["public_key"];
	        this.has_passphrase = source["has_passphrase"];
	        this.is_default = source["is_default"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	    }
	}
	export class Tunnel {
	    id: string;
	    host_id: string;
	    name: string;
	    direction: string;
	    local_addr: string;
	    remote_addr: string;
	    auto_start: boolean;
	    rdp_username: string;
	    rdp_domain: string;
	    has_rdp_password: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Tunnel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.host_id = source["host_id"];
	        this.name = source["name"];
	        this.direction = source["direction"];
	        this.local_addr = source["local_addr"];
	        this.remote_addr = source["remote_addr"];
	        this.auto_start = source["auto_start"];
	        this.rdp_username = source["rdp_username"];
	        this.rdp_domain = source["rdp_domain"];
	        this.has_rdp_password = source["has_rdp_password"];
	    }
	}

}

export namespace ssh {
	
	export class BootstrapResult {
	    host_key_fingerprint: string;
	    key_already_present: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BootstrapResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host_key_fingerprint = source["host_key_fingerprint"];
	        this.key_already_present = source["key_already_present"];
	    }
	}
	export class ImportedHost {
	    name: string;
	    hostname: string;
	    port: number;
	    username: string;
	    identity_file?: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportedHost(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.hostname = source["hostname"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.identity_file = source["identity_file"];
	    }
	}
	export class TunnelStatus {
	    tunnel_id: string;
	    state: string;
	    active_conns: number;
	    last_error?: string;
	
	    static createFrom(source: any = {}) {
	        return new TunnelStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tunnel_id = source["tunnel_id"];
	        this.state = source["state"];
	        this.active_conns = source["active_conns"];
	        this.last_error = source["last_error"];
	    }
	}

}

export namespace update {
	
	export class Info {
	    current: string;
	    latest: string;
	    available: boolean;
	    notes: string;
	    url: string;
	    published: string;
	    asset_name?: string;
	    asset_size?: number;
	    can_install: boolean;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.available = source["available"];
	        this.notes = source["notes"];
	        this.url = source["url"];
	        this.published = source["published"];
	        this.asset_name = source["asset_name"];
	        this.asset_size = source["asset_size"];
	        this.can_install = source["can_install"];
	        this.reason = source["reason"];
	    }
	}

}

