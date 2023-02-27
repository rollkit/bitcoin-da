package main

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
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

type Relayer struct {
	client *rpcclient.Client
}

func (r Relayer) Close() {
	r.client.Shutdown()
}

func NewRelayer() (*Relayer, error) {
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
		return nil, fmt.Errorf("error creating btcd RPC client: %v", err)
	}
	return &Relayer{
		client: client,
	}, nil
}

func (r Relayer) commitTx(addr string) (*chainhash.Hash, error) {
	// Create a transaction that sends 0.001 BTC to the given address.
	address, err := btcutil.DecodeAddress(addr, &chaincfg.RegressionNetParams)
	if err != nil {
		return nil, fmt.Errorf("error decoding recipient address: %v", err)
	}

	amount, err := btcutil.NewAmount(0.001)
	if err != nil {
		return nil, fmt.Errorf("error creating new amount: %v", err)
	}

	hash, err := r.client.SendToAddress(address, amount)
	if err != nil {
		return nil, fmt.Errorf("error sending to address: %v", err)
	}

	// Print the transaction hash.
	return hash, nil
}

func (r Relayer) revealTx(commitHash *chainhash.Hash) error {
	rawCommitTx, err := r.client.GetRawTransaction(commitHash)
	if err != nil {
		return fmt.Errorf("error getting raw commit tx: %v", err)
	}

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
		PreviousOutPoint: wire.OutPoint{
			Hash:  *rawCommitTx.Hash(),
			Index: 0,
		},
	})
	txOut := &wire.TxOut{
		Value: 1e3, PkScript: p2trScript,
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

	var buf bytes.Buffer
	err = tx.Serialize(&buf)
	if err != nil {
		return err
	}

	spew.Dump(hex.EncodeToString(buf.Bytes()))
	return nil
}

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

	// Step 1: Construct the Taproot script with one leaf:
	builder := txscript.NewScriptBuilder()
	builder.AddData(schnorr.SerializePubKey(pubKey))
	builder.AddOp(txscript.OP_CHECKSIG)
	pkScript, err := builder.Script()
	if err != nil {
		return "", fmt.Errorf("error building script: %v", err)
	}

	tapLeaf := txscript.NewBaseTapLeaf(pkScript)
	tapScriptTree := txscript.AssembleTaprootScriptTree(tapLeaf)

	internalPrivKey, err := btcutil.DecodeWIF(internalPrivateKey)
	if err != nil {
		return "", fmt.Errorf("error decoding internal private key: %v", err)
	}

	internalPubKey := internalPrivKey.PrivKey.PubKey()

	tapScriptRootHash := tapScriptTree.RootNode.TapHash()
	outputKey := txscript.ComputeTaprootOutputKey(
		internalPubKey, tapScriptRootHash[:],
	)

	// Step 3: Generate the Bech32 address.
	address, err := btcutil.NewAddressTaproot(
		schnorr.SerializePubKey(outputKey), &chaincfg.RegressionNetParams)
	if err != nil {
		return "", fmt.Errorf("error encoding Taproot address: %v", err)
	}

	return address.String(), nil
}

func main() {
	// Example usage
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
}
