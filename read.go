package rollkitbtc

import (
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

func Read(hash *chainhash.Hash) ([]byte, error) {
	relayer, err := NewRelayer()
	if err != nil {
		return nil, err
	}
	defer relayer.Close()
	tx, err := relayer.client.GetRawTransaction(hash)
	if err != nil {
		return nil, err
	}
	return tx.MsgTx().TxIn[0].Witness[1], nil
}
