import { ErrorBoundary as ReactErrorBoundary, type ErrorBoundaryPropsWithComponent, type FallbackProps } from "react-error-boundary";
import { type ErrorInfo } from "react";
import { Box, Button, Paper, Stack, Typography } from "@mui/material";
import ShieldIcon from "@mui/icons-material/Shield";

type ErrorFallbackProps = FallbackProps;

function ErrorFallback({ error, resetErrorBoundary }: ErrorFallbackProps) {
  const message = error instanceof Error ? error.message : String(error);
  return (
    <Box sx={{ minHeight: "50vh", display: "grid", placeItems: "center", p: 3 }}>
      <Paper
        variant="outlined"
        sx={{ maxWidth: 480, width: "100%", p: 4 }}
      >
        <Stack
          spacing={2}
          alignItems="center"
          textAlign="center"
        >
          <ShieldIcon />
          <Typography variant="h5">Something went wrong</Typography>
          <Typography color="text.secondary">We encountered an unexpected error. Try again or refresh the page to continue.</Typography>
          <Paper
            variant="outlined"
            sx={{ width: "100%", p: 2, bgcolor: "background.default" }}
          >
            <Typography
              variant="caption"
              color="text.secondary"
            >
              Technical details
            </Typography>
            <Typography
              variant="body2"
              component="code"
            >
              {message}
            </Typography>
          </Paper>
          <Stack
            direction="row"
            spacing={1}
          >
            <Button
              variant="contained"
              onClick={resetErrorBoundary}
            >
              Try again
            </Button>
            <Button
              variant="outlined"
              onClick={() => {
                window.location.reload();
              }}
            >
              Refresh page
            </Button>
          </Stack>
        </Stack>
      </Paper>
    </Box>
  );
}

interface ErrorBoundaryProps {
  children: React.ReactNode;
  fallback?: React.ComponentType<ErrorFallbackProps>;
  onError?: (error: Error, errorInfo: ErrorInfo) => void;
}

export function ErrorBoundary({ children, fallback = ErrorFallback, onError }: ErrorBoundaryProps) {
  const props: ErrorBoundaryPropsWithComponent = {
    FallbackComponent: fallback,
    onReset: () => {
      window.location.hash = "#/";
    },
  };

  if (onError) {
    props.onError = onError;
  }

  return <ReactErrorBoundary {...props}>{children}</ReactErrorBoundary>;
}
