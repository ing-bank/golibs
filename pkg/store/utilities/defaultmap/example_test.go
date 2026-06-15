package defaultmap

import (
	"context"
	"fmt"
)

func Example() {
	type Account struct {
		Savings int
	}

	ctx := context.Background()
	bank := New[string, Account]()

	// Apply always overrides
	bank.Apply(ctx, "Foo", Account{Savings: 5})
	bank.Apply(ctx, "Foo", Account{Savings: 6}) // Overrides
	acc, _ := bank.Store.Read(ctx, "Foo")
	fmt.Println(acc.Savings)

	// Update updates an existing entry
	collision := func(old, new Account) Account {
		new.Savings += old.Savings
		return new
	}

	bank.Update(ctx, "Foo", Account{1}, collision) // Foo exists, so the collision function will be called
	acc, _ = bank.Store.Read(ctx, "Foo")
	fmt.Println(acc.Savings)

	// Update is also safe to use if there is no entry yet, collision function not used
	bank.Update(ctx, "Bar", Account{1}, collision)
	acc, _ = bank.Store.Read(ctx, "Bar")
	fmt.Println(acc.Savings)

	// Output:
	// 6
	// 7
	// 1
}
