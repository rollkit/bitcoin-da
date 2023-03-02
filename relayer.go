package bitcoinda

import (
	"bytes"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// PROTOCOL_ID allows data identification by looking at the first few bytes
var PROTOCOL_ID = []byte{0x72, 0x6f, 0x6c, 0x6c}

// Sample data and keys for testing.
// bob key pair is used for signing reveal tx
// internal key pair is used for tweaking
var (
	bobPrivateKey      = "5JoQtsKQuH8hC9MyvfJAqo6qmKLm8ePYNucs7tPu2YxG12trzBt"
	internalPrivateKey = "5JGgKfRy6vEcWBpLJV5FXUfMGNXzvdWzQHUM1rVLEUJfvZUSwvS"
)

// createTaprootAddress returns an address committing to a Taproot script with
// a single leaf containing the spend path with the script:
// <embedded data> OP_DROP <pubkey> OP_CHECKSIG
func createTaprootAddress(embeddedData []byte) (string, error) {
	privKey, err := btcutil.DecodeWIF(bobPrivateKey)
	if err != nil {
		return "", fmt.Errorf("error decoding bob private key: %v", err)
	}

	pubKey := privKey.PrivKey.PubKey()

	// Step 1: Construct the Taproot script with one leaf.
	builder := txscript.NewScriptBuilder()
	builder.AddData(embeddedData)
	builder.AddOp(txscript.OP_DROP)
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

	// Step 2: Generate the Taproot tree.
	tapScriptRootHash := tapScriptTree.RootNode.TapHash()
	outputKey := txscript.ComputeTaprootOutputKey(
		internalPubKey, tapScriptRootHash[:],
	)

	// Step 3: Generate the Bech32m address.
	address, err := btcutil.NewAddressTaproot(
		schnorr.SerializePubKey(outputKey), &chaincfg.RegressionNetParams)
	if err != nil {
		return "", fmt.Errorf("error encoding Taproot address: %v", err)
	}

	return address.String(), nil
}

// payToTaprootScript creates a pk script for a pay-to-taproot output key.
func payToTaprootScript(taprootKey *btcec.PublicKey) ([]byte, error) {
	return txscript.NewScriptBuilder().
		AddOp(txscript.OP_1).
		AddData(schnorr.SerializePubKey(taprootKey)).
		Script()
}

// Relayer is a bitcoin client wrapper which provides reader and writer methods
// to write binary blobs to the blockchain.
type Relayer struct {
	client *rpcclient.Client
}

// close shuts down the client.
func (r Relayer) close() {
	r.client.Shutdown()
}

// commitTx commits an output to the given taproot address, such that the
// output is only spendable by posting the embedded data on chain, as part of
// the script satisfying the tapscript spend path that commits to the data. It
// returns the hash of the commit transaction and error, if any.
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

	return hash, nil
}

// revealTx spends the output from the commit transaction and as part of the
// script satisfying the tapscript spend path, posts the embedded data on
// chain. It returns the hash of the reveal transaction and error, if any.
func (r Relayer) revealTx(embeddedData []byte, commitHash *chainhash.Hash) (*chainhash.Hash, error) {
	rawCommitTx, err := r.client.GetRawTransaction(commitHash)
	if err != nil {
		return nil, fmt.Errorf("error getting raw commit tx: %v", err)
	}

	// TODO: use a better way to find our output
	var commitIndex int
	var commitOutput *wire.TxOut
	for i, out := range rawCommitTx.MsgTx().TxOut {
		if out.Value == 100000 {
			commitIndex = i
			commitOutput = out
			break
		}
	}

	privKey, err := btcutil.DecodeWIF(bobPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding bob private key: %v", err)
	}

	pubKey := privKey.PrivKey.PubKey()

	internalPrivKey, err := btcutil.DecodeWIF(internalPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding internal private key: %v", err)
	}

	internalPubKey := internalPrivKey.PrivKey.PubKey()

	// Our script will be a simple <embedded-data> OP_DROP OP_CHECKSIG as the
	// sole leaf of a tapscript tree.
	builder := txscript.NewScriptBuilder()
	builder.AddData(embeddedData)
	builder.AddOp(txscript.OP_DROP)
	builder.AddData(schnorr.SerializePubKey(pubKey))
	builder.AddOp(txscript.OP_CHECKSIG)
	pkScript, err := builder.Script()
	if err != nil {
		return nil, fmt.Errorf("error building script: %v", err)
	}

	tapLeaf := txscript.NewBaseTapLeaf(pkScript)
	tapScriptTree := txscript.AssembleTaprootScriptTree(tapLeaf)

	ctrlBlock := tapScriptTree.LeafMerkleProofs[0].ToControlBlock(
		internalPubKey,
	)

	tapScriptRootHash := tapScriptTree.RootNode.TapHash()
	outputKey := txscript.ComputeTaprootOutputKey(
		internalPubKey, tapScriptRootHash[:],
	)
	p2trScript, err := payToTaprootScript(outputKey)
	if err != nil {
		return nil, fmt.Errorf("error building p2tr script: %v", err)
	}

	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  *rawCommitTx.Hash(),
			Index: uint32(commitIndex),
		},
	})
	txOut := &wire.TxOut{
		Value: 1e3, PkScript: p2trScript,
	}
	tx.AddTxOut(txOut)

	inputFetcher := txscript.NewCannedPrevOutputFetcher(
		commitOutput.PkScript,
		commitOutput.Value,
	)
	sigHashes := txscript.NewTxSigHashes(tx, inputFetcher)

	sig, err := txscript.RawTxInTapscriptSignature(
		tx, sigHashes, 0, txOut.Value,
		txOut.PkScript, tapLeaf, txscript.SigHashDefault,
		privKey.PrivKey,
	)

	if err != nil {
		return nil, fmt.Errorf("error signing tapscript: %v", err)
	}

	// Now that we have the sig, we'll make a valid witness
	// including the control block.
	ctrlBlockBytes, err := ctrlBlock.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("error including control block: %v", err)
	}
	tx.TxIn[0].Witness = wire.TxWitness{
		sig, pkScript, ctrlBlockBytes,
	}

	hash, err := r.client.SendRawTransaction(tx, false)
	if err != nil {
		return nil, fmt.Errorf("error sending reveal transaction: %v", err)
	}
	return hash, nil
}

type Config struct {
	Host         string
	User         string
	Pass         string
	HTTPPostMode bool
	DisableTLS   bool
}

// NewRelayer returns a new relayer. It can error if there's an RPC connection
// error with the connection config.
func NewRelayer(config Config) (*Relayer, error) {
	// Set up the connection to the btcd RPC server.
	// NOTE: for testing bitcoind can be used in regtest with the following params -
	// bitcoind -chain=regtest -rpcport=18332 -rpcuser=rpcuser -rpcpassword=rpcpass -fallbackfee=0.000001 -txindex=1
	connCfg := &rpcclient.ConnConfig{
		Host:         config.Host,
		User:         config.User,
		Pass:         config.Pass,
		HTTPPostMode: config.HTTPPostMode,
		DisableTLS:   config.DisableTLS,
	}
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating btcd RPC client: %v", err)
	}
	return &Relayer{
		client: client,
	}, nil
}

func (r Relayer) Read(height int64) ([][]byte, error) {
	hash, err := r.client.GetBlockHash(height)
	if err != nil {
		return nil, err
	}
	block, err := r.client.GetBlock(hash)
	if err != nil {
		return nil, err
	}

	var data [][]byte
	for _, tx := range block.Transactions {
		if len(tx.TxIn[0].Witness) > 1 {
			witness := tx.TxIn[0].Witness[1]
			size := uint(witness[0])
			if bytes.HasPrefix(witness[1:], PROTOCOL_ID) {
				data = append(data, witness[1:size+1])
			}
		}
	}
	return data, nil
}

func (r Relayer) Check(hash *chainhash.Hash) (int64, error) {
	tx, err := r.client.GetRawTransactionVerbose(hash)
	if err != nil {
		return 0, err
	}
	hash, err = chainhash.NewHashFromStr(tx.BlockHash)
	if err != nil {
		return 0, err
	}
	block, err := r.client.GetBlockVerbose(hash)
	if err != nil {
		return 0, err
	}
	return block.Height, nil
}

func (r Relayer) Write(data []byte) (*chainhash.Hash, error) {
	address, err := createTaprootAddress(data)
	if err != nil {
		return nil, err
	}
	hash, err := r.commitTx(address)
	if err != nil {
		return nil, err
	}
	hash, err = r.revealTx(data, hash)
	if err != nil {
		return nil, err
	}
	return hash, nil
}
