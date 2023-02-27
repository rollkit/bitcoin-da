rollkit-btc:
============


This package provides a reader / writer interface to bitcoin.

Example:
========

	embeddedData := []byte("00")
	address, err := createTaprootAddress(embeddedData)
	if err != nil {
		fmt.Println(err)
		return
	}
	relayer, err := NewRelayer()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer relayer.Close()
	hash, err := relayer.commitTx(address)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = relayer.revealTx(hash)
	if err != nil {
		fmt.Println(err)
		return
	}


Writer:
=======

A commit transaction containing a taproot with one leaf script

    OP_0
    OP_IF
    <embedded data>
    OP_ENDIF

is used to create a new bech32m address and is sent an output.


A reveal transaction then posts the embedded data on chain and spends the
commit output.


Reader:
========

The address of the reveal transaction is implicity used as a namespace.


Clients may call listunspent on the reveal transaction address to get a list of
transactions and read the embedded data from the first witness input.
