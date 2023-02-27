package main

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

// Sample data and keys for testing.
var (
	samplePrivateKey = "cS5LWK2aUKgP9LmvViG3m9HkfwjaEJpGVbrFHuGZKvW2ae3W9aUe"
)

func createTaprootAddress(embeddedData []byte) (string, error) {
	// Step 1: Construct the Taproot script with two leafs:
	// left: empty
	left, err := txscript.NewScriptBuilder().Script()
	if err != nil {
		return "", fmt.Errorf("error constructing Taproot script: %v", err)
	}

	// right: "OP_0 OP_IF <embedded data> OP_ENDIF".
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_0)
	builder.AddOp(txscript.OP_IF)
	builder.AddData(embeddedData)
	builder.AddOp(txscript.OP_ENDIF)
	right, err := builder.Script()
	if err != nil {
		return "", fmt.Errorf("error constructing Taproot script: %v", err)
	}

	// Step 2: Construct the Taproot merkletree.
	tapBranch := txscript.NewTapBranch(
		txscript.NewTapLeaf(txscript.BaseLeafVersion, left),
		txscript.NewTapLeaf(txscript.BaseLeafVersion, right),
	)

	// Sign the transaction with the sample keypair.
	privKey, err := btcutil.DecodeWIF(samplePrivateKey)
	if err != nil {
		return "", fmt.Errorf("error decoding sample private key: %v", err)
	}

	pubKey := privKey.PrivKey.PubKey()

	hash := tapBranch.TapHash()
	tweakedPubkey := txscript.ComputeTaprootOutputKey(pubKey, hash.CloneBytes())

	// Step 3: Generate the Bech32 address.
	address, err := btcutil.NewAddressTaproot(
		schnorr.SerializePubKey(tweakedPubkey),
		&chaincfg.RegressionNetParams)
	if err != nil {
		return "", fmt.Errorf("error encoding Taproot address: %v", err)
	}

	return address.String(), nil
}

func commitTx() {
	// TODO
}

func revealTx() {
	// TODO
}

func main() {
	// Example usage
	embeddedData := []byte("hello world")
	address, err := createTaprootAddress(embeddedData)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Taproot address:", address)
}
