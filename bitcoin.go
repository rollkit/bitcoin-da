package bitcoinda

import (
	"context"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/rollkit/go-da"
)

const DefaultMaxBytes = 1048576;

type BitcoinDA struct {
  relayer *Relayer
}

func NewBitcoinDA(relayer *Relayer) *BitcoinDA {
  return &BitcoinDA{
    relayer: relayer,
  }
}

// MaxBlobSize returns the max blob size
func (b *BitcoinDA) MaxBlobSize(ctx context.Context) (uint64, error) {
	return DefaultMaxBytes, nil
}

// Get returns Blob for each given ID, or an error.
func (b *BitcoinDA) Get(ctx context.Context, ids []da.ID, ns da.Namespace) ([]da.Blob, error) {
  var blobs []da.Blob
  for _, id := range ids {
  	hash, err := chainhash.NewHash(id)
  	if err != nil {
  		return nil, err
  	}
  	blob, err := b.relayer.ReadTransaction(hash)
  	if err != nil {
  		return nil, err
  	}
  	blobs = append(blobs, blob)
  }
  return blobs, nil
}


// Commit creates a Commitment for each given Blob.
func (b *BitcoinDA) Commit(ctx context.Context, daBlobs []da.Blob, ns da.Namespace) ([]da.Commitment, error) {
	// not implemented
	return nil, nil
}

// GetIDs returns IDs of all Blobs located in DA at given height.
func (b *BitcoinDA) GetIDs(ctx context.Context, height uint64, ns da.Namespace) ([]da.ID, error) {
	// not implemented
	return nil, nil
}

// GetProofs returns the inclusion proofs for the given IDs.
func (b *BitcoinDA) GetProofs(ctx context.Context, daIDs []da.ID, ns da.Namespace) ([]da.Proof, error) {
	// not implemented
	return nil, nil
}

// Submit submits the Blobs to Data Availability layer.
func (b *BitcoinDA) Submit(ctx context.Context, daBlobs []da.Blob, gasPrice float64, ns da.Namespace) ([]da.ID, error) {
	var ids []da.ID
	for _, blob := range daBlobs {
	  hash, err := b.relayer.Write(blob)
	  if err != nil {
	    return nil, err
	  }
	  ids = append(ids, hash.CloneBytes())
	}
	return nil, nil
}

// Validate validates Commitments against the corresponding Proofs. This should be possible without retrieving the Blobs.
func (b *BitcoinDA) Validate(ctx context.Context, ids []da.ID, daProofs []da.Proof, ns da.Namespace) ([]bool, error) {
	// not implemented
	return nil, nil
}

var _ da.DA = &BitcoinDA{}
