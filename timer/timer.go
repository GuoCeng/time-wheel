package timer

import (
	"context"
	"sync"
	"time"

	"github.com/GuoCeng/time-wheel/queue"
)

type Timer interface {

	/**
	 * Add a new task to this executor. It will be executed after the task's delay
	 * (beginning from the time of submission)
	 * @param timerTask the task to add
	 */
	Add(timerTask *Task)

	/**
	   * Advance the internal clock, executing any tasks whose expiration has been
	  *          * reached within the duration of the passed timeout.
	  *          * @param timeoutMs
	  *          * @return whether or not any tasks were executed
	*/
	AdvanceClock(timeoutMs time.Duration) bool

	/**
	 * Get the number of tasks pending execution
	 * @return the number of tasks
	 */
	Size() int64

	/**
	 * Shutdown the timer service, leaving pending tasks unexecuted
	 */
	Shutdown()
}

func NewSystemTimer(tickMs time.Duration, wheelSize int) *SystemTimer {
	startMs := time.Duration(time.Now().Nanosecond())
	q := queue.NewDelay()
	return &SystemTimer{
		tickMs:      tickMs,
		wheelSize:   wheelSize,
		startMs:     startMs,
		delayQueue:  q,
		taskCounter: 0,
		timingWheel: NewTimingWheel(tickMs, wheelSize, startMs, 0, q, 0),
	}
}

type SystemTimer struct {
	mu          sync.RWMutex
	tickMs      time.Duration
	wheelSize   int
	startMs     time.Duration
	delayQueue  *queue.DelayQueue
	taskCounter int64
	timingWheel *TimingWheel
}

func (t *SystemTimer) Add(task *Task) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	t.addTimerTaskEntry(NewTaskEntry(task, task.delayMs+time.Duration(time.Now().Nanosecond())))
}

func (t *SystemTimer) addTimerTaskEntry(taskEntry *TaskEntry) {
	if !t.timingWheel.add(taskEntry) {
		// Already expired or cancelled
		if !taskEntry.cancelled() {
			go func() {
				taskEntry.task.run()
			}()
		}
	}
}

// Advances the clock if there is an expired bucket. If there isn't any expired bucket when called,
// waits up to timeoutMs before giving up.
func (t *SystemTimer) AdvanceClock(ctx context.Context) bool {
	bucket := t.delayQueue.Pop(ctx)
	if bucket != nil {
		if v, ok := bucket.(*TaskList); ok {
			t.mu.Lock()
			defer t.mu.Unlock()
			for v != nil {
				t.timingWheel.advanceClock(v.GetDelay())
				v.flush(func(e *TaskEntry) {
					go func() {
						e.task.run()
					}()
				})
				x := t.delayQueue.Pop(ctx)
				if x != nil {
					v = x.(*TaskList)
				} else {
					v = nil
				}
			}
			return true
		} else {
			panic("did not found TaskList")
		}
	}
	return false
}

func (t *SystemTimer) Size() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.taskCounter
}

func (t *SystemTimer) Shutdown() {

}