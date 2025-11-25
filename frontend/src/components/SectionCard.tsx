import { Card, CardContent, CardHeader, type CardContentProps, type CardProps } from "@mui/material";
import type { ReactNode } from "react";
import type { SxProps, Theme } from "@mui/material/styles";

export interface SectionCardProps extends CardProps {
  title: string;
  subheader?: string;
  children: ReactNode;
  contentProps?: CardContentProps;
}

function mergeSx(base: SxProps<Theme>, extra?: SxProps<Theme>): SxProps<Theme> {
  if (extra == null) return base;
  if (Array.isArray(extra)) {
    return [base, ...(extra as SxProps<Theme>[])] as SxProps<Theme>;
  }
  return [base, extra] as SxProps<Theme>;
}

export function SectionCard({ title, subheader, children, contentProps, sx, ...cardProps }: SectionCardProps) {
  const { sx: contentSx, ...restContentProps } = contentProps ?? {};
  const baseCardSx: SxProps<Theme> = { display: "flex", flexDirection: "column", height: "100%" };
  const baseContentSx: SxProps<Theme> = { flexGrow: 1, display: "flex", flexDirection: "column", gap: 2 };
  const cardSx = mergeSx(baseCardSx, sx);
  const contentStyles = mergeSx(baseContentSx, contentSx);

  return (
    <Card
      elevation={1}
      {...cardProps}
      sx={cardSx}
    >
      <CardHeader
        title={title}
        subheader={subheader}
      />
      <CardContent
        {...restContentProps}
        sx={contentStyles}
      >
        {children}
      </CardContent>
    </Card>
  );
}
