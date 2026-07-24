package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// weatherTool gets weather from wttr.in (free, no API key).
type weatherTool struct {
	httpClient *http.Client
}

// NewWeatherTool creates a weather tool with optional custom http.Client.
func NewWeatherTool(client *http.Client) Tool {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &weatherTool{httpClient: client}
}

func (t *weatherTool) Name() string { return "weather" }

func (t *weatherTool) Description() string {
	return "Get current weather for a city via wttr.in (free, no API key). Returns temperature, conditions, humidity, wind."
}

func (t *weatherTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"city":{"type":"string","description":"city name, e.g. London, Tokyo, 'New York'"}
		},
		"required":["city"],
		"additionalProperties":false
	}`)
}

func (t *weatherTool) Kind() Kind { return KindRead }

func (t *weatherTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		City string `json:"city"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("weather: invalid args: %w", err)
	}
	if args.City == "" {
		return Result{}, fmt.Errorf("weather: city is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	reqURL := fmt.Sprintf("https://wttr.in/%s?format=j1", url.PathEscape(args.City))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("weather: create request: %w", err)
	}
	req.Header.Set("User-Agent", "agent-go/1.0")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("weather: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<19)) // max 512KB
	if err != nil {
		return Result{}, fmt.Errorf("weather: read body: %w", err)
	}

	// Parse wttr.in JSON
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
