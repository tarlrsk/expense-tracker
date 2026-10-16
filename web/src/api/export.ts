// The CSV export: GET /api/export.csv (docs/04-api.md, ADR-0087, ADR-0092). The file is fetched
// with the bearer token like every call; the screen then hands it to the browser to save.

import { apiDownload } from './client'
import type { ExportFile, ExportPeriod } from './types'

/** The name the API gives a file of every date; used when the response suggests none. */
export const defaultExportFileName = 'satang-transactions.csv'

/** GET /api/export.csv — the caller's transactions for `period`, oldest first. */
export async function getTransactionsCsv(
  period: ExportPeriod = {},
  signal?: AbortSignal,
): Promise<ExportFile> {
  const q = new URLSearchParams()
  if (period.from !== undefined) {
    q.set('from', period.from)
  }
  if (period.to !== undefined) {
    q.set('to', period.to)
  }
  const query = q.size > 0 ? `?${q.toString()}` : ''
  const { blob, fileName } = await apiDownload(`/export.csv${query}`, 'text/csv', { signal })
  return {
    blob,
    fileName: fileName ?? defaultExportFileName,
    empty: !(await hasRows(blob)),
  }
}

/**
 * Whether a CSV file has anything after its header row. The header has no quoted cells, so the
 * first line break ends it; a file of only the header ends there (ADR-0092: CRLF lines).
 */
async function hasRows(blob: Blob): Promise<boolean> {
  const text = await blob.text()
  const end = text.indexOf('\n')
  return end !== -1 && text.slice(end + 1).trim() !== ''
}
