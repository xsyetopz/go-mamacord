// Default is same-origin (admin API serves/proxies the dashboard in dev).
// In production (GitHub Pages), `apiBase` is set at runtime from /config.json.
export let apiBase = "";
export let apiBaseError: string | null = null;

const TRAILING_SLASH_RE = /\/$/;

export function setAPIBase(next: string) {
	apiBase = next;
}

export function setAPIBaseError(next: string | null) {
	apiBaseError = next;
}

export class APIError extends Error {
	readonly status: number;

	constructor(message: string, status: number) {
		super(message);
		this.status = status;
	}
}

function looksLikeHTML(text: string): boolean {
	const head = text.trimStart().slice(0, 64).toLowerCase();
	return head.startsWith("<!doctype") || head.startsWith("<html");
}

function requestHeaders(init?: RequestInit): HeadersInit {
	const method = String(init?.method ?? "GET").toUpperCase();
	const isSafeMethod = method === "GET" || method === "HEAD";

	// Avoid forcing a JSON Content-Type on safe requests.
	// `Content-Type: application/json` turns even a GET into a CORS preflight,
	// which is needless noise in dev and makes misconfigurations feel worse.
	const base: HeadersInit = isSafeMethod
		? {}
		: { "Content-Type": "application/json" };
	return {
		...base,
		...(init?.headers ?? {}),
	};
}

async function fetchResponse(
	url: string,
	init?: RequestInit,
): Promise<Response> {
	try {
		return await fetch(url, {
			credentials: "include",
			...init,
			headers: requestHeaders(init),
		});
	} catch {
		throw new APIError("Could not reach the admin API.", 0);
	}
}

async function apiErrorMessage(response: Response): Promise<string> {
	let message = response.statusText;
	try {
		const payload = (await response.json()) as {
			error?: string;
			retry_after_ms?: number;
		};
		if (payload.error) {
			message = payload.error;
		}
		const retryMS = Number(payload.retry_after_ms ?? 0);
		if (Number.isFinite(retryMS) && retryMS > 0) {
			const seconds = Math.max(1, Math.ceil(retryMS / 1000));
			message = `${message} (retry in ${seconds}s)`;
		}
	} catch {
		// ignore body parse failures
	}
	return message;
}

async function parseJSONResponse<T>(response: Response): Promise<T> {
	if (response.status === 204) {
		return undefined as T;
	}

	// Sometimes a misconfigured origin/proxy returns HTML (e.g. Vite index.html)
	// for an API route, which causes confusing "Unexpected token '<'" errors.
	const clone = response.clone();
	try {
		return (await response.json()) as T;
	} catch {
		let body = "";
		try {
			body = await clone.text();
		} catch {
			// ignore
		}
		if (looksLikeHTML(body)) {
			throw new APIError(
				"This URL is returning HTML for an API request. You are not talking to the mamacord admin API. Start `go run ./cmd/mamacord dev`. Then either open the admin dashboard at `http://127.0.0.1:8081/` (recommended) or run the Vite dev server (`cd apps/dashboard && bun run dev`) and open `http://127.0.0.1:5173/`.",
				-1,
			);
		}
		throw new APIError("Admin API returned invalid JSON.", -1);
	}
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
	const url =
		apiBase && path.startsWith("/")
			? apiBase.replace(TRAILING_SLASH_RE, "") + path
			: path;

	const response = await fetchResponse(url, init);
	if (!response.ok) {
		throw new APIError(await apiErrorMessage(response), response.status);
	}
	return await parseJSONResponse<T>(response);
}

export function get<T>(path: string): Promise<T> {
	return request<T>(path, { method: "GET" });
}

export function post<T>(
	path: string,
	body: unknown,
	csrfToken?: string,
): Promise<T> {
	const headers = csrfToken ? { "X-CSRF-Token": csrfToken } : {};
	return request<T>(path, {
		method: "POST",
		body: JSON.stringify(body),
		headers,
	});
}

export function put<T>(
	path: string,
	body: unknown,
	csrfToken: string,
): Promise<T> {
	return request<T>(path, {
		method: "PUT",
		body: JSON.stringify(body),
		headers: { "X-CSRF-Token": csrfToken },
	});
}
