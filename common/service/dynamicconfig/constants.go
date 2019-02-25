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

package dynamicconfig

// Key represents a key/property stored in dynamic config
type Key int

func (k Key) String() string {
	keyName, ok := keys[k]
	if !ok {
		return keys[unknownKey]
	}
	return keyName
}

// Mapping from Key to keyName, where keyName are used dynamic config source.
var keys = map[Key]string{
	unknownKey: "unknownKey",

	// tests keys
	testGetPropertyKey:                               "testGetPropertyKey",
	testGetIntPropertyKey:                            "testGetIntPropertyKey",
	testGetFloat64PropertyKey:                        "testGetFloat64PropertyKey",
	testGetDurationPropertyKey:                       "testGetDurationPropertyKey",
	testGetBoolPropertyKey:                           "testGetBoolPropertyKey",
	testGetIntPropertyFilteredByDomainKey:            "testGetIntPropertyFilteredByDomainKey",
	testGetDurationPropertyFilteredByDomainKey:       "testGetDurationPropertyFilteredByDomainKey",
	testGetIntPropertyFilteredByTaskListInfoKey:      "testGetIntPropertyFilteredByTaskListInfoKey",
	testGetDurationPropertyFilteredByTaskListInfoKey: "testGetDurationPropertyFilteredByTaskListInfoKey",
	testGetBoolPropertyFilteredByTaskListInfoKey:     "testGetBoolPropertyFilteredByTaskListInfoKey",

	// system settings
	EnableGlobalDomain:              "system.enableGlobalDomain",
	EnableNewKafkaClient:            "system.enableNewKafkaClient",
	EnableVisibilitySampling:        "system.enableVisibilitySampling",
	EnableReadFromClosedExecutionV2: "system.enableReadFromClosedExecutionV2",
	EnableVisibilityToKafka:         "system.enableVisibilityToKafka",
	EnableReadVisibilityFromES:      "system.enableReadVisibilityFromES",
	EnableArchival:                  "system.enableArchival",

	// size limit
	BlobSizeLimitError:     "limit.blobSize.error",
	BlobSizeLimitWarn:      "limit.blobSize.warn",
	HistorySizeLimitError:  "limit.historySize.error",
	HistorySizeLimitWarn:   "limit.historySize.warn",
	HistoryCountLimitError: "limit.historyCount.error",
	HistoryCountLimitWarn:  "limit.historyCount.warn",
	MaxIDLengthLimit:       "limit.maxIDLength",

	// frontend settings
	FrontendPersistenceMaxQPS:      "frontend.persistenceMaxQPS",
	FrontendVisibilityMaxPageSize:  "frontend.visibilityMaxPageSize",
	FrontendVisibilityListMaxQPS:   "frontend.visibilityListMaxQPS",
	FrontendESVisibilityListMaxQPS: "frontend.esVisibilityListMaxQPS",
	FrontendHistoryMaxPageSize:     "frontend.historyMaxPageSize",
	FrontendRPS:                    "frontend.rps",
	FrontendHistoryMgrNumConns:     "frontend.historyMgrNumConns",
	MaxDecisionStartToCloseTimeout: "frontend.maxDecisionStartToCloseTimeout",
	DisableListVisibilityByFilter:  "frontend.disableListVisibilityByFilter",

	// matching settings
	MatchingRPS:                             "matching.rps",
	MatchingPersistenceMaxQPS:               "matching.persistenceMaxQPS",
	MatchingMinTaskThrottlingBurstSize:      "matching.minTaskThrottlingBurstSize",
	MatchingGetTasksBatchSize:               "matching.getTasksBatchSize",
	MatchingLongPollExpirationInterval:      "matching.longPollExpirationInterval",
	MatchingEnableSyncMatch:                 "matching.enableSyncMatch",
	MatchingUpdateAckInterval:               "matching.updateAckInterval",
	MatchingIdleTasklistCheckInterval:       "matching.idleTasklistCheckInterval",
	MaxTasklistIdleTime:                     "matching.maxTasklistIdleTime",
	MatchingOutstandingTaskAppendsThreshold: "matching.outstandingTaskAppendsThreshold",
	MatchingMaxTaskBatchSize:                "matching.maxTaskBatchSize",

	// history settings
	// TODO remove after DC migration is over
	EnableDCMigration:                                     "history.enableDCMigration",
	HistoryRPS:                                            "history.rps",
	HistoryPersistenceMaxQPS:                              "history.persistenceMaxQPS",
	HistoryVisibilityOpenMaxQPS:                           "history.historyVisibilityOpenMaxQPS",
	HistoryVisibilityClosedMaxQPS:                         "history.historyVisibilityClosedMaxQPS",
	HistoryLongPollExpirationInterval:                     "history.longPollExpirationInterval",
	HistoryCacheInitialSize:                               "history.cacheInitialSize",
	HistoryCacheMaxSize:                                   "history.cacheMaxSize",
	HistoryCacheTTL:                                       "history.cacheTTL",
	EventsCacheInitialSize:                                "history.eventsCacheInitialSize",
	EventsCacheMaxSize:                                    "history.eventsCacheMaxSize",
	EventsCacheTTL:                                        "history.eventsCacheTTL",
	AcquireShardInterval:                                  "history.acquireShardInterval",
	StandbyClusterDelay:                                   "history.standbyClusterDelay",
	TimerTaskBatchSize:                                    "history.timerTaskBatchSize",
	TimerTaskWorkerCount:                                  "history.timerTaskWorkerCount",
	TimerTaskMaxRetryCount:                                "history.timerTaskMaxRetryCount",
	TimerProcessorStartDelay:                              "history.timerProcessorStartDelay",
	TimerProcessorFailoverStartDelay:                      "history.timerProcessorFailoverStartDelay",
	TimerProcessorGetFailureRetryCount:                    "history.timerProcessorGetFailureRetryCount",
	TimerProcessorCompleteTimerFailureRetryCount:          "history.timerProcessorCompleteTimerFailureRetryCount",
	TimerProcessorUpdateShardTaskCount:                    "history.timerProcessorUpdateShardTaskCount",
	TimerProcessorUpdateAckInterval:                       "history.timerProcessorUpdateAckInterval",
	TimerProcessorUpdateAckIntervalJitterCoefficient:      "history.timerProcessorUpdateAckIntervalJitterCoefficient",
	TimerProcessorCompleteTimerInterval:                   "history.timerProcessorCompleteTimerInterval",
	TimerProcessorFailoverMaxPollRPS:                      "history.timerProcessorFailoverMaxPollRPS",
	TimerProcessorMaxPollRPS:                              "history.timerProcessorMaxPollRPS",
	TimerProcessorMaxPollInterval:                         "history.timerProcessorMaxPollInterval",
	TimerProcessorMaxPollIntervalJitterCoefficient:        "history.timerProcessorMaxPollIntervalJitterCoefficient",
	TimerProcessorMaxTimeShift:                            "history.timerProcessorMaxTimeShift",
	TransferTaskBatchSize:                                 "history.transferTaskBatchSize",
	TransferProcessorFailoverMaxPollRPS:                   "history.transferProcessorFailoverMaxPollRPS",
	TransferProcessorMaxPollRPS:                           "history.transferProcessorMaxPollRPS",
	TransferTaskWorkerCount:                               "history.transferTaskWorkerCount",
	TransferTaskMaxRetryCount:                             "history.transferTaskMaxRetryCount",
	TransferProcessorStartDelay:                           "history.transferProcessorStartDelay",
	TransferProcessorFailoverStartDelay:                   "history.transferProcessorFailoverStartDelay",
	TransferProcessorCompleteTransferFailureRetryCount:    "history.transferProcessorCompleteTransferFailureRetryCount",
	TransferProcessorUpdateShardTaskCount:                 "history.transferProcessorUpdateShardTaskCount",
	TransferProcessorMaxPollInterval:                      "history.transferProcessorMaxPollInterval",
	TransferProcessorMaxPollIntervalJitterCoefficient:     "history.transferProcessorMaxPollIntervalJitterCoefficient",
	TransferProcessorUpdateAckInterval:                    "history.transferProcessorUpdateAckInterval",
	TransferProcessorUpdateAckIntervalJitterCoefficient:   "history.transferProcessorUpdateAckIntervalJitterCoefficient",
	TransferProcessorCompleteTransferInterval:             "history.transferProcessorCompleteTransferInterval",
	ReplicatorTaskBatchSize:                               "history.replicatorTaskBatchSize",
	ReplicatorTaskWorkerCount:                             "history.replicatorTaskWorkerCount",
	ReplicatorTaskMaxRetryCount:                           "history.replicatorTaskMaxRetryCount",
	ReplicatorProcessorStartDelay:                         "history.replicatorProcessorStartDelay",
	ReplicatorProcessorMaxPollRPS:                         "history.replicatorProcessorMaxPollRPS",
	ReplicatorProcessorUpdateShardTaskCount:               "history.replicatorProcessorUpdateShardTaskCount",
	ReplicatorProcessorMaxPollInterval:                    "history.replicatorProcessorMaxPollInterval",
	ReplicatorProcessorMaxPollIntervalJitterCoefficient:   "history.replicatorProcessorMaxPollIntervalJitterCoefficient",
	ReplicatorProcessorUpdateAckInterval:                  "history.replicatorProcessorUpdateAckInterval",
	ReplicatorProcessorUpdateAckIntervalJitterCoefficient: "history.replicatorProcessorUpdateAckIntervalJitterCoefficient",
	ExecutionMgrNumConns:                                  "history.executionMgrNumConns",
	HistoryMgrNumConns:                                    "history.historyMgrNumConns",
	MaximumBufferedEventsBatch:                            "history.maximumBufferedEventsBatch",
	MaximumSignalsPerExecution:                            "history.maximumSignalsPerExecution",
	ShardUpdateMinInterval:                                "history.shardUpdateMinInterval",
	ShardSyncMinInterval:                                  "history.shardSyncMinInterval",
	DefaultEventEncoding:                                  "history.defaultEventEncoding",
	EnableAdminProtection:                                 "history.enableAdminProtection",
	AdminOperationToken:                                   "history.adminOperationToken",
	EnableEventsV2:                                        "history.enableEventsV2",
	NumSystemWorkflows:                                    "history.numSystemWorkflows",

	WorkerPersistenceMaxQPS:                  "worker.persistenceMaxQPS",
	WorkerReplicatorConcurrency:              "worker.replicatorConcurrency",
	WorkerReplicatorActivityBufferRetryCount: "worker.replicatorActivityBufferRetryCount",
	WorkerReplicatorHistoryBufferRetryCount:  "worker.replicatorHistoryBufferRetryCount",
	WorkerReplicationTaskMaxRetry:            "worker.replicationTaskMaxRetry",
	WorkerIndexerConcurrency:                 "worker.indexerConcurrency",
	WorkerESProcessorNumOfWorkers:            "worker.ESProcessorNumOfWorkers",
	WorkerESProcessorBulkActions:             "worker.ESProcessorBulkActions",
	WorkerESProcessorBulkSize:                "worker.ESProcessorBulkSize",
	WorkerESProcessorFlushInterval:           "worker.ESProcessorFlushInterval",
	EnableArchivalCompression:                "worker.EnableArchivalCompression",
	WorkerHistoryPageSize:                    "worker.WorkerHistoryPageSize",
	WorkerTargetArchivalBlobSize:             "worker.WorkerTargetArchivalBlobSize",
}

const (
	unknownKey Key = iota

	// key for tests
	testGetPropertyKey
	testGetIntPropertyKey
	testGetFloat64PropertyKey
	testGetDurationPropertyKey
	testGetBoolPropertyKey
	testGetIntPropertyFilteredByDomainKey
	testGetDurationPropertyFilteredByDomainKey
	testGetIntPropertyFilteredByTaskListInfoKey
	testGetDurationPropertyFilteredByTaskListInfoKey
	testGetBoolPropertyFilteredByTaskListInfoKey

	// EnableGlobalDomain is key for enable global domain
	EnableGlobalDomain
	// EnableNewKafkaClient is key for using New Kafka client
	EnableNewKafkaClient
	// EnableVisibilitySampling is key for enable visibility sampling
	EnableVisibilitySampling
	// EnableReadFromClosedExecutionV2 is key for enable read from cadence_visibility.closed_executions_v2
	EnableReadFromClosedExecutionV2
	// EnableVisibilityToKafka is key for enable kafka
	EnableVisibilityToKafka
	// EnableReadVisibilityFromES is key for enable read from elastic search
	EnableReadVisibilityFromES
	// DisableListVisibilityByFilter is config to disable list open/close workflow using filter
	DisableListVisibilityByFilter
	// EnableArchival is key for enable archival
	EnableArchival

	// BlobSizeLimitError is the per event blob size limit
	BlobSizeLimitError
	// BlobSizeLimitWarn is the per event blob size limit for warning
	BlobSizeLimitWarn
	// HistorySizeLimitError is the per workflow execution history size limit
	HistorySizeLimitError
	// HistorySizeLimitWarn is the per workflow execution history size limit for warning
	HistorySizeLimitWarn
	// HistoryCountLimitError is the per workflow execution history event count limit
	HistoryCountLimitError
	// HistoryCountLimitWarn is the per workflow execution history event count limit for warning
	HistoryCountLimitWarn

	// MaxIDLengthLimit is the length limit for various IDs, including: Domain, TaskList, WorkflowID, ActivityID, TimerID,
	// WorkflowType, ActivityType, SignalName, MarkerName, ErrorReason/FailureReason/CancelCause, Identity, RequestID
	MaxIDLengthLimit

	// key for frontend

	// FrontendPersistenceMaxQPS is the max qps frontend host can query DB
	FrontendPersistenceMaxQPS
	// FrontendVisibilityMaxPageSize is default max size for ListWorkflowExecutions in one page
	FrontendVisibilityMaxPageSize
	// FrontendVisibilityListMaxQPS is max qps frontend can list open/close workflows
	FrontendVisibilityListMaxQPS
	// FrontendESVisibilityListMaxQPS is max qps frontend can list open/close workflows from ElasticSearch
	FrontendESVisibilityListMaxQPS
	// FrontendHistoryMaxPageSize is default max size for GetWorkflowExecutionHistory in one page
	FrontendHistoryMaxPageSize
	// FrontendRPS is workflow rate limit per second
	FrontendRPS
	// FrontendHistoryMgrNumConns is for persistence cluster.NumConns
	FrontendHistoryMgrNumConns
	// MaxDecisionStartToCloseTimeout is max decision timeout in seconds
	MaxDecisionStartToCloseTimeout

	// key for matching

	// MatchingRPS is request rate per second for each matching host
	MatchingRPS
	// MatchingPersistenceMaxQPS is the max qps matching host can query DB
	MatchingPersistenceMaxQPS
	// MatchingMinTaskThrottlingBurstSize is the minimum burst size for task list throttling
	MatchingMinTaskThrottlingBurstSize
	// MatchingGetTasksBatchSize is the maximum batch size to fetch from the task buffer
	MatchingGetTasksBatchSize
	// MatchingLongPollExpirationInterval is the long poll expiration interval in the matching service
	MatchingLongPollExpirationInterval
	// MatchingEnableSyncMatch is to enable sync match
	MatchingEnableSyncMatch
	// MatchingUpdateAckInterval is the interval for update ack
	MatchingUpdateAckInterval
	// MatchingIdleTasklistCheckInterval is the IdleTasklistCheckInterval
	MatchingIdleTasklistCheckInterval
	// MaxTasklistIdleTime is the max time tasklist being idle
	MaxTasklistIdleTime
	// MatchingOutstandingTaskAppendsThreshold is the threshold for outstanding task appends
	MatchingOutstandingTaskAppendsThreshold
	// MatchingMaxTaskBatchSize is max batch size for task writer
	MatchingMaxTaskBatchSize

	// key for history

	// EnableDCMigration whether DC migration is enabled or not
	// TODO remove after DC migration is over
	EnableDCMigration
	// HistoryRPS is request rate per second for each history host
	HistoryRPS
	// HistoryPersistenceMaxQPS is the max qps history host can query DB
	HistoryPersistenceMaxQPS
	// HistoryVisibilityOpenMaxQPS is max qps one history host can write visibility open_executions
	HistoryVisibilityOpenMaxQPS
	// HistoryVisibilityClosedMaxQPS is max qps one history host can write visibility closed_executions
	HistoryVisibilityClosedMaxQPS
	// HistoryLongPollExpirationInterval is the long poll expiration interval in the history service
	HistoryLongPollExpirationInterval
	// HistoryCacheInitialSize is initial size of history cache
	HistoryCacheInitialSize
	// HistoryCacheMaxSize is max size of history cache
	HistoryCacheMaxSize
	// HistoryCacheTTL is TTL of history cache
	HistoryCacheTTL
	// EventsCacheInitialSize is initial size of events cache
	EventsCacheInitialSize
	// EventsCacheMaxSize is max size of events cache
	EventsCacheMaxSize
	// EventsCacheTTL is TTL of events cache
	EventsCacheTTL
	// AcquireShardInterval is interval that timer used to acquire shard
	AcquireShardInterval
	// StandbyClusterDelay is the atrificial delay added to standby cluster's view of active cluster's time
	StandbyClusterDelay
	// TimerTaskBatchSize is batch size for timer processor to process tasks
	TimerTaskBatchSize
	// TimerTaskWorkerCount is number of task workers for timer processor
	TimerTaskWorkerCount
	// TimerTaskMaxRetryCount is max retry count for timer processor
	TimerTaskMaxRetryCount
	// TimerProcessorStartDelay is the start delay
	TimerProcessorStartDelay
	// TimerProcessorFailoverStartDelay is the failover start delay
	TimerProcessorFailoverStartDelay
	// TimerProcessorGetFailureRetryCount is retry count for timer processor get failure operation
	TimerProcessorGetFailureRetryCount
	// TimerProcessorCompleteTimerFailureRetryCount is retry count for timer processor complete timer operation
	TimerProcessorCompleteTimerFailureRetryCount
	// TimerProcessorUpdateShardTaskCount is update shard count for timer processor
	TimerProcessorUpdateShardTaskCount
	// TimerProcessorUpdateAckInterval is update interval for timer processor
	TimerProcessorUpdateAckInterval
	// TimerProcessorUpdateAckIntervalJitterCoefficient is the update interval jitter coefficient
	TimerProcessorUpdateAckIntervalJitterCoefficient
	// TimerProcessorCompleteTimerInterval is complete timer interval for timer processor
	TimerProcessorCompleteTimerInterval
	// TimerProcessorFailoverMaxPollRPS is max poll rate per second for timer processor
	TimerProcessorFailoverMaxPollRPS
	// TimerProcessorMaxPollRPS is max poll rate per second for timer processor
	TimerProcessorMaxPollRPS
	// TimerProcessorMaxPollInterval is max poll interval for timer processor
	TimerProcessorMaxPollInterval
	// TimerProcessorMaxPollIntervalJitterCoefficient is the max poll interval jitter coefficient
	TimerProcessorMaxPollIntervalJitterCoefficient
	// TimerProcessorMaxTimeShift is the max shift timer processor can have
	TimerProcessorMaxTimeShift
	// TransferTaskBatchSize is batch size for transferQueueProcessor
	TransferTaskBatchSize
	// TransferProcessorFailoverMaxPollRPS is max poll rate per second for transferQueueProcessor
	TransferProcessorFailoverMaxPollRPS
	// TransferProcessorMaxPollRPS is max poll rate per second for transferQueueProcessor
	TransferProcessorMaxPollRPS
	// TransferTaskWorkerCount is number of worker for transferQueueProcessor
	TransferTaskWorkerCount
	// TransferTaskMaxRetryCount is max times of retry for transferQueueProcessor
	TransferTaskMaxRetryCount
	// TransferProcessorStartDelay is the start delay
	TransferProcessorStartDelay
	// TransferProcessorFailoverStartDelay is the failover start delay
	TransferProcessorFailoverStartDelay
	// TransferProcessorCompleteTransferFailureRetryCount is times of retry for failure
	TransferProcessorCompleteTransferFailureRetryCount
	// TransferProcessorUpdateShardTaskCount is update shard count for transferQueueProcessor
	TransferProcessorUpdateShardTaskCount
	// TransferProcessorMaxPollInterval max poll interval for transferQueueProcessor
	TransferProcessorMaxPollInterval
	// TransferProcessorMaxPollIntervalJitterCoefficient is the max poll interval jitter coefficient
	TransferProcessorMaxPollIntervalJitterCoefficient
	// TransferProcessorUpdateAckInterval is update interval for transferQueueProcessor
	TransferProcessorUpdateAckInterval
	// TransferProcessorUpdateAckIntervalJitterCoefficient is the update interval jitter coefficient
	TransferProcessorUpdateAckIntervalJitterCoefficient
	// TransferProcessorCompleteTransferInterval is complete timer interval for transferQueueProcessor
	TransferProcessorCompleteTransferInterval
	// ReplicatorTaskBatchSize is batch size for ReplicatorProcessor
	ReplicatorTaskBatchSize
	// ReplicatorTaskWorkerCount is number of worker for ReplicatorProcessor
	ReplicatorTaskWorkerCount
	// ReplicatorTaskMaxRetryCount is max times of retry for ReplicatorProcessor
	ReplicatorTaskMaxRetryCount
	// ReplicatorProcessorStartDelay is the start delay
	ReplicatorProcessorStartDelay
	// ReplicatorProcessorMaxPollRPS is max poll rate per second for ReplicatorProcessor
	ReplicatorProcessorMaxPollRPS
	// ReplicatorProcessorUpdateShardTaskCount is update shard count for ReplicatorProcessor
	ReplicatorProcessorUpdateShardTaskCount
	// ReplicatorProcessorMaxPollInterval is max poll interval for ReplicatorProcessor
	ReplicatorProcessorMaxPollInterval
	// ReplicatorProcessorMaxPollIntervalJitterCoefficient is the max poll interval jitter coefficient
	ReplicatorProcessorMaxPollIntervalJitterCoefficient
	// ReplicatorProcessorUpdateAckInterval is update interval for ReplicatorProcessor
	ReplicatorProcessorUpdateAckInterval
	// ReplicatorProcessorUpdateAckIntervalJitterCoefficient is the update interval jitter coefficient
	ReplicatorProcessorUpdateAckIntervalJitterCoefficient
	// ExecutionMgrNumConns is persistence connections number for ExecutionManager
	ExecutionMgrNumConns
	// HistoryMgrNumConns is persistence connections number for HistoryManager
	HistoryMgrNumConns
	// MaximumBufferedEventsBatch is max number of buffer event in mutable state
	MaximumBufferedEventsBatch
	// MaximumSignalsPerExecution is max number of signals supported by single execution
	MaximumSignalsPerExecution
	// ShardUpdateMinInterval is the minimal time interval which the shard info can be updated
	ShardUpdateMinInterval
	// ShardSyncMinInterval is the minimal time interval which the shard info should be sync to remote
	ShardSyncMinInterval
	// DefaultEventEncoding is the encoding type for history events
	DefaultEventEncoding
	// NumSystemWorkflows is key for number of system workflows running in total
	NumSystemWorkflows

	// EnableAdminProtection is whether to enable admin checking
	EnableAdminProtection
	// AdminOperationToken is the token to pass admin checking
	AdminOperationToken

	// EnableEventsV2 is whether to use eventsV2
	EnableEventsV2

	// key for worker

	// WorkerPersistenceMaxQPS is the max qps worker host can query DB
	WorkerPersistenceMaxQPS
	// WorkerReplicatorConcurrency is the max concurrent tasks to be processed at any given time
	WorkerReplicatorConcurrency
	// WorkerReplicatorActivityBufferRetryCount is the retry attempt when encounter retry error on activity
	WorkerReplicatorActivityBufferRetryCount
	// WorkerReplicatorHistoryBufferRetryCount is the retry attempt when encounter retry error on history
	WorkerReplicatorHistoryBufferRetryCount
	// WorkerReplicationTaskMaxRetry is the max retry for any task
	WorkerReplicationTaskMaxRetry
	// WorkerIndexerConcurrency is the max concurrent messages to be processed at any given time
	WorkerIndexerConcurrency
	// WorkerESProcessorNumOfWorkers is num of workers for esProcessor
	WorkerESProcessorNumOfWorkers
	// WorkerESProcessorBulkActions is max number of requests in bulk for esProcessor
	WorkerESProcessorBulkActions
	// WorkerESProcessorBulkSize is max total size of bulk in bytes for esProcessor
	WorkerESProcessorBulkSize
	// WorkerESProcessorFlushInterval is flush interval for esProcessor
	WorkerESProcessorFlushInterval
	// EnableArchivalCompression indicates whether blobs are compressed before they are archived
	EnableArchivalCompression
	// WorkerHistoryPageSize indicates the page size of history fetched from persistence for archival
	WorkerHistoryPageSize
	// WorkerTargetArchivalBlobSize indicates the target blob size in bytes for archival, actual blob size may vary
	WorkerTargetArchivalBlobSize

	// lastKeyForTest must be the last one in this const group for testing purpose
	lastKeyForTest
)

// Filter represents a filter on the dynamic config key
type Filter int

func (f Filter) String() string {
	if f <= unknownFilter || f > TaskListName {
		return filters[unknownFilter]
	}
	return filters[f]
}

var filters = []string{
	"unknownFilter",
	"domainName",
	"taskListName",
	"taskType",
}

const (
	unknownFilter Filter = iota
	// DomainName is the domain name
	DomainName
	// TaskListName is the tasklist name
	TaskListName
	// TaskType is the task type (0:Decision, 1:Activity)
	TaskType

	// lastFilterTypeForTest must be the last one in this const group for testing purpose
	lastFilterTypeForTest
)

// FilterOption is used to provide filters for dynamic config keys
type FilterOption func(filterMap map[Filter]interface{})

// TaskListFilter filters by task list name
func TaskListFilter(name string) FilterOption {
	return func(filterMap map[Filter]interface{}) {
		filterMap[TaskListName] = name
	}
}

// DomainFilter filters by domain name
func DomainFilter(name string) FilterOption {
	return func(filterMap map[Filter]interface{}) {
		filterMap[DomainName] = name
	}
}

// TaskTypeFilter filters by task type
func TaskTypeFilter(taskType int) FilterOption {
	return func(filterMap map[Filter]interface{}) {
		filterMap[TaskType] = taskType
	}
}
