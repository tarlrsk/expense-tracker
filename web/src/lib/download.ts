// Saving a file the app already holds, such as the CSV export (ADR-0093). The API is called with
// the bearer token, so the browser cannot follow a link to it; the file is handed over instead
// through a temporary object URL and a link with `download`.

/** Asks the browser to save `blob` as `fileName`: Downloads, or a preview on an iPhone. */
export function saveFile(blob: Blob, fileName: string): void {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = fileName
  link.rel = 'noopener'
  link.hidden = true
  document.body.append(link)
  try {
    link.click()
  } finally {
    link.remove()
    // Some browsers read the URL after click returns; free it once they have.
    setTimeout(() => {
      URL.revokeObjectURL(url)
    }, 60_000)
  }
}
