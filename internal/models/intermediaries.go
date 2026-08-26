package models

// this file is basically structs that are for internal communicaton between this server and other servers.
// for example between this and prometheus. the api response is another file (between frontend and this)

type PromQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Value []any `json:"value"` // [unix_timestamp, "value_as_string"]
		} `json:"result"`
	} `json:"data"`
}

type PromQueryRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Values [][]any           `json:"values"`
		} `json:"result"`
	} `json:"data"`
	ErrorType string `json:"errorType,omitempty"`
	Error     string `json:"error,omitempty"`
}

type PromSample struct {
	Timestamp float64
	Value     float64
}

type PromTimeSeries struct {
	Metric map[string]string
	Values []PromSample
}

type Metric string

const (
	MetricTPS        Metric = "tps"
	MetricMSPT       Metric = "mspt"
	MetricPlayers    Metric = "players"
	MetricEntities   Metric = "entities"
	MetricChunks     Metric = "chunks"
	MetricHandshakes Metric = "handshakes"
)

func (m Metric) Query() (string, bool) {
	switch m {
	case MetricTPS:
		return "minecraft_tps", true
	case MetricMSPT:
		return "minecraft_mspt", true
	case MetricPlayers:
		return "minecraft_players_online", true
	case MetricEntities:
		return "minecraft_entities", true
	case MetricChunks:
		return "minecraft_loaded_chunks", true
	case MetricHandshakes:
		return "rate(minecraft_handshakes[5m])", true
	default:
		return "", false
	}
}
