package core

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"rictusd/internal/agents"
	"rictusd/internal/tasks"
)

// Controller orchestrates queue workers, agents, and task execution.
type Controller struct {
	cfg       *Config
	queue     *tasks.Queue
	agents    []agents.Agent
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	started   bool
	startTime time.Time
}

// NewController creates and initializes a controller instance.
func NewController(cfg *Config) *Controller {
	return &Controller{cfg: cfg}
}

// Start begins queue processing and agent rebuild.
func (c *Controller) Start(ctx context.Context) error {
	if c.started {
		return errors.New("controller already started")
	}

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.startTime = time.Now()

	// (Re)build agent registry
	c.rebuildAgentsLocked()

	// Start task queue
	c.queue = tasks.NewQueue(c.cfg.MaxWorkers, c.runner)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.queue.Run(ctx)
	}()

	c.started = true
	log.Printf("[core] controller started with %d workers", c.cfg.MaxWorkers)
	return nil
}

// Stop signals shutdown and waits for all workers.
func (c *Controller) Stop() {
	if !c.started {
		return
	}
	log.Println("[core] stopping controller")
	c.cancel()
	c.wg.Wait()
	c.started = false
	log.Println("[core] controller stopped")
}

// rebuildAgentsLocked initializes agent registry.
func (c *Controller) rebuildAgentsLocked() {
	c.agents = nil

	// Always include echo for test and diagnostics
	c.agents = append(c.agents, agents.EchoAgent{})

	// Optional Digit integration
	if c.cfg.Digit.Enabled {
		c.agents = append(c.agents, agents.NewDigitAgent(c.cfg.Digit))
	}

	// Add Go build agent
	c.agents = append(c.agents, agents.GoBuildAgent{})

	log.Printf("[core] %d agents registered", len(c.agents))
}

// findAgentByName returns the registered agent by name.
func (c *Controller) findAgentByName(name string) agents.Agent {
	for _, a := range c.agents {
		if strings.EqualFold(a.Name(), name) {
			return a
		}
	}
	return nil
}

// runner executes a single task pulled from the queue.
func (c *Controller) runner(ctx context.Context, t *tasks.Task) {
	log.Printf("[runner] handling task %s (agent=%s op=%s)", t.ID, t.Agent, t.Op)

	// Fallback: route by language if no agent set
	if strings.TrimSpace(t.Agent) == "" {
		if lang, ok := t.Payload["language"].(string); ok && strings.EqualFold(lang, "go") {
			t.Agent = "go"
		}
	}

	ag := c.findAgentByName(t.Agent)
	if ag == nil {
		log.Printf("[runner] unknown agent '%s' for task %s", t.Agent, t.ID)
		_ = c.queue.MarkFailed(t.ID, fmt.Sprintf("unknown agent '%s'", t.Agent))
		return
	}

	start := time.Now()
	resMap, err := ag.Run(ctx, t.Op, t.Payload)
	if err != nil {
		log.Printf("[runner] agent '%s' error: %v", t.Agent, err)
		_ = c.queue.MarkError(t.ID, err.Error())
		return
	}

	// Normalize result
	status, _ := resMap["status"].(string)
	if status == "" {
		status = "SUCCESS"
	}
	duration := time.Since(start)

	report := tasks.Result{
		ID:        t.ID,
		Agent:     t.Agent,
		StartedAt: start,
		EndedAt:   start.Add(duration),
		Status:    status,
		Output:    resMap,
	}

	if err := c.queue.MarkDone(t.ID, &report); err != nil {
		log.Printf("[runner] failed to mark done for %s: %v", t.ID, err)
	}

	log.Printf("[runner] task %s completed (status=%s, duration=%s)", t.ID, status, duration.Round(time.Millisecond))
}

// Stats returns controller runtime stats.
func (c *Controller) Stats() map[string]any {
	uptime := time.Since(c.startTime).Round(time.Second)
	return map[string]any{
		"started":  c.started,
		"uptime":   uptime.String(),
		"agents":   len(c.agents),
		"workers":  c.cfg.MaxWorkers,
		"capacity": c.cfg.QueueCapacity,
	}
}

// Reload triggers agent rebuild and queue restart.
func (c *Controller) Reload() {
	log.Println("[core] reloading agents")
	c.rebuildAgentsLocked()
	log.Println("[core] reload complete")
}

