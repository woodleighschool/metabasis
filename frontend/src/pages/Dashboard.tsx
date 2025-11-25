import { useEffect, useState } from "react";
import { Card, CardContent, Paper, Chip, Typography, Stack, TextField } from "@mui/material";
import { DataGrid, GridActionsCellItem, type GridColDef, type GridRowParams } from "@mui/x-data-grid";

import { EmptyState, PageHeader } from "../components";
import { useCurrentScheduleSummary } from "../hooks/useQueries";
import { useToast } from "../hooks/useToast";
import { formatDateTime } from "../utils/dates";

type ScheduleRow = {
	id: string;
	display_name: string;
	upn: string;
	leavingDate: string;
	returningDate: string;
	currentlyOverseas: boolean;
	lastUpdatedBy: string;
	lastUpdated: string;
}

function createScheduleColumns(): GridColDef<ScheduleRow>[] {
	return [
		{
			field: "display_name",
			headerName: "User",
			flex: 1,
			sortable: true,
			filterable: true
		},
		{
			field: "upn",
			headerName: "Email",
			flex: 1,
			sortable: true,
			filterable: true
		},
		{
			field: "leavingDate",
			headerName: "Leaving Date",
			flex: 1,
			sortable: true,
			filterable: false,
			renderCell: (params) => formatDateTime(params.row.leavingDate)
		},
		{
			field: "returningDate",
			headerName: "Returning Date",
			flex: 1,
			sortable: true,
			filterable: false,
			renderCell: (params) => formatDateTime(params.row.returningDate)
		},
		{
			field: "currentlyOverseas",
			headerName: "Currently Overseas",
			flex: 1,
			sortable: true,
			filterable: true,
			renderCell: (params) => {
				const state = params.value
				let color: "error" | "success"
				var label: string

				if (state == true) {
					color = "success"
					label = "YES"
				} else {
					color = "error"
					label = "NO"
				}

				return (
					<Chip
						size="small"
						color={color}
						variant="filled"
						label={label}
					/>
				)
			}
		},
		{
			field: "lastUpdatedBy",
			headerName: "Last Updated By",
			flex: 1,
			sortable: true,
		},
		{
			field: "lastUpdated",
			headerName: "Last Updated",
			flex: 1,
			sortable: true,
			renderCell: (params) => formatDateTime(params.row.lastUpdated)
		}
		// {
		// 	field: "actions",
		// 	type: "actions",
		// 	renderCell: (params) => {
		// 		<ActionCell></ActionCell>
		// 	}
		// }
	]
}

export default function State() {
	const { schedules, loading, error } = useCurrentScheduleSummary();
	const { showToast } = useToast();

	useEffect(() => {
		if (!error) return;

		const message = error || "Failed to load schedules";
		showToast({
			message,
			severity: "error",
		});
	}, [error, showToast]);

	const columns = createScheduleColumns();
	
	const rows: ScheduleRow[] = schedules.map((schedule) => ({
			id: schedule.id,
			display_name: schedule.display_name,
			upn: schedule.upn,
			leavingDate: schedule.leaving_date,
			returningDate: schedule.returning_date,
			currentlyOverseas: schedule.overseas,
			lastUpdatedBy: schedule.last_updated_by,
			lastUpdated: schedule.last_updated,
	}));

	return (
		<Stack spacing={3}>
				<PageHeader
					title="Overseas Schedules"
					subtitle="All currently logged overseas schedules"
				/>
				
				<Paper sx={{ height: 640, width: "100%" }}>
					<DataGrid
					rows={rows}
					columns={columns}
					showToolbar
					loading={loading}
					disableRowSelectionOnClick
					initialState={{
						sorting: {
							sortModel: [{ field: "leavingDate", sort: "asc"}],
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
	)
}