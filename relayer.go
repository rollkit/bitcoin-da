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

const (
	DEFAULT_SAT_AMOUNT  = 1000
	DEFAULT_SAT_FEE     = 200
	DEFAULT_PRIVATE_KEY = "5JoQtsKQuH8hC9MyvfJAqo6qmKLm8ePYNucs7tPu2YxG12trzBt"
)

// Sample data and keys for testing.
// bob key pair is used for signing reveal tx
// internal key pair is used for tweaking
// chunkSlice splits input slice into max chunkSize length slices
func chunkSlice(slice []byte, chunkSize int) [][]byte {
	var chunks [][]byte
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize

		// necessary check to avoid slicing beyond
		// slice capacity
		if end > len(slice) {
			end = len(slice)
		}

		chunks = append(chunks, slice[i:end])
	}

	return chunks
}

// createTaprootAddress returns an address committing to a Taproot script with
// a single leaf containing the spend path with the script:
// <embedded data> OP_DROP <pubkey> OP_CHECKSIG
func createTaprootAddress(embeddedData []byte, network *chaincfg.Params, revealPrivateKeyWIF *btcutil.WIF) (string, error) {
	pubKey := revealPrivateKeyWIF.PrivKey.PubKey()

	// Step 1: Construct the Taproot script with one leaf.
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_0)
	builder.AddOp(txscript.OP_IF)
	chunks := chunkSlice(embeddedData, 520)
	for _, chunk := range chunks {
		builder.AddData(chunk)
	}
	builder.AddOp(txscript.OP_ENDIF)
	builder.AddData(schnorr.SerializePubKey(pubKey))
	builder.AddOp(txscript.OP_CHECKSIG)
	pkScript, err := builder.Script()
	if err != nil {
		return "", fmt.Errorf("error building script: %v", err)
	}

	tapLeaf := txscript.NewBaseTapLeaf(pkScript)
	tapScriptTree := txscript.AssembleTaprootScriptTree(tapLeaf)

	// Step 2: Generate the Taproot tree.
	tapScriptRootHash := tapScriptTree.RootNode.TapHash()
	outputKey := txscript.ComputeTaprootOutputKey(
		pubKey, tapScriptRootHash[:],
	)

	// Step 3: Generate the Bech32m address.
	address, err := btcutil.NewAddressTaproot(
		schnorr.SerializePubKey(outputKey), network)
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
	client              *rpcclient.Client
	network             *chaincfg.Params
	revealSatAmount     btcutil.Amount
	revealSatFee        btcutil.Amount
	revealPrivateKeyWIF *btcutil.WIF
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
	// Create a transaction that sends revealSatAmount BTC to the given address.
	address, err := btcutil.DecodeAddress(addr, r.network)
	if err != nil {
		return nil, fmt.Errorf("error decoding recipient address: %v", err)
	}

	hash, err := r.client.SendToAddress(address, btcutil.Amount(r.revealSatAmount))
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
		if out.Value == int64(r.revealSatAmount) {
			commitIndex = i
			commitOutput = out
			break
		}
	}

	pubKey := r.revealPrivateKeyWIF.PrivKey.PubKey()

	// Our script will be a simple <embedded-data> OP_DROP OP_CHECKSIG as the
	// sole leaf of a tapscript tree.
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_0)
	builder.AddOp(txscript.OP_IF)
	chunks := chunkSlice(embeddedData, 520)
	for _, chunk := range chunks {
		builder.AddData(chunk)
	}
	builder.AddOp(txscript.OP_ENDIF)
	builder.AddData(schnorr.SerializePubKey(pubKey))
	builder.AddOp(txscript.OP_CHECKSIG)
	pkScript, err := builder.Script()
	if err != nil {
		return nil, fmt.Errorf("error building script: %v", err)
	}

	tapLeaf := txscript.NewBaseTapLeaf(pkScript)
	tapScriptTree := txscript.AssembleTaprootScriptTree(tapLeaf)

	ctrlBlock := tapScriptTree.LeafMerkleProofs[0].ToControlBlock(
		pubKey,
	)

	tapScriptRootHash := tapScriptTree.RootNode.TapHash()
	outputKey := txscript.ComputeTaprootOutputKey(
		pubKey, tapScriptRootHash[:],
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
		Value:    int64(r.revealSatAmount - r.revealSatFee),
		PkScript: p2trScript,
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
		r.revealPrivateKeyWIF.PrivKey,
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
	Host                string
	User                string
	Pass                string
	HTTPPostMode        bool
	DisableTLS          bool
	Network             string
	RevealSatAmount     int64
	RevealPrivateKeyWIF string
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
	var network *chaincfg.Params
	switch config.Network {
	case "mainnet":
		network = &chaincfg.MainNetParams
	case "testnet":
		network = &chaincfg.TestNet3Params
	case "regtest":
		network = &chaincfg.RegressionNetParams
	default:
		network = &chaincfg.RegressionNetParams
	}
	revealPrivateKeyWIF := config.RevealPrivateKeyWIF
	if revealPrivateKeyWIF == "" {
		revealPrivateKeyWIF = DEFAULT_PRIVATE_KEY
	}
	wif, err := btcutil.DecodeWIF(revealPrivateKeyWIF)
	if err != nil {
		return nil, fmt.Errorf("error decoding reveal private key: %v", err)
	}
	amount := btcutil.Amount(config.RevealSatAmount)
	if amount == 0 {
		amount = btcutil.Amount(DEFAULT_SAT_AMOUNT)
	}
	fee := btcutil.Amount(config.RevealSatAmount)
	if fee == 0 {
		fee = btcutil.Amount(DEFAULT_SAT_FEE)
	}
	return &Relayer{
		client:              client,
		network:             network,
		revealSatAmount:     amount,
		revealSatFee:        fee,
		revealPrivateKeyWIF: wif,
	}, nil
}

func (r Relayer) ReadTransaction(hash *chainhash.Hash) ([]byte, error) {
	tx, err := r.client.GetRawTransaction(hash)
	if err != nil {
		return nil, err
	}
	if len(tx.MsgTx().TxIn[0].Witness) > 1 {
		witness := tx.MsgTx().TxIn[0].Witness[1]
		pushData, err := ExtractPushData(0, witness)
		if err != nil {
			return nil, err
		}
		// skip PROTOCOL_ID
		if pushData != nil && bytes.HasPrefix(pushData, PROTOCOL_ID) {
			return pushData[4:], nil
		}
	}
	return nil, nil
}

func (r Relayer) Read(height uint64) ([][]byte, error) {
	hash, err := r.client.GetBlockHash(int64(height))
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
			pushData, err := ExtractPushData(0, witness)
			if err != nil {
				return nil, err
			}
			// skip PROTOCOL_ID
			if pushData != nil && bytes.HasPrefix(pushData, PROTOCOL_ID) {
				data = append(data, pushData[4:])
			}
		}
	}
	return data, nil
}

func (r Relayer) Write(data []byte) (*chainhash.Hash, error) {
	data = append(PROTOCOL_ID, data...)
	address, err := createTaprootAddress(data, r.network, r.revealPrivateKeyWIF)
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

func ExtractPushData(version uint16, pkScript []byte) ([]byte, error) {
	type templateMatch struct {
		expectPushData bool
		maxPushDatas   int
		opcode         byte
		extractedData  []byte
	}
	var template = [6]templateMatch{
		{opcode: txscript.OP_FALSE},
		{opcode: txscript.OP_IF},
		{expectPushData: true, maxPushDatas: 10},
		{opcode: txscript.OP_ENDIF},
		{expectPushData: true, maxPushDatas: 1},
		{opcode: txscript.OP_CHECKSIG},
	}

	var templateOffset int
	tokenizer := txscript.MakeScriptTokenizer(version, pkScript)
out:
	for tokenizer.Next() {
		// Not a rollkit script if it has more opcodes than expected in the
		// template.
		if templateOffset >= len(template) {
			return nil, nil
		}

		op := tokenizer.Opcode()
		tplEntry := &template[templateOffset]
		if tplEntry.expectPushData {
			for i := 0; i < tplEntry.maxPushDatas; i++ {
				data := tokenizer.Data()
				if data == nil {
					break out
				}
				tplEntry.extractedData = append(tplEntry.extractedData, data...)
				tokenizer.Next()
			}
		} else if op != tplEntry.opcode {
			return nil, nil
		}

		templateOffset++
	}
	// TODO: skipping err checks
	return template[2].extractedData, nil
}
