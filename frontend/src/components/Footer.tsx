import { Paper, Container, Toolbar, Stack, IconButton, Typography, Button } from "@mui/material";
import GitHubIcon from "@mui/icons-material/GitHub";

export interface FooterProps {
  versionLabel?: string | undefined;
  onOpenCredits: () => void;
}

export function Footer({ versionLabel, onOpenCredits }: FooterProps) {
  return (
    <Paper
      component="footer"
      elevation={1}
      square
      sx={{ mt: "auto" }}
    >
      <Container maxWidth="lg">
        <Toolbar
          disableGutters
          sx={{ py: 1.5 }}
        >
          <Stack
            direction={{ xs: "column", sm: "row" }}
            spacing={1}
            alignItems={{ xs: "flex-start", sm: "center" }}
            justifyContent="space-between"
            sx={{ width: "100%" }}
          >
            <Stack
              direction="row"
              spacing={1}
              alignItems="center"
            >
              <IconButton
                component="a"
                href="https://github.com/woodleighschool/grinch"
                target="_blank"
                rel="noopener noreferrer"
                size="small"
                color="inherit"
              >
                <GitHubIcon fontSize="inherit" />
              </IconButton>

              {versionLabel ? (
                <Typography
                  variant="caption"
                  color="text.secondary"
                >
                  {versionLabel}
                </Typography>
              ) : null}
            </Stack>

            <Button
              size="small"
              onClick={onOpenCredits}
            >
              Credits
            </Button>
          </Stack>
        </Toolbar>
      </Container>
    </Paper>
  );
}
