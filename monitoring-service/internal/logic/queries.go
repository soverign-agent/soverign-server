package logic

// PromQL templates used by the Service. Each template embeds the tenant ID
// using %q so it is rendered as a quoted PromQL string literal — this prevents
// PromQL injection if a tenant identifier ever contains a quote or backslash.
//
// Range vector window is fixed at 5m for instant aggregations; this matches
// the recording window used elsewhere in the platform and the RFC.
const (
	overviewHallucinationRateQuery = `histogram_quantile(0.5, sum by (le) (rate(ai_hallucination_rate_bucket{tenant_id=%q}[5m])))`
	overviewAvgTTFTQuery           = `histogram_quantile(0.5, sum by (le) (rate(ai_inference_ttft_seconds_bucket{tenant_id=%q}[5m])))`
	overviewAvgTPOTQuery           = `histogram_quantile(0.5, sum by (le) (rate(ai_inference_tpot_seconds_bucket{tenant_id=%q}[5m])))`

	hallucinationTimeSeriesQuery = `histogram_quantile(0.5, sum by (le) (rate(ai_hallucination_rate_bucket{tenant_id=%q}[5m])))`
	biasTimeSeriesQuery          = `histogram_quantile(0.5, sum by (le) (rate(ai_bias_score_bucket{tenant_id=%q}[5m])))`
	toxicityTimeSeriesQuery      = `histogram_quantile(0.5, sum by (le) (rate(ai_toxicity_score_bucket{tenant_id=%q}[5m])))`

	ttftTimeSeriesQuery = `histogram_quantile(0.5, sum by (le) (rate(ai_inference_ttft_seconds_bucket{tenant_id=%q}[5m])))`
	tpotTimeSeriesQuery = `histogram_quantile(0.5, sum by (le) (rate(ai_inference_tpot_seconds_bucket{tenant_id=%q}[5m])))`
)

// Numeric thresholds for the overview alert flag.
//   - hallucination rate > 4% (0.04 as a 0..1 fraction)
//   - TTFT > 140ms (0.14 seconds)
//   - TPOT > 60ms  (0.06 seconds)
const (
	alertHallucinationRateSeconds = 0.04
	alertTTFTSeconds              = 0.14
	alertTPOTSeconds              = 0.06
)

// Default range-query window when caller passes a zero start/end pair.
const (
	defaultRangeWindow = 24 * 60 // minutes — kept as int for clarity in callers
	defaultRangeStep   = 60      // minutes
)
