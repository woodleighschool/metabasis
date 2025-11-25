import { useCallback } from "react";
import { useSnackbar, type VariantType } from "notistack";
import type { AlertColor } from "@mui/material";

export interface ToastOptions {
  message: string;
  severity?: AlertColor;
}

const severityToVariant: Record<AlertColor, VariantType> = {
  error: "error",
  info: "info",
  success: "success",
  warning: "warning",
};

export function useToast(defaultSeverity: AlertColor = "error") {
  const { enqueueSnackbar } = useSnackbar();

  const showToast = useCallback(
    ({ message, severity }: ToastOptions) => {
      enqueueSnackbar(message, {
        variant: severityToVariant[severity ?? defaultSeverity],
      });
    },
    [enqueueSnackbar, defaultSeverity],
  );

  return { showToast };
}
