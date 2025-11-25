import type { PaletteMode, PaletteOptions, ThemeOptions } from "@mui/material";
import { createTheme } from "@mui/material/styles";

const getPalette = (mode: PaletteMode): PaletteOptions => {
  const common = {
    primary: {
      main: "#d047fd",
      light: "#fd71ff",
      dark: "#8a00c7",
      contrastText: "#fff9ff",
    },
    secondary: {
      main: "#75fd47",
      light: "#b6ff8a",
      dark: "#3ca524",
      contrastText: "#060409",
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
        default: "#050109",
        paper: "#0d0716",
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
  } as ThemeOptions);
