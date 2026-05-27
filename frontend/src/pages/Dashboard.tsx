import { useEffect, useState, useMemo, useCallback } from "react";
import { useConfirm } from "material-ui-confirm";
import { Avatar, Button, Chip, Paper, Stack, Typography } from "@mui/material";
import { DataGrid, GridActionsCellItem, type GridColDef } from "@mui/x-data-grid";
import AddIcon from "@mui/icons-material/Add";
import EditIcon from "@mui/icons-material/Edit";
import DeleteIcon from "@mui/icons-material/Delete";
import HomeFilledIcon from "@mui/icons-material/HomeFilled";
import PublicIcon from "@mui/icons-material/Public";

import { type Schedule } from "../api";
import { EmptyState, PageHeader } from "../components";
import { useCurrentScheduleSummary, useDeleteSchedule, useUsers } from "../hooks/useQueries";
import { useToast } from "../hooks/useToast";
import { formatDateTime } from "../utils/dates";
import { ScheduleDialog } from "../components/ScheduleDialog";

type DialogMode = "create" | "edit";

interface DialogConfig {
	mode: DialogMode;
	schedule?: Schedule | null;
}

function createScheduleColumns({ onEdit, onRequestDelete, deletingScheduleId }: ScheduleColumnOptions): GridColDef<Schedule>[] {
	return [
		{
			field: "display_name",
			headerName: "User",
			flex: 1.2,
			sortable: true,
			filterable: true,
			renderCell: (params) => (
				<Stack direction="row" spacing={2} sx={{ alignItems: "center", justifyContent: "left" }}>
					<Avatar
						alt={params.row.display_name[0] ?? "A"}
						src={`/api/v1/users/${params.row.user}/photo`}
					/>
					<Typography>
						{params.row.display_name}
					</Typography>
				</Stack>
			)
		},
		{
			field: "upn",
			headerName: "Email",
			flex: 1.5,
			sortable: true,
			filterable: true,
		},
		{
			field: "leaving_date",
			headerName: "Leaving Date",
			flex: 1,
			sortable: true,
			filterable: false,
			renderCell: (params) => formatDateTime(params.row.leaving_date),
		},
		{
			field: "returning_date",
			headerName: "Returning Date",
			flex: 1,
			sortable: true,
			filterable: false,
			renderCell: (params) => formatDateTime(params.row.returning_date),
		},
		{
			field: "overseas",
			type: "boolean",
			headerName: "Currently Overseas",
			flex: 1,
			sortable: true,
			filterable: true,
			renderCell: (params) => {
				if (params.row.overseas) {
					return (
						<Chip
							label="Away"
							color="success"
							icon={<PublicIcon/>}
							size="small"
							variant="outlined"
						/>
					)
				} else {
					return (
						<Chip
							label="Home"
							color="error"
							icon={<HomeFilledIcon/>}
							size="small"
							variant="outlined"
						/>
					)
				}
			}
		},
		{
			field: "last_updated_by",
			headerName: "Last Updated By",
			flex: 1,
			sortable: true,
		},
		{
			field: "last_updated",
			headerName: "Last Updated",
			flex: 1,
			sortable: true,
			renderCell: (params) => formatDateTime(params.row.last_updated),
		},
		{
			field: "actions",
			type: "actions",
			getActions: (params) => [
				<GridActionsCellItem
					key="edit"
					icon={<EditIcon />}
					label="Edit"
					onClick={(event) => {
						event.stopPropagation();
						onEdit(params.row);
					}}
					showInMenu
				/>,
				<GridActionsCellItem
					key="delete"
					icon={<DeleteIcon color="error" />}
					label="Delete"
					disabled={deletingScheduleId === String(params.id)}
					onClick={(event) => {
						event.stopPropagation();
						void onRequestDelete(String(params.id));
					}}
					showInMenu
				/>,
			],
		},
	];
}

interface ScheduleColumnOptions {
	onEdit: (schedule: Schedule) => void;
	onRequestDelete: (scheduleId: string) => Promise<void>;
	deletingScheduleId: string | null;
}

export default function State() {
	const { schedules, loading, error } = useCurrentScheduleSummary();
	const { users, loading: userLoading, error: userError } = useUsers();
	const { showToast } = useToast();

	const deleteSchedule = useDeleteSchedule();

	const [dialogConfig, setDialogConfig] = useState<DialogConfig | null>(null);
	const [deletingScheduleId, setDeletingScheduleId] = useState<string | null>(null);
	const confirm = useConfirm();

	const dialogMode: DialogMode = dialogConfig?.mode ?? "create";

	const openCreateDialog = useCallback(() => {
		setDialogConfig({ mode: "create", schedule: null });
	}, []);

	const openEditDialog = useCallback((schedule: Schedule) => {
		setDialogConfig({ mode: "edit", schedule });
	}, []);

	const closeDialog = useCallback(() => {
		setDialogConfig(null);
	}, []);

	const handleDeleteSchedule = useCallback(
		async (scheduleId: string) => {
			setDeletingScheduleId(scheduleId);

			try {
				await deleteSchedule.mutateAsync(scheduleId);
			} catch (error) {
				console.error("Delete schedule failed", error);
			} finally {
				setDeletingScheduleId(null);
			}
		},
		[deleteSchedule],
	);
	
	const handleConfirmDelete = useCallback(
		async (scheduleId: string) => {
			const { confirmed } = await confirm({
				title: "Delete Schedule",
				description: "Are you sure you wish to delete this schedule, this operation cannot be undone.",
				cancellationText: "Cancel",
				cancellationButtonProps: { color: "info", variant: "contained" },
				confirmationText: "Delete",
				confirmationButtonProps: { color: "error", variant: "contained" },
			});

			if (confirmed) {
				try {
					await handleDeleteSchedule(scheduleId)
					showToast({
						message: "Successfully deleted schedule",
						severity: "info",
					})
				} catch {
					showToast({
						message: "Failed to delete schedule",
						severity: "error",
					})
				}
			}
		},
		[confirm, handleDeleteSchedule, showToast],
	);

	useEffect(() => {
		if (!error && !userError) return;

		if (error) {
			const message = error || "Failed to load schedules";
			showToast({
				message,
				severity: "error",
			});
		} else {
			const message = userError || "Failed to load users";
			showToast({
				message,
				severity: "error",
			});
		}
	}, [error, userError, showToast]);

	const columns = useMemo(
		() =>
			createScheduleColumns({
				onEdit: openEditDialog,
				onRequestDelete: handleConfirmDelete,
				deletingScheduleId,
			}),
		[openEditDialog, handleConfirmDelete, deletingScheduleId],
	);

	const handleDialogSuccess = useCallback(() => {
		const message = dialogMode === "edit" ? "Schedule updated" : "Schedule created";
		showToast({ message, severity: "success" });
	}, [dialogMode, showToast]);

	const handleDialogError = useCallback(
		(message: string) => {
			showToast({ message, severity: "error" });
		},
		[showToast],
	);

	const rows: Schedule[] = schedules.map((schedule) => ({
		id: schedule.id,
		user: schedule.user,
		display_name: schedule.display_name,
		upn: schedule.upn,
		leaving_date: schedule.leaving_date,
		returning_date: schedule.returning_date,
		overseas: schedule.overseas,
		last_updated_by: schedule.last_updated_by,
		last_updated: schedule.last_updated,
	}));

	const action = (
		<Button
			variant="contained"
			startIcon={<AddIcon />}
			onClick={openCreateDialog}
			disabled={userLoading || loading}
		>
			Add Schedule
		</Button>
	);

	return (
		<>
			<Stack spacing={3}>
				<PageHeader
					title="Overseas Schedules"
					subtitle="All currently logged overseas schedules"
					action={action}
				/>
				<Paper sx={{ height: "70vh", width: "100%" }}>
					<DataGrid
						rows={rows}
						columns={columns}
						showToolbar
						loading={loading || userLoading}
						disableRowSelectionOnClick
						initialState={{
							sorting: {
								sortModel: [{ field: "leavingDate", sort: "asc" }],
							},
						}}
						slots={{
							noRowsOverlay: () => (
								<EmptyState
									title="No schedules found"
									description="No users are currently scheduled for overseas travel"
								/>
							),
						}}
					/>
				</Paper>
			</Stack>
			<ScheduleDialog
				open={Boolean(dialogConfig)}
				mode={dialogMode}
				schedule={dialogConfig?.schedule ?? null}
				users={users}
				onClose={closeDialog}
				onSuccess={handleDialogSuccess}
				onError={handleDialogError}
			/>
		</>
	);
}
