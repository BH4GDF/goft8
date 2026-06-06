package goft8

// Logger is the minimal interface goft8 uses for diagnostic output.
// It is satisfied by the stdlib *log.Logger and most third-party
// loggers; a *slog.Logger can be adapted in a few lines.
type Logger interface {
	Printf(format string, args ...any)
}

// DecoderOption configures a Decoder at construction time.
type DecoderOption func(*decoderConfig)

// decoderConfig holds all tunables set via functional options. It is
// flattened into DecodeParams inside Decoder.Decode.
type decoderConfig struct {
	myCall            string
	dxCall            string
	freqMin           int
	freqMax           int
	depth             int
	maxPasses         int
	apEnabled         bool
	apEnabledSet      bool
	cqOnlyAP          bool
	audioStartSeconds float64
	logger            Logger
	workers           int
}

func defaultDecoderConfig() decoderConfig {
	return decoderConfig{
		freqMin:           100,
		freqMax:           3000,
		depth:             DepthNormal,
		maxPasses:         3,
		audioStartSeconds: 0.5,
	}
}

// WithMyCall sets the operator's callsign, used for AP decoding of
// messages addressed to the user. Standard ITU form, or compound
// (e.g. "W1ABC/P"). If empty, AP types 2+ are disabled.
func WithMyCall(call string) DecoderOption {
	return func(c *decoderConfig) { c.myCall = call }
}

// WithDxCall sets the DX station's callsign for AP type 3+ decoding
// during an active QSO.
func WithDxCall(call string) DecoderOption {
	return func(c *decoderConfig) { c.dxCall = call }
}

// WithFreqRange restricts the audio frequency search to [min, max] Hz.
// Defaults to 100..3000 Hz.
func WithFreqRange(min, max int) DecoderOption {
	return func(c *decoderConfig) {
		c.freqMin = min
		c.freqMax = max
	}
}

// WithDepth selects LDPC decoder depth. Use DepthFast, DepthNormal, or
// DepthDeep.
func WithDepth(depth int) DecoderOption {
	return func(c *decoderConfig) { c.depth = depth }
}

// WithMaxPasses sets the number of iterative subtraction passes.
// Defaults to 3.
func WithMaxPasses(n int) DecoderOption {
	return func(c *decoderConfig) { c.maxPasses = n }
}

// WithAPEnabled turns a-priori decoding on or off. Defaults to true
// when MyCall is set, false otherwise.
func WithAPEnabled(enabled bool) DecoderOption {
	return func(c *decoderConfig) {
		c.apEnabled = enabled
		c.apEnabledSet = true
	}
}

// WithCQOnlyAP restricts AP to the CQ prior only. Defaults to false
// when MyCall is set.
func WithCQOnlyAP(cqOnly bool) DecoderOption {
	return func(c *decoderConfig) { c.cqOnlyAP = cqOnly }
}

// WithAudioStartSeconds lets the caller specify how many seconds of
// silence to expect before the nominal TX start. Defaults to 0.5 s.
// Advanced users only.
func WithAudioStartSeconds(s float64) DecoderOption {
	return func(c *decoderConfig) { c.audioStartSeconds = s }
}

// WithLogger attaches an optional logger for internal diagnostics.
// Default: no logging.
func WithLogger(logger Logger) DecoderOption {
	return func(c *decoderConfig) { c.logger = logger }
}

// WithWorkers enables concurrent decoding using goroutines.
// n <= 0 uses runtime.NumCPU(). Default 0 means serial decoding.
func WithWorkers(n int) DecoderOption {
	return func(c *decoderConfig) { c.workers = n }
}
