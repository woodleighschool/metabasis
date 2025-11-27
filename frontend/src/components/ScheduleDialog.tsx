import { useEffect, useState } from "react";
import { Controller, useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import { Autocomplete, Button, Dialog, DialogActions, DialogTitle, DialogContent, LinearProgress, TextField, Stack, CircularProgress } from "@mui/material";
import { DateTimePicker } from "@mui/x-date-pickers";
import { ApiValidationError, type Schedule, type User} from "../api";
import { useCurrentUser, useCreateSchedule, useUpdateSchedule, useUsers } from "../hooks/useQueries";
import dayjs, { Dayjs } from "dayjs";


export type ScheduleFormValues = {
	upn: string;
	display_name: string;
	leaving_date: Dayjs;
	returning_date: Dayjs;
	overseas: boolean;
}
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

	const [userData, setUserData] = useState<User | null>(null)
	const [leavingDate, setLeavingDate] = useState<Dayjs | null>(null);
	const [returningDate, setReturningDate] = useState<Dayjs | null>(null);

	const form = useForm<ScheduleFormValues>({
		defaultValues,
	});
	const {
		register,
		control,
		handleSubmit,
		watch,
		reset,
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
		const now = dayjs()
		var overseas: boolean
		if (now.isAfter(leavingDate) && now.isBefore(returningDate)) {
			overseas = true
		} else {
			overseas = false
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
			overseas: overseas
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
		<Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
			<DialogTitle>{dialogTitle}</DialogTitle>
			{isSubmitting && <LinearProgress />}
			<DialogContent dividers>
				<form id="application-form" onSubmit={(e) => void handleSubmit(onSubmit)(e)}>
					<Stack direction={{ xs: "column", md: "row" }} spacing={3}>
						<Stack spacing={2.5} flex={{ xs: "auto" }}>
							<Autocomplete 
								{...register("display_name")} 
								/*error={!!errors.display_name}*/ 
								options={users} 
								getOptionLabel={(option) => option.displayName}
								getOptionKey={(option) => option.upn}
								onChange={(event: any, value: User | null) => {
									setUserData(value);
								}}
								fullWidth
								renderInput={(params) =>
								 <TextField {...params} placeholder="Example User" label="User"/>} 
								 />
							<TextField label="Email" {...register("upn")} placeholder="example@example.org" error={!!errors.upn} fullWidth disabled value={userData? userData.upn : ""}/>
							{/* <Chip {...register("overseas")} label={chipLabel} color={chipColor} size="small" variant="filled" /> */}
						</Stack>
						<Stack spacing={3} flex={{ xs: "auto" }}>
							<DateTimePicker name="leaving_date" onChange={() => setLeavingDate} />
							<DateTimePicker name="returning_date" onChange={() => setReturningDate} />
						</Stack>
					</Stack>
				</form>
			</DialogContent>
			<DialogActions>
				<Button onClick={onClose} disabled={isSubmitting}>
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
