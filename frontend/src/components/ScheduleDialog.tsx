import { useEffect } from "react";
import { Controller, useForm } from "react-hook-form";
import { Autocomplete, Button, Dialog, DialogActions, DialogTitle, DialogContent, LinearProgress, TextField, Stack, CircularProgress } from "@mui/material";
import { DateTimePicker } from "@mui/x-date-pickers";
import { ApiValidationError, type Schedule, type User } from "../api";
import { useCurrentUser, useCreateSchedule, useUpdateSchedule, useUsers } from "../hooks/useQueries";
import dayjs, { Dayjs } from "dayjs";

export type ScheduleFormValues = {
  upn: string;
  display_name: string;
  leaving_date: Dayjs;
  returning_date: Dayjs;
  overseas: boolean;
};
type ScheduleDialogMode = "create" | "edit";

const defaultValues: ScheduleFormValues = {
  upn: "",
  display_name: "",
  leaving_date: dayjs("2025-01-01T00:00:00.000Z"),
  returning_date: dayjs("2025-12-12T00:00:00.000Z"),
  overseas: false,
};

interface ScheduleDialogProps {
  open: boolean;
  mode?: ScheduleDialogMode;
  users: User[];
  schedule?: Schedule | null;
  onClose: () => void;
  onSuccess: () => void;
  onError: (message: string) => void;
}

export function ScheduleDialog({ open, mode = "edit", users, schedule, onClose, onSuccess, onError }: ScheduleDialogProps) {
  const { data: user, error } = useCurrentUser();
  const createSchedule = useCreateSchedule();
  const updateSchedule = useUpdateSchedule();

  const form = useForm<ScheduleFormValues>({
    defaultValues,
  });
  const {
    register,
    control,
    handleSubmit,
    watch,
    reset,
    setValue,
    setError,
    clearErrors,
    formState: { isSubmitting, errors },
  } = form;

  useEffect(() => {
    if (open) {
      if (mode === "edit" && schedule) {
        reset({
          display_name: schedule.display_name,
          upn: schedule.upn,
          leaving_date: schedule.leaving_date,
          returning_date: schedule.returning_date,
          overseas: schedule.overseas,
        });
      } else {
        reset(defaultValues);
      }
      clearErrors();
    }
  }, [open, mode, schedule, reset, clearErrors]);

  const dialogTitle = mode === "edit" ? "Edit Schedule" : "Create Schedule";
  const submitLabel = mode === "edit" ? "Save Changes" : "Create";
  const submittingLabel = mode === "edit" ? "Saving..." : "Creating...";

  const chipLabel = schedule?.overseas ? "YES" : "NO";
  const chipColor = schedule?.overseas ? "success" : "error";

  const buildPayload = (values: ScheduleFormValues) => {
    const now = dayjs();
    var overseas: boolean;
    if (now.isAfter(values.leaving_date) && now.isBefore(values.returning_date)) {
      overseas = true;
    } else {
      overseas = false;
    }
    const payload: {
      upn: string;
      leaving_date: string;
      returning_date: string;
      last_updated_by: string;
      overseas: boolean;
    } = {
      upn: values.upn,
      leaving_date: values.leaving_date.toISOString(),
      returning_date: values.returning_date.toISOString(),
      last_updated_by: user?.display_name ?? "SYSTEM",
      overseas: overseas,
    };
    return payload;
  };

  const onSubmit = async (formData: ScheduleFormValues) => {
    clearErrors();
    try {
      const payload = buildPayload(formData);
      if (mode === "edit") {
        if (!schedule?.id) {
          throw new Error("Missing schedule identifier");
        }
        await updateSchedule.mutateAsync({ scheduleId: schedule.id, payload });
      } else {
        await createSchedule.mutateAsync(payload);
      }

      onSuccess();
      onClose();
    } catch (err) {
      console.error("Schedule dialog failed", err);
      onError(mode === "edit" ? "Failed to update schedule" : "Failed to create schedule");
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="md"
      fullWidth
    >
      <DialogTitle>{dialogTitle}</DialogTitle>
      {isSubmitting && <LinearProgress />}
      <DialogContent dividers>
        <form
          id="application-form"
          onSubmit={(e) => void handleSubmit(onSubmit)(e)}
        >
          <Stack
            direction={{ xs: "column", md: "row" }}
            spacing={3}
          >
            <Stack
              spacing={2.5}
              flex={{ xs: "auto" }}
            >
              <Controller
                name="display_name"
                control={control}
                render={({ field: { onChange, value } }) => (
                  <Autocomplete
                    options={users}
                    getOptionLabel={(option) => option.displayName}
                    isOptionEqualToValue={(option, val) => option.displayName === val.displayName}
                    value={users.find((u) => u.displayName === value) || null}
                    onChange={(_, data) => {
                      onChange(data ? data.displayName : "");
                      setValue("upn", data ? data.upn : "");
                    }}
                    fullWidth
                    renderInput={(params) => (
                      <TextField
                        label="User"
                        placeholder="John Doe"
                        required
                        slotProps={{
                          input: params.InputProps,
                          htmlInput: params.inputProps,
                          inputLabel: params.InputLabelProps,
                        }}
                        fullWidth={params.fullWidth}
                        disabled={params.disabled}
                        id={params.id}
                      />
                    )}
                  />
                )}
              />
              <Controller
                name="upn"
                control={control}
                render={({ field }) => (
                  <TextField
                    {...field}
                    label="Email"
                    placeholder="example@example.org"
                    error={!!errors.upn}
                    fullWidth
                    disabled
                  />
                )}
              />
              {/* <Chip {...register("overseas")} label={chipLabel} color={chipColor} size="small" variant="filled" /> */}
            </Stack>
            <Stack
              spacing={3}
              flex={{ xs: "auto" }}
            >
              <Controller
                name="leaving_date"
                control={control}
                render={({ field }) => (
                  <DateTimePicker
                    label="Leaving Date"
                    disablePast
                    {...field}
                  />
                )}
              />
              <Controller
                name="returning_date"
                control={control}
                render={({ field }) => (
                  <DateTimePicker
                    label="Returning Date"
                    disablePast
                    {...field}
                  />
                )}
              />
            </Stack>
          </Stack>
        </form>
      </DialogContent>
      <DialogActions>
        <Button
          onClick={onClose}
          disabled={isSubmitting}
        >
          Cancel
        </Button>
        <Button
          type="submit"
          form="application-form"
          variant="contained"
          disabled={isSubmitting}
          startIcon={isSubmitting ? <CircularProgress size={16} /> : undefined}
        >
          {isSubmitting ? submittingLabel : submitLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
