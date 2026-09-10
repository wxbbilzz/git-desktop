export namespace engine {
	
	export class BranchDTO {
	    name: string;
	    isHead: boolean;
	    ahead: string;
	    behind: string;
	    upstream: string;
	    subject: string;
	
	    static createFrom(source: any = {}) {
	        return new BranchDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.isHead = source["isHead"];
	        this.ahead = source["ahead"];
	        this.behind = source["behind"];
	        this.upstream = source["upstream"];
	        this.subject = source["subject"];
	    }
	}
	export class CommitDTO {
	    hash: string;
	    shortHash: string;
	    subject: string;
	    author: string;
	    when: string;
	    tags: string[];
	    extraInfo: string;
	
	    static createFrom(source: any = {}) {
	        return new CommitDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hash = source["hash"];
	        this.shortHash = source["shortHash"];
	        this.subject = source["subject"];
	        this.author = source["author"];
	        this.when = source["when"];
	        this.tags = source["tags"];
	        this.extraInfo = source["extraInfo"];
	    }
	}
	export class CommitFileDTO {
	    path: string;
	    oldPath: string;
	    status: string;
	    statusLabel: string;
	    kind: string;
	    additions: number;
	    deletions: number;
	
	    static createFrom(source: any = {}) {
	        return new CommitFileDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.oldPath = source["oldPath"];
	        this.status = source["status"];
	        this.statusLabel = source["statusLabel"];
	        this.kind = source["kind"];
	        this.additions = source["additions"];
	        this.deletions = source["deletions"];
	    }
	}
	export class FileDTO {
	    path: string;
	    previousPath: string;
	    status: string;
	    statusLabel: string;
	    kind: string;
	    isStaged: boolean;
	    isUnstaged: boolean;
	    isTracked: boolean;
	    hasConflicts: boolean;
	    linesAdded: number;
	    linesDeleted: number;
	
	    static createFrom(source: any = {}) {
	        return new FileDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.previousPath = source["previousPath"];
	        this.status = source["status"];
	        this.statusLabel = source["statusLabel"];
	        this.kind = source["kind"];
	        this.isStaged = source["isStaged"];
	        this.isUnstaged = source["isUnstaged"];
	        this.isTracked = source["isTracked"];
	        this.hasConflicts = source["hasConflicts"];
	        this.linesAdded = source["linesAdded"];
	        this.linesDeleted = source["linesDeleted"];
	    }
	}
	export class Param {
	    name: string;
	    label: string;
	    kind: string;
	    required: boolean;
	    default: string;
	    placeholder: string;
	    choices: string[];
	    help: string;
	    flag: string;
	
	    static createFrom(source: any = {}) {
	        return new Param(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.label = source["label"];
	        this.kind = source["kind"];
	        this.required = source["required"];
	        this.default = source["default"];
	        this.placeholder = source["placeholder"];
	        this.choices = source["choices"];
	        this.help = source["help"];
	        this.flag = source["flag"];
	    }
	}
	export class OperationSummary {
	    id: string;
	    category: string;
	    name: string;
	    description: string;
	    params: Param[];
	    dangerous: boolean;
	    readOnly: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OperationSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.category = source["category"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.params = this.convertValues(source["params"], Param);
	        this.dangerous = source["dangerous"];
	        this.readOnly = source["readOnly"];
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
	
	export class RepoSnapshot {
	    repoPath: string;
	    repoName: string;
	    branch: string;
	    isDetached: boolean;
	    identityName: string;
	    identityEmail: string;
	    state: string;
	    files: FileDTO[];
	    commits: CommitDTO[];
	    branches: BranchDTO[];
	
	    static createFrom(source: any = {}) {
	        return new RepoSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repoPath = source["repoPath"];
	        this.repoName = source["repoName"];
	        this.branch = source["branch"];
	        this.isDetached = source["isDetached"];
	        this.identityName = source["identityName"];
	        this.identityEmail = source["identityEmail"];
	        this.state = source["state"];
	        this.files = this.convertValues(source["files"], FileDTO);
	        this.commits = this.convertValues(source["commits"], CommitDTO);
	        this.branches = this.convertValues(source["branches"], BranchDTO);
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
	export class PublishResult {
	    repoUrl: string;
	    cloneUrl: string;
	    command: string;
	    output: string;
	    ok: boolean;
	    error: string;
	    snapshot?: RepoSnapshot;
	
	    static createFrom(source: any = {}) {
	        return new PublishResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repoUrl = source["repoUrl"];
	        this.cloneUrl = source["cloneUrl"];
	        this.command = source["command"];
	        this.output = source["output"];
	        this.ok = source["ok"];
	        this.error = source["error"];
	        this.snapshot = this.convertValues(source["snapshot"], RepoSnapshot);
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
	
	export class RunResult {
	    operationId: string;
	    command: string;
	    output: string;
	    ok: boolean;
	    error: string;
	    snapshot?: RepoSnapshot;
	
	    static createFrom(source: any = {}) {
	        return new RunResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.operationId = source["operationId"];
	        this.command = source["command"];
	        this.output = source["output"];
	        this.ok = source["ok"];
	        this.error = source["error"];
	        this.snapshot = this.convertValues(source["snapshot"], RepoSnapshot);
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

