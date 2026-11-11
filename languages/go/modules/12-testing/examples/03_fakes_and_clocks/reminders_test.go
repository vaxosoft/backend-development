package reminders

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"
	"time"
)

// Fakes: tiny, hand-written, and in the test file because only the tests need them.

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time          { return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

type fakeStore struct {
	tasks []Task
	err   error
}

func (s fakeStore) List(context.Context) ([]Task, error) { return s.tasks, s.err }

// recorder is a spy: it remembers what it was asked to send and can fail for chosen owners.
type recorder struct {
	messages []string
	failFor  map[string]error
}

func (r *recorder) Notify(_ context.Context, owner, message string) error {
	if err := r.failFor[owner]; err != nil {
		return err
	}
	r.messages = append(r.messages, owner+": "+message)
	return nil
}

var (
	_ Store    = fakeStore{}
	_ Notifier = (*recorder)(nil)
	_ Clock    = (*fakeClock)(nil)
)

func day(d int) time.Time { return time.Date(2030, 1, d, 0, 0, 0, 0, time.UTC) }

func TestSendOverdue(t *testing.T) {
	clock := &fakeClock{now: day(10).Add(9 * time.Hour)}
	store := fakeStore{tasks: []Task{
		{ID: 1, Title: "Write docs", Owner: "ada", Due: day(9)},
		{ID: 2, Title: "Ship it", Owner: "bob", Due: day(11)},
		{ID: 3, Title: "Old but done", Owner: "ada", Due: day(1), Done: true},
		{ID: 4, Title: "No deadline", Owner: "bob"},
	}}
	rec := &recorder{}
	svc := &Service{Store: store, Notifier: rec, Clock: clock}

	sent, err := svc.SendOverdue(context.Background())
	if err != nil || sent != 1 {
		t.Fatalf("sent = %d, err = %v", sent, err)
	}
	if want := []string{`ada: task "Write docs" was due on 2030-01-09`}; !slices.Equal(rec.messages, want) {
		t.Fatalf("messages = %q", rec.messages)
	}

	// Time is just a value: move it and the same task becomes overdue, with no sleeping.
	clock.Advance(48 * time.Hour)
	rec.messages = nil
	sent, err = svc.SendOverdue(context.Background())
	if err != nil || sent != 2 {
		t.Fatalf("after advancing: sent = %d, err = %v", sent, err)
	}
}

func TestOneFailedNotificationDoesNotStopTheRest(t *testing.T) {
	errDown := errors.New("mail server down")
	rec := &recorder{failFor: map[string]error{"ada": errDown}}
	svc := &Service{
		Store: fakeStore{tasks: []Task{
			{Title: "a", Owner: "ada", Due: day(1)},
			{Title: "b", Owner: "bob", Due: day(1)},
		}},
		Notifier: rec,
		Clock:    &fakeClock{now: day(5)},
	}
	sent, err := svc.SendOverdue(context.Background())
	if sent != 1 || !errors.Is(err, errDown) {
		t.Fatalf("sent = %d, err = %v; want 1 sent and errDown", sent, err)
	}
	if len(rec.messages) != 1 {
		t.Fatalf("messages = %q", rec.messages)
	}
}

func TestStoreFailureIsWrapped(t *testing.T) {
	errDB := errors.New("db is down")
	svc := &Service{Store: fakeStore{err: errDB}, Notifier: &recorder{}, Clock: &fakeClock{now: day(1)}}
	if _, err := svc.SendOverdue(context.Background()); !errors.Is(err, errDB) {
		t.Fatalf("err = %v, want it to wrap errDB", err)
	}
}

func TestLoadSeed(t *testing.T) {
	fsys := fstest.MapFS{
		"seed.json": {Data: []byte(`[{"title":"Write docs","owner":"ada","due_date":"2030-01-31"},{"title":"Plan","owner":"bob"}]`)},
		"bad.json":  {Data: []byte(`{not json`)},
		"date.json": {Data: []byte(`[{"title":"x","owner":"y","due_date":"31/01/2030"}]`)},
	}

	tasks, err := LoadSeed(fsys, "seed.json")
	if err != nil || len(tasks) != 2 {
		t.Fatalf("tasks = %v, err = %v", tasks, err)
	}
	if !tasks[0].Due.Equal(time.Date(2030, 1, 31, 0, 0, 0, 0, time.UTC)) || !tasks[1].Due.IsZero() {
		t.Errorf("due dates = %v, %v", tasks[0].Due, tasks[1].Due)
	}

	if _, err := LoadSeed(fsys, "missing.json"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: err = %v, want fs.ErrNotExist", err)
	}
	for _, name := range []string{"bad.json", "date.json"} {
		if _, err := LoadSeed(fsys, name); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
