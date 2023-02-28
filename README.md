rollkit-btc:
============


This package provides a reader / writer interface to bitcoin.

Example:
========

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

Tests:
======

Running the tests requires a local regtest node.

	bitcoind -chain=regtest -rpcport=18332 -rpcuser=rpcuser -rpcpassword=rpcpass -fallbackfee=0.000001

	bitcoin-cli -regtest -rpcport=18332 -rpcuser=rpcuser -rpcpassword=rpcpass createwallet w1

	bitcoin-cli -regtest -rpcport=18332 -rpcuser=rpcuser -rpcpassword=rpcpass generatetoaddress 101 `bcli getnewaddress`

Idle for a while till coinbase coins mature.

	go test -v

	=== RUN   ExampleRead
	--- PASS: ExampleRead (0.31s)
	=== RUN   ExampleWrite
	--- PASS: ExampleWrite (0.28s)
	PASS
	ok      github.com/rollkit/rollkit-btc  0.706s


Writer:
=======

A commit transaction containing a taproot with one leaf script

    <embedded data>
    OP_DROP
    <pubkey>
    OP_CHECKSIG

is used to create a new bech32m address and is sent an output.


A reveal transaction then posts the embedded data on chain and spends the
commit output.


Reader:
========

The address of the reveal transaction is implicity used as a namespace.


Clients may call listunspent on the reveal transaction address to get a list of
transactions and read the embedded data from the first witness input.
