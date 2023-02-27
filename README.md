rollkit-btc:
============


This package provides a reader / writer interface to bitcoin.


Writer:
=======

A commit transaction containing a taproot with one leaf script

    OP_0
    OP_IF
    <embedded data>
    OP_ENDIF

is used to create a new bech32m address and is sent an output.


A reveal transaction then posts the embedded data on chain and spends the
commit output.


Reader:
========

The address of the reveal transaction is implicity used as a namespace.


Clients may call listunspent on the reveal transaction address to get a list of
transactions and read the embedded data from the first witness input.
