package rollkitbtc_test

import (
	"fmt"

	rollkitbtc "github.com/rollkit/rollkit-btc"
)

// ExampleRead tests that reading data from the blockchain works as expected.
func ExampleRead() {
	// Example usage
	fmt.Println("writing...")
	hash, err := rollkitbtc.Write([]byte("rollkit-btc: gm"))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("reading...")
	_, err = rollkitbtc.Read(hash)
	if err != nil {
		fmt.Println(err)
		return
	}
	// fmt.Println(expected, got[1:16])
	fmt.Println("done")
	// Output: writing...
	// reading...
	// done
}
