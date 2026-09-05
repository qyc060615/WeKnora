import assert from 'node:assert/strict'
import test from 'node:test'

import {
  defaultAnalyticsDateRange,
  formatCompactNumber,
  formatExactNumber,
  formatLatency,
  formatLocalCalendarDate,
  formatRatio,
  inclusiveDateRangeToExclusiveRFC3339,
} from './modelUsageAnalyticsHelpers'

test('converts an inclusive Sep 1 through Sep 5 selection to an exclusive Sep 6 boundary', () => {
  const bounds = inclusiveDateRangeToExclusiveRFC3339(['2026-09-01', '2026-09-05'])
  assert.equal(bounds.startTime, new Date(2026, 8, 1).toISOString())
  assert.equal(bounds.endTime, new Date(2026, 8, 6).toISOString())
})

test('builds a deterministic 30-calendar-day default range', () => {
  assert.deepEqual(
    defaultAnalyticsDateRange(new Date(2026, 8, 5, 18, 30)),
    ['2026-08-07', '2026-09-05'],
  )
  assert.equal(formatLocalCalendarDate(new Date(2026, 8, 5)), '2026-09-05')
})

test('keeps null distinct from observed zero in metric formatting', () => {
  assert.equal(formatCompactNumber(null), '—')
  assert.equal(formatCompactNumber(0), '0')
  assert.equal(formatLatency(null), '—')
  assert.equal(formatLatency(0), '0 ms')
  assert.equal(formatRatio(null), '—')
  assert.equal(formatRatio(0.875), '87.5%')
})

test('formats large token values compactly while retaining an exact formatter', () => {
  assert.equal(formatCompactNumber(1_200), '1.2K')
  assert.equal(formatCompactNumber(236_213_313), '236.2M')
  assert.equal(formatExactNumber(236_213_313), '236,213,313')
  assert.equal(formatLatency(1320), '1.32 s')
})
