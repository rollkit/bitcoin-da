package main

import (
	"fmt"

	"github.com/btcsuite/btcd/btcutil/bech32"
	"github.com/btcsuite/btcd/txscript"
)

func createTaprootAddress(embeddedData []byte) (string, error) {
	left, err := txscript.NewScriptBuilder().Script()
	if err != nil {
		return "", fmt.Errorf("error constructing Taproot script: %v", err)
	}

	// Step 1: Construct the Taproot script with two leafs: one empty and the other "OP_0 OP_IF <embedded data> OP_ENDIF".
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_0)
	builder.AddOp(txscript.OP_IF)
	builder.AddData(embeddedData)
	builder.AddOp(txscript.OP_ENDIF)
	right, err := builder.Script()
	if err != nil {
		return "", fmt.Errorf("error constructing Taproot script: %v", err)
	}

	// Step 2: Construct the Taproot descriptor.
	tapBranch := txscript.NewTapBranch(txscript.NewTapLeaf(txscript.BaseLeafVersion, left), txscript.NewTapLeaf(txscript.BaseLeafVersion, right))

	hash := tapBranch.TapHash()
	// TODO: tweak pubkey

	// Step 3: Generate the Bech32 address.
	address, err := bech32.EncodeM("bc", hash.CloneBytes())
	if err != nil {
		return "", fmt.Errorf("error encoding Taproot address: %v", err)
	}

	return address, nil
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
