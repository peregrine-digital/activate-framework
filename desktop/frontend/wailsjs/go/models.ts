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

