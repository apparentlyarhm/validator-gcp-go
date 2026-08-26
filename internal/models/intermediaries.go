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

type Metric string

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

func (m Metric) Query() (string, bool) {
	switch m {
	case MetricTPS:
		return "minecraft_tps", true
	case MetricMSPT:
		return "minecraft_mspt", true
	case MetricPlayers:
		return "minecraft_players_online", true
	case MetricEntities:
		return "sum(minecraft_entities{group=~\"creature|monster\",world=\"overworld\"})", true
	case MetricChunksOverworld:
		return "minecraft_loaded_chunks{world=\"overworld\"}", true
	case MetricTotalChunks:
		return "sum(minecraft_loaded_chunks{world=~\"overworld|the_nether|the_end\"})", true
	case MetricHandshakes:
		return "rate(minecraft_handshakes[5m])", true
	case JVMMemoryUsedNonHeap:
		return "jvm_memory_bytes_used{area=\"nonheap\"}", true
	case JVMMemoryUsedHeap:
		return "jvm_memory_bytes_used{area=\"heap\"}", true
	case JVMMemoryMaxNonHeap:
		return "jvm_memory_bytes_max{area=\"nonheap\"}", true
	case JVMMemoryMaxHeap:
		return "jvm_memory_bytes_max{area=\"heap\"}", true
	case JVMGc:
		return "sum(increase(jvm_gc_collection_seconds_count[5m]))", true // how many GC collections happened in the last 5 minutes, across all types
	case Cpu:
		return "rate(process_cpu_seconds_total[5m]) * 100", true
	default:
		return "", false
	}
}
