package rollkitbtc_test

import (
	"encoding/hex"
	"fmt"

	rollkitbtc "github.com/rollkit/rollkit-btc"
)

// ExampleRead tests that reading data from the blockchain works as expected.
func ExampleRead() {
	// Example usage
	hash, err := rollkitbtc.Write([]byte("rollkit-btc: gm"))
	if err != nil {
		fmt.Println(err)
		return
	}
	bytes, err := rollkitbtc.Read(hash)
	if err != nil {
		fmt.Println(err)
		return
	}
	got, err := hex.DecodeString(fmt.Sprintf("%x", bytes))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(got))
	// Output: rollkit-btc: gm
}
