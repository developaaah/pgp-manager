export namespace model {
	
	export class AppConfig {
	    Theme: string;
	    PassphraseCacheTTLMinutes: number;
	    KeysDir: string;
	    CustomKeyservers: string[];
	    StartInTray: boolean;
	    ClipDetectMessages: boolean;
	    ClipDetectPublicKeys: string;
	    ClipDetectPrivateKeys: string;
	    ClipDetectSignatures: boolean;
	    AutoCopyResults: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Theme = source["Theme"];
	        this.PassphraseCacheTTLMinutes = source["PassphraseCacheTTLMinutes"];
	        this.KeysDir = source["KeysDir"];
	        this.CustomKeyservers = source["CustomKeyservers"];
	        this.StartInTray = source["StartInTray"];
	        this.ClipDetectMessages = source["ClipDetectMessages"];
	        this.ClipDetectPublicKeys = source["ClipDetectPublicKeys"];
	        this.ClipDetectPrivateKeys = source["ClipDetectPrivateKeys"];
	        this.ClipDetectSignatures = source["ClipDetectSignatures"];
	        this.AutoCopyResults = source["AutoCopyResults"];
	    }
	}
	export class DecryptResult {
	    Plaintext: string;
	    SignedBy: string;
	    Error: string;
	
	    static createFrom(source: any = {}) {
	        return new DecryptResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Plaintext = source["Plaintext"];
	        this.SignedBy = source["SignedBy"];
	        this.Error = source["Error"];
	    }
	}
	export class EncryptResult {
	    Armored: string;
	    Error: string;
	
	    static createFrom(source: any = {}) {
	        return new EncryptResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Armored = source["Armored"];
	        this.Error = source["Error"];
	    }
	}
	export class FileResult {
	    OutputPath: string;
	    Error: string;
	
	    static createFrom(source: any = {}) {
	        return new FileResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.OutputPath = source["OutputPath"];
	        this.Error = source["Error"];
	    }
	}
	export class KeyImportEntry {
	    Fingerprint: string;
	    UID: string;
	    Error: string;
	
	    static createFrom(source: any = {}) {
	        return new KeyImportEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Fingerprint = source["Fingerprint"];
	        this.UID = source["UID"];
	        this.Error = source["Error"];
	    }
	}
	export class SubkeyInfo {
	    Fingerprint: string;
	    Algorithm: string;
	    Usage: string[];
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    ExpiresAt?: any;
	    IsRevoked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SubkeyInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Fingerprint = source["Fingerprint"];
	        this.Algorithm = source["Algorithm"];
	        this.Usage = source["Usage"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.ExpiresAt = this.convertValues(source["ExpiresAt"], null);
	        this.IsRevoked = source["IsRevoked"];
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
	export class KeyInfo {
	    Fingerprint: string;
	    PrimaryUID: string;
	    Email: string;
	    // Go type: time
	    CreatedAt?: any;
	    // Go type: time
	    Expiry?: any;
	    Status: string;
	    IsPrivate: boolean;
	    AlreadyExists: boolean;
	    Subkeys: SubkeyInfo[];
	
	    static createFrom(source: any = {}) {
	        return new KeyInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Fingerprint = source["Fingerprint"];
	        this.PrimaryUID = source["PrimaryUID"];
	        this.Email = source["Email"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.Expiry = this.convertValues(source["Expiry"], null);
	        this.Status = source["Status"];
	        this.IsPrivate = source["IsPrivate"];
	        this.AlreadyExists = source["AlreadyExists"];
	        this.Subkeys = this.convertValues(source["Subkeys"], SubkeyInfo);
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
	export class KeyserverEntry {
	    Label: string;
	    URL: string;
	    BuiltIn: boolean;
	
	    static createFrom(source: any = {}) {
	        return new KeyserverEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Label = source["Label"];
	        this.URL = source["URL"];
	        this.BuiltIn = source["BuiltIn"];
	    }
	}
	export class MultiImportResult {
	    Entries: KeyImportEntry[];
	
	    static createFrom(source: any = {}) {
	        return new MultiImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Entries = this.convertValues(source["Entries"], KeyImportEntry);
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
	export class SignResult {
	    Armored: string;
	    OutputPath: string;
	    Error: string;
	
	    static createFrom(source: any = {}) {
	        return new SignResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Armored = source["Armored"];
	        this.OutputPath = source["OutputPath"];
	        this.Error = source["Error"];
	    }
	}
	
	export class VerifyResult {
	    Valid: boolean;
	    SignedBy: string;
	    UID: string;
	    Email: string;
	    Error: string;
	
	    static createFrom(source: any = {}) {
	        return new VerifyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Valid = source["Valid"];
	        this.SignedBy = source["SignedBy"];
	        this.UID = source["UID"];
	        this.Email = source["Email"];
	        this.Error = source["Error"];
	    }
	}

}

