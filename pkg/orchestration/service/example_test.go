package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Define our Event data
type Event struct {
	Id   uuid.UUID
	Data string
}

func (e Event) ID() string {
	return e.Id.String()
}

// Define our Service
type MyService struct {
	name string
}

func (m MyService) Apply(ctx context.Context, event *Event) error {
	event.Data += " -> applied by " + m.name
	return nil
}

func Example() {
	ctx := context.Background()
	a, b := MyService{"a"}, MyService{"b"}

	// Initialize workflow, controller should do this only once
	wf := NewApplyWorkflow(a, b)

	// Execute workflows on request
	event := &Event{Id: uuid.New(), Data: "init"}
	err := wf.Execute(ctx, event) // Calls Apply(ctx, event) for each Service
	if err != nil {
		panic(err)
	}
	fmt.Println(event.Data)

	// Output:
	// init -> applied by a -> applied by b
}

var _ Service[*Event] = (*MyService)(nil)

// Additional Service methods for validation and deletion. We dont use them in this example
func (m MyService) Name() string {
	return m.name
}

func (m MyService) Validate(ctx context.Context, event *Event) error {
	event.Data += " -> validated by " + m.name
	return nil
}

func (m MyService) Delete(ctx context.Context, event *Event) error {
	event.Data = " -> deleted by " + m.name
	return nil
}
