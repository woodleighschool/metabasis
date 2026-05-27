import { Buffer } from "buffer";

export function decode(str: string | undefined): string {
	if (str) {
		try {
			return Buffer.from(str, "base64").toString("binary");
		} catch {
			return "";
		}
	} else {
		return "";
	}
}
