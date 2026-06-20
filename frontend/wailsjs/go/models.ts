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
	export class PeijianResult {
	    success: boolean;
	    error?: string;
	    summary: Record<string, number>;
	    total: number;
	    outputDir: string;
	    outputPath: string;
	
	    static createFrom(source: any = {}) {
	        return new PeijianResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.summary = source["summary"];
	        this.total = source["total"];
	        this.outputDir = source["outputDir"];
	        this.outputPath = source["outputPath"];
	    }
	}

}

