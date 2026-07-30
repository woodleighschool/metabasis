import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";

import {
  createSchedule,
  listScheduleSummaries,
  listUsers,
  getCurrentUser,
  getStatus,
  updateSchedule,
  type ApiUser,
  type User,
  type Schedule,
  type AppStatusResponse,
  deleteSchedule,
  type UpdateScheduleRecord,
} from "../api";

const queryKeys = {
  user: (id: string) => ["users", id] as const,
  userPhoto: (id: string) => ["users", id, "photo"] as const,
  users: ["users"] as const,
  currentUser: ["currentUser"] as const,
  schedules: ["schedules"] as const,
  status: ["status"] as const,
};

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
  const query = useQuery<Schedule[]>({
    queryKey: queryKeys.schedules,
    queryFn: () => listScheduleSummaries(),
    select: (results) => (Array.isArray(results) ? results : []),
  });

  return {
    schedules: query.data ?? [],
    loading: query.isLoading,
    error: query.error
      ? query.error instanceof Error
        ? query.error.message
        : String(query.error)
      : null,
    refetch: query.refetch,
  };
}

export function useCreateSchedule() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: createSchedule,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["schedules"] });
    },
  });
}

export function useUpdateSchedule() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ scheduleId, payload }: { scheduleId: string; payload: UpdateScheduleRecord }) =>
      updateSchedule(scheduleId, payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["schedules"] });
    },
  });
}

export function useDeleteSchedule() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: deleteSchedule,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["schedules"] });
    },
  });
}

export function useUsers() {
  const query = useQuery<User[]>({
    queryKey: queryKeys.users,
    queryFn: () => listUsers(),
    select: (results) => (Array.isArray(results) ? results : []),
  });

  return {
    users: query.data ?? [],
    loading: query.isLoading,
    error: query.error
      ? query.error instanceof Error
        ? query.error.message
        : String(query.error)
      : null,
    refetch: query.refetch,
  };
}
