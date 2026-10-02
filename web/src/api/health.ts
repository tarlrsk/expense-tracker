import { apiRequest } from './client'
import type { HealthResponse } from './types'

/** GET /api/healthz — liveness, no auth. */
export function getHealth(signal?: AbortSignal): Promise<HealthResponse> {
  return apiRequest<HealthResponse>('/healthz', { signal })
}
