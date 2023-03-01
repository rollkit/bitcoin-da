# Bitcoin DA

Overview on how to use Bitcoin for data availability.

## Goal

Define a common read and write path that can be used by rollup development kits such as rollkit or op-stack that wish to use Bitcoin for DA.

## Background

- [Ordinals docs](https://docs.ordinals.com/introduction.html)
- [Why a Bitcoiner Loves Rollups | Eric Wall](https://www.youtube.com/watch?v=_hLvvZGST_E&t=924s)

## How?

Ordinals developer Casey Rodarmor found a way to essentially create the equivalent of “calldata” on bitcoin script thanks to the Taproot upgrade. Additionally, this “calldata” can be as large as the bitcoin block size limit (4MB), benefits from the SegWit discount, making “blobspace” cheaper and more abundant than on Ethereum as of February 20, 2023. 

For the purpose of writing data to Bitcoin in order to use it as a DA layer, we will be [inscribing](https://docs.ordinals.com/inscriptions.html) our block data.

## Write Path

In order to inscribe our data into a satoshi, we need to use Taproot scripts, since our data is stored in taproot script-path spend scripts. Taproot script spends can only be made from existing taproot outputs, thus inscriptions are made with a two-phase commit/reveal scheme:

1. First, in the commit transaction, a taproot output committing to a script containing the inscription content is created.
2. Second, in the reveal transaction, the output created by the commit transaction is spent, revealing the inscription content on-chain.

But what about the actual data: 

“Inscription content is serialized using data pushes within unexecuted conditionals, called an "envelopes". Envelopes consist of an `OP_FALSE OP_IF … OP_ENDIF`
 wrapping any number of data pushes. Because envelopes are effectively no-ops, they do not change the semantics of the script in which they are included, and can be combined with any other locking script.”

In the case of a rollup posting data into bitcoin, we could serialize the data as follows:

```nasm
OP_FALSE
OP_IF
	OP_PUSH "block"
	OP_1
	OP_PUSH $ROLLUP_BLOCK_HEIGHT
	OP_0
	OP_PUSH $BLOCK_DATA_PORTION
	...
OP_ENDIF
```

The above would create an inscription that first pushes `block`, in order to disambiguate rollup block inscriptions from other uses of envelopes.  `OP_1` indicates that the next push contains the rollup height for which the data belongs too, and OP_0 indicates that any subsequent data push contains the data itself, since by Taproot restrictions, individual pushes may not be larger than 520 bytes, thus making it necessary to make multiple pushes for block data larger than 520 bytes.

The inscription content is contained within the input of a reveal transaction, and the inscription is made on the first sat of its first output.

## Read Path

Since every rollup using Bitcoin for DA will require a private key with some bitcoin to spend and create inscriptions with, we can always query all inscriptions by listing the rollup’s bitcoin address’ unspent outputs by using `[listunspent](https://developer.bitcoin.org/reference/rpc/listunspent.html)`. This will provide us with a list of utxos:

```nasm
[                                (json array)
  {                              (json object)
    "txid" : "hex",              (string) the transaction id
    "vout" : n,                  (numeric) the vout value
    "address" : "str",           (string) the bitcoin address
    "label" : "str",             (string) The associated label, or "" for the default label
    "scriptPubKey" : "str",      (string) the script key
    "amount" : n,                (numeric) the transaction output amount in BTC
    "confirmations" : n,         (numeric) The number of confirmations
    "redeemScript" : "hex",      (string) The redeemScript if scriptPubKey is P2SH
    "witnessScript" : "str",     (string) witnessScript if the scriptPubKey is P2WSH or P2SH-P2WSH
    "spendable" : true|false,    (boolean) Whether we have the private keys to spend this output
    "solvable" : true|false,     (boolean) Whether we know how to spend this output, ignoring the lack of keys
    "reused" : true|false,       (boolean) (only present if avoid_reuse is set) Whether this output is reused/dirty (sent to an address that was previously spent from)
    "desc" : "str",              (string) (only when solvable) A descriptor for spending this output
    "safe" : true|false          (boolean) Whether this output is considered safe to spend. Unconfirmed transactions
                                 from outside keys and unconfirmed replacement transactions are considered unsafe
                                 and are not eligible for spending by fundrawtransaction and sendtoaddress.
  },
  ...
]
```

We can then take this list of UTXOs and filter it to include only utxos that correspond to our reveal transactions, and additionally, we can order them by timestamp, thus giving us a list of utxo inscriptions for which we can associate an index to a certain block height in our rollup (i.e `utxo[0].witness == rollup_block_height_1_data`)

Once we have found the utxo with the data for a given block, we can call `gettransaction` with the `tx_id` of our utxo, which will return:

```nasm
{                                          (json object)
  "amount" : n,                            (numeric) The amount in BTC
  "fee" : n,                               (numeric) The amount of the fee in BTC. This is negative and only available for the
                                           'send' category of transactions.
  "confirmations" : n,                     (numeric) The number of confirmations for the transaction. Negative confirmations means the
                                           transaction conflicted that many blocks ago.
  "generated" : true|false,                (boolean) Only present if transaction only input is a coinbase one.
  "trusted" : true|false,                  (boolean) Only present if we consider transaction to be trusted and so safe to spend from.
  "blockhash" : "hex",                     (string) The block hash containing the transaction.
  "blockheight" : n,                       (numeric) The block height containing the transaction.
  "blockindex" : n,                        (numeric) The index of the transaction in the block that includes it.
  "blocktime" : xxx,                       (numeric) The block time expressed in UNIX epoch time.
  "txid" : "hex",                          (string) The transaction id.
  "walletconflicts" : [                    (json array) Conflicting transaction ids.
    "hex",                                 (string) The transaction id.
    ...
  ],
  "time" : xxx,                            (numeric) The transaction time expressed in UNIX epoch time.
  "timereceived" : xxx,                    (numeric) The time received expressed in UNIX epoch time.
  "comment" : "str",                       (string) If a comment is associated with the transaction, only present if not empty.
  "bip125-replaceable" : "str",            (string) ("yes|no|unknown") Whether this transaction could be replaced due to BIP125 (replace-by-fee);
                                           may be unknown for unconfirmed transactions not in the mempool
  "details" : [                            (json array)
    {                                      (json object)
      "involvesWatchonly" : true|false,    (boolean) Only returns true if imported addresses were involved in transaction.
      "address" : "str",                   (string) The bitcoin address involved in the transaction.
      "category" : "str",                  (string) The transaction category.
                                           "send"                  Transactions sent.
                                           "receive"               Non-coinbase transactions received.
                                           "generate"              Coinbase transactions received with more than 100 confirmations.
                                           "immature"              Coinbase transactions received with 100 or fewer confirmations.
                                           "orphan"                Orphaned coinbase transactions received.
      "amount" : n,                        (numeric) The amount in BTC
      "label" : "str",                     (string) A comment for the address/transaction, if any
      "vout" : n,                          (numeric) the vout value
      "fee" : n,                           (numeric) The amount of the fee in BTC. This is negative and only available for the
                                           'send' category of transactions.
      "abandoned" : true|false             (boolean) 'true' if the transaction has been abandoned (inputs are respendable). Only available for the
                                           'send' category of transactions.
    },
    ...
  ],
  "hex" : "hex",                           (string) Raw data for transaction
  "decoded" : {                            (json object) Optional, the decoded transaction (only present when `verbose` is passed)
    ...                                    Equivalent to the RPC decoderawtransaction method, or the RPC getrawtransaction method when `verbose` is passed.
  }
}
```

Finally, in the `decoded` field, we can get the witness data by reading the `witness` field for the first input of the decoded transaction. We can then parse our witness data and read our block data accordingly.