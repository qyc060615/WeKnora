import assert from 'node:assert/strict'
import test from 'node:test'

import { toModelUsageAnalyticsQuery } from './modelUsageAnalyticsContract'

test('maps analytics parameters to the backend query contract', () => {
  assert.deepEqual(toModelUsageAnalyticsQuery({
    modelId: 'model-a',
    startTime: '2026-09-01T00:00:00Z',
    endTime: '2026-09-06T00:00:00Z',
    interval: 'day',
  }), {
    model_id: 'model-a',
    start_time: '2026-09-01T00:00:00Z',
    end_time: '2026-09-06T00:00:00Z',
    interval: 'day',
  })
})

test('omits undefined model and never introduces tenant_id', () => {
  const query = toModelUsageAnalyticsQuery({
    modelId: undefined,
    startTime: '2026-09-01T00:00:00Z',
    endTime: '2026-09-06T00:00:00Z',
    interval: 'hour',
  })
  assert.equal('model_id' in query, false)
  assert.equal('tenant_id' in query, false)
  assert.equal(query.interval, 'hour')
})
