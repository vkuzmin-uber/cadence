// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package history

import (
	"time"

	"fmt"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/metrics"
	persistencefactory "github.com/uber/cadence/common/persistence/persistence-factory"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/service/config"
	"github.com/uber/cadence/common/service/dynamicconfig"
)

// Config represents configuration for cadence-history service
type Config struct {
	NumberOfShards int

	// TODO remove after DC migration is over
	// NOTE: DO NOT TURN ON THE FLAG UNLESS YOU KNOW WHAT YOU ARE DOING
	EnableDCMigration               dynamicconfig.BoolPropertyFn
	RPS                             dynamicconfig.IntPropertyFn
	MaxIDLengthLimit                dynamicconfig.IntPropertyFn
	PersistenceMaxQPS               dynamicconfig.IntPropertyFn
	EnableVisibilitySampling        dynamicconfig.BoolPropertyFn
	EnableReadFromClosedExecutionV2 dynamicconfig.BoolPropertyFn
	VisibilityOpenMaxQPS            dynamicconfig.IntPropertyFnWithDomainFilter
	VisibilityClosedMaxQPS          dynamicconfig.IntPropertyFnWithDomainFilter
	EnableVisibilityToKafka         dynamicconfig.BoolPropertyFn

	// HistoryCache settings
	// Change of these configs require shard restart
	HistoryCacheInitialSize dynamicconfig.IntPropertyFn
	HistoryCacheMaxSize     dynamicconfig.IntPropertyFn
	HistoryCacheTTL         dynamicconfig.DurationPropertyFn

	// EventsCache settings
	// Change of these configs require shard restart
	EventsCacheInitialSize dynamicconfig.IntPropertyFn
	EventsCacheMaxSize     dynamicconfig.IntPropertyFn
	EventsCacheTTL         dynamicconfig.DurationPropertyFn

	// ShardController settings
	RangeSizeBits        uint
	AcquireShardInterval dynamicconfig.DurationPropertyFn

	// the artificial delay added to standby cluster's view of active cluster's time
	StandbyClusterDelay dynamicconfig.DurationPropertyFn

	// TimerQueueProcessor settings
	TimerTaskBatchSize                               dynamicconfig.IntPropertyFn
	TimerTaskWorkerCount                             dynamicconfig.IntPropertyFn
	TimerTaskMaxRetryCount                           dynamicconfig.IntPropertyFn
	TimerProcessorStartDelay                         dynamicconfig.DurationPropertyFn
	TimerProcessorFailoverStartDelay                 dynamicconfig.DurationPropertyFn
	TimerProcessorGetFailureRetryCount               dynamicconfig.IntPropertyFn
	TimerProcessorCompleteTimerFailureRetryCount     dynamicconfig.IntPropertyFn
	TimerProcessorUpdateAckInterval                  dynamicconfig.DurationPropertyFn
	TimerProcessorUpdateAckIntervalJitterCoefficient dynamicconfig.FloatPropertyFn
	TimerProcessorCompleteTimerInterval              dynamicconfig.DurationPropertyFn
	TimerProcessorFailoverMaxPollRPS                 dynamicconfig.IntPropertyFn
	TimerProcessorMaxPollRPS                         dynamicconfig.IntPropertyFn
	TimerProcessorMaxPollInterval                    dynamicconfig.DurationPropertyFn
	TimerProcessorMaxPollIntervalJitterCoefficient   dynamicconfig.FloatPropertyFn
	TimerProcessorMaxTimeShift                       dynamicconfig.DurationPropertyFn

	// TransferQueueProcessor settings
	TransferTaskBatchSize                               dynamicconfig.IntPropertyFn
	TransferTaskWorkerCount                             dynamicconfig.IntPropertyFn
	TransferTaskMaxRetryCount                           dynamicconfig.IntPropertyFn
	TransferProcessorStartDelay                         dynamicconfig.DurationPropertyFn
	TransferProcessorFailoverStartDelay                 dynamicconfig.DurationPropertyFn
	TransferProcessorCompleteTransferFailureRetryCount  dynamicconfig.IntPropertyFn
	TransferProcessorFailoverMaxPollRPS                 dynamicconfig.IntPropertyFn
	TransferProcessorMaxPollRPS                         dynamicconfig.IntPropertyFn
	TransferProcessorMaxPollInterval                    dynamicconfig.DurationPropertyFn
	TransferProcessorMaxPollIntervalJitterCoefficient   dynamicconfig.FloatPropertyFn
	TransferProcessorUpdateAckInterval                  dynamicconfig.DurationPropertyFn
	TransferProcessorUpdateAckIntervalJitterCoefficient dynamicconfig.FloatPropertyFn
	TransferProcessorCompleteTransferInterval           dynamicconfig.DurationPropertyFn

	// ReplicatorQueueProcessor settings
	ReplicatorTaskBatchSize                               dynamicconfig.IntPropertyFn
	ReplicatorTaskWorkerCount                             dynamicconfig.IntPropertyFn
	ReplicatorTaskMaxRetryCount                           dynamicconfig.IntPropertyFn
	ReplicatorProcessorStartDelay                         dynamicconfig.DurationPropertyFn
	ReplicatorProcessorMaxPollRPS                         dynamicconfig.IntPropertyFn
	ReplicatorProcessorMaxPollInterval                    dynamicconfig.DurationPropertyFn
	ReplicatorProcessorMaxPollIntervalJitterCoefficient   dynamicconfig.FloatPropertyFn
	ReplicatorProcessorUpdateAckInterval                  dynamicconfig.DurationPropertyFn
	ReplicatorProcessorUpdateAckIntervalJitterCoefficient dynamicconfig.FloatPropertyFn

	// Persistence settings
	ExecutionMgrNumConns dynamicconfig.IntPropertyFn
	HistoryMgrNumConns   dynamicconfig.IntPropertyFn

	// System Limits
	MaximumBufferedEventsBatch dynamicconfig.IntPropertyFn
	MaximumSignalsPerExecution dynamicconfig.IntPropertyFnWithDomainFilter

	// ShardUpdateMinInterval the minimal time interval which the shard info can be updated
	ShardUpdateMinInterval dynamicconfig.DurationPropertyFn
	// ShardSyncMinInterval the minimal time interval which the shard info should be sync to remote
	ShardSyncMinInterval dynamicconfig.DurationPropertyFn

	// Time to hold a poll request before returning an empty response
	// right now only used by GetMutableState
	LongPollExpirationInterval dynamicconfig.DurationPropertyFnWithDomainFilter

	// encoding the history events
	EventEncodingType dynamicconfig.StringPropertyFnWithDomainFilter
	// whether or not using eventsV2
	EnableEventsV2 dynamicconfig.BoolPropertyFnWithDomainFilter

	NumSysWorkflows dynamicconfig.IntPropertyFn

	BlobSizeLimitError     dynamicconfig.IntPropertyFnWithDomainFilter
	BlobSizeLimitWarn      dynamicconfig.IntPropertyFnWithDomainFilter
	HistorySizeLimitError  dynamicconfig.IntPropertyFnWithDomainFilter
	HistorySizeLimitWarn   dynamicconfig.IntPropertyFnWithDomainFilter
	HistoryCountLimitError dynamicconfig.IntPropertyFnWithDomainFilter
	HistoryCountLimitWarn  dynamicconfig.IntPropertyFnWithDomainFilter
}

// NewConfig returns new service config with default values
func NewConfig(dc *dynamicconfig.Collection, numberOfShards int, enableVisibilityToKafka bool) *Config {
	return &Config{
		NumberOfShards: numberOfShards,
		// TODO remove after DC migration is over
		// NOTE: DO NOT TURN ON THE FLAG UNLESS YOU KNOW WHAT YOU ARE DOING
		EnableDCMigration:                                     dc.GetBoolProperty(dynamicconfig.EnableDCMigration, false),
		RPS:                                                   dc.GetIntProperty(dynamicconfig.HistoryRPS, 3000),
		MaxIDLengthLimit:                                      dc.GetIntProperty(dynamicconfig.MaxIDLengthLimit, 1000),
		PersistenceMaxQPS:                                     dc.GetIntProperty(dynamicconfig.HistoryPersistenceMaxQPS, 9000),
		EnableVisibilitySampling:                              dc.GetBoolProperty(dynamicconfig.EnableVisibilitySampling, true),
		EnableReadFromClosedExecutionV2:                       dc.GetBoolProperty(dynamicconfig.EnableReadFromClosedExecutionV2, false),
		VisibilityOpenMaxQPS:                                  dc.GetIntPropertyFilteredByDomain(dynamicconfig.HistoryVisibilityOpenMaxQPS, 300),
		VisibilityClosedMaxQPS:                                dc.GetIntPropertyFilteredByDomain(dynamicconfig.HistoryVisibilityClosedMaxQPS, 300),
		EnableVisibilityToKafka:                               dc.GetBoolProperty(dynamicconfig.EnableVisibilityToKafka, enableVisibilityToKafka),
		HistoryCacheInitialSize:                               dc.GetIntProperty(dynamicconfig.HistoryCacheInitialSize, 128),
		HistoryCacheMaxSize:                                   dc.GetIntProperty(dynamicconfig.HistoryCacheMaxSize, 512),
		HistoryCacheTTL:                                       dc.GetDurationProperty(dynamicconfig.HistoryCacheTTL, time.Hour),
		EventsCacheInitialSize:                                dc.GetIntProperty(dynamicconfig.EventsCacheInitialSize, 128),
		EventsCacheMaxSize:                                    dc.GetIntProperty(dynamicconfig.EventsCacheMaxSize, 512),
		EventsCacheTTL:                                        dc.GetDurationProperty(dynamicconfig.EventsCacheTTL, time.Hour),
		RangeSizeBits:                                         20, // 20 bits for sequencer, 2^20 sequence number for any range
		AcquireShardInterval:                                  dc.GetDurationProperty(dynamicconfig.AcquireShardInterval, time.Minute),
		StandbyClusterDelay:                                   dc.GetDurationProperty(dynamicconfig.AcquireShardInterval, 5*time.Minute),
		TimerTaskBatchSize:                                    dc.GetIntProperty(dynamicconfig.TimerTaskBatchSize, 100),
		TimerTaskWorkerCount:                                  dc.GetIntProperty(dynamicconfig.TimerTaskWorkerCount, 10),
		TimerTaskMaxRetryCount:                                dc.GetIntProperty(dynamicconfig.TimerTaskMaxRetryCount, 100),
		TimerProcessorStartDelay:                              dc.GetDurationProperty(dynamicconfig.TimerProcessorStartDelay, 1*time.Microsecond),
		TimerProcessorFailoverStartDelay:                      dc.GetDurationProperty(dynamicconfig.TimerProcessorFailoverStartDelay, 5*time.Second),
		TimerProcessorGetFailureRetryCount:                    dc.GetIntProperty(dynamicconfig.TimerProcessorGetFailureRetryCount, 5),
		TimerProcessorCompleteTimerFailureRetryCount:          dc.GetIntProperty(dynamicconfig.TimerProcessorCompleteTimerFailureRetryCount, 10),
		TimerProcessorUpdateAckInterval:                       dc.GetDurationProperty(dynamicconfig.TimerProcessorUpdateAckInterval, 30*time.Second),
		TimerProcessorUpdateAckIntervalJitterCoefficient:      dc.GetFloat64Property(dynamicconfig.TimerProcessorUpdateAckIntervalJitterCoefficient, 0.15),
		TimerProcessorCompleteTimerInterval:                   dc.GetDurationProperty(dynamicconfig.TimerProcessorCompleteTimerInterval, 60*time.Second),
		TimerProcessorFailoverMaxPollRPS:                      dc.GetIntProperty(dynamicconfig.TimerProcessorFailoverMaxPollRPS, 1),
		TimerProcessorMaxPollRPS:                              dc.GetIntProperty(dynamicconfig.TimerProcessorMaxPollRPS, 20),
		TimerProcessorMaxPollInterval:                         dc.GetDurationProperty(dynamicconfig.TimerProcessorMaxPollInterval, 5*time.Minute),
		TimerProcessorMaxPollIntervalJitterCoefficient:        dc.GetFloat64Property(dynamicconfig.TimerProcessorMaxPollIntervalJitterCoefficient, 0.15),
		TimerProcessorMaxTimeShift:                            dc.GetDurationProperty(dynamicconfig.TimerProcessorMaxTimeShift, 1*time.Second),
		TransferTaskBatchSize:                                 dc.GetIntProperty(dynamicconfig.TransferTaskBatchSize, 100),
		TransferProcessorFailoverMaxPollRPS:                   dc.GetIntProperty(dynamicconfig.TransferProcessorFailoverMaxPollRPS, 1),
		TransferProcessorMaxPollRPS:                           dc.GetIntProperty(dynamicconfig.TransferProcessorMaxPollRPS, 20),
		TransferTaskWorkerCount:                               dc.GetIntProperty(dynamicconfig.TransferTaskWorkerCount, 10),
		TransferTaskMaxRetryCount:                             dc.GetIntProperty(dynamicconfig.TransferTaskMaxRetryCount, 100),
		TransferProcessorStartDelay:                           dc.GetDurationProperty(dynamicconfig.TransferProcessorStartDelay, 1*time.Microsecond),
		TransferProcessorFailoverStartDelay:                   dc.GetDurationProperty(dynamicconfig.TransferProcessorFailoverStartDelay, 5*time.Second),
		TransferProcessorCompleteTransferFailureRetryCount:    dc.GetIntProperty(dynamicconfig.TransferProcessorCompleteTransferFailureRetryCount, 10),
		TransferProcessorMaxPollInterval:                      dc.GetDurationProperty(dynamicconfig.TransferProcessorMaxPollInterval, 1*time.Minute),
		TransferProcessorMaxPollIntervalJitterCoefficient:     dc.GetFloat64Property(dynamicconfig.TransferProcessorMaxPollIntervalJitterCoefficient, 0.15),
		TransferProcessorUpdateAckInterval:                    dc.GetDurationProperty(dynamicconfig.TransferProcessorUpdateAckInterval, 30*time.Second),
		TransferProcessorUpdateAckIntervalJitterCoefficient:   dc.GetFloat64Property(dynamicconfig.TransferProcessorUpdateAckIntervalJitterCoefficient, 0.15),
		TransferProcessorCompleteTransferInterval:             dc.GetDurationProperty(dynamicconfig.TransferProcessorCompleteTransferInterval, 60*time.Second),
		ReplicatorTaskBatchSize:                               dc.GetIntProperty(dynamicconfig.ReplicatorTaskBatchSize, 100),
		ReplicatorTaskWorkerCount:                             dc.GetIntProperty(dynamicconfig.ReplicatorTaskWorkerCount, 10),
		ReplicatorTaskMaxRetryCount:                           dc.GetIntProperty(dynamicconfig.ReplicatorTaskMaxRetryCount, 100),
		ReplicatorProcessorStartDelay:                         dc.GetDurationProperty(dynamicconfig.ReplicatorProcessorStartDelay, 1*time.Microsecond),
		ReplicatorProcessorMaxPollRPS:                         dc.GetIntProperty(dynamicconfig.ReplicatorProcessorMaxPollRPS, 20),
		ReplicatorProcessorMaxPollInterval:                    dc.GetDurationProperty(dynamicconfig.ReplicatorProcessorMaxPollInterval, 1*time.Minute),
		ReplicatorProcessorMaxPollIntervalJitterCoefficient:   dc.GetFloat64Property(dynamicconfig.ReplicatorProcessorMaxPollIntervalJitterCoefficient, 0.15),
		ReplicatorProcessorUpdateAckInterval:                  dc.GetDurationProperty(dynamicconfig.ReplicatorProcessorUpdateAckInterval, 5*time.Second),
		ReplicatorProcessorUpdateAckIntervalJitterCoefficient: dc.GetFloat64Property(dynamicconfig.ReplicatorProcessorUpdateAckIntervalJitterCoefficient, 0.15),
		ExecutionMgrNumConns:                                  dc.GetIntProperty(dynamicconfig.ExecutionMgrNumConns, 50),
		HistoryMgrNumConns:                                    dc.GetIntProperty(dynamicconfig.HistoryMgrNumConns, 50),
		MaximumBufferedEventsBatch:                            dc.GetIntProperty(dynamicconfig.MaximumBufferedEventsBatch, 100),
		MaximumSignalsPerExecution:                            dc.GetIntPropertyFilteredByDomain(dynamicconfig.MaximumSignalsPerExecution, 0),
		ShardUpdateMinInterval:                                dc.GetDurationProperty(dynamicconfig.ShardUpdateMinInterval, 5*time.Minute),
		ShardSyncMinInterval:                                  dc.GetDurationProperty(dynamicconfig.ShardSyncMinInterval, 5*time.Minute),

		// history client: client/history/client.go set the client timeout 30s
		LongPollExpirationInterval: dc.GetDurationPropertyFilteredByDomain(dynamicconfig.HistoryLongPollExpirationInterval, time.Second*20),
		EventEncodingType:          dc.GetStringPropertyFnWithDomainFilter(dynamicconfig.DefaultEventEncoding, string(common.EncodingTypeJSON)),
		EnableEventsV2:             dc.GetBoolPropertyFnWithDomainFilter(dynamicconfig.EnableEventsV2, false),

		NumSysWorkflows: dc.GetIntProperty(dynamicconfig.NumSystemWorkflows, 1000),

		BlobSizeLimitError:     dc.GetIntPropertyFilteredByDomain(dynamicconfig.BlobSizeLimitError, 2*1024*1024),
		BlobSizeLimitWarn:      dc.GetIntPropertyFilteredByDomain(dynamicconfig.BlobSizeLimitError, 256*1024),
		HistorySizeLimitError:  dc.GetIntPropertyFilteredByDomain(dynamicconfig.HistorySizeLimitError, 200*1024*1024),
		HistorySizeLimitWarn:   dc.GetIntPropertyFilteredByDomain(dynamicconfig.HistorySizeLimitWarn, 50*1024*1024),
		HistoryCountLimitError: dc.GetIntPropertyFilteredByDomain(dynamicconfig.HistoryCountLimitError, 200*1024),
		HistoryCountLimitWarn:  dc.GetIntPropertyFilteredByDomain(dynamicconfig.HistoryCountLimitWarn, 50*1024),
	}
}

// GetShardID return the corresponding shard ID for a given workflow ID
func (config *Config) GetShardID(workflowID string) int {
	return common.WorkflowIDToHistoryShard(workflowID, config.NumberOfShards)
}

// Service represents the cadence-history service
type Service struct {
	stopC         chan struct{}
	params        *service.BootstrapParams
	config        *Config
	metricsClient metrics.Client
}

// NewService builds a new cadence-history service
func NewService(params *service.BootstrapParams) common.Daemon {
	params.UpdateLoggerWithServiceName(common.HistoryServiceName)
	fmt.Println(params.ESConfig)
	return &Service{
		params: params,
		stopC:  make(chan struct{}),
		config: NewConfig(
			dynamicconfig.NewCollection(params.DynamicConfig, params.Logger),
			params.PersistenceConfig.NumHistoryShards,
			params.ESConfig.Enable,
		),
	}
}

// Start starts the service
func (s *Service) Start() {

	var params = s.params
	var log = params.Logger

	log.Infof("%v starting", common.HistoryServiceName)

	base := service.New(params)

	s.metricsClient = base.GetMetricsClient()

	pConfig := params.PersistenceConfig
	pConfig.HistoryMaxConns = s.config.HistoryMgrNumConns()
	pConfig.SetMaxQPS(pConfig.DefaultStore, s.config.PersistenceMaxQPS())
	pConfig.VisibilityConfig = &config.VisibilityConfig{
		VisibilityOpenMaxQPS:            s.config.VisibilityOpenMaxQPS,
		VisibilityClosedMaxQPS:          s.config.VisibilityClosedMaxQPS,
		EnableSampling:                  s.config.EnableVisibilitySampling,
		EnableReadFromClosedExecutionV2: s.config.EnableReadFromClosedExecutionV2,
	}
	pFactory := persistencefactory.New(&pConfig, params.ClusterMetadata.GetCurrentClusterName(), s.metricsClient, log)

	shardMgr, err := pFactory.NewShardManager()
	if err != nil {
		log.Fatalf("failed to create shard manager: %v", err)
	}

	metadata, err := pFactory.NewMetadataManager(persistencefactory.MetadataV1V2)
	if err != nil {
		log.Fatalf("failed to create metadata manager: %v", err)
	}

	visibility, err := pFactory.NewVisibilityManager()
	if err != nil {
		log.Fatalf("failed to create visibility manager: %v", err)
	}

	history, err := pFactory.NewHistoryManager()
	if err != nil {
		log.Fatalf("Creating Cassandra history manager persistence failed: %v", err)
	}

	historyV2, err := pFactory.NewHistoryV2Manager()
	if err != nil {
		// TODO change this to Fatalf when SQL also support eventsV2
		log.Warnf("Creating Cassandra historyV2 manager persistence failed: %v, cannot use eventsV2 features", err)
	}

	handler := NewHandler(base,
		s.config,
		shardMgr,
		metadata,
		visibility,
		history,
		historyV2,
		pFactory)

	handler.Start()

	log.Infof("%v started", common.HistoryServiceName)

	<-s.stopC
	base.Stop()
}

// Stop stops the service
func (s *Service) Stop() {
	select {
	case s.stopC <- struct{}{}:
	default:
	}
	s.params.Logger.Infof("%v stopped", common.HistoryServiceName)
}
