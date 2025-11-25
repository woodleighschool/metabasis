import { useQuery, useMutation, useQueryClient, keepPreviousData } from "@tanstack/react-query"
import { 
	listScheduleSummaries, 
	getCurrentUser, 
	getStatus, 
	type ApiUser, 
	type ScheduleSummaryRecord, 
	type AppStatusResponse
} from "../api"

export const queryKeys = {
	user: (id: string) => ["users", id] as const,
	currentUser: ["currentUser"] as const,
	schedules: ["schedules"] as const,
	status: ["status"] as const
}

// Current user
export function useCurrentUser() {
	return useQuery<ApiUser | null>({
		queryKey: queryKeys.currentUser,
		queryFn: getCurrentUser,
	});
}

export function useStatus() {
	return useQuery<AppStatusResponse>({
		queryKey: queryKeys.status,
		queryFn: () => getStatus(),
		staleTime: 60 * 1000,
	});
}

export function useCurrentScheduleSummary() {
	const query = useQuery<ScheduleSummaryRecord[]>({
		queryKey: queryKeys.schedules,
		queryFn: () => listScheduleSummaries(),
		select: (results) => (Array.isArray(results) ? results : []),
	});

	return {
		schedules: query.data ?? [],
		loading: query.isLoading,
		error: query.error ? (query.error instanceof Error ? query.error.message : String(query.error)) : null,
		refetch: query.refetch,
	};
}