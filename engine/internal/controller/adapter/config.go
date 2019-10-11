package adapter

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/veritone/engine-toolkit/engine/internal/controller/adapter/api"
	"github.com/veritone/engine-toolkit/engine/internal/controller/adapter/messaging"
)

const defaultHeartbeatInterval = "5s"
const defaultConfigFilePath = "./adapter/config.json" // this should be copied over by Dockerfile

var (
	payloadFlag = flag.String("payload", "", "payload file")
	configFlag  = flag.String("config", "", "config file")
	tdoIDFlag   = flag.String("tdo", "", "Temporal data object ID (override payload)")
	urlFlag     = flag.String("url", "", "Override URL to ingest")
	stdoutFlag  = flag.Bool("stdout", false, "Write stream to stdout instead of Kafka")
)

func init() {
	flag.Parse()
}

type engineConfig struct {
	EngineID               string                 `json:"engineId,omitempty"`
	EngineInstanceID       string                 `json:"engineInstanceId,omitempty"`
	VeritoneAPI            api.Config             `json:"veritoneAPI,omitempty"`
	Messaging              messaging.ClientConfig `json:"messaging,omitempty"`
	FFMPEG                 ffmpegConfig           `json:"ffmpeg,omitempty"`
	HeartbeatInterval      string                 `json:"heartbeatInterval,omitempty"`
	OutputTopicName        string                 `json:"outputTopicName,omitempty"`
	OutputTopicPartition   int32                  `json:"outputTopicPartition"`
	OutputTopicKeyPrefix   string                 `json:"outputTopicKeyPrefix"`
	OutputBucketName       string                 `json:"outputBucketName,omitempty"`
	OutputBucketRegion     string                 `json:"outputBucketRegion,omitempty"`
	MinioServer            string                 `json:"minioServer,omitempty"`
	SupportedTextMimeTypes []string               `json:"supportedTextMimeTypes,omitempty"`
}

func (c *engineConfig) String() string {
	j, _ := json.Marshal(c)
	return string(j)
}

func (c *engineConfig) validate() error {
	if _, err := time.ParseDuration(c.HeartbeatInterval); err != nil {
		return fmt.Errorf(`invalid value for "heartbeatInterval": "%s" - %s`, c.HeartbeatInterval, err)
	}

	if (stdoutFlag == nil || !*stdoutFlag) && c.OutputTopicName == "" && c.OutputBucketName == "" {
		return errors.New("an output topic or bucket name is required")
	}

	return nil
}

func (c *engineConfig) defaults() {
	if c.HeartbeatInterval == "" {
		c.HeartbeatInterval = defaultHeartbeatInterval
	}
}

type taskPayload struct {
	RecordStartTime      time.Time          `json:"recordStartTime,omitempty"`
	RecordEndTime        time.Time          `json:"recordEndTime,omitempty"`
	RecordDuration       string             `json:"recordDuration,omitempty"`
	StartTimeOverride    int64              `json:"startTimeOverride,omitempty"`
	TDOOffsetMS          int                `json:"tdoOffsetMS,omitempty"`
	SourceID             string             `json:"sourceId,omitempty"`
	SourceDetails        *api.SourceDetails `json:"sourceDetails,omitempty"`
	ScheduledJobID       string             `json:"scheduledJobId,omitempty"`
	URL                  string             `json:"url,omitempty"`
	CacheToS3Key         string             `json:"cacheToS3Key,omitempty"`
	DisableKafka         bool               `json:"disableKafka,omitempty"`
	DisableS3            bool               `json:"disableS3,omitempty"`
	ScfsWriterBufferSize int                `json:"scfsWriterBufferSize,omitempty"`
	OrganizationID       int64              `json:"organizationId,omitempty"`

	// Optional here in the payload to see if this will speed up anything.. default is like 10K?
	ChunkSize int64 `json:"chunkSize,omitempty"`
}

type enginePayload struct {
	JobID       string `json:"jobId,omitempty"`
	TaskID      string `json:"taskId,omitempty"`
	TDOID       string `json:"recordingId,omitempty"`
	Token       string `json:"token,omitempty"`
	taskPayload `json:"taskPayload,omitempty"`
}

func (p enginePayload) String() string {
	// redact Token field so it doesn't leak into log
	p.Token = ""
	j, _ := json.Marshal(p)
	return string(j)
}

func (p *enginePayload) defaults() {
	if tdoIDFlag != nil && *tdoIDFlag != "" {
		p.TDOID = *tdoIDFlag
	}
	if urlFlag != nil && *urlFlag != "" {
		p.URL = *urlFlag
	}
	if p.ChunkSize == 0 {
		p.ChunkSize = 16 * 1024 //16K
	}
}

func (p *enginePayload) isOfflineMode() bool {
	return p.CacheToS3Key != ""
}

func (p *enginePayload) isTimeBased() bool {
	return !p.RecordStartTime.IsZero() || !p.RecordEndTime.IsZero() || p.RecordDuration != ""
}

func (p *enginePayload) validate() error {
	if p.isOfflineMode() {
		if p.SourceID == "" {
			return errors.New("missing sourceID")
		}
		if p.ScheduledJobID == "" {
			return errors.New("missing scheduledJobId")
		}
		if p.SourceDetails == nil {
			return errors.New("missing sourceDetails")
		}
	} else {
		if p.TDOID == "" {
			return errors.New("missing tdoId")
		}
		if p.SourceID == "" && p.URL == "" {
			return errors.New("either sourceId or URL is required")
		}
	}

	if p.JobID == "" {
		return errors.New("missing jobId")
	}
	if p.TaskID == "" {
		return errors.New("missing taskId")
	}

	if p.RecordDuration != "" {
		if !p.RecordStartTime.IsZero() || !p.RecordEndTime.IsZero() {
			return errors.New(`only one of "recordDuration" or "recordStartTime"/"recordEndTime" should be provided`)
		}
		if _, err := time.ParseDuration(p.RecordDuration); err != nil {
			return fmt.Errorf(`invalid record duration given "%s": %s`, p.RecordDuration, err)
		}
	}

	if p.RecordEndTime.IsZero() && !p.RecordStartTime.IsZero() {
		return errors.New(`"recordEndTime" is required when "recordStartTime" is set`)
	}

	return nil
}

func loadPayloadFromFile(p interface{}, payloadFilePath string) error {
	if payloadFilePath == "" {
		return errors.New("no payload file specified")
	}

	reader, err := os.Open(payloadFilePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	return json.NewDecoder(reader).Decode(p)
}

func loadConfigFromFile(c interface{}, configFilePath string) error {
	if configFilePath == "" {
		configFilePath = defaultConfigFilePath
	}

	reader, err := os.Open(configFilePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	return json.NewDecoder(reader).Decode(c)
}

func loadConfigAndPayload(payloadJSON string, engineID, engineInstanceID string) (*engineConfig, *enginePayload, error) {
	payload, config := new(enginePayload), new(engineConfig)

	if err := json.Unmarshal([]byte(payloadJSON), payload); err != nil {
		return config, payload, fmt.Errorf("failed to unmarshal payload JSON: %s", err)
	}

	payload.defaults()
	if err := payload.validate(); err != nil {
		return config, payload, fmt.Errorf("invalid payload: %s", err)
	}

	// load config from file
	if err := loadConfigFromFile(config, *configFlag); err != nil {
		return config, payload, err
	}
	config.defaults()

	config.EngineID = engineID
	config.EngineInstanceID = engineInstanceID

	if kafkaBrokers := os.Getenv("KAFKA_BROKERS"); len(kafkaBrokers) > 0 {
		config.Messaging.Kafka.Brokers = strings.Split(kafkaBrokers, ",")
	}
	if engineStatusTopic := os.Getenv("KAFKA_ENGINE_STATUS_TOPIC"); len(engineStatusTopic) > 0 {
		config.Messaging.MessageTopics.EngineStatus = engineStatusTopic
	}
	if apiBaseURL := os.Getenv("VERITONE_API_BASE_URL"); len(apiBaseURL) > 0 {
		config.VeritoneAPI.BaseURL = apiBaseURL
	}
	if streamOutputTopic := os.Getenv("STREAM_OUTPUT_TOPIC"); len(streamOutputTopic) > 0 {
		config.OutputTopicName, config.OutputTopicPartition, config.OutputTopicKeyPrefix = messaging.ParseStreamTopic(streamOutputTopic)
	}
	if outputBucketName := os.Getenv("CHUNK_CACHE_BUCKET"); len(outputBucketName) > 0 {
		config.OutputBucketName = outputBucketName
	}
	if minioServer := os.Getenv("MINIO_SERVER"); len(minioServer) > 0 {
		config.MinioServer = minioServer
	}
	if region := os.Getenv("CHUNK_CACHE_AWS_REGION"); len(region) > 0 {
		config.OutputBucketRegion = region
	}

	config.VeritoneAPI.CorrelationID = engineID + ":" + config.EngineInstanceID

	return config, payload, config.validate()
}