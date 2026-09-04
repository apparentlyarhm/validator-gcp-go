package models

import "strings"

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

type Metric string

type MetricsQueryProfile string

const (
	// MetricsProfileDefault is the active profile unless PROMETHEUS_QUERY_PROFILE overrides it.
	MetricsProfileDefault        MetricsQueryProfile = "default"
	MetricsProfileFabricAndSpark MetricsQueryProfile = "FBandS"
	MetricsProfileNeoForge       MetricsQueryProfile = "NF1_21_1"
)

const (
	MetricTPS             Metric = "tps"
	MetricMSPT            Metric = "mspt"
	MetricPlayers         Metric = "players"
	MetricEntities        Metric = "entities"
	MetricChunksOverworld Metric = "chunks"
	MetricTotalChunks     Metric = "totalChunks"
	MetricHandshakes      Metric = "handshakes"
	JVMMemoryUsedNonHeap  Metric = "jvmMem"
	JVMMemoryUsedHeap     Metric = "jvmMemHeap"
	JVMMemoryMaxNonHeap   Metric = "jvmMemMax"
	JVMMemoryMaxHeap      Metric = "jvmMemMaxHeap"
	JVMGc                 Metric = "jvmGc"
	Cpu                   Metric = "cpu"
)

func fabricExporterWithSpark() map[Metric]string {
	return map[Metric]string{
		MetricTPS:             "minecraft_tps",
		MetricMSPT:            "minecraft_mspt",
		MetricPlayers:         "minecraft_players_online",
		MetricEntities:        "sum(minecraft_entities{group=~\"creature|monster\",world=\"overworld\"})",
		MetricChunksOverworld: "minecraft_loaded_chunks{world=\"overworld\"}",
		MetricTotalChunks:     "sum(minecraft_loaded_chunks{world=~\"overworld|the_nether|the_end\"})",
		MetricHandshakes:      "rate(minecraft_handshakes[5m])",
		JVMMemoryUsedNonHeap:  "jvm_memory_bytes_used{area=\"nonheap\"}",
		JVMMemoryUsedHeap:     "jvm_memory_bytes_used{area=\"heap\"}",
		JVMMemoryMaxNonHeap:   "jvm_memory_bytes_max{area=\"nonheap\"}",
		JVMMemoryMaxHeap:      "jvm_memory_bytes_max{area=\"heap\"}",
		JVMGc:                 "sum(increase(jvm_gc_collection_seconds_count[5m]))",
		Cpu:                   "rate(process_cpu_seconds_total[5m]) * 100",
	}
}

func neoForgeMetrics() map[Metric]string {
	return map[Metric]string{
		// since this mod exports histograms, we need to do some calculations ourselves

		// note to future self: tps = 1000 / avg mspt
		// we use clamp_max to ensure TPS does not exceed 20
		// so essentially = TPS = min(20, 1000 / MSPT)
		MetricTPS: "clamp_max(1000/((rate(mc_server_tick_seconds_sum[1m])/rate(mc_server_tick_seconds_count[1m])) * 1000),20)",

		// note to future self: rate of all ticks divided by the number of ticks gives the average tick duration in seconds. we then convert it to ms
		// to get MS Per Tick
		MetricMSPT: "(rate(mc_server_tick_seconds_sum[1m])/rate(mc_server_tick_seconds_count[1m])) * 1000",

		MetricPlayers:         "mc_player_list",
		MetricEntities:        "sum(mc_entities_total{dim=\"overworld\"})",
		MetricChunksOverworld: "mc_dimension_chunks_loaded{name=\"overworld\"}",
		MetricTotalChunks:     "sum(mc_dimension_chunks_loaded{name=~\"overworld|the_nether|the_end\"})",
		MetricHandshakes:      "",
		JVMMemoryUsedNonHeap:  "jvm_memory_bytes_used{area=\"nonheap\"}",
		JVMMemoryUsedHeap:     "jvm_memory_bytes_used{area=\"heap\"}",
		JVMMemoryMaxNonHeap:   "jvm_memory_bytes_max{area=\"nonheap\"}",
		JVMMemoryMaxHeap:      "jvm_memory_bytes_max{area=\"heap\"}",
		JVMGc:                 "sum(increase(jvm_gc_collection_seconds_count[5m]))",
		Cpu:                   "rate(process_cpu_seconds_total[1m]) * 100",
	}
}

func defaultMetricQueryProfile() map[Metric]string {
	return map[Metric]string{}
}

var metricQueriesByProfile = map[MetricsQueryProfile]map[Metric]string{
	MetricsProfileDefault:        defaultMetricQueryProfile(),
	MetricsProfileFabricAndSpark: fabricExporterWithSpark(),
	MetricsProfileNeoForge:       neoForgeMetrics(),
}

func ResolveMetricQuery(metric Metric, profile string) (string, bool) {
	selected := MetricsQueryProfile(profile)
	if selected == "" {
		selected = MetricsProfileDefault
	}

	queryMap, ok := metricQueriesByProfile[selected]
	if !ok {
		queryMap = metricQueriesByProfile[MetricsProfileDefault]
	}

	query, ok := queryMap[metric]
	if !ok || strings.TrimSpace(query) == "" {
		return "", false
	}

	return query, true
}

func (m Metric) Query() (string, bool) {
	return ResolveMetricQuery(m, string(MetricsProfileDefault))
}
