export namespace main {

	export class BatchJob {
	    JobID: string;
	    Status: string;
	    // Go type: time
	    SubmittedAt: any;
	    Progress: string;

	    static createFrom(source: any = {}) {
	        return new BatchJob(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.JobID = source["JobID"];
	        this.Status = source["Status"];
	        this.SubmittedAt = this.convertValues(source["SubmittedAt"], null);
	        this.Progress = source["Progress"];
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
	export class SafetySetting {
	    category: string;
	    threshold: string;

	    static createFrom(source: any = {}) {
	        return new SafetySetting(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.category = source["category"];
	        this.threshold = source["threshold"];
	    }
	}
	export class Config {
	    api_key: string;
	    output_dir: string;
	    default_prompt: string;
	    default_neg_prompt: string;
	    encourage_edt: string;
	    encourage_gen: string;
	    debug: boolean;
	    safety_settings: SafetySetting[];
	    temperature: number;
	    top_p: number;
	    top_k: number;
	    max_output_tokens: number;
	    model_nano_flash: string;
	    model_nano_pro: string;
	    model_nano_2: string;
	    model_imagen: string;
	    model_imagen_ultra: string;
	    window_width: number;
	    window_height: number;
	    is_maximized: boolean;
	    split_offset_main: number;
	    split_offset_left: number;
	    split_offset_top: number;
	    log_split_offset: number;

	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.api_key = source["api_key"];
	        this.output_dir = source["output_dir"];
	        this.default_prompt = source["default_prompt"];
	        this.default_neg_prompt = source["default_neg_prompt"];
	        this.encourage_edt = source["encourage_edt"];
	        this.encourage_gen = source["encourage_gen"];
	        this.debug = source["debug"];
	        this.safety_settings = this.convertValues(source["safety_settings"], SafetySetting);
	        this.temperature = source["temperature"];
	        this.top_p = source["top_p"];
	        this.top_k = source["top_k"];
	        this.max_output_tokens = source["max_output_tokens"];
	        this.model_nano_flash = source["model_nano_flash"];
	        this.model_nano_pro = source["model_nano_pro"];
	        this.model_nano_2 = source["model_nano_2"];
	        this.model_imagen = source["model_imagen"];
	        this.model_imagen_ultra = source["model_imagen_ultra"];
	        this.window_width = source["window_width"];
	        this.window_height = source["window_height"];
	        this.is_maximized = source["is_maximized"];
	        this.split_offset_main = source["split_offset_main"];
	        this.split_offset_left = source["split_offset_left"];
	        this.split_offset_top = source["split_offset_top"];
	        this.log_split_offset = source["log_split_offset"];
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
	export class InlineData {
	    mimeType: string;
	    data: string;

	    static createFrom(source: any = {}) {
	        return new InlineData(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mimeType = source["mimeType"];
	        this.data = source["data"];
	    }
	}
	export class Part {
	    text?: string;
	    inlineData?: InlineData;

	    static createFrom(source: any = {}) {
	        return new Part(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.inlineData = this.convertValues(source["inlineData"], InlineData);
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
	export class Content {
	    parts: Part[];

	    static createFrom(source: any = {}) {
	        return new Content(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.parts = this.convertValues(source["parts"], Part);
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
	export class ImageConfig {
	    aspectRatio?: string;
	    imageSize?: string;

	    static createFrom(source: any = {}) {
	        return new ImageConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.aspectRatio = source["aspectRatio"];
	        this.imageSize = source["imageSize"];
	    }
	}
	export class GenerationConfig {
	    candidateCount: number;
	    responseModalities?: string[];
	    temperature?: number;
	    topP?: number;
	    topK?: number;
	    maxOutputTokens?: number;
	    imageConfig?: ImageConfig;

	    static createFrom(source: any = {}) {
	        return new GenerationConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.candidateCount = source["candidateCount"];
	        this.responseModalities = source["responseModalities"];
	        this.temperature = source["temperature"];
	        this.topP = source["topP"];
	        this.topK = source["topK"];
	        this.maxOutputTokens = source["maxOutputTokens"];
	        this.imageConfig = this.convertValues(source["imageConfig"], ImageConfig);
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
	export class GeminiRequest {
	    contents: Content[];
	    systemInstruction?: Content;
	    safetySettings: SafetySetting[];
	    generationConfig?: GenerationConfig;

	    static createFrom(source: any = {}) {
	        return new GeminiRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.contents = this.convertValues(source["contents"], Content);
	        this.systemInstruction = this.convertValues(source["systemInstruction"], Content);
	        this.safetySettings = this.convertValues(source["safetySettings"], SafetySetting);
	        this.generationConfig = this.convertValues(source["generationConfig"], GenerationConfig);
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


	export class ImageInfo {
	    ID: string;
	    FileName: string;
	    FullPath: string;
	    SizeMB: number;
	    TaskCount: number;
	    Selected: boolean;

	    static createFrom(source: any = {}) {
	        return new ImageInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.FileName = source["FileName"];
	        this.FullPath = source["FullPath"];
	        this.SizeMB = source["SizeMB"];
	        this.TaskCount = source["TaskCount"];
	        this.Selected = source["Selected"];
	    }
	}



	export class TaskInfo {
	    ID: number;
	    ImgIDs: string;
	    Agent: string;
	    Size: string;
	    Ratio: string;
	    Status: string;
	    Cost: number;
	    Prompt: string;
	    NegativePrompt: string;
	    Format: string;
	    Disabled: boolean;
	    SourcePath: string;

	    static createFrom(source: any = {}) {
	        return new TaskInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.ImgIDs = source["ImgIDs"];
	        this.Agent = source["Agent"];
	        this.Size = source["Size"];
	        this.Ratio = source["Ratio"];
	        this.Status = source["Status"];
	        this.Cost = source["Cost"];
	        this.Prompt = source["Prompt"];
	        this.NegativePrompt = source["NegativePrompt"];
	        this.Format = source["Format"];
	        this.Disabled = source["Disabled"];
	        this.SourcePath = source["SourcePath"];
	    }
	}

}
