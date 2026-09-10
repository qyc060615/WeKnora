import { get } from '@/utils/request'
import {
  toModelUsageAnalyticsQuery,
  type ModelUsageAnalyticsParams,
  type ModelUsageAnalyticsResult,
} from './modelUsageAnalyticsContract'

export * from './modelUsageAnalyticsContract'

interface ModelUsageAnalyticsResponse {
  success: boolean
  data?: ModelUsageAnalyticsResult
}

export async function getModelUsageAnalytics(
  params: ModelUsageAnalyticsParams,
): Promise<ModelUsageAnalyticsResult> {
  const response = await get<ModelUsageAnalyticsResponse>('/api/v1/model-usage/analytics', {
    params: toModelUsageAnalyticsQuery(params),
  })
  if (!response.success || !response.data) {
    throw new Error('model usage analytics response is missing data')
  }
  return response.data
}
