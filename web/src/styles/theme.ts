import type { PaletteMode, PaletteOptions } from "@mui/material";
import { createTheme } from "@mui/material/styles";

const getPalette = (mode: PaletteMode): PaletteOptions => {
  const common = {
    primary: {
      main: "#385c3e",
      light: "#67a874",
      dark: "#4a7d52",
    },
    secondary: {
      main: "#92c393",
      light: "#1b5f2f",
      dark: "#92c393",
    },
    error: { main: "#fd4775" },
    warning: { main: "#fdd047" },
    info: { main: "#47d0fd" },
    success: { main: "#47fd9f" },
  };

  if (mode === "dark") {
    return {
      ...common,
      mode,
      background: {
        default: "#262626ff",
        paper: "#2b2b2bff",
      },
      text: {
        primary: "#fdf7ff",
        secondary: "rgba(255,255,255,0.7)",
      },
      divider: "rgba(255,255,255,0.12)",
    };
  }

  return {
    ...common,
    mode,
    background: {
      default: "#faf5ff",
      paper: "#ffffff",
    },
    text: {
      primary: "#140919",
      secondary: "rgba(20,9,25,0.7)",
    },
    divider: "rgba(0,0,0,0.08)",
  };
};

export const createAppTheme = (mode: PaletteMode = "light") =>
  createTheme({
    palette: getPalette(mode),
    shape: {
      borderRadius: 12,
    },
    typography: {
      button: {
        textTransform: "none",
        fontWeight: 600,
      },
    },
    components: {
      MuiPaper: {
        defaultProps: {
          elevation: 2,
        },
      },
      MuiCard: {
        defaultProps: {
          elevation: 3,
        },
      },
    },
  });
