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
	    parents: string[];
	
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
	        this.parents = source["parents"];
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
	export class ConflictBlock {
	    index: number;
	    startLine: number;
	    endLine: number;
	    ours: string[];
	    theirs: string[];
	    base: string[];
	    hasBase: boolean;
	    labelOurs: string;
	    labelTheirs: string;
	
	    static createFrom(source: any = {}) {
	        return new ConflictBlock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.startLine = source["startLine"];
	        this.endLine = source["endLine"];
	        this.ours = source["ours"];
	        this.theirs = source["theirs"];
	        this.base = source["base"];
	        this.hasBase = source["hasBase"];
	        this.labelOurs = source["labelOurs"];
	        this.labelTheirs = source["labelTheirs"];
	    }
	}
	export class ConflictChoice {
	    blockIndex: number;
	    choice: string;
	
	    static createFrom(source: any = {}) {
	        return new ConflictChoice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.blockIndex = source["blockIndex"];
	        this.choice = source["choice"];
	    }
	}
	export class ConflictFile {
	    path: string;
	    lines: string[];
	    blocks: ConflictBlock[];
	    markerSize: number;
	
	    static createFrom(source: any = {}) {
	        return new ConflictFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.lines = source["lines"];
	        this.blocks = this.convertValues(source["blocks"], ConflictBlock);
	        this.markerSize = source["markerSize"];
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
	export class FileContentDTO {
	    path: string;
	    content: string;
	    binary: boolean;
	    truncated: boolean;
	    lines: number;
	    size: number;
	    fromIndex: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileContentDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.content = source["content"];
	        this.binary = source["binary"];
	        this.truncated = source["truncated"];
	        this.lines = source["lines"];
	        this.size = source["size"];
	        this.fromIndex = source["fromIndex"];
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
	export class PatchLineDTO {
	    index: number;
	    kind: string;
	    text: string;
	    marker: string;
	    oldNo: number;
	    newNo: number;
	    selectable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PatchLineDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.kind = source["kind"];
	        this.text = source["text"];
	        this.marker = source["marker"];
	        this.oldNo = source["oldNo"];
	        this.newNo = source["newNo"];
	        this.selectable = source["selectable"];
	    }
	}
	export class FilePatch {
	    path: string;
	    staged: boolean;
	    lines: PatchLineDTO[];
	    hasChanges: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FilePatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.staged = source["staged"];
	        this.lines = this.convertValues(source["lines"], PatchLineDTO);
	        this.hasChanges = source["hasChanges"];
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
	export class FolderInfo {
	    path: string;
	    isRepo: boolean;
	    parentRepo: string;
	    fileCount: number;
	
	    static createFrom(source: any = {}) {
	        return new FolderInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.isRepo = source["isRepo"];
	        this.parentRepo = source["parentRepo"];
	        this.fileCount = source["fileCount"];
	    }
	}
	export class RefOption {
	    value: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new RefOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.label = source["label"];
	    }
	}
	export class OperationChoices {
	    branches: RefOption[];
	    refs: RefOption[];
	    commits: RefOption[];
	    files: RefOption[];
	    remotes: RefOption[];
	    tags: RefOption[];
	    stashes: RefOption[];
	
	    static createFrom(source: any = {}) {
	        return new OperationChoices(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branches = this.convertValues(source["branches"], RefOption);
	        this.refs = this.convertValues(source["refs"], RefOption);
	        this.commits = this.convertValues(source["commits"], RefOption);
	        this.files = this.convertValues(source["files"], RefOption);
	        this.remotes = this.convertValues(source["remotes"], RefOption);
	        this.tags = this.convertValues(source["tags"], RefOption);
	        this.stashes = this.convertValues(source["stashes"], RefOption);
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
	    source: string;
	
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
	        this.source = source["source"];
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
	
	
	export class PublishDefaults {
	    remoteUrl: string;
	    remoteName: string;
	    repoName: string;
	
	    static createFrom(source: any = {}) {
	        return new PublishDefaults(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.remoteUrl = source["remoteUrl"];
	        this.remoteName = source["remoteName"];
	        this.repoName = source["repoName"];
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
	    suggestion: string;
	
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
	        this.suggestion = source["suggestion"];
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
	
	export class RepoFileDTO {
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new RepoFileDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
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
	export class StashEntryDTO {
	    index: number;
	    ref: string;
	    message: string;
	    branch: string;
	
	    static createFrom(source: any = {}) {
	        return new StashEntryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.ref = source["ref"];
	        this.message = source["message"];
	        this.branch = source["branch"];
	    }
	}

}

