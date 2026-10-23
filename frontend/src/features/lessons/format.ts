const dateFormat = new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "short", year: "numeric" });

/** "23 Oct 2026". */
export function formatDate(iso: string): string {
  return dateFormat.format(new Date(iso));
}

/** "840 KB", "1.2 MB": a file size as people read it. */
export function formatBytes(bytes: number): string {
  if (bytes < 1024 * 1024) return `${String(Math.max(1, Math.round(bytes / 1024)))} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function pdfCount(n: number): string {
  return n === 1 ? "1 PDF" : `${String(n)} PDFs`;
}
