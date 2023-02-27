package rollkitbtc_test

import (
	"fmt"

	rollkitbtc "github.com/rollkit/rollkit-btc"
)

// ExampleWrite tests that writing data to the blockchain works as expected.
func ExampleWrite() {
	// Example usage
	fmt.Println("writing...")
	_, err := rollkitbtc.Write([]byte("rollkit-btc: gm"))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("done")
	// Output: writing...
	// done
}
