package tasks

import (
  "context"
  "errors"
  "sync"
  "time"
)

type Task struct {
  ID        string                 `json:"id"`
  Agent     string                 `json:"agent"`
  Op        string                 `json:"op"`
  Payload   map[string]any         `json:"payload"`
  CreatedAt time.Time              `json:"created_at"`
}

type Result struct {
  TaskID   string                 `json:"task_id"`
  Success  bool                   `json:"success"`
  Data     map[string]any         `json:"data,omitempty"`
  Error    string                 `json:"error,omitempty"`
  Finished time.Time              `json:"finished"`
}

type Queue struct {
  ch    chan Task
  wg    sync.WaitGroup
  mu    sync.RWMutex
  store map[string]Result
}

func NewQueue(capacity int) *Queue {
  return &Queue{ ch: make(chan Task, capacity), store: make(map[string]Result) }
}

func (q *Queue) Enqueue(t Task) error {
  if t.ID == "" { return errors.New("task id required") }
  select {
  case q.ch <- t:
    return nil
  default:
    return errors.New("queue full")
  }
}

func (q *Queue) Results(taskID string) (Result, bool) {
  q.mu.RLock(); defer q.mu.RUnlock()
  r, ok := q.store[taskID]; return r, ok
}

func (q *Queue) setResult(r Result) { q.mu.Lock(); q.store[r.TaskID] = r; q.mu.Unlock() }

func (q *Queue) Start(ctx context.Context, workers int, runner func(context.Context, Task) Result) {
  if workers < 1 { workers = 1 }
  for i := 0; i < workers; i++ {
    q.wg.Add(1)
    go func() {
      defer q.wg.Done()
      for {
        select {
        case <-ctx.Done():
          return
        case t := <-q.ch:
          res := runner(ctx, t)
          q.setResult(res)
        }
      }
    }()
  }
}

func (q *Queue) Stop() { close(q.ch); q.wg.Wait() }
