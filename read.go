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
	witness := tx.MsgTx().TxIn[0].Witness[1]
	size := int(witness[0])
	return witness[1 : size+1], nil
}
