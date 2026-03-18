export namespace commands {
	
	export class CategoryInfo {
	    id: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new CategoryInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	    }
	}
	export class DiffResult {
	    file: string;
	    diff: string;
	    identical: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DiffResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.diff = source["diff"];
	        this.identical = source["identical"];
	    }
	}
	export class FileResult {
	    ok: boolean;
	    file: string;
	
	    static createFrom(source: any = {}) {
	        return new FileResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.file = source["file"];
	    }
	}
	export class ListFilesResult {
	    manifest: string;
	    tier: string;
	    categories: model.CategoryGroup[];
	    totalFiles: number;
	
	    static createFrom(source: any = {}) {
	        return new ListFilesResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.manifest = source["manifest"];
	        this.tier = source["tier"];
	        this.categories = this.convertValues(source["categories"], model.CategoryGroup);
	        this.totalFiles = source["totalFiles"];
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
	export class ManifestInfo {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new ManifestInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class RepoAddResult {
	    manifest: string;
	    tier: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new RepoAddResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.manifest = source["manifest"];
	        this.tier = source["tier"];
	        this.count = source["count"];
	    }
	}
	export class SetConfigResult {
	    ok: boolean;
	    scope: string;
	
	    static createFrom(source: any = {}) {
	        return new SetConfigResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.scope = source["scope"];
	    }
	}
	export class StateResult {
	    projectDir: string;
	    installDir: string;
	    telemetryLogPath?: string;
	    state: model.InstallState;
	    config: model.Config;
	    manifests?: ManifestInfo[];
	    tiers?: model.ResolvedTier[];
	    categories?: CategoryInfo[];
	    files?: model.FileStatus[];
	
	    static createFrom(source: any = {}) {
	        return new StateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectDir = source["projectDir"];
	        this.installDir = source["installDir"];
	        this.telemetryLogPath = source["telemetryLogPath"];
	        this.state = this.convertValues(source["state"], model.InstallState);
	        this.config = this.convertValues(source["config"], model.Config);
	        this.manifests = this.convertValues(source["manifests"], ManifestInfo);
	        this.tiers = this.convertValues(source["tiers"], model.ResolvedTier);
	        this.categories = this.convertValues(source["categories"], CategoryInfo);
	        this.files = this.convertValues(source["files"], model.FileStatus);
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
	export class SyncResult {
	    action: string;
	    updated?: string[];
	    skipped?: string[];
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new SyncResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.updated = source["updated"];
	        this.skipped = source["skipped"];
	        this.reason = source["reason"];
	    }
	}
	export class TelemetryRunResult {
	    ok: boolean;
	    entry?: model.TelemetryEntry;
	
	    static createFrom(source: any = {}) {
	        return new TelemetryRunResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.entry = this.convertValues(source["entry"], model.TelemetryEntry);
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
	export class UpdateResult {
	    updated: string[];
	    skipped: string[];
	
	    static createFrom(source: any = {}) {
	        return new UpdateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.updated = source["updated"];
	        this.skipped = source["skipped"];
	    }
	}

}

export namespace main {
	
	export class WorkspaceInfo {
	    path: string;
	    name: string;
	    manifest?: string;
	    tier?: string;
	    fileCount: number;
	    exists: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.manifest = source["manifest"];
	        this.tier = source["tier"];
	        this.fileCount = source["fileCount"];
	        this.exists = source["exists"];
	    }
	}

}

export namespace model {
	
	export class ManifestFile {
	    src: string;
	    dest: string;
	    tier: string;
	    category?: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new ManifestFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.src = source["src"];
	        this.dest = source["dest"];
	        this.tier = source["tier"];
	        this.category = source["category"];
	        this.description = source["description"];
	    }
	}
	export class CategoryGroup {
	    Category: string;
	    Label: string;
	    Files: ManifestFile[];
	
	    static createFrom(source: any = {}) {
	        return new CategoryGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Category = source["Category"];
	        this.Label = source["Label"];
	        this.Files = this.convertValues(source["Files"], ManifestFile);
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
	export class Config {
	    repo?: string;
	    branch?: string;
	    manifest: string;
	    tier: string;
	    fileOverrides?: Record<string, string>;
	    skippedVersions?: Record<string, string>;
	    telemetryEnabled?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repo = source["repo"];
	        this.branch = source["branch"];
	        this.manifest = source["manifest"];
	        this.tier = source["tier"];
	        this.fileOverrides = source["fileOverrides"];
	        this.skippedVersions = source["skippedVersions"];
	        this.telemetryEnabled = source["telemetryEnabled"];
	    }
	}
	export class FileStatus {
	    dest: string;
	    displayName: string;
	    category: string;
	    tier: string;
	    description?: string;
	    installed: boolean;
	    inTier: boolean;
	    bundledVersion?: string;
	    installedVersion?: string;
	    updateAvailable: boolean;
	    skipped: boolean;
	    override?: string;
	
	    static createFrom(source: any = {}) {
	        return new FileStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dest = source["dest"];
	        this.displayName = source["displayName"];
	        this.category = source["category"];
	        this.tier = source["tier"];
	        this.description = source["description"];
	        this.installed = source["installed"];
	        this.inTier = source["inTier"];
	        this.bundledVersion = source["bundledVersion"];
	        this.installedVersion = source["installedVersion"];
	        this.updateAvailable = source["updateAvailable"];
	        this.skipped = source["skipped"];
	        this.override = source["override"];
	    }
	}
	export class InstallState {
	    hasGlobalConfig: boolean;
	    hasProjectConfig: boolean;
	    hasInstallMarker: boolean;
	    installedManifest?: string;
	
	    static createFrom(source: any = {}) {
	        return new InstallState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasGlobalConfig = source["hasGlobalConfig"];
	        this.hasProjectConfig = source["hasProjectConfig"];
	        this.hasInstallMarker = source["hasInstallMarker"];
	        this.installedManifest = source["installedManifest"];
	    }
	}
	export class TierDef {
	    id: string;
	    label?: string;
	
	    static createFrom(source: any = {}) {
	        return new TierDef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	    }
	}
	export class Manifest {
	    id: string;
	    name: string;
	    description?: string;
	    basePath: string;
	    tiers?: TierDef[];
	    files: ManifestFile[];
	
	    static createFrom(source: any = {}) {
	        return new Manifest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.basePath = source["basePath"];
	        this.tiers = this.convertValues(source["tiers"], TierDef);
	        this.files = this.convertValues(source["files"], ManifestFile);
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
	
	export class ResolvedTier {
	    id: string;
	    label: string;
	    includes: string[];
	
	    static createFrom(source: any = {}) {
	        return new ResolvedTier(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.includes = source["includes"];
	    }
	}
	export class TelemetryEntry {
	    date: string;
	    timestamp: string;
	    premium_entitlement?: number;
	    premium_remaining?: number;
	    premium_used?: number;
	    quota_reset_date_utc?: string;
	    source: string;
	    version: number;
	
	    static createFrom(source: any = {}) {
	        return new TelemetryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.timestamp = source["timestamp"];
	        this.premium_entitlement = source["premium_entitlement"];
	        this.premium_remaining = source["premium_remaining"];
	        this.premium_used = source["premium_used"];
	        this.quota_reset_date_utc = source["quota_reset_date_utc"];
	        this.source = source["source"];
	        this.version = source["version"];
	    }
	}

}

