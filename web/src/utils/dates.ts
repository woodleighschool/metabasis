import { format, parseISO, isValid } from "date-fns";

// Format a date string to a localised date and time
export function formatDateTime(input: string | undefined): string {
  if (!input) return "Never";

  try {
    const date = parseISO(input);
    if (!isValid(date)) return "Invalid date";

    return format(date, "PPpp"); // Nov 6, 2025 at 2:30:45 PM
  } catch {
    return "Invalid date";
  }
}
