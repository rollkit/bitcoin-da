package bitcoinda_test

import (
	"encoding/hex"
	"fmt"

	bitcoinda "github.com/rollkit/bitcoin-da"
)

// ExampleRelayer_Write tests that writing data to the blockchain works as
// expected.
func ExampleRelayer_Write() {
	// Example usage
	relayer, err := bitcoinda.NewRelayer(bitcoinda.Config{
		Host:         "localhost:18332",
		User:         "rpcuser",
		Pass:         "rpcpass",
		HTTPPostMode: true,
		DisableTLS:   true,
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Writing...")
	_, err = relayer.Write([]byte("rollkit-btc: gm"))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("done")
	// Output: Writing...
	// done
}

// ExampleRelayer_Read tests that reading data from the blockchain works as
// expected.
func ExampleRelayer_Read() {
	// Example usage
	relayer, err := bitcoinda.NewRelayer(bitcoinda.Config{
		Host:                "localhost:18332",
		User:                "rpcuser",
		Pass:                "rpcpass",
		HTTPPostMode:        true,
		DisableTLS:          true,
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	_, err = relayer.Write([]byte("rollkit-btc: gm"))
	if err != nil {
		fmt.Println(err)
		return
	}
	// TODO: either mock or generate block
	// We're assuming the prev tx was mined at height 146
	height := uint64(146)
	blobs, err := relayer.Read(height)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, blob := range blobs {
		got, err := hex.DecodeString(fmt.Sprintf("%x", blob))
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(string(got))
	}
	// Output: rollkit-btc: gm
}
