package generic

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"unicode"
)

// InferenceConfig holds thresholds for auto-inference.
type InferenceConfig struct {
	AutoApplyThreshold  float64 // Overall confidence threshold for auto_confirmed (default 0.85)
	ConflictMargin      float64 // Max diff between top-1 and top-2 before conflict (default 0.10)
	DefaultExtractEvent string  // Default SSE event type for chain extraction (default "message_end")
}

// DefaultInferenceConfig returns the default inference configuration.
func DefaultInferenceConfig() InferenceConfig {
	return InferenceConfig{
		AutoApplyThreshold:  0.85,
		ConflictMargin:      0.10,
		DefaultExtractEvent: "message_end",
	}
}

// InferIntegrationByMultiRound analyzes 2~5 rounds of request/response packets
// and infers a GenericProfile with confidence scoring.
func InferIntegrationByMultiRound(spec MultiRoundSpec) (*InferredIntegration, error) {
	return InferIntegrationByMultiRoundWithConfig(spec, DefaultInferenceConfig())
}

// InferIntegrationByMultiRoundWithConfig performs inference with custom thresholds.
func InferIntegrationByMultiRoundWithConfig(spec MultiRoundSpec, cfg InferenceConfig) (*InferredIntegration, error) {
	// Validate inputs
	if err := validateMultiRoundSpec(spec); err != nil {
		return nil, err
	}

	// Parse URL
	rawURL := strings.TrimSpace(spec.URL)
	if rawURL == "" {
		return nil, fmt.Errorf("generic: URL is required")
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("generic: invalid URL %q: %w", spec.URL, err)
	}
	baseURL := parsed.Scheme + "://" + parsed.Host
	path := parsed.Path
	if parsed.RawQuery != "" {
		path += "?" + parsed.RawQuery
	}

	// Phase 1: Parse all round bodies into flat path→value maps
	rounds, err := parseRounds(spec.Rounds)
	if err != nil {
		return nil, err
	}

	// Phase 2: Cross-round analysis
	candidates := analyzeFields(rounds, cfg)

	// Phase 3: Resolve conflicts and compute confidence
	fields, warnings := resolveFields(candidates, cfg)

	// Phase 4: Parse first round response for stream config (protocol, delta, done)
	respSpec, _ := parseHTTPResponseSpec(spec.Rounds[0].Response.Body, nil)

	// Phase 5: Build profile from inferred fields + response spec
	profile, err := buildProfileFromFields(fields, rounds, path, &respSpec)
	if err != nil {
		return nil, err
	}
	profile.Conversation = spec.Conversation

	// Phase 6: Extract credential from first round request headers
	var cred interface{}
	var extraHeaders map[string]string
	if len(spec.Rounds) > 0 && len(spec.Rounds[0].Request.Headers) > 0 {
		c, eh, credErr := ExtractCredential(spec.Rounds[0].Request.Headers)
		if credErr == nil {
			cred = c
			extraHeaders = eh
		} else {
			warnings = append(warnings, fmt.Sprintf("credential extraction failed: %v", credErr))
		}
	}

	// Phase 7: Compute overall confidence and status
	report := buildReport(fields, warnings, cfg)

	return &InferredIntegration{
		Profile:      profile,
		Report:       report,
		Credential:   cred,
		BaseURL:      baseURL,
		ExtraHeaders: extraHeaders,
	}, nil
}

// validateMultiRoundSpec validates the MultiRoundSpec input.
func validateMultiRoundSpec(spec MultiRoundSpec) error {
	if spec.Conversation.Mode == "" {
		return fmt.Errorf("generic: conversation mode is required")
	}
	if spec.Conversation.Mode != "remote_session" && spec.Conversation.Mode != "local_history" {
		return fmt.Errorf("generic: invalid conversation mode %q, must be remote_session or local_history", spec.Conversation.Mode)
	}
	if len(spec.Rounds) < 2 || len(spec.Rounds) > 5 {
		return fmt.Errorf("generic: multi_round_spec requires 2-5 rounds with request/response in each round")
	}
	for i, r := range spec.Rounds {
		if strings.TrimSpace(r.Request.Body) == "" {
			return fmt.Errorf("generic: round[%d].request.body is required", i)
		}
		if strings.TrimSpace(r.Response.Body) == "" {
			return fmt.Errorf("generic: round[%d].response.body is required", i)
		}
	}
	return nil
}

// parsedRound holds the flattened JSON paths for a single round.
type parsedRound struct {
	reqPaths  map[string]string // JSONPath → scalar string value
	respPaths map[string]string
}

// parseRounds parses all round bodies into flat path→value maps.
func parseRounds(rounds []RoundPair) ([]parsedRound, error) {
	result := make([]parsedRound, len(rounds))
	for i, r := range rounds {
		reqBody := strings.TrimSpace(r.Request.Body)
		respBody := strings.TrimSpace(r.Response.Body)

		var reqObj any
		if err := json.Unmarshal([]byte(reqBody), &reqObj); err != nil {
			return nil, fmt.Errorf("generic: round[%d].request body not valid JSON: %w", i, err)
		}
		var respObj any
		if err := json.Unmarshal([]byte(respBody), &respObj); err != nil {
			return nil, fmt.Errorf("generic: round[%d].response body not valid JSON: %w", i, err)
		}

		result[i] = parsedRound{
			reqPaths:  flattenJSON(reqObj, ""),
			respPaths: flattenJSON(respObj, ""),
		}
	}
	return result, nil
}

// flattenJSON recursively flattens a JSON value into a map of dotted paths → string values.
func flattenJSON(v any, prefix string) map[string]string {
	result := make(map[string]string)
	flattenRecursive(v, prefix, result)
	return result
}

func flattenRecursive(v any, prefix string, result map[string]string) {
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			flattenRecursive(child, path, result)
		}
	case []any:
		for i, child := range val {
			path := fmt.Sprintf("%d", i)
			if prefix != "" {
				path = prefix + "." + path
			}
			flattenRecursive(child, path, result)
		}
	case string:
		result[prefix] = val
	case float64:
		result[prefix] = fmt.Sprintf("%v", val)
	case bool:
		result[prefix] = fmt.Sprintf("%v", val)
	case nil:
		result[prefix] = ""
	}
}

// fieldCandidate holds classification candidates for a single request field.
type fieldCandidate struct {
	requestPath string
	candidates  []classCandidate
}

type classCandidate struct {
	class      string  // input, session_id, chain, dynamic, static
	confidence float64 // [0, 1]
	reason     string
	respPath   string // for chain/session_id: which response path it maps to
}

// sessionIDKeywords helps identify session_id fields by name.
var sessionIDKeywords = []string{
	"session", "conversation", "thread", "chat_session", "conv_id",
}

// inputSemanticKeywords identify fields that are likely user-authored text.
var inputSemanticKeywords = []string{
	"prompt", "query", "question", "input", "message", "content", "text",
}

// technicalDynamicKeywords identify machine-generated changing fields.
var technicalDynamicKeywords = []string{
	"req_id", "request_id", "parent_req_id", "client_tm", "client_time",
	"timestamp", "nonce", "sign", "signature", "trace_id", "scene_param",
}

// analyzeFields performs cross-round diff analysis and classifies each request field.
func analyzeFields(rounds []parsedRound, cfg InferenceConfig) []fieldCandidate {
	if len(rounds) < 2 {
		return nil
	}

	// Collect all request paths across rounds
	allReqPaths := make(map[string]struct{})
	for _, r := range rounds {
		for p := range r.reqPaths {
			allReqPaths[p] = struct{}{}
		}
	}

	// Build response value index: respPath → (roundIdx → value)
	respIndex := make(map[string]map[int]string)
	for i, r := range rounds {
		for p, v := range r.respPaths {
			if _, ok := respIndex[p]; !ok {
				respIndex[p] = make(map[int]string)
			}
			respIndex[p][i] = v
		}
	}

	var result []fieldCandidate
	sortedPaths := sortedKeys(allReqPaths)

	for _, reqPath := range sortedPaths {
		fc := fieldCandidate{requestPath: reqPath}

		// Gather values across rounds
		values := make([]string, len(rounds))
		present := make([]bool, len(rounds))
		for i, r := range rounds {
			if v, ok := r.reqPaths[reqPath]; ok {
				values[i] = v
				present[i] = true
			}
		}

		// Skip paths only present in one round
		presentCount := 0
		for _, p := range present {
			if p {
				presentCount++
			}
		}
		if presentCount <= 1 {
			continue
		}

		// Check if values change across rounds
		allSame := true
		for i := 1; i < len(values); i++ {
			if present[i] && present[i-1] && values[i] != values[i-1] {
				allSame = false
				break
			}
		}

		// Check if value is a placeholder ($$$)
		isPlaceholder := false
		for _, v := range values {
			if v == "$$$" {
				isPlaceholder = true
				break
			}
		}

		if isPlaceholder {
			// User already marked this as input
			fc.candidates = append(fc.candidates, classCandidate{
				class:      "input",
				confidence: 1.0,
				reason:     "field value is $$$ placeholder (user-marked input)",
			})
			result = append(result, fc)
			continue
		}

		if allSame {
			// Stable fields can still be session_id if they are traceable from response chain.
			fc.candidates = classifyStableField(reqPath, values, present, rounds, respIndex)
			if len(fc.candidates) == 0 {
				// Static field: same value across all rounds
				fc.candidates = append(fc.candidates, classCandidate{
					class:      "static",
					confidence: 0.95,
					reason:     "value unchanged across all rounds",
				})
			}
			result = append(result, fc)
			continue
		}

		// Value changes across rounds — check what it maps to
		fc.candidates = classifyChangingField(reqPath, values, present, rounds, respIndex, cfg)
		if len(fc.candidates) > 0 {
			result = append(result, fc)
		}
	}

	return result
}

// classifyStableField determines classification for unchanged fields.
func classifyStableField(
	reqPath string,
	values []string,
	present []bool,
	rounds []parsedRound,
	respIndex map[string]map[int]string,
) []classCandidate {
	var candidates []classCandidate
	sessionScore, sessionRespPath, sessionReason := checkSessionIDPattern(reqPath, values, present, rounds, respIndex)
	if sessionScore > 0 {
		candidates = append(candidates, classCandidate{
			class:      "session_id",
			confidence: sessionScore,
			reason:     sessionReason,
			respPath:   sessionRespPath,
		})
	}
	return candidates
}

// classifyChangingField determines the classification for a field whose value changes across rounds.
func classifyChangingField(
	reqPath string,
	values []string,
	present []bool,
	rounds []parsedRound,
	respIndex map[string]map[int]string,
	cfg InferenceConfig,
) []classCandidate {
	var candidates []classCandidate

	// Check session_id pattern: response-traceable chain with session semantics.
	sessionScore, sessionRespPath, sessionReason := checkSessionIDPattern(reqPath, values, present, rounds, respIndex)

	// Check chain pattern: round[i+1].req[path] == round[i].resp[somePath]
	// (same as session_id but without the empty-first-round requirement)
	chainScore, chainRespPath := checkChainPattern(reqPath, values, present, rounds, respIndex)

	// No response mapping: classify by input semantics vs technical dynamics.
	inputScore := 0.0
	inputReason := ""
	dynamicScore := 0.0
	dynamicReason := ""
	if sessionScore == 0 && chainScore == 0 {
		inputScore, inputReason = scoreInputCandidate(reqPath, values)
		dynamicScore, dynamicReason = scoreDynamicCandidate(reqPath, values)
	}

	// Add candidates with scores
	if sessionScore > 0 {
		candidates = append(candidates, classCandidate{
			class:      "session_id",
			confidence: sessionScore,
			reason:     sessionReason,
			respPath:   sessionRespPath,
		})
	}

	if chainScore > 0 && chainRespPath != sessionRespPath {
		candidates = append(candidates, classCandidate{
			class:      "chain",
			confidence: chainScore,
			reason:     fmt.Sprintf("value traced from response path %q across rounds", chainRespPath),
			respPath:   chainRespPath,
		})
	}

	if inputScore > 0 {
		candidates = append(candidates, classCandidate{
			class:      "input",
			confidence: inputScore,
			reason:     inputReason,
		})
	}

	// Only promote dynamic when no input evidence exists, or technical evidence is much stronger.
	if dynamicScore > 0 && (inputScore == 0 || dynamicScore >= inputScore+0.25) {
		candidates = append(candidates, classCandidate{
			class:      "dynamic",
			confidence: dynamicScore,
			reason:     dynamicReason,
		})
	}

	// Sort by confidence descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].confidence > candidates[j].confidence
	})

	return candidates
}

// checkSessionIDPattern checks if a request field follows session_id-like behavior:
// request field values are traceable from previous response values and carry session semantics.
func checkSessionIDPattern(
	reqPath string,
	values []string,
	present []bool,
	rounds []parsedRound,
	respIndex map[string]map[int]string,
) (score float64, respPath string, reason string) {
	firstEmpty := !present[0] || values[0] == "" || values[0] == "null"
	reqNameHint := checkNameHint(reqPath, sessionIDKeywords)
	bestScore := 0.0
	bestPath := ""
	bestReason := ""
	for rp, rpVals := range respIndex {
		matchCount := 0
		totalCheck := 0
		sameRoundMatches := 0
		sameRoundChecks := 0
		for i := 1; i < len(rounds); i++ {
			if !present[i] {
				continue
			}
			totalCheck++
			prevRespVal, hasPrev := rpVals[i-1]
			if hasPrev && prevRespVal == values[i] && values[i] != "" {
				matchCount++
			}
		}
		for i := 0; i < len(rounds); i++ {
			if !present[i] {
				continue
			}
			sameRoundChecks++
			respVal, hasSame := rpVals[i]
			if hasSame && respVal == values[i] && values[i] != "" {
				sameRoundMatches++
			}
		}
		if totalCheck == 0 || matchCount == 0 {
			continue
		}
		traceRatio := float64(matchCount) / float64(totalCheck)
		sameRoundRatio := 0.0
		if sameRoundChecks > 0 {
			sameRoundRatio = float64(sameRoundMatches) / float64(sameRoundChecks)
		}
		respNameHint := checkNameHint(rp, sessionIDKeywords)
		pathAligned := lastPathSegment(reqPath) == lastPathSegment(rp)

		candidate := 0.0
		candidateReason := ""
		switch {
		case firstEmpty && traceRatio == 1.0:
			candidate = 0.90
			candidateReason = fmt.Sprintf("value traced from response path %q across rounds with empty/missing first round", rp)
		case traceRatio == 1.0 && (reqNameHint > 0 || respNameHint > 0 || pathAligned):
			candidate = 0.80
			if pathAligned {
				candidate += 0.05
			}
			if reqNameHint > 0 || respNameHint > 0 {
				candidate += 0.05
			}
			if sameRoundRatio >= 0.66 {
				candidate += 0.03
			}
			candidateReason = fmt.Sprintf("value stably traced from response path %q across rounds", rp)
		case traceRatio >= 0.67 && (reqNameHint > 0 || respNameHint > 0):
			candidate = 0.62 + traceRatio*0.18
			candidateReason = fmt.Sprintf("value partially traced from response path %q with session-like naming", rp)
		}
		if candidate > bestScore {
			bestScore = candidate
			bestPath = rp
			bestReason = candidateReason
		}
	}
	if bestScore == 0 {
		return 0, "", ""
	}
	return math.Min(1.0, bestScore), bestPath, bestReason
}

// checkChainPattern checks if a request field's value can be traced to a response field.
func checkChainPattern(
	reqPath string,
	values []string,
	present []bool,
	rounds []parsedRound,
	respIndex map[string]map[int]string,
) (score float64, respPath string) {
	bestScore := 0.0
	bestPath := ""
	for rp, rpVals := range respIndex {
		matchCount := 0
		totalCheck := 0
		for i := 1; i < len(rounds); i++ {
			if !present[i] {
				continue
			}
			totalCheck++
			prevRespVal, hasPrev := rpVals[i-1]
			if hasPrev && prevRespVal == values[i] && values[i] != "" {
				matchCount++
			}
		}
		if totalCheck > 0 && matchCount == totalCheck {
			if 0.85 > bestScore {
				bestScore = 0.85
				bestPath = rp
			}
		}
		if totalCheck > 0 && matchCount > 0 {
			score := float64(matchCount) / float64(totalCheck) * 0.70
			if score > bestScore {
				bestScore = score
				bestPath = rp
			}
		}
	}
	return bestScore, bestPath
}

// checkNameHint checks if a field path name matches known keywords.
func checkNameHint(path string, keywords []string) float64 {
	parts := splitPathTokens(path)
	if len(parts) == 0 {
		return 0
	}
	lastPart := parts[len(parts)-1]
	for _, kw := range keywords {
		if strings.Contains(lastPart, strings.ToLower(kw)) {
			return 0.30
		}
	}
	return 0
}

func scoreInputCandidate(reqPath string, values []string) (float64, string) {
	if isLikelyTechnicalDynamicField(reqPath) {
		return 0, ""
	}
	if isLikelyMessageInputPath(reqPath) {
		if hasHumanInputTrace(values) {
			return 0.80, "message-like field with human input traces across rounds"
		}
		return 0.72, "message-like field changed across rounds"
	}
	if pathHasKeyword(reqPath, inputSemanticKeywords) {
		if hasHumanInputTrace(values) {
			return 0.72, "input-like field with human input traces across rounds"
		}
		return 0.64, "input-like field changed across rounds"
	}
	if hasHumanInputTrace(values) {
		return 0.58, "value changed with human input traces and no response mapping"
	}
	return 0, ""
}

func scoreDynamicCandidate(reqPath string, values []string) (float64, string) {
	if isLikelyTechnicalDynamicField(reqPath) {
		return 0.90, "technical dynamic field (id/timestamp/signature-like) changed across rounds"
	}
	for _, v := range values {
		if looksLikeMachineIdentifier(v) {
			return 0.82, "machine-generated value changed across rounds without response mapping"
		}
	}
	return 0.72, "value changes but cannot be mapped to response fields"
}

func isLikelyMessageInputPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, "messages") && strings.HasSuffix(lower, ".content")
}

func isLikelyTechnicalDynamicField(path string) bool {
	return pathHasKeyword(path, technicalDynamicKeywords)
}

func pathHasKeyword(path string, keywords []string) bool {
	normalized := normalizePath(path)
	tokens := splitPathTokens(path)
	if len(tokens) == 0 {
		return false
	}
	for _, kw := range keywords {
		kwNorm := normalizePath(kw)
		if kwNorm == "" {
			continue
		}
		if normalized == kwNorm || strings.Contains(normalized, kwNorm) {
			return true
		}
		for _, token := range tokens {
			if token == kwNorm || strings.Contains(token, kwNorm) {
				return true
			}
		}
	}
	return false
}

func splitPathTokens(path string) []string {
	normalized := normalizePath(path)
	parts := strings.Split(normalized, "_")
	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		tokens = append(tokens, p)
	}
	return tokens
}

func normalizePath(path string) string {
	lower := strings.ToLower(path)
	replacer := strings.NewReplacer(".", "_", "-", "_", "[", "_", "]", "_")
	return replacer.Replace(lower)
}

func hasHumanInputTrace(values []string) bool {
	for _, raw := range values {
		v := strings.TrimSpace(raw)
		if v == "" || v == "$$$" || looksLikeMachineIdentifier(v) {
			continue
		}
		hasLetter := false
		hasHan := false
		hasSpaceOrPunct := false
		for _, r := range v {
			if unicode.IsLetter(r) {
				hasLetter = true
			}
			if unicode.Is(unicode.Han, r) {
				hasHan = true
			}
			if unicode.IsSpace(r) || strings.ContainsRune(".,!?;:，。！？；：", r) {
				hasSpaceOrPunct = true
			}
		}
		if hasHan || hasSpaceOrPunct || (hasLetter && len(v) >= 6) {
			return true
		}
	}
	return false
}

func looksLikeMachineIdentifier(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	allDigits := true
	hexLike := true
	for _, r := range v {
		if !unicode.IsDigit(r) {
			allDigits = false
		}
		isHex := (r >= '0' && r <= '9') ||
			(r >= 'a' && r <= 'f') ||
			(r >= 'A' && r <= 'F') ||
			r == '-' || r == '_' || r == '.'
		if !isHex {
			hexLike = false
		}
	}
	if allDigits && len(v) >= 8 {
		return true
	}
	if hexLike && len(v) >= 16 {
		return true
	}
	return false
}

// resolveFields resolves conflicts and produces final field classifications.
// Key rule: only ONE field can be session_id; others matching the same pattern become chain.
func resolveFields(candidates []fieldCandidate, cfg InferenceConfig) ([]InferredField, []string) {
	var fields []InferredField
	var warnings []string

	// Priority order for tie-breaking
	priorityOrder := map[string]int{
		"session_id": 5,
		"chain":      4,
		"input":      3,
		"dynamic":    2,
		"static":     1,
	}

	// First pass: determine the best session_id candidate
	bestSessionIdx := -1
	bestSessionScore := 0.0
	for i, fc := range candidates {
		for _, c := range fc.candidates {
			if c.class == "session_id" {
				// Score = confidence + name hint bonus
				score := c.confidence + checkNameHint(fc.requestPath, sessionIDKeywords)
				// Bonus for same-path mapping (req path == resp path)
				if c.respPath == lastPathSegment(fc.requestPath) {
					score += 0.10
				}
				if score > bestSessionScore {
					bestSessionScore = score
					bestSessionIdx = i
				}
			}
		}
	}

	// Second pass: resolve each field
	for i, fc := range candidates {
		if len(fc.candidates) == 0 {
			continue
		}

		best := fc.candidates[0]

		// If this field is classified as session_id but isn't the best candidate,
		// demote to chain (if it has a response mapping) or dynamic.
		if best.class == "session_id" && i != bestSessionIdx {
			// Find the chain candidate
			demoted := false
			for _, c := range fc.candidates {
				if c.class == "chain" {
					best = c
					demoted = true
					break
				}
			}
			if !demoted && best.respPath != "" {
				// Convert session_id to chain
				best = classCandidate{
					class:      "chain",
					confidence: best.confidence * 0.90,
					reason:     "demoted from session_id (another field is a better session_id match)",
					respPath:   best.respPath,
				}
			}
		}

		var conflictClasses []string
		hasConflict := false

		if len(fc.candidates) > 1 {
			second := fc.candidates[1]
			// Skip conflict check if we already demoted
			if best.class == fc.candidates[0].class && best.confidence-second.confidence < cfg.ConflictMargin {
				hasConflict = true
				// Tie-break by priority
				if priorityOrder[second.class] > priorityOrder[best.class] {
					best = second
				}
				for _, c := range fc.candidates {
					if c.class != best.class {
						conflictClasses = append(conflictClasses, c.class)
					}
				}
				warnings = append(warnings, fmt.Sprintf(
					"field %q has close-score conflict between %s (%.2f) and %s (%.2f)",
					fc.requestPath, fc.candidates[0].class, fc.candidates[0].confidence,
					fc.candidates[1].class, fc.candidates[1].confidence,
				))
			}
		}

		placeholder := determinePlaceholder(best.class, fc.requestPath)

		field := InferredField{
			RequestPath:  fc.requestPath,
			ResponsePath: best.respPath,
			Class:        best.class,
			Placeholder:  placeholder,
			Confidence:   best.confidence,
			Reason:       best.reason,
		}
		if hasConflict {
			field.ConflictWith = conflictClasses
			// Reduce confidence for conflicting fields
			field.Confidence = math.Max(0, field.Confidence-0.10)
		}

		fields = append(fields, field)
	}

	return fields, warnings
}

// lastPathSegment returns the last segment of a dotted path.
func lastPathSegment(path string) string {
	parts := strings.Split(path, ".")
	return parts[len(parts)-1]
}

// determinePlaceholder returns the appropriate placeholder string for a field class.
func determinePlaceholder(class, reqPath string) string {
	switch class {
	case "input":
		return "{{input}}"
	case "session_id":
		return "{{session_id}}"
	case "chain":
		// Generate a $$$NAME$$$ placeholder from the field name
		parts := strings.Split(reqPath, ".")
		name := strings.ToUpper(parts[len(parts)-1])
		name = strings.NewReplacer("-", "_", " ", "_").Replace(name)
		return "$$$" + name + "$$$"
	default:
		return ""
	}
}

// buildProfileFromFields constructs a GenericProfile from inferred fields and response spec.
func buildProfileFromFields(fields []InferredField, rounds []parsedRound, path string, respSpec *RawResponseSpec) (*GenericProfile, error) {
	if len(rounds) == 0 {
		return nil, fmt.Errorf("generic: no rounds to build profile from")
	}

	// Build body template from first round request, replacing inferred fields
	firstReqBody := rounds[0].reqPaths
	bodyTemplate := buildBodyTemplate(firstReqBody, fields)

	// Detect session_id field name
	sessionIDField := ""
	for _, f := range fields {
		if f.Class == "session_id" {
			parts := strings.Split(f.RequestPath, ".")
			sessionIDField = parts[len(parts)-1]
			break
		}
	}

	// Detect remote_id_path from session_id field (inferred) or from response spec
	remoteIDPath := ""
	for _, f := range fields {
		if f.Class == "session_id" && f.ResponsePath != "" {
			remoteIDPath = f.ResponsePath
			break
		}
	}
	if remoteIDPath == "" && respSpec != nil {
		remoteIDPath = respSpec.RemoteIDPath
	}

	// Build chain fields from inferred fields
	var chainFields []ChainField
	for _, f := range fields {
		if f.Class == "chain" {
			chainFields = append(chainFields, ChainField{
				Placeholder:    f.Placeholder,
				ResponsePath:   f.ResponsePath,
				ExtractOnEvent: "", // Default: extract from any event
			})
		}
	}

	// Use response spec for stream config (protocol, delta, done)
	streamProfile := StreamProfile{
		Protocol:    "sse",
		ChainFields: chainFields,
	}
	if respSpec != nil {
		streamProfile.Protocol = respSpec.Stream.Protocol
		streamProfile.DeltaPaths = respSpec.Stream.DeltaPaths
		streamProfile.DonePath = respSpec.Stream.DonePath
		streamProfile.DoneValue = respSpec.Stream.DoneValue
		streamProfile.DoneMarker = respSpec.Stream.DoneMarker
		// Merge chain fields from response spec if any
		if len(streamProfile.ChainFields) == 0 {
			streamProfile.ChainFields = respSpec.Stream.ChainFields
		}
	}

	textPath := ""
	if respSpec != nil {
		textPath = respSpec.TextPath
	}

	profile := &GenericProfile{
		Request: RequestProfile{
			Method:         "POST",
			Path:           path,
			BodyTemplate:   bodyTemplate,
			SessionIDField: sessionIDField,
		},
		Response: ResponseProfile{
			TextPath:     textPath,
			RemoteIDPath: remoteIDPath,
			Stream:       streamProfile,
		},
	}

	return profile, nil
}

// buildBodyTemplate reconstructs the request body template from flat paths,
// replacing classified fields with their placeholders.
func buildBodyTemplate(reqPaths map[string]string, fields []InferredField) map[string]any {
	// Create a map of reqPath → placeholder for quick lookup
	replacements := make(map[string]string)
	removals := make(map[string]bool)
	for _, f := range fields {
		switch f.Class {
		case "input":
			replacements[f.RequestPath] = "{{input}}"
		case "session_id":
			replacements[f.RequestPath] = "{{session_id}}"
		case "chain":
			replacements[f.RequestPath] = f.Placeholder
		case "dynamic":
			removals[f.RequestPath] = true
		}
	}

	// Reconstruct the body as a nested map
	result := make(map[string]any)
	for path, value := range reqPaths {
		if removals[path] {
			continue
		}
		if ph, ok := replacements[path]; ok {
			setNestedValue(result, path, ph)
		} else {
			// Static value: try to preserve original type
			setNestedValue(result, path, inferOriginalType(value))
		}
	}

	return result
}

// setNestedValue sets a value at a dotted path in a nested map.
func setNestedValue(m map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := m
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}
		if next, ok := current[part]; ok {
			if nextMap, ok := next.(map[string]any); ok {
				current = nextMap
			} else {
				newMap := make(map[string]any)
				current[part] = newMap
				current = newMap
			}
		} else {
			newMap := make(map[string]any)
			current[part] = newMap
			current = newMap
		}
	}
}

// inferOriginalType attempts to restore the original JSON type from a string representation.
func inferOriginalType(s string) any {
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if s == "" {
		return ""
	}
	// Try number
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
		// Check if it's an integer
		if f == float64(int64(f)) {
			return int64(f)
		}
		return f
	}
	return s
}

// buildReport computes overall confidence and determines the inference status.
func buildReport(fields []InferredField, warnings []string, cfg InferenceConfig) *InferenceReport {
	if len(fields) == 0 {
		return &InferenceReport{
			OverallConfidence: 0,
			Status:            "failed",
			Fields:            fields,
			Warnings:          append(warnings, "no fields could be classified"),
			FallbackSuggested: true,
			Suggestions: []Suggestion{{
				Target:   "",
				Action:   "review",
				Value:    "RawIntegrationSpec",
				Reason:   "auto-inference failed, use manual RawIntegrationSpec mode",
				Priority: "high",
			}},
			FlowSpecMeta: FlowSpecMeta{
				Version: "v1alpha1",
				Source:  "MultiRoundSpec",
			},
		}
	}

	// Compute overall confidence as geometric mean of field confidences
	product := 1.0
	hasConflict := false
	for _, f := range fields {
		if f.Class == "static" || f.Class == "dynamic" {
			continue // Skip non-critical fields
		}
		product *= f.Confidence
		if len(f.ConflictWith) > 0 {
			hasConflict = true
		}
	}

	criticalCount := 0
	for _, f := range fields {
		if f.Class != "static" && f.Class != "dynamic" {
			criticalCount++
		}
	}

	var overallConfidence float64
	if criticalCount > 0 {
		overallConfidence = math.Pow(product, 1.0/float64(criticalCount))
	}

	// Determine status
	status := "auto_confirmed"
	if hasConflict || overallConfidence < cfg.AutoApplyThreshold {
		status = "pending_confirm"
	}
	if status == "pending_confirm" && overallConfidence < cfg.AutoApplyThreshold {
		warnings = append(warnings, fmt.Sprintf(
			"overall confidence %.2f is below auto_apply_threshold %.2f",
			overallConfidence, cfg.AutoApplyThreshold,
		))
	}

	// Build suggestions
	var suggestions []Suggestion
	for _, f := range fields {
		if len(f.ConflictWith) > 0 {
			suggestions = append(suggestions, Suggestion{
				Target:   f.RequestPath,
				Action:   "review",
				Value:    f.Placeholder,
				Reason:   fmt.Sprintf("conflict between %s and %s", f.Class, strings.Join(f.ConflictWith, "/")),
				Priority: "high",
			})
		}
	}
	if status == "pending_confirm" && len(suggestions) == 0 {
		suggestions = append(suggestions, Suggestion{
			Target:   "overall_confidence",
			Action:   "review",
			Value:    fmt.Sprintf("%.4f", overallConfidence),
			Reason:   "overall confidence is below auto apply threshold, manual confirmation required",
			Priority: "high",
		})
	}

	return &InferenceReport{
		OverallConfidence: overallConfidence,
		Status:            status,
		Fields:            fields,
		Warnings:          warnings,
		FallbackSuggested: status == "failed",
		Suggestions:       suggestions,
		FlowSpecMeta: FlowSpecMeta{
			Version: "v1alpha1",
			Source:  "MultiRoundSpec",
		},
	}
}

// sortedKeys returns sorted keys from a map.
func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
