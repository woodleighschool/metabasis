import { format, formatDistanceToNow, parseISO, isValid } from "date-fns";

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

// Format a date string to just the date part
export function formatDate(input: string | undefined): string {
  if (!input) return "Never";

  try {
    const date = parseISO(input);
    if (!isValid(date)) return "Invalid date";

    return format(date, "PP"); // Nov 6, 2025
  } catch {
    return "Invalid date";
  }
}

// Format a date as "time ago"
export function formatTimeAgo(input: string | undefined): string {
  if (!input) return "Never";

  try {
    const date = parseISO(input);
    if (!isValid(date)) return "Invalid date";

    return formatDistanceToNow(date, { addSuffix: true });
  } catch {
    return "Invalid date";
  }
}

//Format a date in compact format for tables
export function formatCompactDateTime(input: string | undefined): string {
  if (!input) return "Never";

  try {
    const date = parseISO(input);
    if (!isValid(date)) return "Invalid";

    return format(date, "MMM d, HH:mm"); // Nov 6, 14:30
  } catch {
    return "Invalid";
  }
}
