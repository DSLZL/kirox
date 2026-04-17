export namespace email {
	
	export class MoeMailConfig {
	    name: string;
	    url: string;
	    apiKey: string;
	
	    static createFrom(source: any = {}) {
	        return new MoeMailConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.apiKey = source["apiKey"];
	    }
	}

}

export namespace proxy {
	
	export class ProxyPolicy {
	    selection_mode: string;
	    allow_countries?: string[];
	    block_countries?: string[];
	    allow_continents?: string[];
	    allow_regions?: string[];
	    allow_ip_types?: string[];
	    otp400_retry_mode: string;
	    otp400_action: string;
	    otp400_cooldown_min: number;
	    otp400_max_retries: number;
	    ban_action: string;
	    ban_cooldown_min: number;
	    ban_max_count: number;
	    conn_fail_action: string;
	    conn_fail_cooldown_min: number;
	    conn_fail_max_retries: number;
	    auto_recover: boolean;
	    blacklist_permanent: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProxyPolicy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.selection_mode = source["selection_mode"];
	        this.allow_countries = source["allow_countries"];
	        this.block_countries = source["block_countries"];
	        this.allow_continents = source["allow_continents"];
	        this.allow_regions = source["allow_regions"];
	        this.allow_ip_types = source["allow_ip_types"];
	        this.otp400_retry_mode = source["otp400_retry_mode"];
	        this.otp400_action = source["otp400_action"];
	        this.otp400_cooldown_min = source["otp400_cooldown_min"];
	        this.otp400_max_retries = source["otp400_max_retries"];
	        this.ban_action = source["ban_action"];
	        this.ban_cooldown_min = source["ban_cooldown_min"];
	        this.ban_max_count = source["ban_max_count"];
	        this.conn_fail_action = source["conn_fail_action"];
	        this.conn_fail_cooldown_min = source["conn_fail_cooldown_min"];
	        this.conn_fail_max_retries = source["conn_fail_max_retries"];
	        this.auto_recover = source["auto_recover"];
	        this.blacklist_permanent = source["blacklist_permanent"];
	    }
	}

}

export namespace task {
	
	export class StartTaskRequest {
	    count: number;
	    concurrency: number;
	    delay: number;
	    proxy: string;
	    outputPath: string;
	    emailProvider: string;
	    moemailDomains: string[];
	    moemailConfigs: Record<string, Array<email.MoeMailConfig>>;
	    moemailRandomMode: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StartTaskRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.count = source["count"];
	        this.concurrency = source["concurrency"];
	        this.delay = source["delay"];
	        this.proxy = source["proxy"];
	        this.outputPath = source["outputPath"];
	        this.emailProvider = source["emailProvider"];
	        this.moemailDomains = source["moemailDomains"];
	        this.moemailConfigs = this.convertValues(source["moemailConfigs"], Array<email.MoeMailConfig>, true);
	        this.moemailRandomMode = source["moemailRandomMode"];
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

}

