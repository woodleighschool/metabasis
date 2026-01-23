export interface BuildInfo {
	version: string;
	gitCommit: string;
	buildDate: string;
}

export interface AppStatusResponse {
	status: string;
	version: BuildInfo;
}

export interface AuthProviders {
	oauth: boolean;
	local: boolean;
}

export interface ValidationSuccess<T> {
	valid: true;
	normalised: T;
}

export interface ApiUser {
	display_name: string;
	user_id: string;
}

export interface ApiErrorResponse {
	error?: string;
	message?: string;
	field_errors?: Record<string, string>;
}

export class ApiValidationError extends Error {
	constructor(
		message: string,
		public code: string,
		public fieldErrors: Record<string, string>,
		public status: number,
	) {
		super(message);
		this.name = "ApiValidationError";
	}
}

export interface Schedule {
	id: string;
	user: string;
	display_name: string;
	upn: string;
	leaving_date: string;
	returning_date: string;
	overseas: boolean;
	last_updated_by: string;
	last_updated: string;
}

export interface CreateSchedulePayload {
	upn: string;
	leaving_date: string;
	returning_date: string;
	last_updated_by: string;
}

export interface UpdateScheduleRecord {
	upn: string;
	leaving_date: string;
	returning_date: string;
	last_updated_by: string;
}

export interface User {
	id: string;
	upn: string;
	displayName: string;
	staff: boolean;
}

const API_BASE = "/api/v1";

function isApiErrorResponse(value: unknown): value is ApiErrorResponse {
	if (typeof value !== "object" || value == null) {
		return false;
	}

	const candidate = value as Record<string, unknown>;
	const hasMessage = typeof candidate.message === "string";
	const hasError = typeof candidate.error === "string";
	const hasFieldErrors = candidate.field_errors !== undefined;

	return hasMessage || hasError || hasFieldErrors;
}

async function handleResponse<T>(res: Response): Promise<T> {
	if (!res.ok) {
		const text = await res.text();

		try {
			const parsed: unknown = JSON.parse(text);

			if (isApiErrorResponse(parsed)) {
				const errorData = parsed;

				if (errorData.field_errors) {
					throw new ApiValidationError(errorData.message || "Validation failed", errorData.error || "VALIDATION_FAILED", errorData.field_errors, res.status);
				}

				throw new Error(errorData.message || errorData.error || text || res.statusText);
			}

			throw new Error(text || res.statusText);
		} catch (parseError) {
			if (parseError instanceof ApiValidationError) {
				throw parseError;
			}

			throw new Error(text || res.statusText);
		}
	}

	if (res.status === 204) {
		return undefined as T;
	}

	return res.json() as Promise<T>;
}

async function apiRequest<T>(path: string, options: RequestInit = {}): Promise<T> {
	const url = `${API_BASE}${path}`;
	const res = await fetch(url, { credentials: "include", ...options });
	return handleResponse<T>(res);
}

// Auth

export async function getCurrentUser(): Promise<ApiUser | null> {
	const res = await fetch("/api/auth/me", {
		credentials: "include",
	});

	if (res.status === 401) {
		return null;
	}

	return handleResponse<ApiUser>(res);
}

export async function getAuthProviders(): Promise<AuthProviders> {
	const res = await fetch("/api/auth/providers", {
		credentials: "include",
	});

	return handleResponse<AuthProviders>(res);
}

// Schedules

export async function listScheduleSummaries(): Promise<Schedule[]> {
	return apiRequest<Schedule[]>(`/schedules`);
}

export async function createSchedule(payload: CreateSchedulePayload): Promise<void> {
	return apiRequest(`/schedules`, {
		method: "POST",
		headers: { "Content-Type": "applications/json" },
		body: JSON.stringify(payload),
	});
}

export async function updateSchedule(scheduleID: string, payload: UpdateScheduleRecord): Promise<void> {
	return apiRequest(`/schedules/${scheduleID}`, {
		method: "PATCH",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(payload),
	});
}

export async function deleteSchedule(scheduleId: string): Promise<void> {
	const res = await fetch(`${API_BASE}/schedules/${scheduleId}`, {
		method: "DELETE",
		credentials: "include",
	});

	if (!res.ok && res.status !== 404) {
		throw new Error("Failed to delete schedule");
	}
}

export async function listUsers(): Promise<User[]> {
	return apiRequest<User[]>(`/users`);
}

// Status

export async function getStatus(): Promise<AppStatusResponse> {
	return apiRequest<AppStatusResponse>("/status");
}
