package bitcoinda_test

import (
	"encoding/hex"
	"fmt"
	"time"

	bitcoinda "github.com/rollkit/bitcoin-da"
)

// NOTE: tests need a local regtest node running like so:
// bitcoind -chain=regtest -rpcport=18332 -rpcuser=rpcuser -rpcpassword=rpcpass -fallbackfee=0.000001 -txindex=1

// ExampleRelayer_Read tests that reading data from the blockchain works as
// expected.
// NOTE: needs a background miner to keep generating regtest blocks
// I like to keep this running in the background:
// export COINBASE=$(bitcoin-cli -regtest -rpcport=18332 -rpcuser=rpcuser -rpcpassword=rpcpass getnewaddress)
// watch -n 5 bitcoin-cli -regtest -rpcport=18332 -rpcuser=rpcuser -rpcpassword=rpcpass generatetoaddress 1 $COINBASE
func ExampleRelayer_Read() {
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
	hash, err := relayer.Write([]byte("rollkit-btc: gm"))
	if err != nil {
		fmt.Println(err)
		return
	}
	time.Sleep(5 * time.Second)
	var height int64
	for i := 0; i < 10; i++ {
		height, err = relayer.Check(hash)
		if err != nil {
			fmt.Printf("%v: retrying in 1s\n", err)
			time.Sleep(time.Second)
			continue
		}
	}
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
