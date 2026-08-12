package promql

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Envelope is the standard Prometheus JSON response wrapper used by the
// CloudWatch PromQL API. On success Status == "success" and Data is populated;
// on failure Status == "error" and Error/ErrorType describe the problem.
type Envelope struct {
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data"`
	Warnings  []string        `json:"warnings"`
	ErrorType string          `json:"errorType"`
	Error     string          `json:"error"`
}

// ResultType discriminates the shape of QueryData.Result.
const (
	ResultVector = "vector" // instant query: one sample per series
	ResultMatrix = "matrix" // range query: a series of samples over time
	ResultScalar = "scalar"
	ResultString = "string"
)

// QueryData is the payload of an instant or range query response.
type QueryData struct {
	ResultType string          `json:"resultType"`
	Result     json.RawMessage `json:"result"`
}

// Sample is a single [timestamp, value] pair. The API encodes the value as a
// string; ParsedValue converts it to float64.
type Sample struct {
	Time  float64
	Value string
}

// UnmarshalJSON decodes the [float64, "string"] tuple form.
func (s *Sample) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if len(raw) != 2 {
		return fmt.Errorf("sample: expected 2 elements, got %d", len(raw))
	}
	if err := json.Unmarshal(raw[0], &s.Time); err != nil {
		return fmt.Errorf("sample time: %w", err)
	}
	return json.Unmarshal(raw[1], &s.Value)
}

// ParsedValue converts the string value to a float64.
func (s Sample) ParsedValue() (float64, error) {
	return strconv.ParseFloat(s.Value, 64)
}

// VectorSample is one series from an instant query.
type VectorSample struct {
	Metric map[string]string `json:"metric"`
	Value  Sample            `json:"value"`
}

// MatrixSample is one series from a range query.
type MatrixSample struct {
	Metric map[string]string `json:"metric"`
	Values []Sample          `json:"values"`
}

// DecodeVector parses the result array of an instant (vector) query.
func (d QueryData) DecodeVector() ([]VectorSample, error) {
	var out []VectorSample
	if err := json.Unmarshal(d.Result, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DecodeMatrix parses the result array of a range (matrix) query.
func (d QueryData) DecodeMatrix() ([]MatrixSample, error) {
	var out []MatrixSample
	if err := json.Unmarshal(d.Result, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// APIError is returned when the API responds with status != "success" or a
// non-2xx HTTP code.
type APIError struct {
	HTTPStatus int
	ErrorType  string
	Message    string
}

func (e *APIError) Error() string {
	if e.ErrorType != "" {
		return fmt.Sprintf("promql api error (http %d, %s): %s", e.HTTPStatus, e.ErrorType, e.Message)
	}
	return fmt.Sprintf("promql api error (http %d): %s", e.HTTPStatus, e.Message)
}
