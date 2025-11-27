import { useState, type MouseEvent, type JSX } from "react";
import { NavLink } from "react-router-dom";
import {
  AppBar,
  Toolbar,
  Typography,
  Box,
  Button,
  Avatar,
  Stack,
  Tabs,
  Tab,
  Divider,
  IconButton,
  Menu,
  MenuItem,
  ListItemIcon,
  ListItemText,
  useMediaQuery,
  useTheme,
} from "@mui/material";

import DashboardIcon from "@mui/icons-material/Dashboard";
import LogoutIcon from "@mui/icons-material/Logout";
import MenuIcon from "@mui/icons-material/Menu";

interface NavItem {
  label: string;
  icon: JSX.Element;
  to: string;
}

const navItems: NavItem[] = [{ label: "Dashboard", icon: <DashboardIcon fontSize="small" />, to: "/" }];

export interface NavbarProps {
  activeTab: string | false;
  userDisplay: string;
  userInitial: string;
  onLogout: () => void | Promise<void>;
}

export function Navbar({ userDisplay, userInitial, onLogout }: NavbarProps) {
  const [navMenuAnchor, setNavMenuAnchor] = useState<HTMLElement | null>(null);
  const theme = useTheme();
  const isDesktop = useMediaQuery(theme.breakpoints.up("md"));
  const navMenuOpen = Boolean(navMenuAnchor);

  const handleMenuOpen = (event: MouseEvent<HTMLButtonElement>) => {
    setNavMenuAnchor(event.currentTarget);
  };

  const handleMenuClose = () => {
    setNavMenuAnchor(null);
  };

  return (
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
                width: 28,
                height: 28,
                fontSize: 13,
              }}
            >
              {userInitial}
            </Avatar>
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
  );
}
