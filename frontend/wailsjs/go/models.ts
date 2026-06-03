export namespace dangkou {
	
	export class StallRule {
	    name: string;
	    appleModel: boolean;
	    keywords: string[];
	
	    static createFrom(source: any = {}) {
	        return new StallRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.appleModel = source["appleModel"];
	        this.keywords = source["keywords"];
	    }
	}
	export class Config {
	    codeFilter: string;
	    stalls: StallRule[];
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.codeFilter = source["codeFilter"];
	        this.stalls = this.convertValues(source["stalls"], StallRule);
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
	
	export class DangkouResult {
	    success: boolean;
	    error?: string;
	    summary: Record<string, number>;
	    total: number;
	    outputDir: string;
	
	    static createFrom(source: any = {}) {
	        return new DangkouResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.summary = source["summary"];
	        this.total = source["total"];
	        this.outputDir = source["outputDir"];
	    }
	}
	export class FilterConfig {
	    doubtKeywords: string[];
	    accessoryKeywords: string[];
	
	    static createFrom(source: any = {}) {
	        return new FilterConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.doubtKeywords = source["doubtKeywords"];
	        this.accessoryKeywords = source["accessoryKeywords"];
	    }
	}
	export class FilterResult {
	    success: boolean;
	    error?: string;
	    summary: any;
	    outputDir: string;
	
	    static createFrom(source: any = {}) {
	        return new FilterResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.summary = source["summary"];
	        this.outputDir = source["outputDir"];
	    }
	}
	export class PeijianConfig {
	    parts: peijian.PartsConfig;
	    columns: peijian.ColumnConfig;
	
	    static createFrom(source: any = {}) {
	        return new PeijianConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.parts = this.convertValues(source["parts"], peijian.PartsConfig);
	        this.columns = this.convertValues(source["columns"], peijian.ColumnConfig);
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
	export class PeijianExtractResult {
	    success: boolean;
	    error?: string;
	    summary: any;
	    outputDir: string;
	    pendingPath: string;
	
	    static createFrom(source: any = {}) {
	        return new PeijianExtractResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.summary = source["summary"];
	        this.outputDir = source["outputDir"];
	        this.pendingPath = source["pendingPath"];
	    }
	}
	export class PeijianMergeResult {
	    success: boolean;
	    error?: string;
	    entries: peijian.MergeEntry[];
	    totalKinds: number;
	    totalQty: number;
	    outputPath: string;
	
	    static createFrom(source: any = {}) {
	        return new PeijianMergeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.entries = this.convertValues(source["entries"], peijian.MergeEntry);
	        this.totalKinds = source["totalKinds"];
	        this.totalQty = source["totalQty"];
	        this.outputPath = source["outputPath"];
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

export namespace peijian {
	
	export class ColumnConfig {
	    spec: string;
	    buyerMsg: string;
	    sellerNote: string;
	    quantity: string;
	
	    static createFrom(source: any = {}) {
	        return new ColumnConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.spec = source["spec"];
	        this.buyerMsg = source["buyerMsg"];
	        this.sellerNote = source["sellerNote"];
	        this.quantity = source["quantity"];
	    }
	}
	export class MergeEntry {
	    name: string;
	    color: string;
	    qty: number;
	
	    static createFrom(source: any = {}) {
	        return new MergeEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.color = source["color"];
	        this.qty = source["qty"];
	    }
	}
	export class PartsConfig {
	    accessories: string[];
	
	    static createFrom(source: any = {}) {
	        return new PartsConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accessories = source["accessories"];
	    }
	}

}

