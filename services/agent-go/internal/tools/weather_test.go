package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWeatherTool(t *testing.T) {
	t.Run("valid city returns weather", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"current_condition": []map[string]any{
					{
						"temp_C":        "22",
						"weatherDesc":   []map[string]string{{"value": "Sunny"}},
						"humidity":      "55",
						"windspeedKmph": "15",
						"FeelsLikeC":    "20",
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		tool := newWeatherToolWithURL(srv.Client(), srv.URL)
		args, _ := json.Marshal(map[string]string{"city": "London"})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			City        string `json:"city"`
			Temperature string `json:"temperature"`
			Condition   string `json:"condition"`
			Humidity    string `json:"humidity"`
			WindSpeed   string `json:"windSpeed"`
			FeelsLike   string `json:"feelsLike"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if out.City != "London" {
			t.Errorf("city: got %q, want %q", out.City, "London")
		}
		if out.Temperature != "22" {
			t.Errorf("temperature: got %q, want %q", out.Temperature, "22")
		}
		if out.Condition != "Sunny" {
			t.Errorf("condition: got %q, want %q", out.Condition, "Sunny")
		}
		if out.Humidity != "55" {
			t.Errorf("humidity: got %q, want %q", out.Humidity, "55")
		}
		if out.WindSpeed != "15" {
			t.Errorf("windSpeed: got %q, want %q", out.WindSpeed, "15")
		}
		if out.FeelsLike != "20" {
			t.Errorf("feelsLike: got %q, want %q", out.FeelsLike, "20")
		}
	})

	t.Run("empty city returns error", func(t *testing.T) {
		tool := NewWeatherTool(nil)
		args, _ := json.Marshal(map[string]string{"city": ""})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for empty city, got nil")
		}
		if !strings.Contains(err.Error(), "city is required") {
			t.Errorf("expected 'city is required' error, got: %v", err)
		}
	})

	t.Run("missing city param", func(t *testing.T) {
		tool := NewWeatherTool(nil)
		args, _ := json.Marshal(map[string]string{})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing city, got nil")
		}
		if !strings.Contains(err.Error(), "city is required") {
			t.Errorf("expected 'city is required' error, got: %v", err)
		}
	})

	t.Run("invalid json args", func(t *testing.T) {
		tool := NewWeatherTool(nil)
		_, err := tool.Execute(context.Background(), json.RawMessage(`{bad`))
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
		if !strings.Contains(err.Error(), "invalid args") {
			t.Errorf("expected 'invalid args' error, got: %v", err)
		}
	})

	t.Run("server error response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		tool := newWeatherToolWithURL(srv.Client(), srv.URL)
		args, _ := json.Marshal(map[string]string{"city": "test"})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for server error, got nil")
		}
	})

	t.Run("empty conditions array", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"current_condition": []map[string]any{},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		tool := newWeatherToolWithURL(srv.Client(), srv.URL)
		args, _ := json.Marshal(map[string]string{"city": "Nowhere"})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for empty conditions, got nil")
		}
		if !strings.Contains(err.Error(), "no data for city") {
			t.Errorf("expected 'no data for city' error, got: %v", err)
		}
	})

	t.Run("timeout handling", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {}
		}))
		defer srv.Close()

		tool := newWeatherToolWithURL(srv.Client(), srv.URL)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		args, _ := json.Marshal(map[string]string{"city": "test"})
		_, err := tool.Execute(ctx, args)
		if err == nil {
			t.Fatal("expected error for cancelled context, got nil")
		}
	})
}

func TestWeatherToolInterface(t *testing.T) {
	tool := NewWeatherTool(nil)
	if tool.Name() != "weather" {
		t.Errorf("Name: got %q, want %q", tool.Name(), "weather")
	}
	if tool.Kind() != KindRead {
		t.Errorf("Kind: got %v, want KindRead", tool.Kind())
	}
	if tool.Description() == "" {
		t.Error("Description is empty")
	}
	if len(tool.Schema()) == 0 {
		t.Error("Schema is empty")
	}

	// Verify schema contains required fields
	var schema map[string]any
	json.Unmarshal(tool.Schema(), &schema)
	if schema["type"] != "object" {
		t.Error("Schema type should be object")
	}
	required, ok := schema["required"].([]any)
	if !ok || len(required) == 0 {
		t.Error("Schema should have required fields")
	}
}

// newWeatherToolWithURL creates a WeatherTool that uses a custom base URL (for testing).
func newWeatherToolWithURL(client *http.Client, baseURL string) Tool {
	return &weatherToolWithURL{
		httpClient: client,
		baseURL:    baseURL,
	}
}

// weatherToolWithURL is a test variant that allows overriding the API base URL.
type weatherToolWithURL struct {
	httpClient *http.Client
	baseURL    string
}

func (t *weatherToolWithURL) Name() string        { return "weather" }
func (t *weatherToolWithURL) Description() string { return "weather (test)" }

func (t *weatherToolWithURL) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`)
}

func (t *weatherToolWithURL) Kind() Kind { return KindRead }

func (t *weatherToolWithURL) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		City string `json:"city"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("weather: invalid args: %w", err)
	}
	if args.City == "" {
		return Result{}, fmt.Errorf("weather: city is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.baseURL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("weather: create request: %w", err)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("weather: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<19))
	if err != nil {
		return Result{}, fmt.Errorf("weather: read body: %w", err)
	}

	var wttr struct {
		CurrentCondition []struct {
			TempC       string `json:"temp_C"`
			WeatherDesc []struct {
				Value string `json:"value"`
			} `json:"weatherDesc"`
			Humidity  string `json:"humidity"`
			WindSpeed string `json:"windspeedKmph"`
			FeelsLike string `json:"FeelsLikeC"`
		} `json:"current_condition"`
	}
	if err := json.Unmarshal(body, &wttr); err != nil {
		return Result{}, fmt.Errorf("weather: parse response: %w", err)
	}

	if len(wttr.CurrentCondition) == 0 {
		return Result{}, fmt.Errorf("weather: no data for city %q", args.City)
	}

	c := wttr.CurrentCondition[0]
	out, _ := json.Marshal(map[string]any{
		"city":        args.City,
		"temperature": c.TempC,
		"condition":   c.WeatherDesc[0].Value,
		"humidity":    c.Humidity,
		"windSpeed":   c.WindSpeed,
		"feelsLike":   c.FeelsLike,
	})
	return Result{Content: string(out)}, nil
}
