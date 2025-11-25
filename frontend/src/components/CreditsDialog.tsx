import { Dialog, DialogTitle, DialogContent, DialogActions, Typography, Link, Button } from "@mui/material";

export interface CreditsDialogProps {
  open: boolean;
  onClose: () => void;
}

export function CreditsDialog({ open, onClose }: CreditsDialogProps) {
  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="xs"
      fullWidth
    >
      <DialogTitle>Credits</DialogTitle>
      <DialogContent dividers>
        <Typography
          variant="body2"
          color="text.secondary"
        >
          <Link
            href="https://www.flaticon.com/free-icons/grinch"
            title="grinch icons"
            target="_blank"
            rel="noopener noreferrer"
          >
            Grinch icons created by Freepik - Flaticon
          </Link>
        </Typography>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
}
