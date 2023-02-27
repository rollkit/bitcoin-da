package main

import (
	"fmt"
	"log"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/davecgh/go-spew/spew"
)

// Sample data and keys for testing.
var (
	bobPrivateKey      = "5JoQtsKQuH8hC9MyvfJAqo6qmKLm8ePYNucs7tPu2YxG12trzBt"
	internalPrivateKey = "5JGgKfRy6vEcWBpLJV5FXUfMGNXzvdWzQHUM1rVLEUJfvZUSwvS"
	revealPrivateKey   = "cP5ycVVC1ByiiJgHNdEedSfQ9h16cjCewywksKvQFVqCyzXbshzp"
)

// payToTaprootScript creates a pk script for a pay-to-taproot output key.
func payToTaprootScript(taprootKey *btcec.PublicKey) ([]byte, error) {
	return txscript.NewScriptBuilder().
		AddOp(txscript.OP_1).
		AddData(schnorr.SerializePubKey(taprootKey)).
		Script()
}

func createTaprootAddress(embeddedData []byte) (string, error) {
	// Sign the transaction with the sample keypair.
	privKey, err := btcutil.DecodeWIF(bobPrivateKey)
	if err != nil {
		return "", fmt.Errorf("error decoding bob private key: %v", err)
	}

	pubKey := privKey.PrivKey.PubKey()

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
	builder.AddData(pubKey.SerializeCompressed()[1:])
	builder.AddOp(txscript.OP_CHECKSIG)
	right, err := builder.Script()
	if err != nil {
		return "", fmt.Errorf("error constructing Taproot script: %v", err)
	}

	// Step 2: Construct the Taproot merkletree.
	tapBranch := txscript.NewTapBranch(
		txscript.NewTapLeaf(txscript.BaseLeafVersion, left),
		txscript.NewTapLeaf(txscript.BaseLeafVersion, right),
	)

	hash := tapBranch.TapHash()

	internalPrivKey, err := btcutil.DecodeWIF(internalPrivateKey)
	if err != nil {
		return "", fmt.Errorf("error decoding internal private key: %v", err)
	}

	internalPubKey := internalPrivKey.PrivKey.PubKey()

	tweakedPubkey := txscript.ComputeTaprootOutputKey(internalPubKey, hash.CloneBytes())

	// Step 3: Generate the Bech32 address.
	address, err := btcutil.NewAddressTaproot(
		schnorr.SerializePubKey(tweakedPubkey), &chaincfg.RegressionNetParams)
	if err != nil {
		return "", fmt.Errorf("error encoding Taproot address: %v", err)
	}

	return address.String(), nil
}

func commitTx(addr string) error {
	// Set up the connection to the btcd RPC server.
	connCfg := &rpcclient.ConnConfig{
		Host:         "localhost:18332",
		User:         "rpcuser",
		Pass:         "rpcpass",
		HTTPPostMode: true,
		DisableTLS:   true,
	}
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return fmt.Errorf("error creating btcd RPC client: %v", err)
	}
	defer client.Shutdown()

	// Create a transaction that sends 0.0001 BTC to the given address.
	address, err := btcutil.DecodeAddress(addr, &chaincfg.RegressionNetParams)
	if err != nil {
		return fmt.Errorf("error decoding recipient address: %v", err)
	}

	amount, err := btcutil.NewAmount(0.001)
	if err != nil {
		return fmt.Errorf("error creating new amount: %v", err)
	}

	hash, err := client.SendToAddress(address, amount)
	if err != nil {
		return fmt.Errorf("error sending to address: %v", err)
	}

	// Print the transaction hash.
	log.Printf("Transaction sent: %s", hash.String())
	return nil
}

func revealTx(prev wire.MsgTx, op wire.OutPoint) error {
	revealPrivKey, err := btcutil.DecodeWIF(revealPrivateKey)
	if err != nil {
		return fmt.Errorf("error decoding reveal private key: %v", err)
	}

	revealPubKey := revealPrivKey.PrivKey.PubKey()

	// Our script will be a simple OP_CHECKSIG as the sole leaf of a
	// tapscript tree. We'll also re-use the internal key as the key in the
	// leaf.
	builder := txscript.NewScriptBuilder()
	builder.AddData(schnorr.SerializePubKey(revealPubKey))
	builder.AddOp(txscript.OP_CHECKSIG)
	pkScript, err := builder.Script()
	if err != nil {
		return fmt.Errorf("error building script: %v", err)
	}

	tapLeaf := txscript.NewBaseTapLeaf(pkScript)
	tapScriptTree := txscript.AssembleTaprootScriptTree(tapLeaf)

	ctrlBlock := tapScriptTree.LeafMerkleProofs[0].ToControlBlock(
		revealPubKey,
	)

	tapScriptRootHash := tapScriptTree.RootNode.TapHash()
	outputKey := txscript.ComputeTaprootOutputKey(
		revealPubKey, tapScriptRootHash[:],
	)
	p2trScript, err := payToTaprootScript(outputKey)
	if err != nil {
		return fmt.Errorf("error building p2tr script: %v", err)
	}

	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: op,
	})
	txOut := &wire.TxOut{
		Value: 1e8, PkScript: p2trScript,
	}
	tx.AddTxOut(txOut)

	sigHashes := txscript.NewTxSigHashes(tx, txscript.NewCannedPrevOutputFetcher(txOut.PkScript, txOut.Value))
	sig, err := txscript.RawTxInTapscriptSignature(
		tx, sigHashes, 0, txOut.Value,
		txOut.PkScript, tapLeaf, txscript.SigHashDefault,
		revealPrivKey.PrivKey,
	)
	if err != nil {
		return fmt.Errorf("error signing tapscript: %v", err)
	}

	// Now that we have the sig, we'll make a valid witness
	// including the control block.
	ctrlBlockBytes, err := ctrlBlock.ToBytes()
	if err != nil {
		return fmt.Errorf("error including control block: %v", err)
	}
	tx.TxIn[0].Witness = wire.TxWitness{
		sig, pkScript, ctrlBlockBytes,
	}
	spew.Dump(tx)
	return nil
}

func main() {
	// Example usage
	embeddedData := []byte("00")
	address, err := createTaprootAddress(embeddedData)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	err = commitTx(address)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
}
