package server

import (
	"context"
	"errors"
	"testing"
)

func TestStartRunsToDone(t *testing.T) {
	j := NewJobs()
	job, err := j.Start("deploy", "web", "v1", func(context.Context) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != Queued {
		t.Fatalf("new job should be queued: %+v", job)
	}
	j.Wait()
	got, ok := j.Get(job.ID)
	if !ok || got.Status != Done || got.Err != "" {
		t.Fatalf("job should be done cleanly: %+v", got)
	}
	if got.FinishedAt.IsZero() {
		t.Fatal("finished at should be set")
	}
}

func TestStartRecordsFailure(t *testing.T) {
	j := NewJobs()
	job, _ := j.Start("deploy", "web", "v9", func(context.Context) error { return errors.New("boom") })
	j.Wait()
	got, _ := j.Get(job.ID)
	if got.Status != Failed || got.Err != "boom" {
		t.Fatalf("failure not recorded: %+v", got)
	}
}

func TestStartSerializesPerStack(t *testing.T) {
	j := NewJobs()
	release := make(chan struct{})
	started := make(chan struct{})
	first, err := j.Start("deploy", "web", "v1", func(context.Context) error {
		close(started)
		<-release
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started // first job is now running and blocking

	if _, err := j.Start("deploy", "web", "v2", func(context.Context) error { return nil }); !errors.Is(err, ErrStackBusy) {
		t.Fatalf("want ErrStackBusy, got %v", err)
	}
	// a different stack is unaffected
	other, err := j.Start("deploy", "rue", "v1", func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("different stack should start: %v", err)
	}
	close(release)
	j.Wait()

	got, _ := j.Get(first.ID)
	if got.Status != Done {
		t.Fatalf("first job should finish after release: %+v", got)
	}
	// the slot is freed after completion
	again, err := j.Start("deploy", "web", "v3", func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("stack slot should be free: %v", err)
	}
	j.Wait()
	if g, _ := j.Get(again.ID); g.Status != Done {
		t.Fatalf("re-run should complete: %+v", g)
	}
	_ = other
}

func TestStartDifferentStacksRunConcurrently(t *testing.T) {
	j := NewJobs()
	const n = 4
	entered := make(chan string, n)
	hold := make(chan struct{})
	for i := 0; i < n; i++ {
		stack := string(rune('a' + i))
		if _, err := j.Start("deploy", stack, "v1", func(context.Context) error {
			entered <- stack
			<-hold
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < n; i++ {
		<-entered
	}
	close(hold)
	j.Wait()
}

func TestGetUnknown(t *testing.T) {
	j := NewJobs()
	if _, ok := j.Get("nope"); ok {
		t.Fatal("unknown job id should not be found")
	}
}

func TestJobIDsUnique(t *testing.T) {
	j := NewJobs()
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		stack := "stack-" + string(rune('a'+i%4)) + string(rune('0'+i%7)) // distinct stacks: no serialization
		job, err := j.Start("deploy", stack, "v1", func(context.Context) error { return nil })
		if err != nil {
			t.Fatal(err)
		}
		if seen[job.ID] {
			t.Fatalf("duplicate job id %s", job.ID)
		}
		seen[job.ID] = true
	}
	j.Wait()
}
