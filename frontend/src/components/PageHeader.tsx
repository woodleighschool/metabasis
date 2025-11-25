import { Link as RouterLink } from "react-router-dom";
import { Breadcrumbs, Link, Stack, Typography, Box } from "@mui/material";
import type { ReactNode } from "react";

export interface PageBreadcrumb {
  label: string;
  to?: string;
}

export interface PageHeaderProps {
  title: string;
  subtitle?: string | undefined;
  breadcrumbs?: PageBreadcrumb[];
  action?: ReactNode;
}

export function PageHeader({ title, subtitle, breadcrumbs, action }: PageHeaderProps) {
  return (
    <Stack spacing={1.5}>
      {breadcrumbs && breadcrumbs.length > 0 && (
        <Breadcrumbs aria-label="breadcrumbs">
          {breadcrumbs.map((crumb, index) => {
            const isLast = index === breadcrumbs.length - 1;

            if (!isLast && crumb.to) {
              return (
                <Link
                  key={`${crumb.label}-${index.toString()}`}
                  component={RouterLink}
                  underline="hover"
                  color="inherit"
                  to={crumb.to}
                >
                  {crumb.label}
                </Link>
              );
            }

            return (
              <Typography
                key={`${crumb.label}-${index.toString()}`}
                color="text.primary"
              >
                {crumb.label}
              </Typography>
            );
          })}
        </Breadcrumbs>
      )}

      <Stack
        direction={{ xs: "column", sm: "row" }}
        spacing={2}
        justifyContent="space-between"
        alignItems={{ xs: "flex-start", sm: "center" }}
      >
        <Box>
          <Typography
            variant="h4"
            component="h1"
          >
            {title}
          </Typography>
          {subtitle && (
            <Typography
              variant="subtitle1"
              color="text.secondary"
            >
              {subtitle}
            </Typography>
          )}
        </Box>

        {action && <Box>{action}</Box>}
      </Stack>
    </Stack>
  );
}
