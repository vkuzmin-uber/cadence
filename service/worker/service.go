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

package worker

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/client/public"
	"github.com/uber/cadence/common/blobstore"

	"github.com/uber/cadence/common/cache"

	"github.com/uber-common/bark"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/metrics"
	persistencefactory "github.com/uber/cadence/common/persistence/persistence-factory"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/service/dynamicconfig"
	"github.com/uber/cadence/service/worker/indexer"
	"github.com/uber/cadence/service/worker/replicator"
	"github.com/uber/cadence/service/worker/sysworkflow"
	"go.uber.org/cadence/.gen/go/shared"
)

const (
	publicClientRetryLimit   = 5
	publicClientPollingDelay = time.Second
)

type (
	// Service represents the cadence-worker service. This service hosts all background processing needed for cadence cluster:
	// 1. Replicator: Handles applying replication tasks generated by remote clusters.
	// 2. Indexer: Handles uploading of visibility records to elastic search.
	// 3. Sysworker: Handles running cadence client worker, thereby enabling cadence to host arbitrary system workflows
	Service struct {
		stopC         chan struct{}
		isStopped     int32
		params        *service.BootstrapParams
		config        *Config
		logger        bark.Logger
		metricsClient metrics.Client
	}

	// Config contains all the service config for worker
	Config struct {
		ReplicationCfg *replicator.Config
		SysWorkflowCfg *sysworkflow.Config
		IndexerCfg     *indexer.Config
	}
)

// NewService builds a new cadence-worker service
func NewService(params *service.BootstrapParams) common.Daemon {
	params.UpdateLoggerWithServiceName(common.WorkerServiceName)
	return &Service{
		params: params,
		config: NewConfig(dynamicconfig.NewCollection(params.DynamicConfig, params.Logger)),
		stopC:  make(chan struct{}),
	}
}

// NewConfig builds the new Config for cadence-worker service
func NewConfig(dc *dynamicconfig.Collection) *Config {
	return &Config{
		ReplicationCfg: &replicator.Config{
			// TODO remove after DC migration is over
			EnableDCMigration:                  dc.GetBoolProperty(dynamicconfig.EnableDCMigration, false),
			PersistenceMaxQPS:                  dc.GetIntProperty(dynamicconfig.WorkerPersistenceMaxQPS, 500),
			ReplicatorConcurrency:              dc.GetIntProperty(dynamicconfig.WorkerReplicatorConcurrency, 1000),
			ReplicatorActivityBufferRetryCount: dc.GetIntProperty(dynamicconfig.WorkerReplicatorActivityBufferRetryCount, 8),
			ReplicatorHistoryBufferRetryCount:  dc.GetIntProperty(dynamicconfig.WorkerReplicatorHistoryBufferRetryCount, 8),
			ReplicationTaskMaxRetry:            dc.GetIntProperty(dynamicconfig.WorkerReplicationTaskMaxRetry, 50),
		},
		SysWorkflowCfg: &sysworkflow.Config{
			EnableArchivalCompression: dc.GetBoolPropertyFnWithDomainFilter(dynamicconfig.EnableArchivalCompression, true),
			HistoryPageSize:           dc.GetIntPropertyFilteredByDomain(dynamicconfig.WorkerHistoryPageSize, 250),
			TargetArchivalBlobSize:    dc.GetIntPropertyFilteredByDomain(dynamicconfig.WorkerTargetArchivalBlobSize, 2*1024*1024), // 2MB
		},
		IndexerCfg: &indexer.Config{
			IndexerConcurrency:       dc.GetIntProperty(dynamicconfig.WorkerIndexerConcurrency, 1000),
			ESProcessorNumOfWorkers:  dc.GetIntProperty(dynamicconfig.WorkerESProcessorNumOfWorkers, 1),
			ESProcessorBulkActions:   dc.GetIntProperty(dynamicconfig.WorkerESProcessorBulkActions, 1000),
			ESProcessorBulkSize:      dc.GetIntProperty(dynamicconfig.WorkerESProcessorBulkSize, 2<<24), // 16MB
			ESProcessorFlushInterval: dc.GetDurationProperty(dynamicconfig.WorkerESProcessorFlushInterval, 10*time.Second),
		},
	}
}

// Start is called to start the service
func (s *Service) Start() {
	base := service.New(s.params)
	base.Start()
	s.logger = base.GetLogger()
	s.metricsClient = base.GetMetricsClient()
	s.logger.Infof("%v starting", common.WorkerServiceName)

	pConfig := s.params.PersistenceConfig
	pConfig.SetMaxQPS(pConfig.DefaultStore, s.config.ReplicationCfg.PersistenceMaxQPS())
	pFactory := persistencefactory.New(&pConfig, s.params.ClusterMetadata.GetCurrentClusterName(), s.metricsClient, s.logger)

	if base.GetClusterMetadata().IsGlobalDomainEnabled() {
		s.startReplicator(base, pFactory)
	}
	if base.GetClusterMetadata().IsArchivalEnabled() {
		s.startSysWorker(base, pFactory)
	}
	if s.params.ESConfig.Enable {
		s.startIndexer(base)
	}

	s.logger.Infof("%v started", common.WorkerServiceName)
	<-s.stopC
	base.Stop()
}

// Stop is called to stop the service
func (s *Service) Stop() {
	if !atomic.CompareAndSwapInt32(&s.isStopped, 0, 1) {
		return
	}
	close(s.stopC)
	s.params.Logger.Infof("%v stopped", common.WorkerServiceName)
}

func (s *Service) startReplicator(base service.Service, pFactory persistencefactory.Factory) {
	metadataV2Mgr, err := pFactory.NewMetadataManager(persistencefactory.MetadataV2)
	if err != nil {
		s.logger.Fatalf("failed to start replicator, could not create MetadataManager: %v", err)
	}
	domainCache := cache.NewDomainCache(metadataV2Mgr, base.GetClusterMetadata(), s.metricsClient, s.logger)
	domainCache.Start()

	replicator := replicator.NewReplicator(
		base.GetClusterMetadata(),
		metadataV2Mgr,
		domainCache,
		base.GetClientBean(),
		s.config.ReplicationCfg,
		base.GetMessagingClient(),
		s.logger,
		s.metricsClient)
	if err := replicator.Start(); err != nil {
		replicator.Stop()
		s.logger.Fatalf("fail to start replicator: %v", err)
	}
}

func (s *Service) startIndexer(base service.Service) {
	indexer := indexer.NewIndexer(
		s.config.IndexerCfg,
		base.GetMessagingClient(),
		s.params.ESClient,
		s.params.ESConfig,
		s.logger,
		s.metricsClient)
	if err := indexer.Start(); err != nil {
		indexer.Stop()
		s.logger.Fatalf("fail to start indexer: %v", err)
	}
}

func (s *Service) startSysWorker(base service.Service, pFactory persistencefactory.Factory) {
	publicClient := public.NewRetryableClient(
		base.GetClientBean().GetPublicClient(),
		common.CreatePublicClientRetryPolicy(),
		common.IsWhitelistServiceTransientError,
	)
	s.waitForFrontendStart(publicClient)

	historyManager, err := pFactory.NewHistoryManager()
	if err != nil {
		s.logger.Fatalf("failed to start sysworker, could not create HistoryManager: %v", err)
	}
	historyV2Manager, err := pFactory.NewHistoryV2Manager()
	if err != nil {
		s.logger.Fatalf("failed to start sysworker, could not create HistoryV2Manager: %v", err)
	}
	metadataMgr, err := pFactory.NewMetadataManager(persistencefactory.MetadataV1V2)
	if err != nil {
		s.logger.Fatalf("failed to start sysworker, could not create MetadataManager: %v", err)
	}
	domainCache := cache.NewDomainCache(metadataMgr, s.params.ClusterMetadata, s.metricsClient, s.logger)
	domainCache.Start()

	blobstoreClient := blobstore.NewRetryableClient(
		blobstore.NewMetricClient(s.params.BlobstoreClient, s.metricsClient),
		common.CreateBlobstoreClientRetryPolicy(),
		common.IsBlobstoreTransientError)

	sysWorkerContainer := &sysworkflow.SysWorkerContainer{
		PublicClient:     publicClient,
		MetricsClient:    s.metricsClient,
		Logger:           s.logger,
		ClusterMetadata:  base.GetClusterMetadata(),
		HistoryManager:   historyManager,
		HistoryV2Manager: historyV2Manager,
		Blobstore:        blobstoreClient,
		DomainCache:      domainCache,
		Config:           s.config.SysWorkflowCfg,
	}
	sysWorker := sysworkflow.NewSysWorker(sysWorkerContainer)
	if err := sysWorker.Start(); err != nil {
		sysWorker.Stop()
		s.logger.Fatalf("failed to start sysworker: %v", err)
	}
}

func (s *Service) waitForFrontendStart(publicClient public.Client) {
	request := &shared.DescribeDomainRequest{
		Name: common.StringPtr(sysworkflow.SystemDomainName),
	}

RetryLoop:
	for i := 0; i < publicClientRetryLimit; i++ {
		if _, err := publicClient.DescribeDomain(context.Background(), request); err == nil {
			return
		}
		select {
		case <-time.After(publicClientPollingDelay):
			continue RetryLoop
		case <-s.stopC:
			return
		}
	}
	s.logger.Fatal("failed to connect to frontend client")
}
