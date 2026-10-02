// Request and response shapes of the API, kept in step with docs/04-api.md and
// the Go handler types (ADR-0031).

/** GET /api/healthz (api/internal/handler/health). */
export interface HealthResponse {
  status: 'ok'
}
