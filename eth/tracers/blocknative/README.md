# Blocknative Tracer

## Options

This tracer has some options that can be passed in to extend the functionality of the tracer, and what it returns. These are defined in the `TracerOpts`.

1. Logs (boolean)

If this is set to 'true', then the logs for the transaction are saved and also returned.

Here is an example of the returned extra segment:

"logs": [
          {
            "address": "0x304a554a310c7e546dfe434669c62820b7d83490",
            "data": "0x00000000000000000000000000000000000000000001819451f999d617dafa93",
            "topics": [
              "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
              "0x000000000000000000000000c0ee9db1a9e07ca63e4ff0d5fb6f86bf68d47b89",
              "0x0000000000000000000000004fd27b205895e698fa350f7ea57cec8a21927fcd"
            ]
          },
          ...
]

2. NetBalChanges (boolean)

This also requires a NBCMethod (string) to be set too, to define the style of how the net balance changes are collated. The net balance changes currently only return ETH, ERC20 and ERC721 token transfers.

Due to the nature of the tracing ecosysem in Geth, there are some limitations put on what information of the contracts can be extracted out of the tracer accesible EVM. So we currently cannot retrieve a full token metadata concept from certain methods

The NBCMethod options are:

2.1. "events"

Here we extract the changes via the events which are triggered during execution. (https://goethereumbook.org/en/event-read-erc20/). These are produced out of ever transfer event being called in execution. However even with this being a part of the standard, this is not a requirement, and can be ommitted from a token contract. So getting NBC from this method holds this risk of being spoofed.

This is the method used by etherscan under their commonly read under the "Transaction Action" section.

2.2. "internalTransactions"

Here we get changes via decoding the contract calls and seeing if we are able to decode them as transfer calls we know about. This method also has issues where the calls may be made but not completed, or there might be different execution logic within the transfer methods being called (possibly a tax or swapped arguments ordering). This method is what historically BN used for our netBalChanges sections.

The return style for both the "events" and "internalTransactions" options for NBC are as follows:

"netBalChanges": {
      "balances": {
        "0xd1220a0cf47c7b9be7a2e6ba89f429762e7b9adb": {
          "eth": "-0.00254265",
          "ethinwei": -2542650000000000
        },
        "0xe2fe6b13287f28e193333fdfe7fedf2f6df6124a": {
          "eth": "0.00254265",
          "ethinwei": 2542650000000000
        }
      },
      "tokenchanges": [
        {
          "from": "0xd1220a0cf47c7b9be7a2e6ba89f429762e7b9adb",
          "to": "0xdbf03b407c01e7cd3cbea99509d93f8dddc8c6fb",
          "asset": 10000000,
          "contractAddress": "0xf4eced2f682ce333f96f2d8966c613ded8fc95dd"
        }
      ]
    }

2.3. "storageSlots"

Here we use a pre-configured database of known storage slot locations for token address balances and meta data. These storage slots can differ widely from different implementations, however are somewhat easily deducable from live chain calls. Since the Geth tracing environment does not allow for access to the live chain for these storage slots, we use a outside grown database for these locations so we can deduce the name, tokenname, decimals, addressbalance locations needed for a "full" NBC. This method is mostly unused and not ready for production usage.

(todo: finish this style of nbc)