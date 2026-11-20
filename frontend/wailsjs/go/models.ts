export namespace auth {
	
	export class KeyInfo {
	    clientEmail: string;
	    projectId: string;
	    keyId: string;
	
	    static createFrom(source: any = {}) {
	        return new KeyInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientEmail = source["clientEmail"];
	        this.projectId = source["projectId"];
	        this.keyId = source["keyId"];
	    }
	}
	export class OAuthStatus {
	    available: boolean;
	    client: string;
	    projectId: string;
	    account: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new OAuthStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.client = source["client"];
	        this.projectId = source["projectId"];
	        this.account = source["account"];
	        this.error = source["error"];
	    }
	}

}

export namespace core {
	
	export class Footprint {
	    projectId: string;
	    number: string;
	    vms: number;
	    running: number;
	    disks: number;
	    other: number;
	
	    static createFrom(source: any = {}) {
	        return new Footprint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectId = source["projectId"];
	        this.number = source["number"];
	        this.vms = source["vms"];
	        this.running = source["running"];
	        this.disks = source["disks"];
	        this.other = source["other"];
	    }
	}
	export class Modules {
	    osconfig: boolean;
	    monitoring: boolean;
	    recommender: boolean;
	    metricsDays: number;
	    pricing: boolean;
	    currency: string;
	    billingTable: string;
	    billingMonth: string;
	    billingMaxGB: number;
	
	    static createFrom(source: any = {}) {
	        return new Modules(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.osconfig = source["osconfig"];
	        this.monitoring = source["monitoring"];
	        this.recommender = source["recommender"];
	        this.metricsDays = source["metricsDays"];
	        this.pricing = source["pricing"];
	        this.currency = source["currency"];
	        this.billingTable = source["billingTable"];
	        this.billingMonth = source["billingMonth"];
	        this.billingMaxGB = source["billingMaxGB"];
	    }
	}
	export class Project {
	    id: string;
	    number: string;
	    name: string;
	    parent: string;
	    state: string;
	    created: string;
	    labels: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.number = source["number"];
	        this.name = source["name"];
	        this.parent = source["parent"];
	        this.state = source["state"];
	        this.created = source["created"];
	        this.labels = source["labels"];
	    }
	}
	export class ScanOptions {
	    concurrency: number;
	    health: health.Options;
	    modules: Modules;
	    saveHistory: boolean;
	    profile: string;
	
	    static createFrom(source: any = {}) {
	        return new ScanOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.concurrency = source["concurrency"];
	        this.health = this.convertValues(source["health"], health.Options);
	        this.modules = this.convertValues(source["modules"], Modules);
	        this.saveHistory = source["saveHistory"];
	        this.profile = source["profile"];
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

export namespace health {
	
	export class Options {
	    stoppedDays: number;
	    snapshotDays: number;
	    imageDays: number;
	    cpuLowPct: number;
	    cpuHighPct: number;
	    memHighPct: number;
	    quotaPct: number;
	    cudDays: number;
	    disabled: string[];
	
	    static createFrom(source: any = {}) {
	        return new Options(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stoppedDays = source["stoppedDays"];
	        this.snapshotDays = source["snapshotDays"];
	        this.imageDays = source["imageDays"];
	        this.cpuLowPct = source["cpuLowPct"];
	        this.cpuHighPct = source["cpuHighPct"];
	        this.memHighPct = source["memHighPct"];
	        this.quotaPct = source["quotaPct"];
	        this.cudDays = source["cudDays"];
	        this.disabled = source["disabled"];
	    }
	}
	export class Rule {
	    id: string;
	    title: string;
	    category: string;
	    severity: string;
	    param?: string;
	    labels: string;
	
	    static createFrom(source: any = {}) {
	        return new Rule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.category = source["category"];
	        this.severity = source["severity"];
	        this.param = source["param"];
	        this.labels = source["labels"];
	    }
	}

}

export namespace history {
	
	export class Summary {
	    id: string;
	    // Go type: time
	    startedAt: any;
	    identity: string;
	    version: string;
	    profile: string;
	    projects: number;
	    projectsOk: number;
	    cancelled: boolean;
	    vms: number;
	    running: number;
	    vcpu: number;
	    memoryGB: number;
	    disks: number;
	    diskGB: number;
	    orphans: number;
	    currency: string;
	    costList: number;
	    costReal: number;
	    savings: number;
	    high: number;
	    medium: number;
	    low: number;
	    sizeBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.identity = source["identity"];
	        this.version = source["version"];
	        this.profile = source["profile"];
	        this.projects = source["projects"];
	        this.projectsOk = source["projectsOk"];
	        this.cancelled = source["cancelled"];
	        this.vms = source["vms"];
	        this.running = source["running"];
	        this.vcpu = source["vcpu"];
	        this.memoryGB = source["memoryGB"];
	        this.disks = source["disks"];
	        this.diskGB = source["diskGB"];
	        this.orphans = source["orphans"];
	        this.currency = source["currency"];
	        this.costList = source["costList"];
	        this.costReal = source["costReal"];
	        this.savings = source["savings"];
	        this.high = source["high"];
	        this.medium = source["medium"];
	        this.low = source["low"];
	        this.sizeBytes = source["sizeBytes"];
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

export namespace main {
	
	export class ADCStatus {
	    found: boolean;
	    identity: string;
	
	    static createFrom(source: any = {}) {
	        return new ADCStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.found = source["found"];
	        this.identity = source["identity"];
	    }
	}
	export class AboutInfo {
	    version: string;
	    website: string;
	    donate: string;
	    license: string;
	    licenseUrl: string;
	    releases: string;
	
	    static createFrom(source: any = {}) {
	        return new AboutInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.website = source["website"];
	        this.donate = source["donate"];
	        this.license = source["license"];
	        this.licenseUrl = source["licenseUrl"];
	        this.releases = source["releases"];
	    }
	}
	export class SessionInfo {
	    identity: string;
	    method: string;
	    projectHint: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.identity = source["identity"];
	        this.method = source["method"];
	        this.projectHint = source["projectHint"];
	    }
	}
	export class SnapshotInfo {
	    path: string;
	    name: string;
	    // Go type: time
	    startedAt: any;
	    identity: string;
	    version: string;
	    projects: number;
	    vms: number;
	
	    static createFrom(source: any = {}) {
	        return new SnapshotInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.identity = source["identity"];
	        this.version = source["version"];
	        this.projects = source["projects"];
	        this.vms = source["vms"];
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

export namespace model {
	
	export class Column {
	    key: string;
	    title: string;
	    type: string;
	    width?: number;
	
	    static createFrom(source: any = {}) {
	        return new Column(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.title = source["title"];
	        this.type = source["type"];
	        this.width = source["width"];
	    }
	}
	export class ScanError {
	    project: string;
	    collector: string;
	    kind: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ScanError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.project = source["project"];
	        this.collector = source["collector"];
	        this.kind = source["kind"];
	        this.message = source["message"];
	    }
	}
	export class Sheet {
	    name: string;
	    title: string;
	    description: string;
	    columns: Column[];
	    rows: any[];
	
	    static createFrom(source: any = {}) {
	        return new Sheet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.columns = this.convertValues(source["columns"], Column);
	        this.rows = source["rows"];
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
	export class Meta {
	    tool: string;
	    version: string;
	    identity: string;
	    authMethod: string;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    finishedAt: any;
	    projects: number;
	    projectsOk: number;
	    errors: number;
	    cancelled: boolean;
	    currency: string;
	
	    static createFrom(source: any = {}) {
	        return new Meta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.version = source["version"];
	        this.identity = source["identity"];
	        this.authMethod = source["authMethod"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.finishedAt = this.convertValues(source["finishedAt"], null);
	        this.projects = source["projects"];
	        this.projectsOk = source["projectsOk"];
	        this.errors = source["errors"];
	        this.cancelled = source["cancelled"];
	        this.currency = source["currency"];
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
	export class Inventory {
	    meta: Meta;
	    sheets: Sheet[];
	    errors: ScanError[];
	
	    static createFrom(source: any = {}) {
	        return new Inventory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.meta = this.convertValues(source["meta"], Meta);
	        this.sheets = this.convertValues(source["sheets"], Sheet);
	        this.errors = this.convertValues(source["errors"], ScanError);
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

export namespace profile {
	
	export class Profile {
	    name: string;
	    auth?: string;
	    keyPath?: string;
	    impersonate?: string;
	    query?: string;
	    projects?: string[];
	    discover?: string;
	    concurrency?: number;
	    modules: core.Modules;
	    health: health.Options;
	    // Go type: time
	    updated: any;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.auth = source["auth"];
	        this.keyPath = source["keyPath"];
	        this.impersonate = source["impersonate"];
	        this.query = source["query"];
	        this.projects = source["projects"];
	        this.discover = source["discover"];
	        this.concurrency = source["concurrency"];
	        this.modules = this.convertValues(source["modules"], core.Modules);
	        this.health = this.convertValues(source["health"], health.Options);
	        this.updated = this.convertValues(source["updated"], null);
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

export namespace update {
	
	export class Info {
	    current: string;
	    latest: string;
	    available: boolean;
	    url: string;
	    notes: string;
	    published: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.available = source["available"];
	        this.url = source["url"];
	        this.notes = source["notes"];
	        this.published = source["published"];
	    }
	}

}

