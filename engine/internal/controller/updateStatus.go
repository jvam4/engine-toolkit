package controller

import (
	"context"
	"github.com/antihax/optional"
	controllerClient "github.com/veritone/realtime/modules/controller/client"

	"fmt"
	"log"
	"time"
)

/**
TODO update Status work

Theoretically the curEngineStatus has a TaskStatus that should be updated on another worker thread
the rest is just housekeeping
*/
func (c *ControllerUniverse) UpdateEngineInstanceStatus(ctx context.Context) {
	method := fmt.Sprintf("[UpdateEngineInstanceStatus:%s]", c.engineInstanceId)
	// update status every N seconds
	updateStatusTimer := time.NewTimer(c.controllerConfig.updateStatusDuration)
	for {
		select {
		case <-ctx.Done():
			return
		case <-updateStatusTimer.C:
			c.batchLock.Lock()
			now := time.Now().Unix()
			if c.curTaskStatusUpdatesForTheBatch != nil {
				for i := 0; i < len(c.curTaskStatusUpdatesForTheBatch); i++ {
					c.curTaskStatusUpdatesForTheBatch[i].PriorTimestamp = c.priorTimestamp
					c.curTaskStatusUpdatesForTheBatch[i].Timestamp = now
				}
			}
			curEngineInstanceStatus := controllerClient.EngineInstanceStatus{
				WorkRequestId:      c.curWorkRequestId,
				WorkRequestStatus:  c.curWorkRequestStatus,
				WorkRequestDetails: c.curWorkRequestDetails,
				Mode:               c.curEngineMode,
				SecondsToTTL:       c.engineInstanceRegistrationInfo.RuntimeExpirationSeconds - int32(now-c.universeStartTime),
				HostId:             c.engineInstanceInfo.HostId,
				ContainerStatus:    c.curContainerStatus,
				TaskStatuses:       c.curTaskStatusUpdatesForTheBatch,
				PriorTimestamp:     c.priorTimestamp,
				Timestamp:          now,
			}
			headerOpts := &controllerClient.UpdateEngineInstanceStatusOpts{
				XCorrelationId: optional.NewInterface(c.correlationId),
			}

			_, err := c.controllerAPIClient.EngineApi.UpdateEngineInstanceStatus(
				context.WithValue(ctx, controllerClient.ContextAccessToken,
					c.engineInstanceRegistrationInfo.EngineInstanceToken),
				c.engineInstanceId,
				curEngineInstanceStatus, headerOpts)
			if err != nil {
				// TODO error handling
				log.Printf("%s Got error calling UpdaetEngineInstanceStatus Controller API, err=%v", method, err)
			} else {
				// reset timestamps, processed cout
				c.priorTimestamp = curEngineInstanceStatus.Timestamp
				// need to clear out the task status!!
				if c.curTaskStatusUpdatesForTheBatch != nil {
					for i := 0; i < len(c.curTaskStatusUpdatesForTheBatch); i++ {
						c.curTaskStatusUpdatesForTheBatch[i].PriorTimestamp = c.priorTimestamp
						// for the input and output, reset the processedCount
						for ii := 0; ii < len(c.curTaskStatusUpdatesForTheBatch[i].Inputs); ii++ {
							c.curTaskStatusUpdatesForTheBatch[i].Inputs[ii].ProcessedCount = 0
						}
						for ii := 0; ii < len(c.curTaskStatusUpdatesForTheBatch[i].Outputs); ii++ {
							c.curTaskStatusUpdatesForTheBatch[i].Outputs[ii].ProcessedCount = 0
						}
					}
				}
			}
			c.batchLock.Unlock()
			updateStatusTimer = time.NewTimer(c.controllerConfig.updateStatusDuration)
		}
	}
}
