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
	"context"
	"time"

	"github.com/pborman/uuid"
	"github.com/uber-common/bark"
	h "github.com/uber/cadence/.gen/go/history"
	"github.com/uber/cadence/.gen/go/shared"
	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/logging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
)

var (
	errNoHistoryFound = errors.NewInternalFailureError("no history events found")

	workflowTerminationReason   = "Terminate Workflow Due To Version Conflict."
	workflowTerminationIdentity = "worker-service"
)

type (
	conflictResolverProvider func(context workflowExecutionContext, logger bark.Logger) conflictResolver
	stateBuilderProvider     func(msBuilder mutableState, logger bark.Logger) stateBuilder
	mutableStateProvider     func(version int64, logger bark.Logger) mutableState

	historyReplicator struct {
		shard             ShardContext
		historyEngine     *historyEngineImpl
		historyCache      *historyCache
		domainCache       cache.DomainCache
		historySerializer persistence.HistorySerializer
		historyMgr        persistence.HistoryManager
		historyV2Mgr      persistence.HistoryV2Manager
		clusterMetadata   cluster.Metadata
		metricsClient     metrics.Client
		logger            bark.Logger
		resetor           workflowResetor

		getNewConflictResolver conflictResolverProvider
		getNewStateBuilder     stateBuilderProvider
		getNewMutableState     mutableStateProvider
	}
)

var (
	// ErrRetryEntityNotExists is returned to indicate workflow execution is not created yet and replicator should
	// try this task again after a small delay.
	ErrRetryEntityNotExists = &shared.RetryTaskError{Message: "entity not exists"}
	// ErrRetrySyncActivityMsg is returned when sync activity replication tasks are arriving out of order, should retry
	ErrRetrySyncActivityMsg = "retry on applying sync activity"
	// ErrRetryBufferEventsMsg is returned when events are arriving out of order, should retry, or specify force apply
	ErrRetryBufferEventsMsg = "retry on applying buffer events"
	// ErrRetryEmptyEventsMsg is returned when events size is 0
	ErrRetryEmptyEventsMsg = "retry on applying empty events"
	// ErrWorkflowNotFoundMsg is returned when workflow not found
	ErrWorkflowNotFoundMsg = "retry on workflow not found"
	// ErrRetryExistingWorkflowMsg is returned when events are arriving out of order, and there is another workflow with same version running
	ErrRetryExistingWorkflowMsg = "workflow with same version is running"
	// ErrRetryExecutionAlreadyStarted is returned to indicate another workflow execution already started,
	// this error can be return if we encounter race condition, i.e. terminating the target workflow while
	// the target workflow has done continue as new.
	// try this task again after a small delay.
	ErrRetryExecutionAlreadyStarted = &shared.RetryTaskError{Message: "another workflow execution is running"}
	// ErrCorruptedReplicationInfo is returned when replication task has corrupted replication information from source cluster
	ErrCorruptedReplicationInfo = &shared.BadRequestError{Message: "replication task is has corrupted cluster replication info"}
	// ErrCorruptedMutableStateDecision is returned when mutable state decision is corrupted
	ErrCorruptedMutableStateDecision = &shared.BadRequestError{Message: "mutable state decision is corrupted"}
	// ErrMoreThan2DC is returned when there are more than 2 data center
	ErrMoreThan2DC = &shared.BadRequestError{Message: "more than 2 data center"}
	// ErrImpossibleLocalRemoteMissingReplicationInfo is returned when replication task is missing replication info, as well as local replication info being empty
	ErrImpossibleLocalRemoteMissingReplicationInfo = &shared.BadRequestError{Message: "local and remote both are missing replication info"}
	// ErrImpossibleRemoteClaimSeenHigherVersion is returned when replication info contains higher version then this cluster ever emitted.
	ErrImpossibleRemoteClaimSeenHigherVersion = &shared.BadRequestError{Message: "replication info contains higher version then this cluster ever emitted"}
	// ErrInternalFailure is returned when encounter code bug
	ErrInternalFailure = &shared.BadRequestError{Message: "fail to apply history events due bug"}
	// ErrEmptyHistoryRawEventBatch indicate that one single batch of history raw events is of size 0
	ErrEmptyHistoryRawEventBatch = &shared.BadRequestError{Message: "encounter empty history batch"}
	// ErrUnknownEncodingType indicate that the encoding type is unknown
	ErrUnknownEncodingType = &shared.BadRequestError{Message: "unknown encoding type"}
)

func newHistoryReplicator(shard ShardContext, historyEngine *historyEngineImpl, historyCache *historyCache, domainCache cache.DomainCache,
	historyMgr persistence.HistoryManager, historyV2Mgr persistence.HistoryV2Manager, logger bark.Logger) *historyReplicator {
	replicator := &historyReplicator{
		shard:             shard,
		historyEngine:     historyEngine,
		historyCache:      historyCache,
		domainCache:       domainCache,
		historySerializer: persistence.NewHistorySerializer(),
		historyMgr:        historyMgr,
		historyV2Mgr:      historyV2Mgr,
		clusterMetadata:   shard.GetService().GetClusterMetadata(),
		metricsClient:     shard.GetMetricsClient(),
		logger:            logger.WithField(logging.TagWorkflowComponent, logging.TagValueHistoryReplicatorComponent),

		getNewConflictResolver: func(context workflowExecutionContext, logger bark.Logger) conflictResolver {
			return newConflictResolver(shard, context, historyMgr, historyV2Mgr, logger)
		},
		getNewStateBuilder: func(msBuilder mutableState, logger bark.Logger) stateBuilder {
			return newStateBuilder(shard, msBuilder, logger)
		},
		getNewMutableState: func(version int64, logger bark.Logger) mutableState {
			return newMutableStateBuilderWithReplicationState(
				shard.GetService().GetClusterMetadata().GetCurrentClusterName(),
				shard.GetConfig(),
				shard.GetEventsCache(),
				logger,
				version,
			)
		},
	}
	replicator.resetor = newWorkflowResetor(historyEngine, replicator)

	return replicator
}

func (r *historyReplicator) SyncActivity(ctx context.Context, request *h.SyncActivityRequest) (retError error) {

	// sync activity info will only be sent from active side, when
	// 1. activity has retry policy and activity got started
	// 2. activity heart beat
	// no sync activity task will be sent when active side fail / timeout activity,
	// since standby side does not have activity retry timer

	domainID := request.GetDomainId()
	execution := workflow.WorkflowExecution{
		WorkflowId: request.WorkflowId,
		RunId:      request.RunId,
	}

	context, release, err := r.historyCache.getOrCreateWorkflowExecutionWithTimeout(ctx, domainID, execution)
	if err != nil {
		// for get workflow execution context, with valid run id
		// err will not be of type EntityNotExistsError
		return err
	}
	defer func() { release(retError) }()

	msBuilder, err := context.loadWorkflowExecution()
	if err != nil {
		if _, ok := err.(*workflow.EntityNotExistsError); !ok {
			return err
		}

		// this can happen if the workflow start event and this sync activity task are out of order
		// or the target workflow is long gone
		// the safe solution to this is to throw away the sync activity task
		// or otherwise, worker attempt will exceeds limit and put this message to DLQ
		return nil
	}

	if !msBuilder.IsWorkflowExecutionRunning() {
		// perhaps conflict resolution force termination
		return nil
	}

	version := request.GetVersion()
	scheduleID := request.GetScheduledId()
	if scheduleID >= msBuilder.GetNextEventID() {
		if version < msBuilder.GetLastWriteVersion() {
			// activity version < workflow last write version
			// this can happen if target workflow has
			return nil
		}

		// version >= last write version
		// this can happen if out of order delivery heppens
		return newRetryTaskErrorWithHint(ErrRetrySyncActivityMsg, domainID, execution.GetWorkflowId(), execution.GetRunId(), msBuilder.GetNextEventID())
	}

	ai, isRunning := msBuilder.GetActivityInfo(scheduleID)
	if !isRunning {
		// this should not retry, can be caused by out of order delivery
		// since the activity is already finished
		return nil
	}

	if ai.Version > request.GetVersion() {
		// this should not retry, can be caused by failover or reset
		return nil
	}

	if ai.Version == request.GetVersion() {
		if ai.Attempt > request.GetAttempt() {
			// this should not retry, can be caused by failover or reset
			return nil
		}
		if ai.Attempt == request.GetAttempt() {
			lastHeartbeatTime := time.Unix(0, request.GetLastHeartbeatTime())
			if ai.LastHeartBeatUpdatedTime.After(lastHeartbeatTime) {
				// this should not retry, can be caused by out of order delivery
				return nil
			}
			// version equal & attempt equal & last heartbeat after existing heartbeat
			// should update activity
		}
		// version equal & attempt larger then existing, should update activity
	}
	// version larger then existing, should update activity

	// calculate whether to reset the activity timer task status bits
	// reset timer task status bits if
	// 1. same source cluster & attempt changes
	// 2. different source cluster
	resetActivityTimerTaskStatus := false
	if !r.clusterMetadata.IsVersionFromSameCluster(request.GetVersion(), ai.Version) {
		resetActivityTimerTaskStatus = true
	} else if ai.Attempt < request.GetAttempt() {
		resetActivityTimerTaskStatus = true
	}
	err = msBuilder.ReplicateActivityInfo(request, resetActivityTimerTaskStatus)
	if err != nil {
		return err
	}

	// see whether we need to refresh the activity timer
	eventTime := request.GetScheduledTime()
	if eventTime < request.GetStartedTime() {
		eventTime = request.GetStartedTime()
	}
	if eventTime < request.GetLastHeartbeatTime() {
		eventTime = request.GetLastHeartbeatTime()
	}
	now := time.Unix(0, eventTime)
	timerTasks := []persistence.Task{}
	timeSource := common.NewEventTimeSource()
	timeSource.Update(now)
	timerBuilder := newTimerBuilder(r.shard.GetConfig(), r.logger, timeSource)
	if tt := timerBuilder.GetActivityTimerTaskIfNeeded(msBuilder); tt != nil {
		timerTasks = append(timerTasks, tt)
	}

	return r.updateMutableStateWithTimer(context, msBuilder, now, timerTasks)
}

func (r *historyReplicator) ApplyRawEvents(ctx context.Context, requestIn *h.ReplicateRawEventsRequest) (retError error) {
	var err error
	var events []*workflow.HistoryEvent
	var newRunEvents []*workflow.HistoryEvent

	events, err = r.deserializeBlob(requestIn.History)
	if err != nil {
		return err
	}

	version := events[0].GetVersion()
	firstEventID := events[0].GetEventId()
	nextEventID := events[len(events)-1].GetEventId() + 1
	sourceCluster := r.clusterMetadata.ClusterNameForFailoverVersion(version)

	requestOut := &h.ReplicateEventsRequest{
		SourceCluster:           common.StringPtr(sourceCluster),
		DomainUUID:              requestIn.DomainUUID,
		WorkflowExecution:       requestIn.WorkflowExecution,
		FirstEventId:            common.Int64Ptr(firstEventID),
		NextEventId:             common.Int64Ptr(nextEventID),
		Version:                 common.Int64Ptr(version),
		ReplicationInfo:         requestIn.ReplicationInfo,
		History:                 &shared.History{Events: events},
		EventStoreVersion:       requestIn.EventStoreVersion,
		NewRunHistory:           nil,
		NewRunEventStoreVersion: nil,
		ForceBufferEvents:       common.BoolPtr(true),
	}

	if requestIn.NewRunHistory != nil {
		newRunEvents, err = r.deserializeBlob(requestIn.NewRunHistory)
		if err != nil {
			return err
		}
		requestOut.NewRunHistory = &shared.History{Events: newRunEvents}
		requestOut.NewRunEventStoreVersion = requestIn.NewRunEventStoreVersion
	}
	return r.ApplyEvents(ctx, requestOut, true)
}

func (r *historyReplicator) ApplyEvents(ctx context.Context, request *h.ReplicateEventsRequest, inRetry bool) (retError error) {
	logger := r.logger.WithFields(bark.Fields{
		logging.TagWorkflowExecutionID: request.WorkflowExecution.GetWorkflowId(),
		logging.TagWorkflowRunID:       request.WorkflowExecution.GetRunId(),
		logging.TagSourceCluster:       request.GetSourceCluster(),
		logging.TagIncomingVersion:     request.GetVersion(),
		logging.TagFirstEventID:        request.GetFirstEventId(),
		logging.TagNextEventID:         request.GetNextEventId(),
	})

	r.metricsClient.RecordTimer(
		metrics.ReplicateHistoryEventsScope,
		metrics.ReplicationEventsSizeTimer,
		time.Duration(len(request.History.Events)),
	)

	defer func() {
		if retError != nil {
			switch retError.(type) {
			case *shared.EntityNotExistsError:
				logger.Debugf("Encounter EntityNotExistsError: %v", retError)
				retError = ErrRetryEntityNotExists
			case *shared.WorkflowExecutionAlreadyStartedError:
				logger.Debugf("Encounter WorkflowExecutionAlreadyStartedError: %v", retError)
				retError = ErrRetryExecutionAlreadyStarted
			case *persistence.WorkflowExecutionAlreadyStartedError:
				logger.Debugf("Encounter WorkflowExecutionAlreadyStartedError: %v", retError)
				retError = ErrRetryExecutionAlreadyStarted
			case *errors.InternalFailureError:
				logError(logger, "Encounter InternalFailure.", retError)
				retError = ErrInternalFailure
			}
		}
	}()

	if request == nil || request.History == nil || len(request.History.Events) == 0 {
		logger.Warn("Dropping empty replication task")
		r.metricsClient.IncCounter(metrics.ReplicateHistoryEventsScope, metrics.EmptyReplicationEventsCounter)
		return nil
	}
	domainID, err := validateDomainUUID(request.DomainUUID)
	if err != nil {
		return err
	}

	execution := *request.WorkflowExecution
	context, release, err := r.historyCache.getOrCreateWorkflowExecutionWithTimeout(ctx, domainID, execution)
	if err != nil {
		// for get workflow execution context, with valid run id
		// err will not be of type EntityNotExistsError
		return err
	}
	defer func() { release(retError) }()

	firstEvent := request.History.Events[0]
	switch firstEvent.GetEventType() {
	case shared.EventTypeWorkflowExecutionStarted:
		_, err := context.loadWorkflowExecution()
		if err == nil {
			// Workflow execution already exist, looks like a duplicate start event, it is safe to ignore it
			logger.Debugf("Dropping stale replication task for start event.")
			r.metricsClient.IncCounter(metrics.ReplicateHistoryEventsScope, metrics.DuplicateReplicationEventsCounter)
			return nil
		}
		if _, ok := err.(*shared.EntityNotExistsError); !ok {
			// GetWorkflowExecution failed with some transient error. Return err so we can retry the task later
			return err
		}
		return r.ApplyStartEvent(ctx, context, request, logger)

	default:
		// apply events, other than simple start workflow execution
		// the continue as new + start workflow execution combination will also be processed here
		msBuilder, err := context.loadWorkflowExecution()
		if err != nil {
			if _, ok := err.(*shared.EntityNotExistsError); !ok {
				return err
			}
			// mutable state for the target workflow ID & run ID combination does not exist
			// we need to check the existing workflow ID
			release(err)
			lastEvent := request.History.Events[len(request.History.Events)-1]
			return r.ApplyOtherEventsMissingMutableState(ctx, domainID, request.WorkflowExecution.GetWorkflowId(),
				request.WorkflowExecution.GetRunId(), firstEvent.GetVersion(), lastEvent.GetTimestamp(), logger, request)
		}

		logger.WithField(logging.TagCurrentVersion, msBuilder.GetReplicationState().LastWriteVersion)
		err = r.flushReplicationBuffer(ctx, context, msBuilder, logger)
		if err != nil {
			logError(logger, "Fail to pre-flush buffer.", err)
			return err
		}
		msBuilder, err = r.ApplyOtherEventsVersionChecking(ctx, context, msBuilder, request, logger, inRetry)
		if err != nil || msBuilder == nil {
			return err
		}
		return r.ApplyOtherEvents(ctx, context, msBuilder, request, logger)
	}
}

func (r *historyReplicator) ApplyStartEvent(ctx context.Context, context workflowExecutionContext,
	request *h.ReplicateEventsRequest,
	logger bark.Logger) error {
	msBuilder := r.getNewMutableState(request.GetVersion(), logger)
	err := r.ApplyReplicationTask(ctx, context, msBuilder, request, logger)
	return err
}

func (r *historyReplicator) ApplyOtherEventsMissingMutableState(ctx context.Context, domainID string, workflowID string,
	runID string, incomingVersion int64, incomingTimestamp int64, logger bark.Logger, request *h.ReplicateEventsRequest) error {
	// we need to check the current workflow execution
	_, currentMutableState, currentRelease, err := r.getCurrentWorkflowMutableState(ctx, domainID, workflowID)
	if err != nil {
		if _, ok := err.(*shared.EntityNotExistsError); !ok {
			return err
		}
		return newRetryTaskErrorWithHint(ErrWorkflowNotFoundMsg, domainID, workflowID, runID, common.FirstEventID)
	}
	currentRunID := currentMutableState.GetExecutionInfo().RunID
	currentLastWriteVersion := currentMutableState.GetLastWriteVersion()
	currentRelease(nil)

	if currentLastWriteVersion > incomingVersion {
		logger.Info("Dropping replication task.")
		r.metricsClient.IncCounter(metrics.ReplicateHistoryEventsScope, metrics.StaleReplicationEventsCounter)
		return nil
	} else if currentLastWriteVersion < incomingVersion && !request.GetResetWorkflow() {
		err = r.terminateWorkflow(ctx, domainID, workflowID, currentRunID, incomingVersion, incomingTimestamp, logger)
		if err != nil {
			if _, ok := err.(*shared.EntityNotExistsError); !ok {
				return err
			}
			// if workflow is completed just when the call is made, will get EntityNotExistsError
			// we are not sure whether the workflow to be terminated ends with continue as new or not
			// so when encounter EntityNotExistsError, just contiue to execute, if err occurs,
			// there will be retry on the worker level
		}
		return newRetryTaskErrorWithHint(ErrWorkflowNotFoundMsg, domainID, workflowID, runID, common.FirstEventID)

	}
	// currentLastWriteVersion <= incomingVersion
	logger.Debugf("Retrying replication task. Current RunID: %v, Current LastWriteVersion: %v, Incoming Version: %v.",
		currentRunID, currentLastWriteVersion, incomingVersion)

	// try flush the current workflow buffer
	currentRunID, currentNextEventID, currentStillRunning, err := r.flushCurrentWorkflowBuffer(ctx, domainID, workflowID, logger)
	if err != nil {
		return err
	}

	if currentStillRunning {
		return newRetryTaskErrorWithHint(ErrWorkflowNotFoundMsg, domainID, workflowID, currentRunID, currentNextEventID)
	}

	if request.GetResetWorkflow() {
		//Note that at this point, current run is already closed and currentLastWriteVersion <= incomingVersion
		return r.resetor.ApplyResetEvent(ctx, request, domainID, workflowID, currentRunID)
	}
	return newRetryTaskErrorWithHint(ErrWorkflowNotFoundMsg, domainID, workflowID, runID, common.FirstEventID)
}

func (r *historyReplicator) ApplyOtherEventsVersionChecking(ctx context.Context, context workflowExecutionContext,
	msBuilder mutableState, request *h.ReplicateEventsRequest, logger bark.Logger, inRetry bool) (mutableState, error) {
	var err error
	// check if to buffer / drop / conflict resolution
	incomingVersion := request.GetVersion()
	replicationInfo := request.ReplicationInfo
	rState := msBuilder.GetReplicationState()
	if rState.LastWriteVersion > incomingVersion {
		// Replication state is already on a higher version, we can drop this event
		// TODO: We need to replay external events like signal to the new version
		logger.Info("Dropping stale replication task.")
		r.metricsClient.IncCounter(metrics.ReplicateHistoryEventsScope, metrics.StaleReplicationEventsCounter)
		_, err = r.garbageCollectSignals(context, msBuilder, request.History.Events)
		return nil, err
	}

	if rState.LastWriteVersion == incomingVersion {
		// for ri.GetLastEventId() == rState.LastWriteEventID, ideally we should not do anything
		return msBuilder, nil
	}

	// we have rState.LastWriteVersion < incomingVersion

	// the code below only deal with 2 data center case
	// for multiple data center cases, wait for #840

	// Check if this is the first event after failover
	previousActiveCluster := r.clusterMetadata.ClusterNameForFailoverVersion(rState.LastWriteVersion)
	logger.WithFields(bark.Fields{
		logging.TagPrevActiveCluster: previousActiveCluster,
		logging.TagReplicationInfo:   request.ReplicationInfo,
	})
	logger.Info("First Event after replication.")

	// first check whether the replication info
	// the reason is, if current cluster was active, and sent out replication task
	// to remote, there is no guarantee that the replication task is going to be applied,
	// if not applied, the replication info will not be up to date.

	if previousActiveCluster != r.clusterMetadata.GetCurrentClusterName() {
		doDCMigration, err := r.canDoDCMigration(context.getDomainID())
		if err != nil {
			return nil, err
		}
		// this cluster is previously NOT active, this also means there is no buffered event
		if r.clusterMetadata.IsVersionFromSameCluster(incomingVersion, rState.LastWriteVersion) {
			// it is possible that a workflow will not generate any event in few rounds of failover
			// meaning that the incoming version > last write version and
			// (incoming version - last write version) % failover version increment == 0
			if !doDCMigration {
				return msBuilder, nil
			}
		}

		// TODO remove after DC migration is over
		// NOTE: DO NOT TURN ON THE FLAG UNLESS YOU KNOW WHAT YOU ARE DOING
		if r.shard.GetConfig().EnableDCMigration() {
			if doDCMigration && inRetry {
				return msBuilder, nil
			}

			if doDCMigration {
				dcMigrationHandler := newDCMigrationHandler(r.historyMgr, r.historyV2Mgr)
				expectedLastEventID, err := dcMigrationHandler.getLastMatchEventID(
					ctx, context, msBuilder, request, logger,
				)
				if err != nil {
					return nil, err
				}
				if expectedLastEventID < msBuilder.GetReplicationState().LastWriteEventID {
					lastEvent := request.History.Events[len(request.History.Events)-1]
					logger.Infof("Resetting to %v - %v\n.", expectedLastEventID, msBuilder.GetReplicationState().LastWriteEventID)
					return r.resetMutableState(ctx, context, msBuilder, expectedLastEventID,
						lastEvent.GetVersion(), lastEvent.GetTimestamp(), logger)
				}
				return msBuilder, nil
			}
		}

		err = ErrMoreThan2DC
		logError(logger, err.Error(), err)
		return nil, err
	}

	// previousActiveCluster == current cluster
	ri, ok := replicationInfo[previousActiveCluster]
	// this cluster is previously active, we need to check whether the events is applied by remote cluster
	if !ok || rState.LastWriteVersion > ri.GetVersion() {
		logger.Info("Encounter case where events are rejected by remote.")
		// use the last valid version && event ID to do a reset
		lastValidVersion, lastValidEventID := r.getLatestCheckpoint(
			replicationInfo,
			rState.LastReplicationInfo,
		)

		if lastValidVersion == common.EmptyVersion {
			err = ErrImpossibleLocalRemoteMissingReplicationInfo
			logError(logger, err.Error(), err)
			return nil, err
		}
		logger.Info("Reset to latest common checkpoint.")

		// NOTE: this conflict resolution do not handle fast >= 2 failover
		lastEvent := request.History.Events[len(request.History.Events)-1]
		incomingTimestamp := lastEvent.GetTimestamp()
		return r.resetMutableState(ctx, context, msBuilder, lastValidEventID, incomingVersion, incomingTimestamp, logger)
	}
	if rState.LastWriteVersion < ri.GetVersion() {
		err = ErrImpossibleRemoteClaimSeenHigherVersion
		logError(logger, err.Error(), err)
		return nil, err
	}

	// remote replication info last write version is the same as local last write version, check reset
	// Detect conflict
	if ri.GetLastEventId() > rState.LastWriteEventID {
		// if there is any bug in the replication protocol or implementation, this case can happen
		logError(logger, "Conflict detected, but cannot resolve.", ErrCorruptedReplicationInfo)
		// Returning BadRequestError to force the message to land into DLQ
		return nil, ErrCorruptedReplicationInfo
	}

	err = r.flushEventsBuffer(context, msBuilder)
	if err != nil {
		return nil, err
	}

	if ri.GetLastEventId() < msBuilder.GetReplicationState().LastWriteEventID || msBuilder.HasBufferedEvents() {
		// the reason to reset mutable state if mutable state has buffered events
		// is: what buffered event actually do is delay generation of event ID,
		// the actual action of those buffered event are already applied to mutable state.

		logger.Info("Conflict detected.")
		lastEvent := request.History.Events[len(request.History.Events)-1]
		incomingTimestamp := lastEvent.GetTimestamp()
		return r.resetMutableState(ctx, context, msBuilder, ri.GetLastEventId(), incomingVersion, incomingTimestamp, logger)
	}

	// event ID match, no reset
	return msBuilder, nil
}

func (r *historyReplicator) ApplyOtherEvents(ctx context.Context, context workflowExecutionContext,
	msBuilder mutableState, request *h.ReplicateEventsRequest, logger bark.Logger) error {
	var err error
	firstEventID := request.GetFirstEventId()
	if firstEventID < msBuilder.GetNextEventID() {
		// duplicate replication task
		replicationState := msBuilder.GetReplicationState()
		logger.Debugf("Dropping replication task.  State: {NextEvent: %v, Version: %v, LastWriteV: %v, LastWriteEvent: %v}",
			msBuilder.GetNextEventID(), replicationState.CurrentVersion, replicationState.LastWriteVersion, replicationState.LastWriteEventID)
		r.metricsClient.IncCounter(metrics.ReplicateHistoryEventsScope, metrics.DuplicateReplicationEventsCounter)
		return nil
	}
	if firstEventID > msBuilder.GetNextEventID() {

		if !msBuilder.IsWorkflowExecutionRunning() {
			logger.Warnf("Workflow already terminated due to conflict resolution.")
			return nil
		}

		// out of order replication task and store it in the buffer
		logger.Debugf("Buffer out of order replication task.  NextEvent: %v, FirstEvent: %v",
			msBuilder.GetNextEventID(), firstEventID)

		if !request.GetForceBufferEvents() {
			return newRetryTaskErrorWithHint(
				ErrRetryBufferEventsMsg,
				context.getDomainID(),
				context.getExecution().GetWorkflowId(),
				context.getExecution().GetRunId(),
				msBuilder.GetNextEventID(),
			)
		}

		r.metricsClient.RecordTimer(
			metrics.ReplicateHistoryEventsScope,
			metrics.BufferReplicationTaskTimer,
			time.Duration(len(request.History.Events)),
		)

		bt, ok := msBuilder.GetAllBufferedReplicationTasks()[request.GetFirstEventId()]
		if ok && bt.Version >= request.GetVersion() {
			// Have an existing replication task
			return nil
		}

		err = msBuilder.BufferReplicationTask(request)
		if err != nil {
			logError(logger, "Failed to buffer out of order replication task.", err)
			return err
		}
		return r.updateMutableStateOnly(context, msBuilder)
	}

	// Apply the replication task
	err = r.ApplyReplicationTask(ctx, context, msBuilder, request, logger)
	if err != nil {
		logError(logger, "Fail to Apply Replication task.", err)
		return err
	}

	// Flush buffered replication tasks after applying the update
	err = r.flushReplicationBuffer(ctx, context, msBuilder, logger)
	if err != nil {
		logError(logger, "Fail to flush buffer.", err)
	}

	return err
}

func (r *historyReplicator) ApplyReplicationTask(ctx context.Context, context workflowExecutionContext,
	msBuilder mutableState, request *h.ReplicateEventsRequest, logger bark.Logger) error {

	if !msBuilder.IsWorkflowExecutionRunning() {
		logger.Warnf("Workflow already terminated due to conflict resolution.")
		return nil
	}

	domainID, err := validateDomainUUID(request.DomainUUID)
	if err != nil {
		return err
	}
	if len(request.History.Events) == 0 {
		return nil
	}

	execution := *request.WorkflowExecution

	requestID := uuid.New() // requestID used for start workflow execution request.  This is not on the history event.
	sBuilder := r.getNewStateBuilder(msBuilder, logger)
	var newRunHistory []*shared.HistoryEvent
	if request.NewRunHistory != nil {
		newRunHistory = request.NewRunHistory.Events
	}

	// directly use stateBuilder to apply events for other events(including continueAsNew)
	lastEvent, di, newRunStateBuilder, err := sBuilder.applyEvents(domainID, requestID, execution, request.History.Events, newRunHistory, request.GetEventStoreVersion(), request.GetNewRunEventStoreVersion())
	if err != nil {
		return err
	}

	// If replicated events has ContinueAsNew event, then append the new run history
	if newRunStateBuilder != nil {
		// Generate a transaction ID for appending events to history
		transactionID, err := r.shard.GetNextTransferTaskID()
		if err != nil {
			return err
		}
		// contineueAsNew
		err = context.appendFirstBatchHistoryForContinueAsNew(newRunStateBuilder, transactionID)
		if err != nil {
			return err
		}
	}

	firstEvent := request.History.Events[0]
	switch firstEvent.GetEventType() {
	case shared.EventTypeWorkflowExecutionStarted:
		err = r.replicateWorkflowStarted(ctx, context, msBuilder, di, request.GetSourceCluster(), request.History, sBuilder,
			logger)
	default:
		// Generate a transaction ID for appending events to history
		transactionID, err2 := r.shard.GetNextTransferTaskID()
		if err2 != nil {
			return err2
		}
		now := time.Unix(0, lastEvent.GetTimestamp())
		err = context.replicateWorkflowExecution(request, sBuilder.getTransferTasks(), sBuilder.getTimerTasks(), lastEvent.GetEventId(), transactionID, now)
	}

	if err == nil {
		now := time.Unix(0, lastEvent.GetTimestamp())
		r.notify(request.GetSourceCluster(), now, sBuilder.getTransferTasks(), sBuilder.getTimerTasks())
	}

	return err
}

func (r *historyReplicator) flushReplicationBuffer(ctx context.Context, context workflowExecutionContext, msBuilder mutableState,
	logger bark.Logger) error {

	if !msBuilder.IsWorkflowExecutionRunning() {
		return nil
	}

	domainID := msBuilder.GetExecutionInfo().DomainID
	execution := shared.WorkflowExecution{
		WorkflowId: common.StringPtr(msBuilder.GetExecutionInfo().WorkflowID),
		RunId:      common.StringPtr(msBuilder.GetExecutionInfo().RunID),
	}

	flushedCount := 0
	defer func() {
		r.metricsClient.RecordTimer(
			metrics.ReplicateHistoryEventsScope,
			metrics.UnbufferReplicationTaskTimer,
			time.Duration(flushedCount),
		)
	}()

	// remove all stale buffered replication tasks
	for firstEventID, bt := range msBuilder.GetAllBufferedReplicationTasks() {
		if msBuilder.IsWorkflowExecutionRunning() && bt.Version < msBuilder.GetLastWriteVersion() {
			msBuilder.DeleteBufferedReplicationTask(firstEventID)
			applied, err := r.garbageCollectSignals(context, msBuilder, bt.History)
			if err != nil {
				return err
			}
			if !applied {
				err = r.updateMutableStateOnly(context, msBuilder)
				if err != nil {
					return err
				}
			}
		}
	}

	// Keep on applying on applying buffered replication tasks in a loop
	for msBuilder.IsWorkflowExecutionRunning() && msBuilder.HasBufferedReplicationTasks() {
		nextEventID := msBuilder.GetNextEventID()
		bt, ok := msBuilder.GetAllBufferedReplicationTasks()[nextEventID]
		if !ok {
			// Bail out if nextEventID is not in the buffer or version is stale
			return nil
		}

		// We need to delete the task from buffer first to make sure delete update is queued up
		// Applying replication task commits the transaction along with the delete
		msBuilder.DeleteBufferedReplicationTask(nextEventID)
		sourceCluster := r.clusterMetadata.ClusterNameForFailoverVersion(bt.Version)
		req := &h.ReplicateEventsRequest{
			SourceCluster:           common.StringPtr(sourceCluster),
			DomainUUID:              common.StringPtr(domainID),
			WorkflowExecution:       &execution,
			FirstEventId:            common.Int64Ptr(bt.FirstEventID),
			NextEventId:             common.Int64Ptr(bt.NextEventID),
			Version:                 common.Int64Ptr(bt.Version),
			History:                 &workflow.History{Events: bt.History},
			NewRunHistory:           &workflow.History{Events: bt.NewRunHistory},
			EventStoreVersion:       &bt.EventStoreVersion,
			NewRunEventStoreVersion: &bt.NewRunEventStoreVersion,
		}

		// Apply replication task to workflow execution
		if err := r.ApplyReplicationTask(ctx, context, msBuilder, req, logger); err != nil {
			return err
		}
		flushedCount += int(bt.NextEventID - bt.FirstEventID)
	}

	return nil
}

func (r *historyReplicator) replicateWorkflowStarted(ctx context.Context, context workflowExecutionContext,
	msBuilder mutableState, di *decisionInfo,
	sourceCluster string, history *shared.History, sBuilder stateBuilder, logger bark.Logger) error {
	executionInfo := msBuilder.GetExecutionInfo()
	domainID := executionInfo.DomainID
	execution := shared.WorkflowExecution{
		WorkflowId: common.StringPtr(executionInfo.WorkflowID),
		RunId:      common.StringPtr(executionInfo.RunID),
	}
	var parentExecution *shared.WorkflowExecution
	initiatedID := common.EmptyEventID
	parentDomainID := ""
	if executionInfo.ParentDomainID != "" {
		initiatedID = executionInfo.InitiatedID
		parentDomainID = executionInfo.ParentDomainID
		parentExecution = &shared.WorkflowExecution{
			WorkflowId: common.StringPtr(executionInfo.ParentWorkflowID),
			RunId:      common.StringPtr(executionInfo.ParentRunID),
		}
	}
	firstEvent := history.Events[0]
	lastEvent := history.Events[len(history.Events)-1]

	// Generate a transaction ID for appending events to history
	transactionID, err := r.shard.GetNextTransferTaskID()
	if err != nil {
		return err
	}

	var historySize int
	if msBuilder.GetEventStoreVersion() == persistence.EventStoreVersionV2 {
		historySize, err = r.shard.AppendHistoryV2Events(&persistence.AppendHistoryNodesRequest{
			IsNewBranch:   true,
			Info:          historyGarbageCleanupInfo(domainID, execution.GetWorkflowId(), execution.GetRunId()),
			BranchToken:   msBuilder.GetCurrentBranch(),
			Events:        history.Events,
			TransactionID: transactionID,
		}, msBuilder.GetExecutionInfo().DomainID)
	} else {
		historySize, err = r.shard.AppendHistoryEvents(&persistence.AppendHistoryEventsRequest{
			DomainID:          domainID,
			Execution:         execution,
			TransactionID:     transactionID,
			FirstEventID:      firstEvent.GetEventId(),
			EventBatchVersion: firstEvent.GetVersion(),
			Events:            history.Events,
		})
	}

	if err != nil {
		return err
	}

	// TODO this pile of logic should be merge into workflow execution context / mutable state
	executionInfo.SetLastFirstEventID(firstEvent.GetEventId())
	executionInfo.SetNextEventID(lastEvent.GetEventId() + 1)
	incomingVersion := firstEvent.GetVersion()
	msBuilder.UpdateReplicationStateLastEventID(sourceCluster, incomingVersion, lastEvent.GetEventId())
	replicationState := msBuilder.GetReplicationState()

	// Set decision attributes after replication of history events
	decisionVersionID := common.EmptyVersion
	decisionScheduleID := common.EmptyEventID
	decisionStartID := common.EmptyEventID
	decisionTimeout := int32(0)
	if di != nil {
		decisionVersionID = di.Version
		decisionScheduleID = di.ScheduleID
		decisionStartID = di.StartedID
		decisionTimeout = di.DecisionTimeout
	}
	transferTasks := sBuilder.getTransferTasks()
	timerTasks := sBuilder.getTimerTasks()
	setTaskInfo(
		msBuilder.GetCurrentVersion(),
		time.Unix(0, lastEvent.GetTimestamp()),
		transferTasks,
		timerTasks,
	)

	createWorkflow := func(isBrandNew bool, prevRunID string, prevLastWriteVersion int64) error {
		createRequest := &persistence.CreateWorkflowExecutionRequest{
			// NOTE: should not set the replication task, since we are in the standby
			RequestID:                   executionInfo.CreateRequestID,
			DomainID:                    domainID,
			Execution:                   execution,
			ParentDomainID:              parentDomainID,
			ParentExecution:             parentExecution,
			InitiatedID:                 initiatedID,
			TaskList:                    executionInfo.TaskList,
			WorkflowTypeName:            executionInfo.WorkflowTypeName,
			WorkflowTimeout:             executionInfo.WorkflowTimeout,
			DecisionTimeoutValue:        executionInfo.DecisionTimeoutValue,
			ExecutionContext:            nil,
			NextEventID:                 msBuilder.GetNextEventID(),
			LastProcessedEvent:          common.EmptyEventID,
			HistorySize:                 int64(historySize),
			TransferTasks:               transferTasks,
			DecisionVersion:             decisionVersionID,
			DecisionScheduleID:          decisionScheduleID,
			DecisionStartedID:           decisionStartID,
			DecisionStartToCloseTimeout: decisionTimeout,
			TimerTasks:                  timerTasks,
			PreviousRunID:               prevRunID,
			PreviousLastWriteVersion:    prevLastWriteVersion,
			ReplicationState:            replicationState,
			EventStoreVersion:           msBuilder.GetEventStoreVersion(),
			BranchToken:                 msBuilder.GetCurrentBranch(),
		}
		createRequest.CreateWorkflowMode = persistence.CreateWorkflowModeBrandNew
		if !isBrandNew {
			createRequest.CreateWorkflowMode = persistence.CreateWorkflowModeWorkflowIDReuse
		}
		_, err = r.shard.CreateWorkflowExecution(createRequest)
		return err
	}
	deleteHistory := func() {
		// this function should be only called when we drop start workflow execution
		if msBuilder.GetEventStoreVersion() == persistence.EventStoreVersionV2 {
			r.shard.GetHistoryV2Manager().DeleteHistoryBranch(&persistence.DeleteHistoryBranchRequest{
				BranchToken: msBuilder.GetCurrentBranch(),
			})
		} else {
			r.shard.GetHistoryManager().DeleteWorkflowExecutionHistory(&persistence.DeleteWorkflowExecutionHistoryRequest{
				DomainID:  domainID,
				Execution: execution,
			})
		}

	}

	// try to create the workflow execution
	isBrandNew := true
	err = createWorkflow(isBrandNew, "", 0)
	if err == nil {
		return nil
	}
	if _, ok := err.(*persistence.WorkflowExecutionAlreadyStartedError); !ok {
		logger.WithField(logging.TagErr, err).Info("Create workflow failed after appending history events.")
		return err
	}

	// we have WorkflowExecutionAlreadyStartedError
	errExist := err.(*persistence.WorkflowExecutionAlreadyStartedError)
	currentRunID := errExist.RunID
	currentState := errExist.State
	currentLastWriteVersion := errExist.LastWriteVersion

	logger.WithField(logging.TagCurrentVersion, currentLastWriteVersion)
	if currentRunID == execution.GetRunId() {
		logger.Info("Dropping stale start replication task.")
		r.metricsClient.IncCounter(metrics.ReplicateHistoryEventsScope, metrics.DuplicateReplicationEventsCounter)
		return nil
	}

	// current workflow is completed
	if currentState == persistence.WorkflowStateCompleted {
		// allow the application of worrkflow creation if currentLastWriteVersion > incomingVersion
		// because this can be caused by missing replication events
		// proceed to create workflow
		isBrandNew = false
		return createWorkflow(isBrandNew, currentRunID, currentLastWriteVersion)
	}

	// current workflow is still running
	if currentLastWriteVersion > incomingVersion {
		logger.Info("Dropping stale start replication task.")
		r.metricsClient.IncCounter(metrics.ReplicateHistoryEventsScope, metrics.StaleReplicationEventsCounter)
		deleteHistory()
		return nil
	}
	if currentLastWriteVersion == incomingVersion {
		currentRunID, currentNextEventID, _, err := r.flushCurrentWorkflowBuffer(ctx, domainID, execution.GetWorkflowId(), logger)
		if err != nil {
			return err
		}
		return newRetryTaskErrorWithHint(ErrRetryExistingWorkflowMsg, domainID, execution.GetWorkflowId(), currentRunID, currentNextEventID)
	}

	// currentStartVersion < incomingVersion && current workflow still running
	// this can happen during the failover; since we have no idea
	// whether the remote active cluster is aware of the current running workflow,
	// the only thing we can do is to terminate the current workflow and
	// start the new workflow from the request

	// same workflow ID, same shard
	incomingTimestamp := lastEvent.GetTimestamp()
	err = r.terminateWorkflow(ctx, domainID, executionInfo.WorkflowID, currentRunID, incomingVersion, incomingTimestamp, logger)
	if err != nil {
		if _, ok := err.(*shared.EntityNotExistsError); !ok {
			return err
		}
		// if workflow is completed just when the call is made, will get EntityNotExistsError
		// we are not sure whether the workflow to be terminated ends with continue as new or not
		// so when encounter EntityNotExistsError, just contiue to execute, if err occurs,
		// there will be retry on the worker level
	}
	isBrandNew = false
	return createWorkflow(isBrandNew, currentRunID, incomingVersion)
}

func (r *historyReplicator) flushCurrentWorkflowBuffer(ctx context.Context, domainID string, workflowID string,
	logger bark.Logger) (string, int64, bool, error) {
	currentContext, currentMutableState, currentRelease, err := r.getCurrentWorkflowMutableState(ctx, domainID,
		workflowID)
	if err != nil {
		return "", 0, false, err
	}
	// since this new workflow cannot make progress due to existing workflow being open
	// try flush the existing workflow's buffer see if we can make it move forward
	// First check if there are events which needs to be flushed before applying the update
	err = r.flushReplicationBuffer(ctx, currentContext, currentMutableState, logger)
	currentRelease(err)
	if err != nil {
		logError(logger, "Fail to flush buffer for current workflow.", err)
		return "", 0, false, err
	}
	return currentContext.getExecution().GetRunId(), currentMutableState.GetNextEventID(), currentMutableState.IsWorkflowExecutionRunning(), nil
}

func (r *historyReplicator) conflictResolutionTerminateCurrentRunningIfNotSelf(ctx context.Context,
	msBuilder mutableState, incomingVersion int64, incomingTimestamp int64, logger bark.Logger) (currentRunID string, retError error) {
	// this function aims to solve the edge case when this workflow, when going through
	// reset, has already started a next generation (continue as new-ed workflow)

	if msBuilder.IsWorkflowExecutionRunning() {
		// workflow still running, no continued as new edge case to solve
		logger.Info("Conflict resolution self workflow running, skip.")
		return msBuilder.GetExecutionInfo().RunID, nil
	}

	// terminate the current running workflow
	// cannot use history cache to get current workflow since there can be deadlock
	domainID := msBuilder.GetExecutionInfo().DomainID
	workflowID := msBuilder.GetExecutionInfo().WorkflowID
	resp, err := r.shard.GetExecutionManager().GetCurrentExecution(&persistence.GetCurrentExecutionRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
	})
	if err != nil {
		logError(logger, "Conflict resolution error getting current workflow.", err)
		return "", err
	}
	currentRunID = resp.RunID
	currentCloseStatus := resp.CloseStatus

	if currentCloseStatus != persistence.WorkflowCloseStatusNone {
		// current workflow finished
		// note, it is impossible that a current workflow ends with continue as new as close status
		logger.Info("Conflict resolution current workflow finished.")
		return currentRunID, nil
	}

	// need to terminate the current workflow
	// same workflow ID, same shard
	err = r.terminateWorkflow(ctx, domainID, workflowID, currentRunID, incomingVersion, incomingTimestamp, logger)
	if err != nil {
		logError(logger, "Conflict resolution err terminating current workflow.", err)
	}
	return currentRunID, err
}

// func (r *historyReplicator) getCurrentWorkflowInfo(domainID string, workflowID string) (runID string, lastWriteVersion int64, closeStatus int, retError error) {
func (r *historyReplicator) getCurrentWorkflowMutableState(ctx context.Context, domainID string,
	workflowID string) (workflowExecutionContext, mutableState, releaseWorkflowExecutionFunc, error) {
	// we need to check the current workflow execution
	context, release, err := r.historyCache.getOrCreateWorkflowExecutionWithTimeout(ctx,
		domainID,
		// only use the workflow ID, to get the current running one
		shared.WorkflowExecution{WorkflowId: common.StringPtr(workflowID)},
	)
	if err != nil {
		return nil, nil, nil, err
	}

	msBuilder, err := context.loadWorkflowExecution()
	if err != nil {
		// no matter what error happen, we need to retry
		release(err)
		return nil, nil, nil, err
	}
	return context, msBuilder, release, nil
}

func (r *historyReplicator) terminateWorkflow(ctx context.Context, domainID string, workflowID string,
	runID string, incomingVersion int64, incomingTimestamp int64, logger bark.Logger) (retError error) {

	execution := shared.WorkflowExecution{
		WorkflowId: common.StringPtr(workflowID),
		RunId:      common.StringPtr(runID),
	}
	context, release, err := r.historyCache.getOrCreateWorkflowExecutionWithTimeout(ctx, domainID, execution)
	if err != nil {
		return err
	}
	defer func() { release(retError) }()

	msBuilder, err := context.loadWorkflowExecution()
	if err != nil {
		return err
	}
	if !msBuilder.IsWorkflowExecutionRunning() {
		return nil
	}

	nextEventID := msBuilder.GetNextEventID()
	sourceCluster := r.clusterMetadata.ClusterNameForFailoverVersion(incomingVersion)
	terminationEvent := &shared.HistoryEvent{
		EventId:   common.Int64Ptr(nextEventID),
		Timestamp: common.Int64Ptr(incomingTimestamp),
		Version:   common.Int64Ptr(incomingVersion),
		EventType: shared.EventTypeWorkflowExecutionTerminated.Ptr(),
		WorkflowExecutionTerminatedEventAttributes: &shared.WorkflowExecutionTerminatedEventAttributes{
			Reason:   common.StringPtr(workflowTerminationReason),
			Identity: common.StringPtr(workflowTerminationIdentity),
			Details:  nil,
		},
	}
	history := &shared.History{Events: []*shared.HistoryEvent{terminationEvent}}

	req := &h.ReplicateEventsRequest{
		SourceCluster:     common.StringPtr(sourceCluster),
		DomainUUID:        common.StringPtr(domainID),
		WorkflowExecution: &execution,
		FirstEventId:      common.Int64Ptr(nextEventID),
		NextEventId:       common.Int64Ptr(nextEventID + 1),
		Version:           common.Int64Ptr(incomingVersion),
		History:           history,
		NewRunHistory:     nil,
	}
	return r.ApplyReplicationTask(ctx, context, msBuilder, req, logger)
}

func (r *historyReplicator) getLatestCheckpoint(replicationInfoRemote map[string]*workflow.ReplicationInfo,
	replicationInfoLocal map[string]*persistence.ReplicationInfo) (int64, int64) {

	// this only applies to 2 data center case

	lastValidVersion := common.EmptyVersion
	lastValidEventID := common.EmptyEventID

	for _, ri := range replicationInfoRemote {
		if lastValidVersion == common.EmptyVersion || ri.GetVersion() > lastValidVersion {
			lastValidVersion = ri.GetVersion()
			lastValidEventID = ri.GetLastEventId()
		}
	}

	for _, ri := range replicationInfoLocal {
		if lastValidVersion == common.EmptyVersion || ri.Version > lastValidVersion {
			lastValidVersion = ri.Version
			lastValidEventID = ri.LastEventID
		}
	}

	return lastValidVersion, lastValidEventID
}

func (r *historyReplicator) resetMutableState(ctx context.Context, context workflowExecutionContext,
	msBuilder mutableState, lastEventID int64, incomingVersion int64, incomingTimestamp int64, logger bark.Logger) (mutableState, error) {

	r.metricsClient.IncCounter(metrics.ReplicateHistoryEventsScope, metrics.HistoryConflictsCounter)

	// handling edge case when resetting a workflow, and this workflow has done continue as new
	// we need to terminate the continue as new-ed workflow
	currentRunID, err := r.conflictResolutionTerminateCurrentRunningIfNotSelf(ctx, msBuilder, incomingVersion, incomingTimestamp, logger)
	if err != nil {
		return nil, err
	}

	resolver := r.getNewConflictResolver(context, logger)
	msBuilder, err = resolver.reset(currentRunID, uuid.New(), lastEventID, msBuilder.GetExecutionInfo())
	logger.Info("Completed Resetting of workflow execution.")
	if err != nil {
		return nil, err
	}
	return msBuilder, nil
}

func (r *historyReplicator) updateMutableStateOnly(context workflowExecutionContext, msBuilder mutableState) error {
	return r.updateMutableStateWithTimer(context, msBuilder, time.Time{}, nil)
}

func (r *historyReplicator) updateMutableStateWithTimer(context workflowExecutionContext, msBuilder mutableState, now time.Time, timerTasks []persistence.Task) error {
	// Generate a transaction ID for appending events to history
	transactionID, err := r.shard.GetNextTransferTaskID()
	if err != nil {
		return err
	}
	// we need to handcraft some of the variables
	// since this is a persisting the buffer replication task,
	// so nothing on the replication state should be changed
	lastWriteVersion := msBuilder.GetLastWriteVersion()
	sourceCluster := r.clusterMetadata.ClusterNameForFailoverVersion(lastWriteVersion)
	return context.updateHelper(nil, timerTasks, transactionID, now, false, nil, sourceCluster)
}

func (r *historyReplicator) notify(clusterName string, now time.Time, transferTasks []persistence.Task,
	timerTasks []persistence.Task) {
	now = now.Add(-r.shard.GetConfig().StandbyClusterDelay())
	r.shard.SetCurrentTime(clusterName, now)
	r.historyEngine.txProcessor.NotifyNewTask(clusterName, transferTasks)
	r.historyEngine.timerProcessor.NotifyNewTimers(clusterName, now, timerTasks)
}

func (r *historyReplicator) deserializeBlob(blob *workflow.DataBlob) ([]*workflow.HistoryEvent, error) {

	if blob.GetEncodingType() != workflow.EncodingTypeThriftRW {
		return nil, ErrUnknownEncodingType
	}
	historyEvents, err := r.historySerializer.DeserializeBatchEvents(&persistence.DataBlob{
		Encoding: common.EncodingTypeThriftRW,
		Data:     blob.Data,
	})
	if err != nil {
		return nil, err
	}
	if len(historyEvents) == 0 {
		return nil, ErrEmptyHistoryRawEventBatch
	}
	return historyEvents, nil
}

func (r *historyReplicator) flushEventsBuffer(context workflowExecutionContext, msBuilder mutableState) error {

	if !msBuilder.IsWorkflowExecutionRunning() || !msBuilder.HasBufferedEvents() || !r.canModifyWorkflow(msBuilder) {
		return nil
	}

	di, ok := msBuilder.GetInFlightDecisionTask()
	if !ok {
		return ErrCorruptedMutableStateDecision
	}
	msBuilder.UpdateReplicationStateVersion(msBuilder.GetLastWriteVersion(), true)
	msBuilder.AddDecisionTaskFailedEvent(di.ScheduleID, di.StartedID,
		workflow.DecisionTaskFailedCauseFailoverCloseDecision, nil, identityHistoryService, "", "", "", 0)

	// there is no need to generate a new decision and corresponding decision timer task
	// here, the intent is to flush the buffered events

	transactionID, err := r.shard.GetNextTransferTaskID()
	if err != nil {
		return err
	}
	return context.updateWorkflowExecution(nil, nil, transactionID)
}

func (r *historyReplicator) garbageCollectSignals(context workflowExecutionContext,
	msBuilder mutableState, events []*workflow.HistoryEvent) (bool, error) {

	// this function modify the mutable state passed in applying stale signals
	// so the check of workflow still running and the ability to modify this workflow
	// is utterly necessary
	if !msBuilder.IsWorkflowExecutionRunning() || !r.canModifyWorkflow(msBuilder) {
		return false, nil
	}

	// we are garbage collecting signals already applied to mutable states,
	// so targeting child workflow only check is not necessary

	// TODO should we also include the request ID in the signal request in the event?
	updateMutableState := false
	msBuilder.UpdateReplicationStateVersion(msBuilder.GetLastWriteVersion(), true)
	for _, event := range events {
		switch event.GetEventType() {
		case workflow.EventTypeWorkflowExecutionSignaled:
			updateMutableState = true
			attr := event.WorkflowExecutionSignaledEventAttributes
			if msBuilder.AddWorkflowExecutionSignaled(attr.GetSignalName(), attr.Input, attr.GetIdentity()) == nil {
				return false, &workflow.InternalServiceError{Message: "Unable to signal workflow execution."}
			}
		}
	}

	if !updateMutableState {
		return false, nil
	}

	transactionID, err := r.shard.GetNextTransferTaskID()
	if err != nil {
		return false, err
	}
	return true, context.updateWorkflowExecution(nil, nil, transactionID)
}

func (r *historyReplicator) canModifyWorkflow(msBuilder mutableState) bool {
	lastWriteVersion := msBuilder.GetLastWriteVersion()
	return r.clusterMetadata.ClusterNameForFailoverVersion(lastWriteVersion) == r.clusterMetadata.GetCurrentClusterName()
}

func logError(logger bark.Logger, msg string, err error) {
	logger.WithFields(bark.Fields{
		logging.TagErr: err,
	}).Error(msg)
}

func newRetryTaskErrorWithHint(msg string, domainID string, workflowID string, runID string, nextEventID int64) *shared.RetryTaskError {
	return &shared.RetryTaskError{
		Message:     msg,
		DomainId:    common.StringPtr(domainID),
		WorkflowId:  common.StringPtr(workflowID),
		RunId:       common.StringPtr(runID),
		NextEventId: common.Int64Ptr(nextEventID),
	}
}

func (r *historyReplicator) canDoDCMigration(domainID string) (bool, error) {
	domainEntry, err := r.domainCache.GetDomainByID(domainID)
	if err != nil {
		return false, err
	}

	doDCMigration := true
	for _, targetCluster := range domainEntry.GetReplicationConfig().Clusters {
		if targetCluster.ClusterName == r.clusterMetadata.GetCurrentClusterName() {
			// if target cluster contains current cluster,
			// then do not do dc migration
			doDCMigration = false
		}
	}
	return doDCMigration, nil
}
