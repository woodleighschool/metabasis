import { useEffect, useMemo, Suspense, lazy, useCallback, useState } from "react";
import { Route, Routes, useLocation } from "react-router-dom";
import { useCurrentUser, useStatus } from "./hooks/useQueries";

import { Box, Container, LinearProgress } from "@mui/material";

import { Footer, CreditsDialog, Navbar } from "./components";
import { useToast } from "./hooks/useToast";

const Login = lazy(() => import("./pages/Login"));
const Dashboard = lazy(() => import("./pages/Dashboard"));

const SuspenseFallback = () => (
  <Box p={2}>
    <LinearProgress />
  </Box>
);

export default function App() {
  const { data: user, error } = useCurrentUser();
  const { data: status } = useStatus();
  const location = useLocation();
  const { showToast } = useToast();

  const [creditsOpen, setCreditsOpen] = useState(false);
  useEffect(() => {
    if (!error) return;

    console.error("Failed to load current user", error);
    showToast({
      message: "Failed to load user. Some features may be unavailable.",
      severity: "error",
    });
  }, [error, showToast]);

  const activeTab = useMemo(() => {
    const path = location.pathname;

    if (path === "/") return "/";

    return false;
  }, [location.pathname]);

  const handleLogout = useCallback(async () => {
    try {
      const res = await fetch("/api/auth/logout", {
        method: "POST",
        credentials: "include",
      });

      if (!res.ok) {
        throw new Error(`Logout failed: ${String(res.status)}`);
      }

      window.location.href = "/";
    } catch (err) {
      console.error("Logout error", err);
      showToast({
        message: "Logout failed. Please try again.",
        severity: "error",
      });
    }
  }, [showToast]);

  const handleOpenCredits = useCallback(() => {
    setCreditsOpen(true);
  }, []);

  const handleCloseCredits = useCallback(() => {
    setCreditsOpen(false);
  }, []);

  const userDisplay = user?.display_name ?? "Administrator";
  const userInitial = (userDisplay[0] ?? "A").toUpperCase();
  const versionLabel = status?.version.version;

  if (!user) {
    return (
      <Suspense fallback={<SuspenseFallback />}>
        <Login
          onLogin={() => {
            window.location.reload();
          }}
        />
      </Suspense>
    );
  }

  return (
    <Box
      sx={{
        minHeight: "100dvh",
        display: "flex",
        flexDirection: "column",
      }}
    >
      <Navbar
        activeTab={activeTab}
        userDisplay={userDisplay}
        userInitial={userInitial}
        onLogout={handleLogout}
      />
      <Container
        component="main"
        maxWidth="xl"
        sx={{
          flexGrow: 1,
          py: 4,
          display: "flex",
          flexDirection: "column",
          minHeight: 0,
        }}
      >
        <Box sx={{ flex: 1, display: "flex", flexDirection: "column", minHeight: 0 }}>
          <Suspense fallback={<SuspenseFallback />}>
            <Routes>
              <Route
                path="/"
                element={<Dashboard />}
              />
            </Routes>
          </Suspense>
        </Box>
      </Container>

      <Footer
        versionLabel={versionLabel}
        onOpenCredits={handleOpenCredits}
      />

      <CreditsDialog
        open={creditsOpen}
        onClose={handleCloseCredits}
      />
    </Box>
  );
}
