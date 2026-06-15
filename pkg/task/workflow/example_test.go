package workflow

import (
	"context"
	"fmt"

	"github.com/ing-bank/golibs/pkg/retry"
)

var _ Activity[*int] = &NumberActivity{}

type NumberActivity struct {
	name string
}

func (n NumberActivity) Name() string {
	return n.name
}

// Run always increments the given number, and then fails if that number is even
func (n NumberActivity) Run(ctx context.Context, number *int) (err error) {
	*number += 1 // Always increment counter

	if *number%2 == 0 {
		err = fmt.Errorf("only odd numbers work") // How odd!
	}

	fmt.Printf("%s: counter=%d, err='%v'\n", n.Name(), *number, err)
	return
}

func ExampleNewWorkflow() {
	// Chain describes a list of tasks that will be executed in order
	tasks := Chain[*int]{&NumberActivity{"worker 1"}, &NumberActivity{"worker 2"}}

	// Create the workflow
	wf := NewWorkflow("numbers", tasks).WithRetryPolicy(retry.NoRetry) // Other retry options are: DefaultBackoff, RetryOnce, RunForever, or define your own

	fmt.Println("--- Workflow without retries ---")
	counter := 0
	err := wf.Execute(context.Background(), &counter)                   // Run the tasks
	fmt.Printf("Workflow result: counter=%d, err='%v'\n", counter, err) // 2, only odd numbers work

	fmt.Println("\n--- Workflow with default retries ---")
	counter = 0
	wf = wf.WithRetryPolicy(retry.DefaultBackoff)
	err = wf.Execute(context.Background(), &counter)                    // Run the tasks
	fmt.Printf("Workflow result: counter=%d, err='%v'\n", counter, err) // 3, nil

	// Output:
	// --- Workflow without retries ---
	// worker 1: counter=1, err='<nil>'
	// worker 2: counter=2, err='only odd numbers work'
	// Workflow result: counter=2, err='worker 2: only odd numbers work'
	//
	// --- Workflow with default retries ---
	// worker 1: counter=1, err='<nil>'
	// worker 2: counter=2, err='only odd numbers work'
	// worker 2: counter=3, err='<nil>'
	// Workflow result: counter=3, err='<nil>'
}

func ExampleChain_Weave() {
	// Chain describes a list of tasks that will be executed in order
	tasks := Chain[*int]{&NumberActivity{"worker 1"}, &NumberActivity{"worker 2"}}

	// Add a new task after every already existing task
	tasks = tasks.Weave(NewActivity("example database commit", func(ctx context.Context, counter *int) error {
		fmt.Printf("committing %d to database\n", *counter)
		return nil
	}))

	// Print the names of all the tasks in the chain
	fmt.Println(tasks.Describe()) // worker 1,example database commit,worker 2,example database commit

	// Create the workflow
	wf := NewWorkflow("numbers", tasks)
	counter := 0
	err := wf.Execute(context.Background(), &counter)
	fmt.Printf("Workflow result: counter=%d, err='%v'\n", counter, err) // 3, nil

	// Output:
	// worker 1, example database commit, worker 2, example database commit
	// worker 1: counter=1, err='<nil>'
	// committing 1 to database
	// worker 2: counter=2, err='only odd numbers work'
	// worker 2: counter=3, err='<nil>'
	// committing 3 to database
	// Workflow result: counter=3, err='<nil>'
}
