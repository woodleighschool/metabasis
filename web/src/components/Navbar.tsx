import { NavLink } from "react-router-dom";
import {
	AppBar,
	Toolbar,
	Typography,
	Button,
	Avatar,
	Stack,
	Divider,
	useMediaQuery,
	useTheme,
} from "@mui/material";

import LogoutIcon from "@mui/icons-material/Logout";

export interface NavbarProps {
	activeTab: string | false;
	userDisplay: string;
	userInitial: string;
	userID: string;
	onLogout: () => void | Promise<void>;
}

export function Navbar({ userDisplay, userInitial, userID, onLogout }: NavbarProps) {
	const theme = useTheme();
	const isDesktop = useMediaQuery(theme.breakpoints.up("md"));

	return (
	<>
		<AppBar
			position="sticky"
			enableColorOnDark
		>
			<Toolbar sx={{ gap: 1.5, alignItems: "center" }}>
				<Button
					component={NavLink}
					to="/"
					color="inherit"
					sx={{ px: 1, minWidth: 0 }}
				>
					<Typography
						variant="h6"
						component="span"
						sx={{ display: { xs: "none", sm: "inline" } }}
					>
						ADOverseas
					</Typography>
				</Button>
				<Stack sx={{ width: "100%" }} />
				<Button
					color="inherit"
					variant={isDesktop ? "outlined" : "text"}
					onClick={() => {
						void onLogout();
					}}
					startIcon={
						<Avatar 
							sx={{
								width: 20,
								height: 20,
								fontSize: 13,
							}}
							alt={userInitial} 
							src={`/api/v1/users/${userID}/photo`}
						/>
					}
					endIcon={<LogoutIcon fontSize="small" />}
					aria-label={`Logout ${userDisplay}`}
					sx={{
						width: "100%",
						maxWidth: { xs: 44, sm: 200 },
						pl: { xs: 0.5, sm: 1.5 },
						pr: { xs: 0.5, sm: 1.5 },
					}}
				>
					<Typography
						variant="body2"
						noWrap
						sx={{ display: { xs: "none", sm: "block" } }}
					>
						{userDisplay}
					</Typography>
				</Button>
			</Toolbar>
			<Divider />
		</AppBar>
	</>
	);
}
