package defaultmap

import "fmt"

func Example() {
	type Account struct {
		Savings int
	}

	bank := DefaultMap[string, Account]{}

	// Set always overrides
	bank.Set("Foo", Account{Savings: 5})
	bank.Set("Foo", Account{Savings: 6}) // Overrides
	fmt.Println(bank["Foo"].Savings)

	// Update updates an existing entry
	collision := func(old, new Account) Account {
		new.Savings += old.Savings
		return new
	}

	bank.Update("Foo", Account{1}, collision) // Foo exists, so the collision function will be called
	fmt.Println(bank["Foo"].Savings)

	// Update is also safe to use if there is no entry yet, collision function not used
	bank.Update("Bar", Account{1}, collision)
	fmt.Println(bank["Bar"].Savings)

	// Output:
	// 6
	// 7
	// 1
}
