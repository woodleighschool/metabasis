import { z } from "zod";

const buildInfoSchema = z.object({
  version: z.string(),
  gitCommit: z.string(),
  buildDate: z.string(),
});

const appStatusResponseSchema = z.object({
  status: z.string(),
  version: buildInfoSchema,
});

const authProvidersSchema = z.object({
  oauth: z.boolean(),
  local: z.boolean(),
});

const apiUserSchema = z.object({
  display_name: z.string(),
  user_id: z.string(),
});

const apiErrorResponseSchema = z.object({
  error: z.string().optional(),
  message: z.string().optional(),
  field_errors: z.record(z.string(), z.string()).optional(),
});

const scheduleSchema = z.object({
  id: z.string(),
  user: z.string(),
  display_name: z.string(),
  upn: z.string(),
  leaving_date: z.string(),
  returning_date: z.string(),
  overseas: z.boolean(),
  last_updated_by: z.string(),
  last_updated: z.string(),
});

const userSchema = z.object({
  id: z.string(),
  upn: z.string(),
  displayName: z.string(),
  staff: z.boolean(),
});

export type AppStatusResponse = z.infer<typeof appStatusResponseSchema>;
export type AuthProviders = z.infer<typeof authProvidersSchema>;
export type ApiUser = z.infer<typeof apiUserSchema>;
export type Schedule = z.infer<typeof scheduleSchema>;
export type User = z.infer<typeof userSchema>;

class ApiValidationError extends Error {
  public code: string;
  public fieldErrors: Record<string, string>;
  public status: number;

  constructor(message: string, code: string, fieldErrors: Record<string, string>, status: number) {
    super(message);
    this.name = "ApiValidationError";
    this.code = code;
    this.fieldErrors = fieldErrors;
    this.status = status;
  }
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

const API_BASE = "/api/v1";

async function throwResponseError(res: Response): Promise<void> {
  if (!res.ok) {
    const text = await res.text();

    try {
      const parsed: unknown = JSON.parse(text);
      const errorResult = apiErrorResponseSchema.safeParse(parsed);

      if (errorResult.success) {
        const errorData = errorResult.data;

        if (errorData.field_errors) {
          throw new ApiValidationError(
            errorData.message || "Validation failed",
            errorData.error || "VALIDATION_FAILED",
            errorData.field_errors,
            res.status,
          );
        }

        throw new Error(errorData.message || errorData.error || text || res.statusText);
      }

      throw new Error(text || res.statusText);
    } catch (parseError) {
      if (parseError instanceof ApiValidationError) {
        throw parseError;
      }

      throw new Error(text || res.statusText, { cause: parseError });
    }
  }
}

async function handleResponse<T>(res: Response, schema: z.ZodType<T>): Promise<T> {
  await throwResponseError(res);

  const data: unknown = await res.json();
  return schema.parse(data);
}

async function handleEmptyResponse(res: Response): Promise<void> {
  await throwResponseError(res);
}

async function apiRequest<T>(
  path: string,
  schema: z.ZodType<T>,
  options: RequestInit = {},
): Promise<T> {
  const url = `${API_BASE}${path}`;
  const res = await fetch(url, { credentials: "include", ...options });
  return handleResponse(res, schema);
}

async function emptyApiRequest(path: string, options: RequestInit): Promise<void> {
  const url = `${API_BASE}${path}`;
  const res = await fetch(url, { credentials: "include", ...options });
  return handleEmptyResponse(res);
}

// Auth

export async function getCurrentUser(): Promise<ApiUser | null> {
  const res = await fetch("/api/auth/me", {
    credentials: "include",
  });

  if (res.status === 401) {
    return null;
  }

  return handleResponse(res, apiUserSchema);
}

export async function getAuthProviders(): Promise<AuthProviders> {
  const res = await fetch("/api/auth/providers", {
    credentials: "include",
  });

  return handleResponse(res, authProvidersSchema);
}

// Schedules

export async function listScheduleSummaries(): Promise<Schedule[]> {
  return apiRequest(`/schedules`, z.array(scheduleSchema));
}

export async function createSchedule(payload: CreateSchedulePayload): Promise<void> {
  return emptyApiRequest(`/schedules`, {
    method: "POST",
    headers: { "Content-Type": "applications/json" },
    body: JSON.stringify(payload),
  });
}

export async function updateSchedule(
  scheduleID: string,
  payload: UpdateScheduleRecord,
): Promise<void> {
  return emptyApiRequest(`/schedules/${scheduleID}`, {
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
  return apiRequest(`/users`, z.array(userSchema));
}

// Status

export async function getStatus(): Promise<AppStatusResponse> {
  return apiRequest("/status", appStatusResponseSchema);
}
