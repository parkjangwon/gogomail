export function exportToCSV(data: any[], filename: string) {
  if (!data || data.length === 0) {
    console.warn("No data to export");
    return;
  }

  const headers = Object.keys(data[0]);
  const csv = [
    headers.join(","),
    ...data.map((row) =>
      headers
        .map((header) => {
          const value = row[header];
          if (value === null || value === undefined) return "";
          if (typeof value === "string" && value.includes(",")) {
            return `"${value.replace(/"/g, '""')}"`;
          }
          return value;
        })
        .join(",")
    ),
  ].join("\n");

  const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
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

export function formatDataAsHTML(data: any[], title: string): string {
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
