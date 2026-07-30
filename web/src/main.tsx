import { ThemeProvider, CssBaseline, useMediaQuery } from "@mui/material";
import type { PaletteMode } from "@mui/material";
import { LocalizationProvider } from "@mui/x-date-pickers";
import { AdapterDayjs } from "@mui/x-date-pickers/AdapterDayjs";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ConfirmProvider } from "material-ui-confirm";
import React, { type ErrorInfo, useMemo } from "react";
import ReactDOM from "react-dom/client";
import "dayjs/locale/en-au";
import { BrowserRouter } from "react-router-dom";

import App from "./App";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { createAppTheme } from "./styles/theme";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000,
      gcTime: 10 * 60 * 1000,
    },
  },
});

const handleError = (error: unknown, info: ErrorInfo) => {
  console.error("App error", { error, info });
};

export function Root() {
  const prefersDark = useMediaQuery("(prefers-color-scheme: dark)");
  const paletteMode: PaletteMode = prefersDark ? "dark" : "light";
  const theme = useMemo(() => createAppTheme(paletteMode), [paletteMode]);

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <LocalizationProvider dateAdapter={AdapterDayjs} adapterLocale="en-au">
        <ConfirmProvider>
          <BrowserRouter>
            <App />
          </BrowserRouter>
        </ConfirmProvider>
      </LocalizationProvider>
    </ThemeProvider>
  );
}

const rootElement = document.getElementById("root");
if (rootElement == null) {
  throw new Error("Root element not found");
}

ReactDOM.createRoot(rootElement).render(
  <React.StrictMode>
    <ErrorBoundary onError={handleError}>
      <QueryClientProvider client={queryClient}>
        <Root />
      </QueryClientProvider>
    </ErrorBoundary>
  </React.StrictMode>,
);
