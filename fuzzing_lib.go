package shield

import (
	"encoding/binary"
	"foundation"
	"foundation/entropy"
	"math"
	"shield/internal"
	"time"
)

/*
SHIELD_Fuzzing_Pattern defines the statistical heuristic the generator uses to yield data.
It dictates the *shape* of the entropy, not the *volume* of the execution.
*/
type SHIELD_Fuzzing_Pattern = internal.FuzzingPattern

const (
	/*
		SHIELD_Fuzzing_EdgeCases strictly yields known boundary values (e.g., 0, 1, -1, Min, Max).
		Use this for fast, minimal-overhead Level 0 sanity checks to ensure catastrophic bounds are caught immediately.
	*/
	SHIELD_Fuzzing_EdgeCases = internal.FuzzingPattern_EdgeCases

	/*
		SHIELD_Fuzzing_Standard yields a uniform distribution of pseudo-random values.
		Use this for typical CI runs to ensure the generic logic paths can handle a wide spread of valid data.
	*/
	SHIELD_Fuzzing_Standard = internal.FuzzingPattern_Standard

	/*
		SHIELD_Fuzzing_Adversarial heavily biases generation towards dangerous boundaries and applies
		bitwise mutations (e.g., MaxValue - 1, random bit flips). Use this for time-boxed stress tests
		to actively hunt for overflows and off-by-one errors.
	*/
	SHIELD_Fuzzing_Adversarial = internal.FuzzingPattern_Adversarial
)

/*
SHIELD_Testing_FuzzingContext encapsulates the information used for the generators to produce input.
*/
type SHIELD_Testing_FuzzingContext = internal.FuzzingContext

/*
SHIELD_Fuzzing_IntGenerator creates a fuzzing generator for any integer type.

[Context]
Integer-heavy systems often fail at boundaries first (overflow edges, sign flips,
off-by-one conditions). This generator centralizes integer input production so
all guards can share deterministic behavior from the same entropy stream.

[Algorithmic Approach]
The function precomputes integer edge values for TInt (0, 1, min, max, and -1
for signed types). Each iteration consumes entropy and dispatches behavior by
pattern:
  - EdgeCases: select one precomputed edge.
  - Standard: cast raw entropy to TInt for broad coverage.
  - Adversarial: mutate edge values via near-boundary deltas (+/-1 and +/-small
    offsets) to stress off-by-one and threshold logic.

[Use Cases]
- Fast boundary validation for arithmetic and indexing code paths.
- CI broad sampling over integer domains with deterministic replay.
- Stress runs targeting overflow and near-boundary logic defects.

[Parameters]
TInt is any integer type supported by foundation.Integer.

[Returns]
An InputGenerator that binds to a FuzzingContext and yields an InputIterator for
deterministic integer values.

[Side Effects]
No global side effects. Generated iterators consume and advance the entropy
provider stored in the provided FuzzingContext.

[Thread Safety]
The generator function itself is pure; iterator safety depends on the shared
EntropyProvider in the context and is not concurrent-safe without external
synchronization.

[Complexity]
Generator construction: O(1)
Each yielded value: O(1)
*/
func SHIELD_Fuzzing_IntGenerator[TInt foundation.Integer]() internal.InputGenerator[TInt] {
	return func(ctx internal.FuzzingContext) internal.InputIterator[TInt] {
		edges := precomputeIntegerEdges[TInt]()

		return func() TInt {
			entropy := ctx.EntropyProvider.NextUint64()

			switch ctx.Effort {
			case internal.FuzzingPattern_EdgeCases:
				return selectIntegerEdge(entropy, edges)
			case internal.FuzzingPattern_Standard:
				return TInt(entropy)
			case internal.FuzzingPattern_Adversarial:
				return generateAdversarialInt(entropy, edges)
			default:
				return edges[0]
			}
		}
	}
}

func precomputeIntegerEdges[TInt foundation.Integer]() []TInt {
	zero := foundation.FromInt[TInt](0)
	edges := []TInt{
		zero,
		foundation.FromInt[TInt](1),
		foundation.MinValue[TInt](),
		foundation.MaxValue[TInt](),
	}

	if foundation.MinValue[TInt]() < zero {
		edges = append(edges, foundation.FromInt[TInt](-1))
	}
	return edges
}

func selectIntegerEdge[TInt foundation.Integer](entropy uint64, edges []TInt) TInt {
	return edges[entropy%uint64(len(edges))]
}

func generateAdversarialInt[TInt foundation.Integer](entropy uint64, edges []TInt) TInt {
	baseEdge := selectIntegerEdge(entropy, edges)
	mutationType := (entropy >> 60) % 4

	switch mutationType {
	case 0:
		return baseEdge + 1
	case 1:
		return baseEdge - 1
	case 2:
		return baseEdge + TInt(entropy%10)
	case 3:
		return baseEdge - TInt(entropy%10)
	default:
		return baseEdge ^ TInt(entropy)
	}
}

/*
SHIELD_Fuzzing_Float64Generator creates a fuzzing generator specialized for float64.

[Context]
Floating-point behavior has non-obvious edge domains (NaN payloads, signed zero,
infinities, denormals) that frequently expose hidden assumptions in numeric
logic. This generator explicitly targets those values while still supporting
broad randomized bit-pattern exploration.

[Algorithmic Approach]
The iterator consumes entropy and dispatches by pattern:
  - EdgeCases: chooses from a fixed table of critical IEEE-754 values.
  - Standard: interprets entropy bits directly as float64.
  - Adversarial: selects an edge value and applies bit-level mutations (sign
    flip, mantissa perturbation, or minimal-bit nudge) to explore unstable
    numeric neighborhoods.

[Use Cases]
- Numeric parser and math-pipeline validation.
- Guarding against NaN/Inf propagation regressions.
- Stressing sign-sensitive code around +/-0 and extreme magnitudes.

[Returns]
An InputGenerator that binds to a FuzzingContext and yields float64 values.

[Side Effects]
No global side effects. Generated iterators consume provider-local entropy from
the context.

[Edge Cases]
Generated values may include NaN and infinities. Callers must ensure predicates
and comparators handle IEEE-754 special values correctly.

[Complexity]
Generator construction: O(1)
Each yielded value: O(1)
*/
func SHIELD_Fuzzing_Float64Generator() internal.InputGenerator[float64] {
	return func(ctx internal.FuzzingContext) internal.InputIterator[float64] {
		edges := precomputeFloat64Edges()

		return func() float64 {
			entropy := ctx.EntropyProvider.NextUint64()

			switch ctx.Effort {
			case internal.FuzzingPattern_EdgeCases:
				return edges[entropy%uint64(len(edges))]
			case internal.FuzzingPattern_Standard:
				return math.Float64frombits(entropy)
			case internal.FuzzingPattern_Adversarial:
				return generateAdversarialFloat64(entropy, edges)
			default:
				return 0.0
			}
		}
	}
}

func precomputeFloat64Edges() []float64 {
	return []float64{
		0.0,
		math.Copysign(0, -1),
		math.MaxFloat64,
		-math.MaxFloat64,
		math.SmallestNonzeroFloat64,
		math.Inf(1),
		math.Inf(-1),
		math.NaN(),
	}
}

func generateAdversarialFloat64(entropy uint64, edges []float64) float64 {
	baseBits := math.Float64bits(edges[entropy%uint64(len(edges))])
	mutationType := (entropy >> 56) % 3

	switch mutationType {
	case 0:
		return math.Float64frombits(baseBits ^ (1 << 63))
	case 1:
		return math.Float64frombits(baseBits ^ (entropy & 0x000FFFFFFFFFFFFF))
	case 2:
		return math.Float64frombits(baseBits ^ 1)
	default:
		return math.Float64frombits(baseBits)
	}
}

/*
SHIELD_Fuzzing_StringGenerator creates a fuzzing generator for strings.

[Context]
String handling bugs often emerge from control characters, quoting/escaping,
embedded null bytes, and unusually large payloads. This generator provides
patterned string synthesis to cover both typical and adversarial cases.

[Algorithmic Approach]
The iterator consumes entropy and dispatches by pattern:
  - EdgeCases: returns predefined hazardous literals (empty, null byte, quotes,
    escapes, control-sequence payloads, and non-ASCII samples).
  - Standard: builds rune-based strings from broad Unicode code-point coverage.
  - Adversarial: emits bounded raw byte-heavy strings from entropy blocks.

[Use Cases]
- Parser and serializer robustness checks.
- Input sanitation and escaping validation.
- Memory/latency pressure testing on text-processing paths.

[Returns]
An InputGenerator that binds to a FuzzingContext and yields string values.

[Side Effects]
Allocates per generated string in Standard and Adversarial modes. No global
state is mutated.

[Edge Cases]
Adversarial mode may generate binary-like strings with embedded nulls and
non-printable bytes. Callers should avoid assumptions about UTF-8 validity or
printability in downstream assertions.

[Complexity]
Generator construction: O(1)
Each yielded value: O(n) where n is generated string length.
*/
func SHIELD_Fuzzing_StringGenerator() internal.InputGenerator[string] {
	return func(ctx internal.FuzzingContext) internal.InputIterator[string] {
		edges := precomputeStringEdges()

		return func() string {
			entropy := ctx.EntropyProvider.NextUint64()

			switch ctx.Effort {
			case internal.FuzzingPattern_EdgeCases:
				return edges[entropy%uint64(len(edges))]
			case internal.FuzzingPattern_Standard:
				return generateStandardString(entropy, ctx.EntropyProvider)
			case internal.FuzzingPattern_Adversarial:
				return generateAdversarialString(entropy, ctx.EntropyProvider)
			default:
				return ""
			}
		}
	}
}

func precomputeStringEdges() []string {
	return []string{
		"",
		"\x00",
		"\"",
		"'",
		"\\",
		"\n\r\t",
		"🤷‍♂️",
	}
}

func generateStandardString(entropy uint64, provider *entropy.EntropyProvider) string {
	length := int(entropy % 256)
	runes := make([]rune, length)

	for i := 0; i < length; i++ {
		runes[i] = rune(provider.NextUint64() % 0x10FFFF)
	}

	return string(runes)
}

func generateAdversarialString(entropy uint64, provider *entropy.EntropyProvider) string {
	length := int(entropy % 4096)
	buf := make([]byte, length)

	for i := 0; i < length; i += 8 {
		val := provider.NextUint64()
		writeSafelyToBuffer(buf, i, val)
	}

	return string(buf)
}

func writeSafelyToBuffer(buf []byte, offset int, val uint64) {
	remaining := len(buf) - offset
	if remaining >= 8 {
		binary.LittleEndian.PutUint64(buf[offset:], val)
		return
	}

	for j := 0; j < remaining; j++ {
		buf[offset+j] = byte(val >> (8 * j))
	}
}

/*
SHIELD_Fuzzing_SliceGenerator lifts an item generator into a slice generator.

[Context]
Many system failures occur in collection handling (empty vs nil assumptions,
allocation growth, iteration logic). This combinator enables fuzzing slices
without rewriting element generation logic.

[Algorithmic Approach]
The generator creates an item iterator from itemGen(ctx), then selects slice
length from entropy according to pattern:
  - EdgeCases: zero-length (returned as nil).
  - Standard: bounded moderate sizes.
  - Adversarial: larger bounded sizes intended to pressure allocation and
    traversal paths.

It then fills the slice by repeatedly pulling from the item iterator.

[Use Cases]
- Fuzzing API endpoints or functions that process variable-size collections.
- Stressing allocation behavior and loop invariants.
- Reusing existing scalar generators to test nested/compound inputs.

[Parameters]
itemGen yields deterministic item values for the same FuzzingContext.

[Returns]
An InputGenerator producing slices of T values.

[Side Effects]
Allocates output slices for non-zero lengths. Consumes entropy from the shared
context provider via both length selection and item generation.

[Invariants]
Returned slices are deterministic for a deterministic entropy stream and
deterministic item generator.

[Complexity]
Generator construction: O(1)
Each yielded value: O(n) time and O(n) space, where n is generated slice length.
*/
func SHIELD_Fuzzing_SliceGenerator[T any](
	itemGen internal.InputGenerator[T],
) internal.InputGenerator[[]T] {
	return func(ctx internal.FuzzingContext) internal.InputIterator[[]T] {
		itemIterator := itemGen(ctx)

		return func() []T {
			entropy := ctx.EntropyProvider.NextUint64()
			length := determineSliceLength(ctx.Effort, entropy)

			if length == 0 {
				return nil
			}

			return populateSlice(length, itemIterator)
		}
	}
}

func determineSliceLength(effort internal.FuzzingPattern, entropy uint64) int {
	switch effort {
	case internal.FuzzingPattern_EdgeCases:
		return 0
	case internal.FuzzingPattern_Standard:
		return int(entropy % 100)
	case internal.FuzzingPattern_Adversarial:
		return int(entropy % 1024)
	default:
		return 0
	}
}

func populateSlice[T any](length int, iterator internal.InputIterator[T]) []T {
	out := make([]T, length)

	for i := 0; i < length; i++ {
		out[i] = iterator()
	}

	return out
}

/*
SHIELD_Fuzzing_BytesGenerator creates a fuzzing generator for byte slices.

[Context]
Binary protocols and serialization layers frequently fail on malformed payloads,
empty/nil distinctions, and large opaque byte blobs. This generator focuses on
those failure surfaces while remaining deterministic under a fixed entropy tape.

[Algorithmic Approach]
The iterator consumes entropy and dispatches by pattern:
  - EdgeCases: selects from predefined byte-edge payloads (nil, empty, BOM,
    CRLF, single-byte boundaries).
  - Standard: emits bounded random byte arrays.
  - Adversarial: emits larger bounded byte arrays packed from raw 64-bit blocks.

[Use Cases]
- Decoder/encoder robustness checks.
- Validation of nil vs empty payload semantics.
- Stressing binary parsers with high-entropy blobs.

[Returns]
An InputGenerator that binds to a FuzzingContext and yields []byte values.

[Side Effects]
Allocates output buffers in Standard and Adversarial modes. No global state is
mutated.

[Complexity]
Generator construction: O(1)
Each yielded value: O(n) where n is output byte length.
*/
func SHIELD_Fuzzing_BytesGenerator() internal.InputGenerator[[]byte] {
	return func(ctx internal.FuzzingContext) internal.InputIterator[[]byte] {
		edges := precomputeBytesEdges()

		return func() []byte {
			entropy := ctx.EntropyProvider.NextUint64()

			switch ctx.Effort {
			case internal.FuzzingPattern_EdgeCases:
				return selectBytesEdge(entropy, edges)
			case internal.FuzzingPattern_Standard:
				return generateStandardBytes(entropy, ctx.EntropyProvider)
			case internal.FuzzingPattern_Adversarial:
				return generateAdversarialBytes(entropy, ctx.EntropyProvider)
			default:
				return nil
			}
		}
	}
}

func precomputeBytesEdges() [][]byte {
	return [][]byte{
		nil,
		{},
		{0x00},
		{0xFF},
		{0xEF, 0xBB, 0xBF}, // UTF-8 BOM
		{0x0D, 0x0A},       // CRLF
	}
}

func selectBytesEdge(entropy uint64, edges [][]byte) []byte {
	return edges[entropy%uint64(len(edges))]
}

func generateStandardBytes(entropy uint64, provider *entropy.EntropyProvider) []byte {
	length := int(entropy % 256)
	buf := make([]byte, length)

	for i := 0; i < length; i++ {
		buf[i] = byte(provider.NextUint64() % 256)
	}

	return buf
}

func generateAdversarialBytes(entropy uint64, provider *entropy.EntropyProvider) []byte {
	length := int(entropy % 4096)
	buf := make([]byte, length)

	for i := 0; i < length; i += 8 {
		writeSafelyToBuffer(buf, i, provider.NextUint64())
	}

	return buf
}

/*
SHIELD_Fuzzing_DurationGenerator creates a fuzzing generator for time.Duration.

[Context]
Duration logic commonly breaks around sign, unit boundaries, and near-overflow
values. This generator targets those regions to expose timer, retry, and timeout
bugs quickly.

[Algorithmic Approach]
The iterator consumes entropy and dispatches by pattern:
  - EdgeCases: chooses from a fixed set containing zero, +/-1, unit anchors,
    and duration min/max values.
  - Standard: samples bounded positive durations up to roughly one year.
  - Adversarial: mutates edge durations via +/-1, bitwise xor, and sign inversion.

[Use Cases]
- Timeout/retry policy validation.
- Scheduler and backoff testing.
- Overflow and sign-handling regression checks.

[Returns]
An InputGenerator that binds to a FuzzingContext and yields time.Duration values.

[Edge Cases]
Adversarial mutations can produce negative durations and unusual bit patterns.
Callers should ensure downstream code explicitly handles signed durations.

[Complexity]
Generator construction: O(1)
Each yielded value: O(1)
*/
func SHIELD_Fuzzing_DurationGenerator() internal.InputGenerator[time.Duration] {
	return func(ctx internal.FuzzingContext) internal.InputIterator[time.Duration] {
		edges := precomputeDurationEdges()

		return func() time.Duration {
			entropy := ctx.EntropyProvider.NextUint64()

			switch ctx.Effort {
			case internal.FuzzingPattern_EdgeCases:
				return edges[entropy%uint64(len(edges))]
			case internal.FuzzingPattern_Standard:
				return time.Duration(entropy % uint64(time.Hour*24*365)) // Up to ~1 year
			case internal.FuzzingPattern_Adversarial:
				return generateAdversarialDuration(entropy, edges)
			default:
				return 0
			}
		}
	}
}

func precomputeDurationEdges() []time.Duration {
	return []time.Duration{
		0,
		1,
		-1,
		time.Nanosecond,
		-time.Nanosecond,
		time.Second,
		time.Hour,
		time.Duration(1<<63 - 1),  // Max
		time.Duration(-(1 << 63)), // Min
	}
}

func generateAdversarialDuration(entropy uint64, edges []time.Duration) time.Duration {
	base := edges[entropy%uint64(len(edges))]
	mutation := (entropy >> 62)

	switch mutation {
	case 0:
		return base + 1
	case 1:
		return base - 1
	case 2:
		return base ^ time.Duration(entropy)
	default:
		return -base
	}
}

/*
SHIELD_Fuzzing_TimeGenerator creates a fuzzing generator for time.Time.

[Context]
Temporal logic often fails at representational boundaries (zero value, epoch,
large-year limits, signed epoch transitions). This generator targets those
regions with deterministic coverage.

[Algorithmic Approach]
The iterator consumes entropy and dispatches by pattern:
  - EdgeCases: selects from a fixed set of boundary timestamps.
  - Standard: builds UTC unix timestamps within a bounded second/nanosecond range.
  - Adversarial: perturbs edge timestamps by +/- bounded offsets.

[Use Cases]
- Timestamp normalization and storage tests.
- Date/time parser and formatter validation.
- Boundary checks around epoch transitions and extreme calendar values.

[Returns]
An InputGenerator that binds to a FuzzingContext and yields time.Time values.

[Side Effects]
No global side effects. Iterator consumes provider entropy from context.

[Complexity]
Generator construction: O(1)
Each yielded value: O(1)
*/
func SHIELD_Fuzzing_TimeGenerator() internal.InputGenerator[time.Time] {
	return func(ctx internal.FuzzingContext) internal.InputIterator[time.Time] {
		edges := precomputeTimeEdges()

		return func() time.Time {
			entropy := ctx.EntropyProvider.NextUint64()

			switch ctx.Effort {
			case internal.FuzzingPattern_EdgeCases:
				return edges[entropy%uint64(len(edges))]
			case internal.FuzzingPattern_Standard:
				return time.Unix(int64(entropy%2000000000), int64(ctx.EntropyProvider.NextUint64()%1e9)).UTC()
			case internal.FuzzingPattern_Adversarial:
				return generateAdversarialTime(entropy, edges)
			default:
				return time.Time{}
			}
		}
	}
}

func precomputeTimeEdges() []time.Time {
	return []time.Time{
		{},                                       // Zero time
		time.Unix(0, 0).UTC(),                    // Epoch
		time.Unix(1<<31-1, 0).UTC(),              // Y2K38
		time.Unix(-(1 << 31), 0).UTC(),           // Negative Y2K38
		time.Unix(253402300799, 999999999).UTC(), // 9999-12-31T23:59:59.999999999Z
	}
}

func generateAdversarialTime(entropy uint64, edges []time.Time) time.Time {
	base := edges[entropy%uint64(len(edges))]
	offset := time.Duration(entropy % uint64(time.Hour*24))

	if entropy%2 == 0 {
		return base.Add(offset)
	}
	return base.Add(-offset)
}

/*
SHIELD_Fuzzing_MapGenerator lifts key/value generators into a map generator.

[Context]
Map-heavy logic can fail on empty-map assumptions, key-collision behavior,
mutation ordering assumptions, and scale pressure. This combinator enables map
fuzzing while reusing deterministic key/value generators.

[Algorithmic Approach]
The generator creates key/value iterators from the provided generators, selects
map size by pattern, and inserts generated pairs into a new map:
  - EdgeCases: zero size (returned as nil).
  - Standard: bounded moderate sizes.
  - Adversarial: larger bounded sizes with strict cap to avoid OOM blowups.

Key collisions are naturally resolved by map overwrite semantics.

[Use Cases]
- Fuzzing APIs that ingest dictionaries/config maps.
- Testing key-collision resilience and overwrite behavior.
- Composing nested fuzz inputs from reusable scalar generators.

[Parameters]
keyGen yields deterministic keys for the supplied FuzzingContext.
valGen yields deterministic values for the supplied FuzzingContext.

[Returns]
An InputGenerator producing map[K]V values.

[Side Effects]
Allocates maps for non-zero sizes. Consumes entropy through size selection and
through key/value iterators.

[Complexity]
Generator construction: O(1)
Each yielded value: O(n) expected time and O(n) space, where n is target map
size (subject to key-collision overwrite effects).
*/
func SHIELD_Fuzzing_MapGenerator[K comparable, V any](
	keyGen internal.InputGenerator[K],
	valGen internal.InputGenerator[V],
) internal.InputGenerator[map[K]V] {
	return func(ctx internal.FuzzingContext) internal.InputIterator[map[K]V] {
		keyIter := keyGen(ctx)
		valIter := valGen(ctx)

		return func() map[K]V {
			entropy := ctx.EntropyProvider.NextUint64()
			length := determineMapLength(ctx.Effort, entropy)

			if length == 0 {
				return nil
			}

			return populateMap(length, keyIter, valIter)
		}
	}
}

func determineMapLength(effort internal.FuzzingPattern, entropy uint64) int {
	switch effort {
	case internal.FuzzingPattern_EdgeCases:
		return 0
	case internal.FuzzingPattern_Standard:
		return int(entropy % 50)
	case internal.FuzzingPattern_Adversarial:
		return int(entropy % 512) // Strictly capped to prevent combinatoric OOMs
	default:
		return 0
	}
}

func populateMap[K comparable, V any](length int, keyIter internal.InputIterator[K], valIter internal.InputIterator[V]) map[K]V {
	out := make(map[K]V, length)

	for i := 0; i < length; i++ {
		out[keyIter()] = valIter()
	}

	return out
}
