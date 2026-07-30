import {
  Card,
  CardContent,
  CardHeader,
  type CardContentProps,
  type CardProps,
} from "@mui/material";
import { styled } from "@mui/material/styles";
import type { ReactNode } from "react";

export interface SectionCardProps extends CardProps {
  title: string;
  subheader?: string;
  children: ReactNode;
  contentProps?: CardContentProps;
}

const StyledCard = styled(Card)({
  display: "flex",
  flexDirection: "column",
  height: "100%",
});

const StyledCardContent = styled(CardContent)({
  flexGrow: 1,
  display: "flex",
  flexDirection: "column",
  gap: 16,
});

export function SectionCard({
  title,
  subheader,
  children,
  contentProps,
  sx,
  ...cardProps
}: SectionCardProps) {
  const { sx: contentSx, ...restContentProps } = contentProps ?? {};
  return (
    <StyledCard elevation={1} {...cardProps} sx={sx}>
      <CardHeader title={title} subheader={subheader} />
      <StyledCardContent {...restContentProps} sx={contentSx}>
        {children}
      </StyledCardContent>
    </StyledCard>
  );
}
