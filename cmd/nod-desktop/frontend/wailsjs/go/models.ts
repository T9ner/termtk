export namespace main {
	
	export class ContactInfo {
	    uuid: string;
	    username: string;
	    online: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ContactInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uuid = source["uuid"];
	        this.username = source["username"];
	        this.online = source["online"];
	    }
	}
	export class MessageInfo {
	    id: string;
	    sender: string;
	    content: string;
	    timestamp: string;
	    status: string;
	    encrypted: boolean;
	    isMe: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MessageInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sender = source["sender"];
	        this.content = source["content"];
	        this.timestamp = source["timestamp"];
	        this.status = source["status"];
	        this.encrypted = source["encrypted"];
	        this.isMe = source["isMe"];
	    }
	}

}

