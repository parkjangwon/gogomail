/**
 * UTF-8 byte-order mark. Prefixing CSV downloads with this makes Windows Excel
 * decode the file as UTF-8, preventing garbled multibyte text (e.g. Korean).
 */
export const CSV_BOM = "\uFEFF";

/**
 * Neutralize CSV formula injection (CWE-1236). Spreadsheet applications treat a
 * cell whose text begins with `=`, `+`, `-`, `@`, or leading tab/CR as a
 * formula, so an attacker-controlled value such as `=cmd|'/c calc'!A0` in a
 * mail subject or display name executes when an admin opens the export. We
 * prefix such cells with a single quote, which spreadsheets strip on display
 * while forcing the value to be treated as literal text.
 */
export function sanitizeCsvCell(value: string): string {
  if (value.length === 0) return value;
  if (/^[=+\-@\t\r]/.test(value)) {
    return `'${value}`;
  }
  return value;
}

/**
 * Escape a value for inclusion in an RFC-4180 CSV field. The value is first
 * sanitized against formula injection, then wrapped in double quotes when it
 * contains a quote, comma, or newline, with inner quotes doubled.
 */
export function escapeCsvCell(value: unknown): string {
  if (value === null || value === undefined) return "";
  const sanitized = sanitizeCsvCell(String(value));
  if (/[",\n\r]/.test(sanitized)) {
    return `"${sanitized.replace(/"/g, '""')}"`;
  }
  return sanitized;
}

export function exportToCSV(data: Record<string, unknown>[], filename: string) {
  if (!data || data.length === 0) {
    return;
  }

  const headers = Object.keys(data[0]);
  const csv = [
    headers.map((header) => escapeCsvCell(header)).join(","),
    ...data.map((row) =>
      headers.map((header) => escapeCsvCell(row[header])).join(",")
    ),
  ].join("\n");

  const blob = new Blob([CSV_BOM + csv], { type: "text/csv;charset=utf-8;" });
  downloadBlob(blob, filename);
}

/**
 * Trigger a client-side CSV download, prepending a UTF-8 BOM so Windows Excel
 * decodes multibyte text correctly. Use this for every CSV download path.
 */
export function downloadCsv(csv: string, filename: string): void {
  const blob = new Blob([CSV_BOM + csv], { type: "text/csv;charset=utf-8;" });
  downloadBlob(blob, filename);
}

function downloadBlob(blob: Blob, filename: string) {
  const link = document.createElement("a");
  const url = URL.createObjectURL(blob);
  link.setAttribute("href", url);
  link.setAttribute("download", filename);
  link.style.visibility = "hidden";
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}

export function generatePDFReport(
  title: string,
  content: string,
  filename: string
) {
  const htmlContent = `
    <html>
      <head>
        <meta charset="utf-8">
        <title>${title}</title>
        <style>
          body { font-family: Arial, sans-serif; margin: 20px; }
          h1 { color: #0972d3; border-bottom: 2px solid #0972d3; padding-bottom: 10px; }
          .section { margin-bottom: 30px; }
          .timestamp { color: #666; font-size: 12px; }
          table { width: 100%; border-collapse: collapse; margin-top: 20px; }
          th { background-color: #f0f2f5; padding: 10px; text-align: left; border: 1px solid #ddd; }
          td { padding: 10px; border: 1px solid #ddd; }
        </style>
      </head>
      <body>
        <h1>${title}</h1>
        <p class="timestamp">Generated on ${new Date().toLocaleString()}</p>
        ${content}
      </body>
    </html>
  `;

  const blob = new Blob([htmlContent], { type: "text/html;charset=utf-8;" });
  downloadBlob(blob, filename);
}

export function formatDataAsHTML(data: Record<string, unknown>[], title: string): string {
  if (!data || data.length === 0) {
    return "<p>No data available</p>";
  }

  const headers = Object.keys(data[0]);
  const rows = data
    .map(
      (row) =>
        `<tr>${headers.map((h) => `<td>${row[h] ?? "—"}</td>`).join("")}</tr>`
    )
    .join("");

  return `
    <div class="section">
      <h2>${title}</h2>
      <table>
        <thead>
          <tr>${headers.map((h) => `<th>${h}</th>`).join("")}</tr>
        </thead>
        <tbody>${rows}</tbody>
      </table>
    </div>
  `;
}
