package labels

import (
	"context"
	"fmt"

	"github.com/ing-bank/golibs/pkg/store/backends/memory"
)

func Example() {
	backend := memory.NewOrDie[string, *LabeledData[string, int]]()
	db, err := New[string, int](backend, Config[int]{
		ImmutableLabels: map[string]string{"immutable": "true"},

		LabelsEnricher: func(obj int) (map[string]string, error) {
			if obj < 0 {
				return map[string]string{"sign": "negative"}, nil
			}
			return map[string]string{"sign": "positive"}, nil
		},
	})
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	_ = db.Apply(ctx, "1", 1)
	_ = db.Apply(ctx, "2", -2)
	_ = db.Apply(ctx, "3", 3)

	// These 2 lines are the same
	_ = db.Apply(ctx, "4", -4, WithLabels(map[string]string{"custom": "label"}))
	_ = db.Apply(ctx, "4", -4, WithLabel("custom", "label"))

	items, err := db.List(ctx, WithLabelSelector("sign=negative"))
	if err != nil {
		panic(err)
	}
	fmt.Println(items) // 2, 4

	items, err = db.List(ctx, WithLabelSelector("custom=label"))
	if err != nil {
		panic(err)
	}
	fmt.Println(items) // 4

	raw, err := backend.Read(ctx, "4")
	if err != nil {
		panic(err)
	}
	fmt.Println(raw) // 4 {custom:label immutable:true sign:negative}

	// Output:
	// [{2 -2} {4 -4}]
	// [{4 -4}]
	// &{4 map[custom:label immutable:true sign:negative] -4}
}
