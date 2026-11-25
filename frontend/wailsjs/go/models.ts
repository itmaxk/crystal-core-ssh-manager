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

