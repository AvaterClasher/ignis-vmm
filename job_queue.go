// job_queue.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	log "github.com/sirupsen/logrus"
)

// natsMsg wraps the payload from NATS
type natsMsg struct {
	Body []byte
}

// jobQueue handles incoming jobs and publishes status via NATS
type jobQueue struct {
	nc   *nats.Conn
	jobs <-chan natsMsg
}

type jobStatus struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	Error        string `json:"error"`
	StdErr       string `json:"stderr"`
	StdOut       string `json:"stdout"`
	ExecDuration int    `json:"exec_duration"`
	MemUsage     int64  `json:"mem_usage"`
}

// newJobQueue connects to NATS and subscribes to the 'jobs' subject
func newJobQueue(endpoint string) jobQueue {
	nc, err := nats.Connect(endpoint, nats.MaxReconnects(-1), nats.ReconnectWait(2*time.Second))
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to NATS")
	}
	// Channel for incoming jobs
	jobsCh := make(chan natsMsg, 64)
	// Subscribe to job submissions
	_, err = nc.Subscribe("jobs", func(m *nats.Msg) {
		jobsCh <- natsMsg{Body: m.Data}
	})
	if err != nil {
		log.WithError(err).Fatal("Failed to subscribe to 'jobs' subject")
	}
	return jobQueue{nc: nc, jobs: jobsCh}
}

// getQueueForJob is a no-op for NATS
func (q jobQueue) getQueueForJob(ctx context.Context) error {
	return nil
}

// setjobStatus publishes a structured status message for a job
func (q jobQueue) setjobStatus(ctx context.Context, job benchJob, status string, res agentExecRes) error {
	js := &jobStatus{
		ID:           job.ID,
		Status:       status,
		Message:      res.Message,
		Error:        res.Error,
		StdErr:       res.StdErr,
		StdOut:       res.StdOut,
		ExecDuration: res.ExecDuration,
		MemUsage:     res.MemUsage,
	}
	b, err := json.Marshal(js)
	if err != nil {
		return err
	}
	subj := fmt.Sprintf("job_status.%s", job.ID)
	return q.nc.Publish(subj, b)
}

// setjobReceived marks a job as received
func (q jobQueue) setjobReceived(ctx context.Context, job benchJob) error {
	return q.setjobStatus(ctx, job, "received", agentExecRes{})
}

// setjobRunning marks a job as running
func (q jobQueue) setjobRunning(ctx context.Context, job benchJob) error {
	return q.setjobStatus(ctx, job, "running", agentExecRes{})
}

// setjobFailed marks a job as failed
func (q jobQueue) setjobFailed(ctx context.Context, job benchJob, res agentExecRes) error {
	return q.setjobStatus(ctx, job, "failed", res)
}

// setjobResult publishes the final result of a job
func (q jobQueue) setjobResult(ctx context.Context, job benchJob, res agentExecRes) error {
	return q.setjobStatus(ctx, job, "done", res)
}
