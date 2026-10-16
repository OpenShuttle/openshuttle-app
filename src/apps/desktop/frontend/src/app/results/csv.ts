/** Quotes a CSV field when it holds the separator, a quote or a line break. */
export function csvField(value: string | number, separator: string): string {
  const text = String(value);
  return text.includes(separator) || /["\r\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text;
}

/** Joins rows into CSV text, one line per row. */
export function toCsv(rows: (string | number)[][], separator: string): string {
  return rows.map((row) => row.map((v) => csvField(v, separator)).join(separator)).join('\r\n') + '\r\n';
}
