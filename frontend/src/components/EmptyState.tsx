import { Box, Stack, Typography } from "@mui/material";
import type { ReactNode } from "react";

interface EmptyStateProps {
  title: string;
  description?: string;
  icon?: ReactNode;
  action?: ReactNode;
}

export function EmptyState({ title, description, icon, action }: EmptyStateProps) {
  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        p: 4,
        textAlign: "center",
        height: "100%",
        minHeight: 300,
        bgcolor: "background.paper",
        borderRadius: 1,
      }}
    >
      <Stack
        spacing={2}
        alignItems="center"
      >
        {icon && <Box sx={{ color: "text.secondary", fontSize: 64 }}>{icon}</Box>}
        <Typography
          variant="h6"
          color="text.primary"
        >
          {title}
        </Typography>
        {description && (
          <Typography
            variant="body2"
            color="text.secondary"
            maxWidth={400}
          >
            {description}
          </Typography>
        )}
        {action && <Box pt={1}>{action}</Box>}
      </Stack>
    </Box>
  );
}
